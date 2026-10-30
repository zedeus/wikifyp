// Package ingest stores fetched articles as sentences with stable addresses.
//
// An article goes through one pipeline whatever its source: validate, clean
// the extract text, split it into sentences, store everything in one
// transaction. Ingest is idempotent per revision: storing the same revision
// again changes nothing, and a newer revision of the same page becomes a new
// article row, so sentence addresses already stored are never renumbered.
package ingest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/text"
	"github.com/zedeus/wikifyp/server/internal/wiki"
)

// Result reports what one ingest did.
type Result struct {
	ArticleID int64
	Created   bool // false: this revision was already stored and nothing changed
	Sentences int  // sentences stored (0 when Created is false)
}

// SentenceRow is one row of the sentences table.
type SentenceRow struct {
	Idx     int
	Heading int // 0 = prose, 2-6 = section heading level
	Text    string
}

// Article stores one fetched article. The caller fetches; this validates,
// cleans, splits and writes.
func Article(ctx context.Context, db *sql.DB, a *source.Article) (Result, error) {
	rows, tx, err := prepare(ctx, db, a)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	return article(ctx, tx, a, rows)
}

// article is Article minus prepare: insert at ingest_seq 0, or treat the
// conflict as "this revision is already stored".
func article(ctx context.Context, tx *sql.Tx, a *source.Article, rows []SentenceRow) (Result, error) {
	id, err := insertArticle(ctx, tx, a, 0)
	if errors.Is(err, sql.ErrNoRows) {
		// This revision is already stored (a fresh insert is ingest_seq 0,
		// which every stored revision has); re-ingesting changes nothing for
		// sentences, links or categories — addressed text is immutable, and
		// storing the current pipeline's output for an already-stored
		// revision is Reingest's job, asked for explicitly.
		return answerExisting(ctx, tx, a)
	}
	if err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, id, a, rows)
}

// answerExisting is every "this revision is already stored" ending: refresh
// the presentation metadata on the row that answers for the revision, commit,
// and name that row with Created false.
func answerExisting(ctx context.Context, tx *sql.Tx, a *source.Article) (Result, error) {
	id, _, err := latestRevision(ctx, tx, a)
	if errors.Is(err, sql.ErrNoRows) {
		// Unreachable short of concurrent row deletion: a conflict just
		// proved the revision stored.
		return Result{}, fmt.Errorf("ingest: article %q vanished during ingest: %w", a.Title, err)
	}
	if err != nil {
		return Result{}, err
	}
	if err := refreshMetadata(ctx, tx, id, a); err != nil {
		return Result{}, err
	}
	return Result{ArticleID: id, Created: false}, tx.Commit()
}

// Reingest stores the current pipeline's output for a revision that is
// already stored, as a new article row with the next ingest_seq. The rows
// already stored — and every address pointing at them — are untouched;
// latest_articles simply prefers the new row. When the pipeline's output
// matches the latest stored row this is a no-op, so running it over
// everything is safe. A revision not yet stored is ingested normally.
func Reingest(ctx context.Context, db *sql.DB, a *source.Article) (Result, error) {
	rows, tx, err := prepare(ctx, db, a)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()

	latest, seq, err := latestRevision(ctx, tx, a)
	if errors.Is(err, sql.ErrNoRows) {
		return article(ctx, tx, a, rows)
	}
	if err != nil {
		return Result{}, err
	}
	same, err := sameRows(ctx, tx, latest, rows)
	if err != nil {
		return Result{}, err
	}
	if same {
		return answerExisting(ctx, tx, a)
	}

	if seq == math.MaxInt16 {
		// A wrapped seq would sort as the oldest row and resurrect stale
		// text; reaching this means something is reingesting in a loop.
		return Result{}, fmt.Errorf("ingest: article %q: revision %d already superseded %d times", a.Title, a.RevisionID, seq)
	}
	id, err := insertArticle(ctx, tx, a, seq+1)
	if err == nil {
		// The superseded row may carry richer presentation metadata than
		// this fetch (an image stored by the API that a dump or fixture
		// lacks); the preferred row must not lose it, the same promise
		// refreshMetadata makes on the already-stored paths.
		if _, err := tx.ExecContext(ctx, `
			UPDATE articles SET
				image_url = COALESCE(articles.image_url, old.image_url),
				image_width = COALESCE(articles.image_width, old.image_width),
				image_height = COALESCE(articles.image_height, old.image_height),
				description = COALESCE(articles.description, old.description)
			FROM articles old
			WHERE articles.id = $1 AND old.id = $2`,
			id, latest); err != nil {
			return Result{}, fmt.Errorf("ingest: carry metadata forward: %w", err)
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		// A concurrent reingest committed this seq first. This is a
		// single-writer CLI by contract (like the MediaWiki client), so the
		// winner ran the same pipeline and its row carries the same output;
		// answer with it instead of surfacing a bare conflict.
		return answerExisting(ctx, tx, a)
	}
	if err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, id, a, rows)
}

