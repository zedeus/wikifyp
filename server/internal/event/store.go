package event

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// InsertResult reports what happened to each event in a batch insert.
type InsertResult struct {
	ID     string
	Stored bool   // true if newly inserted, false if already existed
	Error  string // non-empty if the event was rejected (validation, range check)
}

// Store inserts valid events into reading_events, deduplicating by ID
// (ON CONFLICT DO NOTHING). It returns one InsertResult per input event
// in the same order as validEvents.
func Store(ctx context.Context, db *sql.DB, readerID string, events []Event) ([]InsertResult, error) {
	if len(events) == 0 {
		return nil, nil
	}

	results := make([]InsertResult, len(events))

	// Build a single INSERT … ON CONFLICT DO NOTHING for the batch.
	// This is more efficient than per-event inserts and the ON CONFLICT
	// handles duplicates atomically.
	var b strings.Builder
	b.WriteString(`INSERT INTO reading_events (
		id, reader_id, schema_version, session_id, type, timestamp,
		article_id, sentence_idx, token_idx, wpm,
		landed_article_id, landed_sentence_idx, landed_token_idx,
		distance, dwell_ms
	) VALUES `)

	args := make([]any, 0, len(events)*15)
	first := true
	for i, e := range events {
		results[i].ID = e.ID
		if !first {
			b.WriteString(", ")
		}
		first = false
		base := i * 15
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
			base+8, base+9, base+10, base+11, base+12, base+13, base+14, base+15)

		var landedArticleID, landedSentenceIdx, landedTokenIdx *int64
		var distance *int
		var dwellMs *int64
		if e.LandedToken != nil {
			la := int64(e.LandedToken.ArticleID)
			ls := int64(e.LandedToken.SentenceIdx)
			lt := int64(e.LandedToken.TokenIdx)
			landedArticleID = &la
			landedSentenceIdx = &ls
			landedTokenIdx = &lt
		}
		if e.Distance != nil {
			d := *e.Distance
			distance = &d
		}
		if e.DwellMs != nil {
			d := *e.DwellMs
			dwellMs = &d
		}

		args = append(args,
			e.ID, readerID, e.SchemaVersion, e.SessionID, e.Type, e.Timestamp,
			e.Token.ArticleID, e.Token.SentenceIdx, e.Token.TokenIdx, e.WPM,
			landedArticleID, landedSentenceIdx, landedTokenIdx,
			distance, dwellMs,
		)
	}
	b.WriteString(" ON CONFLICT (id, timestamp) DO NOTHING RETURNING id")

	rows, err := db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("event: store batch: %w", err)
	}
	defer rows.Close()

	inserted := make(map[string]bool, len(events))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("event: scan returned id: %w", err)
		}
		inserted[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("event: store batch: %w", err)
	}

	for i, e := range events {
		results[i].Stored = inserted[e.ID]
	}
	return results, nil
}

// ValidateTokenAddress checks that the event's token address refers to
// an existing sentence and that token_idx is within the sentence's
// token count. Returns nil if valid, an error describing the problem
// otherwise.
func ValidateTokenAddress(ctx context.Context, db *sql.DB, e Event) error {
	var text string
	err := db.QueryRowContext(ctx,
		"SELECT text FROM sentences WHERE article_id = $1 AND sentence_idx = $2",
		e.Token.ArticleID, e.Token.SentenceIdx).Scan(&text)
	if err == sql.ErrNoRows {
		return fmt.Errorf("event %s: no sentence at (%d, %d)",
			e.ID, e.Token.ArticleID, e.Token.SentenceIdx)
	}
	if err != nil {
		return fmt.Errorf("event %s: lookup sentence: %w", e.ID, err)
	}
	count := TokenCount(text)
	if e.Token.TokenIdx >= count {
		return fmt.Errorf("event %s: token_idx %d out of range (sentence has %d tokens)",
			e.ID, e.Token.TokenIdx, count)
	}

	// Also validate landed_token for scroll_back events.
	if e.LandedToken != nil {
		err := db.QueryRowContext(ctx,
			"SELECT text FROM sentences WHERE article_id = $1 AND sentence_idx = $2",
			e.LandedToken.ArticleID, e.LandedToken.SentenceIdx).Scan(&text)
		if err == sql.ErrNoRows {
			return fmt.Errorf("event %s: no sentence at landed_token (%d, %d)",
				e.ID, e.LandedToken.ArticleID, e.LandedToken.SentenceIdx)
		}
		if err != nil {
			return fmt.Errorf("event %s: lookup landed sentence: %w", e.ID, err)
		}
		count = TokenCount(text)
		if e.LandedToken.TokenIdx >= count {
			return fmt.Errorf("event %s: landed_token.token_idx %d out of range (sentence has %d tokens)",
				e.ID, e.LandedToken.TokenIdx, count)
		}
	}
	return nil
}

// ArticleMeta holds the source and page_id for resolving an article_id
// to the feed state key.
type ArticleMeta struct {
	Source string
	PageID int64
}

// LookupArticle resolves an article_id to its source and page_id for
// feeding into feed.SetState.
func LookupArticle(ctx context.Context, db *sql.DB, articleID int64) (ArticleMeta, error) {
	var m ArticleMeta
	err := db.QueryRowContext(ctx,
		"SELECT source, page_id FROM articles WHERE id = $1", articleID).
		Scan(&m.Source, &m.PageID)
	if err == sql.ErrNoRows {
		return ArticleMeta{}, fmt.Errorf("event: no article with id %d", articleID)
	}
	if err != nil {
		return ArticleMeta{}, fmt.Errorf("event: lookup article %d: %w", articleID, err)
	}
	return m, nil
}
