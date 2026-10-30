// Package history provides reading history, stats, interests and deletion
// for a reader's data. The history is the reader's acted-on articles
// (reader_article_state) joined to their metadata; stats and interests are
// derived from reading_events and the feed's scoring model.
package history

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Entry is one article in a reader's history, ordered by last interaction.
type Entry struct {
	ArticleID   int64    `json:"id"`
	PageID      int64    `json:"page_id"`
	Source      string   `json:"source"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	ImageWidth  int      `json:"image_width,omitempty"`
	ImageHeight int      `json:"image_height,omitempty"`
	State       string   `json:"state,omitempty"` // "finished", "skipped", or "" (saved-only)
	Saved       bool     `json:"saved,omitempty"`
	Completion  float64  `json:"completion"`
	Words       int      `json:"words"`
	UpdatedAt   string   `json:"updated_at"` // RFC3339
	ResumeToken *Resume  `json:"resume_token,omitempty"`
	Categories  []string `json:"categories,omitempty"`
}

// Resume is the token address where the reader left off.
type Resume struct {
	ArticleID   int64 `json:"article_id"`
	SentenceIdx int   `json:"sentence_idx"`
	TokenIdx    int   `json:"token_idx"`
}

// Page is a paginated history result.
type Page struct {
	Entries    []Entry `json:"entries"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// List returns a page of the reader's history, newest first. Pagination
// uses keyset on (updated_at DESC, page_id). search filters by title
// (case-insensitive substring). afterDate seeks to the first entry at or
// before the given date (RFC3339 or YYYY-MM-DD).
func List(ctx context.Context, db *sql.DB, readerID string, limit int, cursor, search, afterDate string) (*Page, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var args []any
	args = append(args, readerID)
	argN := 1

	// Build WHERE clauses.
	var where []string
	where = append(where, fmt.Sprintf("s.reader_id = $%d", argN))

	// Cursor: "timestamp|page_id" — keyset pagination.
	if cursor != "" {
		parts := strings.SplitN(cursor, "|", 2)
		if len(parts) == 2 {
			argN++
			args = append(args, parts[0])
			cursorTS := argN
			argN++
			args = append(args, parts[1])
			cursorPID := argN
			where = append(where, fmt.Sprintf(
				"(s.updated_at, s.page_id) < ($%d::timestamptz, $%d::bigint)",
				cursorTS, cursorPID))
		}
	}

	// Date seek: jump to entries at or before a date.
	if afterDate != "" && cursor == "" {
		argN++
		args = append(args, afterDate)
		where = append(where, fmt.Sprintf("s.updated_at <= $%d::timestamptz + interval '1 day'", argN))
	}

	// Search: case-insensitive title substring.
	if search != "" {
		argN++
		args = append(args, "%"+search+"%")
		where = append(where, fmt.Sprintf("a.title ILIKE $%d", argN))
	}

	// Only rows with a state or saved flag (not bare completion-only rows).
	where = append(where, "(s.state IS NOT NULL OR s.saved)")

	argN++
	args = append(args, limit+1) // fetch one extra to detect next page

	q := fmt.Sprintf(`
		SELECT a.id, s.page_id, s.source, a.title,
		       COALESCE(a.description, ''), COALESCE(a.image_url, ''),
		       COALESCE(a.image_width, 0), COALESCE(a.image_height, 0),
		       s.state, s.saved, s.completion,
		       COALESCE((SELECT SUM(array_length(string_to_array(t.text, ' '), 1))
		                 FROM sentences t WHERE t.article_id = a.id AND t.heading_level = 0), 0),
		       s.updated_at
		FROM reader_article_state s
		JOIN latest_articles a ON a.source = s.source AND a.page_id = s.page_id
		WHERE %s
		ORDER BY s.updated_at DESC, s.page_id DESC
		LIMIT $%d`,
		strings.Join(where, " AND "), argN)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("history: list: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var state sql.NullString
		var updatedAt time.Time
		if err := rows.Scan(&e.ArticleID, &e.PageID, &e.Source, &e.Title,
			&e.Description, &e.ImageURL, &e.ImageWidth, &e.ImageHeight,
			&state, &e.Saved, &e.Completion, &e.Words, &updatedAt); err != nil {
			return nil, fmt.Errorf("history: list scan: %w", err)
		}
		if state.Valid {
			e.State = state.String
		}
		e.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history: list rows: %w", err)
	}

	page := &Page{Entries: entries}
	if len(entries) > limit {
		page.Entries = entries[:limit]
		last := page.Entries[limit-1]
		// Parse the UpdatedAt back to get the cursor timestamp.
		page.NextCursor = last.UpdatedAt + "|" + fmt.Sprintf("%d", last.PageID)
	}
	return page, nil
}

// ResumePosition returns the latest token address a reader reached in an
// article, derived from reading_events. Returns nil if no events exist.
func ResumePosition(ctx context.Context, db *sql.DB, readerID string, articleID int64) (*Resume, error) {
	var r Resume
	err := db.QueryRowContext(ctx, `
		SELECT article_id, sentence_idx, token_idx
		FROM reading_events
		WHERE reader_id = $1 AND article_id = $2
		  AND type IN ('play', 'pause')
		ORDER BY sentence_idx DESC, token_idx DESC
		LIMIT 1`,
		readerID, articleID).Scan(&r.ArticleID, &r.SentenceIdx, &r.TokenIdx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("history: resume: %w", err)
	}
	return &r, nil
}

// FillResume enriches history entries with resume positions for unfinished
// articles. Only the first pageSize entries are checked to keep the query
// bounded.
func FillResume(ctx context.Context, db *sql.DB, readerID string, entries []Entry) error {
	for i := range entries {
		if entries[i].State == "finished" || entries[i].Completion <= 0 {
			continue
		}
		r, err := ResumePosition(ctx, db, readerID, entries[i].ArticleID)
		if err != nil {
			return err
		}
		entries[i].ResumeToken = r
	}
	return nil
}