// insertArticle inserts one article row at the given ingest_seq and returns
// its id, or sql.ErrNoRows when that (source, revision_id, ingest_seq) is
// already stored.
func insertArticle(ctx context.Context, tx *sql.Tx, a *source.Article, seq int16) (int64, error) {
	imgURL, imgW, imgH := imageCols(a)
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO articles (source, title, url, page_id, revision_id, license, attribution, fetched_at,
			image_url, image_width, image_height, description, ingest_seq)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (source, revision_id, ingest_seq) DO NOTHING
		RETURNING id`,
		a.Source, a.Title, a.URL, a.PageID, a.RevisionID, a.License, a.Attribution, a.Fetched,
		imgURL, imgW, imgH, descCol(a), seq,
	).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("ingest: insert article: %w", err)
	}
	return id, err
}

// latestRevision returns the stored row with the highest ingest_seq for the
// article's revision — the one rule for which row answers for a revision.
// sql.ErrNoRows means the revision is not stored.
func latestRevision(ctx context.Context, tx *sql.Tx, a *source.Article) (id int64, seq int16, err error) {
	err = tx.QueryRowContext(ctx, `
		SELECT id, ingest_seq FROM articles WHERE source = $1 AND revision_id = $2
		ORDER BY ingest_seq DESC LIMIT 1`,
		a.Source, a.RevisionID).Scan(&id, &seq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("ingest: find existing revision: %w", err)
	}
	return id, seq, err
}

// finish writes the article's dependent rows and commits.
func finish(ctx context.Context, tx *sql.Tx, id int64, a *source.Article, rows []SentenceRow) (Result, error) {
	if err := storeRows(ctx, tx, id, a, rows); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	return Result{ArticleID: id, Created: true, Sentences: len(rows)}, nil
}

// Rows is the pipeline's front half — validate, clean, split — the one
// definition of what stored rows an article's text produces. Everything
// that writes article sentences (ingest here, the asset exporter) goes
// through it, so the outputs cannot drift.
func Rows(a *source.Article) ([]SentenceRow, error) {
	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}
	rows := SentenceRows(source.Clean(a.Text))
	if len(rows) == 0 {
		// For example a page that is only an infobox; a dump import logs
		// and skips these.
		return nil, fmt.Errorf("ingest: article %q: no sentences after cleaning", a.Title)
	}
	return rows, nil
}

// prepare runs Rows and opens the transaction — the shared front half of
// Article and Reingest.
func prepare(ctx context.Context, db *sql.DB, a *source.Article) ([]SentenceRow, *sql.Tx, error) {
	rows, err := Rows(a)
	if err != nil {
		return nil, nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ingest: %w", err)
	}
	return rows, tx, nil
}

// sameRows reports whether the stored sentences of article id are exactly
// the given rows.
func sameRows(ctx context.Context, tx *sql.Tx, id int64, rows []SentenceRow) (bool, error) {
	stored, err := tx.QueryContext(ctx, `
		SELECT sentence_idx, heading_level, text FROM sentences
		WHERE article_id = $1 ORDER BY sentence_idx`, id)
	if err != nil {
		return false, fmt.Errorf("ingest: load stored sentences: %w", err)
	}
	defer stored.Close()
	n := 0
	for stored.Next() {
		var r SentenceRow
		if err := stored.Scan(&r.Idx, &r.Heading, &r.Text); err != nil {
			return false, fmt.Errorf("ingest: load stored sentences: %w", err)
		}
		if n >= len(rows) || rows[n] != r {
			return false, nil
		}
		n++
	}
	if err := stored.Err(); err != nil {
		return false, fmt.Errorf("ingest: load stored sentences: %w", err)
	}
	return n == len(rows), nil
}

// refreshMetadata updates presentation metadata on an already-stored row.
// Image and description are not addressed text, so a re-ingest that carries
// them may refresh the stored row (the wiki swaps lead images without a new
// text revision). Only non-empty incoming metadata is written: dump imports
// and older fixtures carry none, and re-ingesting from those must not wipe
// metadata a richer fetch stored earlier.
func refreshMetadata(ctx context.Context, tx *sql.Tx, id int64, a *source.Article) error {
	imgURL, imgW, imgH := imageCols(a)
	desc := descCol(a)
	if imgURL == nil && desc == nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE articles SET
			image_url = COALESCE($2, image_url),
			image_width = COALESCE($3, image_width),
			image_height = COALESCE($4, image_height),
			description = COALESCE($5, description)
		WHERE id = $1`,
		id, imgURL, imgW, imgH, desc); err != nil {
		return fmt.Errorf("ingest: refresh metadata: %w", err)
	}
	return nil
}

