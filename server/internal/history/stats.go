package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Stats is a reader's aggregate reading statistics.
type Stats struct {
	TotalWordsRead    int64          `json:"total_words_read"`
	TotalArticles     int            `json:"total_articles"`
	ArticlesFinished  int            `json:"articles_finished"`
	ArticlesSkipped   int            `json:"articles_skipped"`
	TotalReadingTimeS int64          `json:"total_reading_time_s"`
	CurrentStreak     int            `json:"current_streak"`
	LongestStreak     int            `json:"longest_streak"`
	WPMOverTime       []WPMBucket    `json:"wpm_over_time"`
	TopCategories     []CategoryStat `json:"top_categories"`
}

// WPMBucket is one time bucket of average reading speed.
type WPMBucket struct {
	Week   string  `json:"week"` // ISO week start (YYYY-MM-DD)
	AvgWPM float64 `json:"avg_wpm"`
	Events int     `json:"events"`
}

// CategoryStat is reading time and article count per category.
type CategoryStat struct {
	Category string  `json:"category"`
	Articles int     `json:"articles"`
	Words    int64   `json:"words"`
	AvgCompl float64 `json:"avg_completion"`
}

// ReadingStats computes aggregate stats for a reader from their events and
// state. All queries are bounded by reader_id.
func ReadingStats(ctx context.Context, db *sql.DB, readerID string) (*Stats, error) {
	s := &Stats{}

	// Article counts from reader_article_state.
	err := db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE state IS NOT NULL OR saved),
			COUNT(*) FILTER (WHERE state = 'finished'),
			COUNT(*) FILTER (WHERE state = 'skipped')
		FROM reader_article_state
		WHERE reader_id = $1`, readerID).Scan(
		&s.TotalArticles, &s.ArticlesFinished, &s.ArticlesSkipped)
	if err != nil {
		return nil, fmt.Errorf("stats: article counts: %w", err)
	}

	// Total words read: sum of (completion × article words) for positive states.
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			s.completion * COALESCE((
				SELECT SUM(array_length(string_to_array(t.text, ' '), 1))
				FROM sentences t WHERE t.article_id = a.id AND t.heading_level = 0
			), 0)
		), 0)::bigint
		FROM reader_article_state s
		JOIN latest_articles a ON a.source = s.source AND a.page_id = s.page_id
		WHERE s.reader_id = $1 AND (s.state = 'finished' OR s.saved OR s.completion > 0)`,
		readerID).Scan(&s.TotalWordsRead)
	if err != nil {
		return nil, fmt.Errorf("stats: total words: %w", err)
	}

	// Total reading time: sum of play→pause durations (timestamps are
	// epoch milliseconds). Each pause after a play represents one segment.
	err = db.QueryRowContext(ctx, `
		WITH ordered AS (
			SELECT type, timestamp,
			       LAG(type) OVER (PARTITION BY session_id ORDER BY timestamp) AS prev_type,
			       LAG(timestamp) OVER (PARTITION BY session_id ORDER BY timestamp) AS prev_ts
			FROM reading_events
			WHERE reader_id = $1
		)
		SELECT (COALESCE(SUM(timestamp - prev_ts), 0) / 1000)::bigint
		FROM ordered
		WHERE type = 'pause' AND prev_type = 'play'`,
		readerID).Scan(&s.TotalReadingTimeS)
	if err != nil {
		return nil, fmt.Errorf("stats: reading time: %w", err)
	}

	// WPM over time: weekly averages from play events.
	wpmRows, err := db.QueryContext(ctx, `
		SELECT date_trunc('week', to_timestamp(timestamp / 1000.0))::date AS week,
		       AVG(wpm)::float8, COUNT(*)
		FROM reading_events
		WHERE reader_id = $1 AND type = 'play' AND wpm > 0
		GROUP BY week
		ORDER BY week`,
		readerID)
	if err != nil {
		return nil, fmt.Errorf("stats: wpm: %w", err)
	}
	defer wpmRows.Close()
	for wpmRows.Next() {
		var b WPMBucket
		var week time.Time
		if err := wpmRows.Scan(&week, &b.AvgWPM, &b.Events); err != nil {
			return nil, fmt.Errorf("stats: wpm scan: %w", err)
		}
		b.Week = week.Format("2006-01-02")
		s.WPMOverTime = append(s.WPMOverTime, b)
	}
	if err := wpmRows.Err(); err != nil {
		return nil, fmt.Errorf("stats: wpm rows: %w", err)
	}

	// Streaks: consecutive days with at least one play event.
	streakRows, err := db.QueryContext(ctx, `
		SELECT DISTINCT to_timestamp(timestamp / 1000.0)::date AS day
		FROM reading_events
		WHERE reader_id = $1 AND type = 'play'
		ORDER BY day`,
		readerID)
	if err != nil {
		return nil, fmt.Errorf("stats: streaks: %w", err)
	}
	defer streakRows.Close()
	var days []time.Time
	for streakRows.Next() {
		var d time.Time
		if err := streakRows.Scan(&d); err != nil {
			return nil, fmt.Errorf("stats: streaks scan: %w", err)
		}
		days = append(days, d)
	}
	if err := streakRows.Err(); err != nil {
		return nil, fmt.Errorf("stats: streaks rows: %w", err)
	}
	s.CurrentStreak, s.LongestStreak = computeStreaks(days, time.Now().UTC())

	// Top categories by article count and engagement.
	catRows, err := db.QueryContext(ctx, `
		SELECT c.category, COUNT(DISTINCT s.page_id),
		       COALESCE(SUM(
		           s.completion * COALESCE((
		               SELECT SUM(array_length(string_to_array(t.text, ' '), 1))
		               FROM sentences t WHERE t.article_id = a.id AND t.heading_level = 0
		           ), 0)
		       ), 0)::bigint,
		       AVG(s.completion)::float8
		FROM reader_article_state s
		JOIN latest_articles a ON a.source = s.source AND a.page_id = s.page_id
		JOIN article_categories c ON c.article_id = a.id
		WHERE s.reader_id = $1 AND (s.state IS NOT NULL OR s.saved)
		GROUP BY c.category
		ORDER BY COUNT(DISTINCT s.page_id) DESC
		LIMIT 20`,
		readerID)
	if err != nil {
		return nil, fmt.Errorf("stats: categories: %w", err)
	}
	defer catRows.Close()
	for catRows.Next() {
		var c CategoryStat
		if err := catRows.Scan(&c.Category, &c.Articles, &c.Words, &c.AvgCompl); err != nil {
			return nil, fmt.Errorf("stats: categories scan: %w", err)
		}
		s.TopCategories = append(s.TopCategories, c)
	}
	if err := catRows.Err(); err != nil {
		return nil, fmt.Errorf("stats: categories rows: %w", err)
	}

	return s, nil
}

// computeStreaks returns the current and longest consecutive-day streaks
// from a sorted slice of active days. today anchors the current streak:
// a streak that ran through yesterday or today is current.
func computeStreaks(days []time.Time, today time.Time) (current, longest int) {
	if len(days) == 0 {
		return 0, 0
	}
	streak := 1
	longest = 1
	todayDate := today.Truncate(24 * time.Hour)
	lastDay := days[len(days)-1].Truncate(24 * time.Hour)

	for i := 1; i < len(days); i++ {
		prev := days[i-1].Truncate(24 * time.Hour)
		cur := days[i].Truncate(24 * time.Hour)
		if cur.Sub(prev) == 24*time.Hour {
			streak++
		} else {
			streak = 1
		}
		if streak > longest {
			longest = streak
		}
	}
	// Current streak: must include today or yesterday.
	diff := todayDate.Sub(lastDay)
	if diff <= 24*time.Hour {
		current = streak
	}
	return current, longest
}
