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
)

// Result reports what one ingest did.
type Result struct {
	ArticleID int64
	Created   bool // false: this revision was already stored and nothing changed
	Sentences int  // sentences stored (0 when Created is false)
}

// sentenceRow is one row of the sentences table.
type sentenceRow struct {
	idx     int
	heading int // 0 = prose, 2-6 = section heading level
	text    string
}

// Article stores one fetched article. The caller fetches; this validates,
// cleans, splits and writes.
func Article(ctx context.Context, db *sql.DB, a *source.Article) (Result, error) {
	if err := a.Validate(); err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	rows := sentenceRows(source.Clean(a.Text))
	if len(rows) == 0 {
		return Result{}, fmt.Errorf("ingest: article %q: no sentences after cleaning", a.Title)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	defer tx.Rollback()

	var pageID *int64
	if a.PageID > 0 {
		pageID = &a.PageID
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO articles (source, title, url, page_id, revision_id, license, attribution, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (source, revision_id) DO NOTHING
		RETURNING id`,
		a.Source, a.Title, a.URL, pageID, a.RevisionID, a.License, a.Attribution, a.Fetched,
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

	// One prepared insert per row is plenty for the ingest command's pace;
	// it is also the known bottleneck should bulk ingestion (a dump import)
	// ever land — that path would switch these loops to COPY or unnest.
	insertSentence, err := tx.PrepareContext(ctx,
		"INSERT INTO sentences (article_id, sentence_idx, heading_level, text) VALUES ($1, $2, $3, $4)")
	if err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	defer insertSentence.Close()
	for _, r := range rows {
		if _, err := insertSentence.ExecContext(ctx, id, r.idx, r.heading, r.text); err != nil {
			return Result{}, fmt.Errorf("ingest: insert sentence %d: %w", r.idx, err)
		}
	}

	// The API can hand back the same target twice across continuation
	// pages; the conflict clauses keep these inserts idempotent within one
	// article.
	if err := insertEach(ctx, tx, "link",
		"INSERT INTO article_links (article_id, target) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		id, a.Links); err != nil {
		return Result{}, err
	}
	if err := insertEach(ctx, tx, "category",
		"INSERT INTO article_categories (article_id, category) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		id, a.Categories); err != nil {
		return Result{}, err
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}
	return Result{ArticleID: id, Created: true, Sentences: len(rows)}, nil
}

// insertEach runs one prepared two-column insert per value.
func insertEach(ctx context.Context, tx *sql.Tx, what, query string, id int64, values []string) error {
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	defer stmt.Close()
	for _, v := range values {
		if _, err := stmt.ExecContext(ctx, id, v); err != nil {
			return fmt.Errorf("ingest: insert %s %q: %w", what, v, err)
		}
	}
	return nil
}

// sentenceRows turns cleaned article text into sentence rows: headings become
// single sentences carrying their level, prose lines go through the sentence
// splitter. Indices are assigned in order and, like the splitter's, are
// stable: the same text always yields the same rows.
func sentenceRows(cleaned string) []sentenceRow {
	var rows []sentenceRow
	for line := range strings.Lines(cleaned) {
		line = strings.TrimRight(line, "\n")
		if level, title, ok := source.ParseHeading(line); ok {
			// A heading is one sentence whatever punctuation it holds.
			rows = append(rows, sentenceRow{
				idx:     len(rows),
				heading: level,
				text:    text.Normalize(title),
			})
			continue
		}
		for _, s := range text.Split(line) {
			rows = append(rows, sentenceRow{idx: len(rows), text: s.Text})
		}
	}
	return rows
}
