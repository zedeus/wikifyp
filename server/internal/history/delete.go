package history

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PageRef identifies an article for deletion.
type PageRef struct {
	Source string `json:"source"`
	PageID int64  `json:"page_id"`
}

// DeleteResult reports what was removed.
type DeleteResult struct {
	PagesDeleted  int `json:"pages_deleted"`
	EventsDeleted int `json:"events_deleted"`
}

// DeletePages removes the specified articles from a reader's history:
// reader_article_state, reading_events, and reader_impressions. The feed
// and profile recompute on demand from remaining state, so they behave
// as if the articles were never seen. This is the one sanctioned delete
// path (see decisions.md, 2026-10-31).
func DeletePages(ctx context.Context, db *sql.DB, readerID string, pages []PageRef) (*DeleteResult, error) {
	if len(pages) == 0 {
		return &DeleteResult{}, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("history: delete: begin: %w", err)
	}
	defer tx.Rollback()

	result := &DeleteResult{}

	// Resolve article IDs for event deletion: events reference article_id
	// (a revision row), not page_id, so we need all article rows for these
	// pages.
	articleIDs, err := resolveArticleIDs(ctx, tx, pages)
	if err != nil {
		return nil, err
	}

	// Delete reading events.
	if len(articleIDs) > 0 {
		placeholders := make([]string, len(articleIDs))
		args := []any{readerID}
		for i, id := range articleIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+2)
			args = append(args, id)
		}
		var count int64
		res, err := tx.ExecContext(ctx, fmt.Sprintf(
			"DELETE FROM reading_events WHERE reader_id = $1 AND article_id IN (%s)",
			strings.Join(placeholders, ",")), args...)
		if err != nil {
			return nil, fmt.Errorf("history: delete events: %w", err)
		}
		count, _ = res.RowsAffected()
		result.EventsDeleted = int(count)
	}

	// Delete reader_impressions.
	for _, p := range pages {
		_, err := tx.ExecContext(ctx,
			"DELETE FROM reader_impressions WHERE reader_id = $1 AND source = $2 AND page_id = $3",
			readerID, p.Source, p.PageID)
		if err != nil {
			return nil, fmt.Errorf("history: delete impressions: %w", err)
		}
	}

	// Delete reader_article_state.
	for _, p := range pages {
		res, err := tx.ExecContext(ctx,
			"DELETE FROM reader_article_state WHERE reader_id = $1 AND source = $2 AND page_id = $3",
			readerID, p.Source, p.PageID)
		if err != nil {
			return nil, fmt.Errorf("history: delete state: %w", err)
		}
		n, _ := res.RowsAffected()
		result.PagesDeleted += int(n)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("history: delete: commit: %w", err)
	}
	return result, nil
}

// DeleteAll removes all of a reader's history. Same cascade as DeletePages
// but unbounded.
func DeleteAll(ctx context.Context, db *sql.DB, readerID string) (*DeleteResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("history: delete all: begin: %w", err)
	}
	defer tx.Rollback()

	result := &DeleteResult{}

	res, err := tx.ExecContext(ctx,
		"DELETE FROM reading_events WHERE reader_id = $1", readerID)
	if err != nil {
		return nil, fmt.Errorf("history: delete all events: %w", err)
	}
	n, _ := res.RowsAffected()
	result.EventsDeleted = int(n)

	_, err = tx.ExecContext(ctx,
		"DELETE FROM reader_impressions WHERE reader_id = $1", readerID)
	if err != nil {
		return nil, fmt.Errorf("history: delete all impressions: %w", err)
	}

	res, err = tx.ExecContext(ctx,
		"DELETE FROM reader_article_state WHERE reader_id = $1", readerID)
	if err != nil {
		return nil, fmt.Errorf("history: delete all state: %w", err)
	}
	n, _ = res.RowsAffected()
	result.PagesDeleted = int(n)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("history: delete all: commit: %w", err)
	}
	return result, nil
}

// resolveArticleIDs finds all article row IDs for the given pages (across
// all revisions and ingest sequences).
func resolveArticleIDs(ctx context.Context, tx *sql.Tx, pages []PageRef) ([]int64, error) {
	if len(pages) == 0 {
		return nil, nil
	}
	// Build a VALUES list for the pages.
	var vals []string
	var args []any
	for i, p := range pages {
		vals = append(vals, fmt.Sprintf("($%d::text, $%d::bigint)", i*2+1, i*2+2))
		args = append(args, p.Source, p.PageID)
	}
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`
		SELECT a.id FROM articles a
		JOIN (VALUES %s) AS v(source, page_id)
		  ON a.source = v.source AND a.page_id = v.page_id`,
		strings.Join(vals, ",")), args...)
	if err != nil {
		return nil, fmt.Errorf("history: resolve articles: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("history: resolve articles scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
