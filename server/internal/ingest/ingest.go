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
	if err := a.Validate(); err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	rows := SentenceRows(source.Clean(a.Text))
	if len(rows) == 0 {
		// For example a page that is only an infobox; a dump import logs
		// and skips these.
		return Result{}, fmt.Errorf("ingest: article %q: no sentences after cleaning", a.Title)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO articles (source, title, url, page_id, revision_id, license, attribution, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (source, revision_id) DO NOTHING
		RETURNING id`,
		a.Source, a.Title, a.URL, a.PageID, a.RevisionID, a.License, a.Attribution, a.Fetched,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// This revision is already stored; re-ingesting changes nothing.
		err = tx.QueryRowContext(ctx,
			"SELECT id FROM articles WHERE source = $1 AND revision_id = $2",
			a.Source, a.RevisionID).Scan(&id)
		if err != nil {
			return Result{}, fmt.Errorf("ingest: find existing revision: %w", err)
		}
		return Result{ArticleID: id, Created: false}, tx.Commit()
	}
	if err != nil {
		return Result{}, fmt.Errorf("ingest: insert article: %w", err)
	}

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
		return Result{}, fmt.Errorf("ingest: insert sentences: %w", err)
	}

	// The API can hand back the same target twice across continuation
	// pages; the conflict clauses keep these inserts idempotent within one
	// article.
	if err := insertValues(ctx, tx, "links",
		"INSERT INTO article_links (article_id, target) SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING",
		id, a.Links); err != nil {
		return Result{}, err
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
			return Result{}, fmt.Errorf("ingest: insert categories: %w", err)
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
		return Result{}, fmt.Errorf("ingest: insert article domain: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	return Result{ArticleID: id, Created: true, Sentences: len(rows)}, nil
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