// imageCols and descCol are the nullable presentation metadata columns,
// absent for pages with no free lead image or no short description.
func imageCols(a *source.Article) (url *string, w, h *int) {
	if a.Image == nil {
		return nil, nil, nil
	}
	return &a.Image.URL, &a.Image.Width, &a.Image.Height
}

func descCol(a *source.Article) *string {
	if a.Description == "" {
		return nil
	}
	return &a.Description
}

// storeRows writes an article's sentences, links, categories and domain.
func storeRows(ctx context.Context, tx *sql.Tx, id int64, a *source.Article, rows []SentenceRow) error {
	// One statement per table, with the rows passed as arrays: a dump import
	// ingests hundreds of thousands of articles, so per-row round trips are
	// the difference between minutes and days.
	indices := make([]int32, len(rows))
	headings := make([]int16, len(rows))
	texts := make([]string, len(rows))
	for i, r := range rows {
		indices[i], headings[i], texts[i] = int32(r.Idx), int16(r.Heading), r.Text
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sentences (article_id, sentence_idx, heading_level, text)
		SELECT $1, * FROM unnest($2::integer[], $3::smallint[], $4::text[])`,
		id, indices, headings, texts); err != nil {
		return fmt.Errorf("ingest: insert sentences: %w", err)
	}

	// The API can hand back the same target twice across continuation
	// pages; the conflict clauses keep these inserts idempotent within one
	// article.
	if err := insertValues(ctx, tx, "links",
		"INSERT INTO article_links (article_id, target) SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING",
		id, a.Links); err != nil {
		return err
	}
	// Categories count for recommendations only when the wiki's own metadata
	// (category_pages, refreshed by fetch-categories) says the category is a
	// visible content category; maintenance categories are hidden or have no
	// page at all. A source with no fetched metadata keeps everything — dump
	// records carry hidden categories the API source never sends, so the
	// filter is what makes the two agree. The one exemption: the wiki flags
	// its featured markers hidden too, and those seed the zero-history feed
	// (wiki.FeaturedCategories), so they survive the filter.
	if len(a.Categories) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO article_categories (article_id, category)
			SELECT $1, c FROM unnest($2::text[]) AS c
			WHERE EXISTS (
			          SELECT 1 FROM category_pages cp
			          WHERE cp.source = $3 AND cp.category = c AND NOT cp.hidden)
			   OR c = ANY($4)
			   OR NOT EXISTS (SELECT 1 FROM category_pages WHERE source = $3)
			ON CONFLICT DO NOTHING`,
			id, a.Categories, a.Source, wiki.FeaturedCategories[a.Source]); err != nil {
			return fmt.Errorf("ingest: insert categories: %w", err)
		}
	}

	// The article's exploration domain: the modal domain of its stored
	// categories, from the wiki's topic tree (category_pages.domain, set by
	// fetch-categories). No fetched tree means no row, and exploration
	// falls back to uniform sampling.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO article_domains (article_id, domain)
		SELECT article_id, domain FROM (
			SELECT c.article_id, cp.domain
			FROM article_categories c
			JOIN category_pages cp ON cp.source = $2 AND cp.category = c.category
			WHERE c.article_id = $1 AND cp.domain IS NOT NULL
			GROUP BY c.article_id, cp.domain
			ORDER BY count(*) DESC, cp.domain
			LIMIT 1) d`,
		id, a.Source); err != nil {
		return fmt.Errorf("ingest: insert article domain: %w", err)
	}
	return nil
}

// insertValues runs one two-column insert with the values as an array.
func insertValues(ctx context.Context, tx *sql.Tx, what, query string, id int64, values []string) error {
	if len(values) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, query, id, values); err != nil {
		return fmt.Errorf("ingest: insert %s: %w", what, err)
	}
	return nil
}

// SentenceRows turns cleaned article text into sentence rows: headings become
// single sentences carrying their level, prose lines go through the sentence
// splitter. Indices are assigned in order and, like the splitter's, are
// stable: the same text always yields the same rows. Exported so tooling
// (the dev UI's pipeline view) shows exactly what ingest would store.
func SentenceRows(cleaned string) []SentenceRow {
	var rows []SentenceRow
	for line := range strings.Lines(cleaned) {
		line = strings.TrimRight(line, "\n")
		if level, title, ok := source.ParseHeading(line); ok {
			// A heading is one sentence whatever punctuation it holds.
			rows = append(rows, SentenceRow{Heading: level, Text: text.Normalize(title)})
			continue
		}
		for _, s := range text.Split(line) {
			rows = append(rows, SentenceRow{Text: s.Text})
		}
	}
	rows = dropEmptySections(rows)
	// Indices are assigned after the empty sections are gone, so the stored
	// addresses have no holes.
	for i := range rows {
		rows[i].Idx = i
	}
	return rows
}

// dropEmptySections removes headings whose section holds no prose down to
// the next heading of the same or a higher level. Sections like a gallery
// or a reference list keep their heading after their non-prose content is
// stripped; a reader has nothing to read there, so the heading goes too.
// A parent heading survives when any of its subsections has prose.
func dropEmptySections(rows []SentenceRow) []SentenceRow {
	keep := rows[:0]
	for i, r := range rows {
		if r.Heading == 0 {
			keep = append(keep, r)
			continue
		}
		empty := true
		for _, after := range rows[i+1:] {
			if after.Heading == 0 {
				empty = false
				break
			}
			if after.Heading <= r.Heading {
				break
			}
		}
		if !empty {
			keep = append(keep, r)
		}
	}
	return keep
}

// PruneSuperseded deletes article rows that nothing needs anymore: rows that
// are not the latest for their page (an older revision, or a lower
// ingest_seq) and that no stored reading event addresses. Old rows exist
// only to keep recorded token addresses resolvable, so a non-latest row no
// event references is dead weight — at dump scale, keeping every superseded
// copy of 7M articles would double storage per pipeline fix for nothing.
// Rows a reading event points at (as the event's article or its rewind
// landing site) are never touched, whatever their age. Returns how many
// article rows were removed; their sentences, links and categories cascade.
func PruneSuperseded(ctx context.Context, db *sql.DB) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ingest: prune superseded: %w", err)
	}
	defer tx.Rollback()

	// One definition of "stale", used by both deletes in this transaction:
	// not the latest (revision_id, ingest_seq) for its page, and unreferenced
	// by any event.
	const stale = `
		SELECT a.id FROM articles a
		WHERE EXISTS (
		    SELECT 1 FROM articles newer
		    WHERE newer.source = a.source
		      AND newer.page_id = a.page_id
		      AND (newer.revision_id, newer.ingest_seq) > (a.revision_id, a.ingest_seq))
		  AND NOT EXISTS (
		    SELECT 1 FROM reading_events e
		    WHERE e.article_id = a.id OR e.landed_article_id = a.id)`

	// article_domains references articles without a cascade; clear it first.
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM article_domains WHERE article_id IN ("+stale+")"); err != nil {
		return 0, fmt.Errorf("ingest: prune superseded domains: %w", err)
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM articles WHERE id IN ("+stale+")")
	if err != nil {
		return 0, fmt.Errorf("ingest: prune superseded: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ingest: prune superseded: %w", err)
	}
	return n, tx.Commit()
}

// PruneCategories drops stored article categories that one source's fresh
// category metadata (category_pages) excludes, so articles ingested under
// older metadata — or from dumps, which carry every hidden maintenance
// category — agree with what ingest would store today. The featured
// markers survive despite being flagged hidden (see wiki.FeaturedCategories): they
// are what a zero-history reader's feed is built from. Returns how many
// rows were removed.
func PruneCategories(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, source string) (int64, error) {
	res, err := db.ExecContext(ctx, `
		DELETE FROM article_categories ac
		USING articles a
		WHERE a.id = ac.article_id
		  AND a.source = $1
		  AND ac.category <> ALL($2)
		  AND NOT EXISTS (
		      SELECT 1 FROM category_pages cp
		      WHERE cp.source = a.source AND cp.category = ac.category AND NOT cp.hidden
		  )`, source, wiki.FeaturedCategories[source])
	if err != nil {
		return 0, fmt.Errorf("ingest: prune categories: %w", err)
	}
	return res.RowsAffected()
}
