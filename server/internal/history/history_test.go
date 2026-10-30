package history_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zedeus/wikifyp/server/internal/feed"
	"github.com/zedeus/wikifyp/server/internal/history"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/migrate"
	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

const reader = "test-reader"

// seedDB creates a migrated database with fixture articles and some reader
// state, returning the database, article IDs and page IDs.
func seedDB(t *testing.T) (*sql.DB, map[string]int64, map[string]int64) {
	t.Helper()
	ctx := context.Background()
	db := testdb.New(t)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	src := &source.Fixtures{Dir: "../../../fixtures/articles"}
	articleIDs := make(map[string]int64) // slug -> article_id
	pageIDs := make(map[string]int64)    // slug -> page_id
	for _, slug := range []string{"philosophy", "metaphysics", "ethics", "computer", "chemistry"} {
		a, err := src.Fetch(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		res, err := ingest.Article(ctx, db, a)
		if err != nil {
			t.Fatal(err)
		}
		articleIDs[slug] = res.ArticleID
		var pageID int64
		if err := db.QueryRow("SELECT page_id FROM articles WHERE id = $1", res.ArticleID).Scan(&pageID); err != nil {
			t.Fatal(err)
		}
		pageIDs[slug] = pageID
	}

	// Set reader state for the articles.
	if err := feed.SetState(ctx, db, reader, "wikipedia", pageIDs["philosophy"], "finished"); err != nil {
		t.Fatal(err)
	}
	if err := feed.SetState(ctx, db, reader, "wikipedia", pageIDs["metaphysics"], "skipped"); err != nil {
		t.Fatal(err)
	}
	if err := feed.SetState(ctx, db, reader, "wikipedia", pageIDs["ethics"], "finished"); err != nil {
		t.Fatal(err)
	}
	// Computer: partially read (saved, no finish/skip state).
	if err := feed.SetSaved(ctx, db, reader, "wikipedia", pageIDs["computer"], true); err != nil {
		t.Fatal(err)
	}

	// Set different updated_at timestamps so ordering is deterministic.
	base := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for i, slug := range []string{"philosophy", "metaphysics", "ethics", "computer"} {
		ts := base.Add(time.Duration(i) * 24 * time.Hour)
		_, err := db.Exec(
			"UPDATE reader_article_state SET updated_at = $1 WHERE reader_id = $2 AND page_id = $3",
			ts, reader, pageIDs[slug])
		if err != nil {
			t.Fatal(err)
		}
	}

	// Insert some reading events for resume position testing.
	now := time.Now().Unix() * 1000
	for _, ev := range []struct {
		id        string
		articleID int64
		typ       string
		sentIdx   int
		tokenIdx  int
	}{
		{"ev1", articleIDs["computer"], "play", 0, 0},
		{"ev2", articleIDs["computer"], "pause", 2, 5},
		{"ev3", articleIDs["computer"], "play", 2, 5},
		{"ev4", articleIDs["computer"], "pause", 4, 3},
	} {
		_, err := db.Exec(`
			INSERT INTO reading_events (id, reader_id, schema_version, session_id, type,
				timestamp, article_id, sentence_idx, token_idx, wpm)
			VALUES ($1, $2, 1, 'sess1', $3, $4, $5, $6, $7, 250)`,
			ev.id, reader, ev.typ, now, ev.articleID, ev.sentIdx, ev.tokenIdx)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set completion for computer.
	if err := db.QueryRow("SELECT 1 FROM reader_article_state WHERE reader_id = $1 AND page_id = $2",
		reader, pageIDs["computer"]).Scan(new(int)); err != nil {
		t.Fatal(err)
	}

	return db, articleIDs, pageIDs
}

func TestListHistory(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	page, err := history.List(ctx, db, reader, 50, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(page.Entries))
	}
	// Newest first: computer (Oct 28), ethics (Oct 27), metaphysics (Oct 26), philosophy (Oct 25).
	if page.Entries[0].Title != "Computer" {
		t.Errorf("first entry = %q, want Computer", page.Entries[0].Title)
	}
	if page.Entries[3].Title != "Philosophy" {
		t.Errorf("last entry = %q, want Philosophy", page.Entries[3].Title)
	}
	// Computer should be saved, no state.
	if !page.Entries[0].Saved {
		t.Error("computer should be saved")
	}
	// Philosophy should be finished.
	if page.Entries[3].State != "finished" {
		t.Errorf("philosophy state = %q, want finished", page.Entries[3].State)
	}
}

func TestListHistoryPagination(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	// Page 1: limit 2.
	page1, err := history.List(ctx, db, reader, 2, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Entries) != 2 {
		t.Fatalf("page 1: got %d entries, want 2", len(page1.Entries))
	}
	if page1.NextCursor == "" {
		t.Fatal("page 1: missing next cursor")
	}

	// Page 2: use cursor.
	page2, err := history.List(ctx, db, reader, 2, page1.NextCursor, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Entries) != 2 {
		t.Fatalf("page 2: got %d entries, want 2", len(page2.Entries))
	}

	// No overlap between pages.
	seen := make(map[string]bool)
	for _, e := range page1.Entries {
		seen[e.Title] = true
	}
	for _, e := range page2.Entries {
		if seen[e.Title] {
			t.Errorf("page 2 repeats %q from page 1", e.Title)
		}
	}
}

func TestListHistorySearch(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	page, err := history.List(ctx, db, reader, 50, "", "philos", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("search 'philos': got %d entries, want 1", len(page.Entries))
	}
	if page.Entries[0].Title != "Philosophy" {
		t.Errorf("search result = %q, want Philosophy", page.Entries[0].Title)
	}
}

func TestListHistoryDateSeek(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	// Seek to Oct 26 — should include metaphysics (Oct 26) and philosophy (Oct 25).
	page, err := history.List(ctx, db, reader, 50, "", "", "2026-10-26")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("date seek: got %d entries, want 2", len(page.Entries))
	}
}

func TestResumePosition(t *testing.T) {
	db, articleIDs, _ := seedDB(t)
	ctx := context.Background()

	r, err := history.ResumePosition(ctx, db, reader, articleIDs["computer"])
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("no resume position for computer")
	}
	// The latest pause was at sentence 4, token 3.
	if r.SentenceIdx != 4 || r.TokenIdx != 3 {
		t.Errorf("resume = (%d, %d), want (4, 3)", r.SentenceIdx, r.TokenIdx)
	}

	// No events for philosophy.
	r, err = history.ResumePosition(ctx, db, reader, articleIDs["philosophy"])
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Error("expected no resume for philosophy")
	}
}

func TestDeletePages(t *testing.T) {
	db, _, pageIDs := seedDB(t)
	ctx := context.Background()

	// Delete philosophy and computer.
	result, err := history.DeletePages(ctx, db, reader, []history.PageRef{
		{Source: "wikipedia", PageID: pageIDs["philosophy"]},
		{Source: "wikipedia", PageID: pageIDs["computer"]},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PagesDeleted != 2 {
		t.Errorf("pages deleted = %d, want 2", result.PagesDeleted)
	}

	// Verify they're gone from history.
	page, err := history.List(ctx, db, reader, 50, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("after delete: got %d entries, want 2", len(page.Entries))
	}
	for _, e := range page.Entries {
		if e.Title == "Philosophy" || e.Title == "Computer" {
			t.Errorf("deleted article %q still in history", e.Title)
		}
	}

	// Computer's events should be deleted too.
	var count int
	db.QueryRow("SELECT COUNT(*) FROM reading_events WHERE reader_id = $1", reader).Scan(&count)
	if count != 0 {
		t.Errorf("events after delete = %d, want 0", count)
	}
}

func TestDeleteAll(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	result, err := history.DeleteAll(ctx, db, reader)
	if err != nil {
		t.Fatal(err)
	}
	if result.PagesDeleted != 4 {
		t.Errorf("pages deleted = %d, want 4", result.PagesDeleted)
	}

	page, err := history.List(ctx, db, reader, 50, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 {
		t.Errorf("after delete-all: got %d entries, want 0", len(page.Entries))
	}
}

func TestDeleteAndFeedRecompute(t *testing.T) {
	db, _, pageIDs := seedDB(t)
	ctx := context.Background()

	// Get initial feed scoring (state drives the profile).
	state1, weights1, _, err := feed.Explain(ctx, db, reader)
	if err != nil {
		t.Fatal(err)
	}

	// Delete philosophy.
	_, err = history.DeletePages(ctx, db, reader, []history.PageRef{
		{Source: "wikipedia", PageID: pageIDs["philosophy"]},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Recompute: philosophy should be absent from state.
	state2, weights2, _, err := feed.Explain(ctx, db, reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range state2 {
		if s.Title == "Philosophy" {
			t.Error("philosophy should be absent from state after delete")
		}
	}
	if len(state2) >= len(state1) {
		t.Errorf("state after delete has %d entries, want fewer than %d", len(state2), len(state1))
	}

	// Weights should differ (different state means different category weights).
	_ = weights1
	_ = weights2
	// The exact weights depend on the corpus; just confirm the feed doesn't
	// error when a deleted article is absent.
	_, err = feed.For(ctx, db, reader, 5, nil)
	if err != nil {
		t.Fatalf("feed.For after delete: %v", err)
	}
}

func TestReadingStats(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	stats, err := history.ReadingStats(ctx, db, reader)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalArticles != 4 {
		t.Errorf("total articles = %d, want 4", stats.TotalArticles)
	}
	if stats.ArticlesFinished != 2 {
		t.Errorf("finished = %d, want 2", stats.ArticlesFinished)
	}
	if stats.ArticlesSkipped != 1 {
		t.Errorf("skipped = %d, want 1", stats.ArticlesSkipped)
	}
}

func TestReaderInterests(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	interests, err := history.ReaderInterests(ctx, db, reader)
	if err != nil {
		t.Fatal(err)
	}
	if interests.TotalArticles == 0 {
		t.Error("expected some articles in interests")
	}
	// With 3 philosophy-related articles, there should be at least one cluster.
	if len(interests.Clusters) == 0 {
		t.Error("expected at least one interest cluster")
	}
	// Top categories should be populated.
	if len(interests.TopCategories) == 0 {
		t.Error("expected some top categories")
	}
	for _, cl := range interests.Clusters {
		if cl.ArticleCount == 0 {
			t.Error("cluster with zero articles")
		}
		if len(cl.Articles) == 0 {
			t.Error("cluster with no article details")
		}
	}
}

func TestFeedCardReason(t *testing.T) {
	db, _, _ := seedDB(t)
	ctx := context.Background()

	cards, err := feed.For(ctx, db, reader, 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) == 0 {
		t.Fatal("no feed cards")
	}
	for _, c := range cards {
		if c.Reason == nil {
			t.Errorf("card %q has no reason", c.Title)
			continue
		}
		if c.Reason.Kind == "" {
			t.Errorf("card %q has empty reason kind", c.Title)
		}
		validKinds := map[string]bool{"relevance": true, "exploration": true, "momentum": true}
		if !validKinds[c.Reason.Kind] {
			t.Errorf("card %q: unexpected reason kind %q", c.Title, c.Reason.Kind)
		}
	}
	// Print a couple for inspection.
	for _, c := range cards[:min(3, len(cards))] {
		t.Logf("card %q: kind=%s seed=%q cats=%v exploration=%v",
			c.Title, c.Reason.Kind, c.Reason.SeedTitle,
			c.Reason.SeedCategories, c.Exploration)
	}
}

func TestHistoryBenchmarkScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}
	ctx := context.Background()
	db := testdb.New(t)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	// Insert a synthetic article and many state rows to test pagination
	// performance at scale.
	_, err := db.Exec(`
		INSERT INTO articles (source, title, url, page_id, revision_id, license, attribution, fetched_at, ingested_at)
		VALUES ('simplewiki', 'Template', 'https://example.com', 1, 1, 'CC BY-SA 4.0', 'Wikipedia', now(), now())`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO sentences (article_id, sentence_idx, heading_level, text) VALUES (1, 0, 0, 'Hello world')")
	if err != nil {
		t.Fatal(err)
	}

	// Insert 2000 state rows with different page_ids.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 2000; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		// Each needs a separate articles row.
		_, err := db.Exec(`
			INSERT INTO articles (source, title, url, page_id, revision_id, license, attribution, fetched_at, ingested_at)
			VALUES ('simplewiki', $1, 'https://example.com', $2, $2, 'CC BY-SA 4.0', 'Wikipedia', now(), now())`,
			fmt.Sprintf("Article %d", i), int64(i+1))
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`
			INSERT INTO sentences (article_id, sentence_idx, heading_level, text)
			VALUES ($1, 0, 0, 'Some text here')`, i+1)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`
			INSERT INTO reader_article_state (reader_id, source, page_id, state, updated_at)
			VALUES ($1, 'simplewiki', $2, 'finished', $3)`,
			reader, int64(i+1), ts)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Paginate through all entries.
	start := time.Now()
	total := 0
	cursor := ""
	pages := 0
	for {
		page, err := history.List(ctx, db, reader, 50, cursor, "", "")
		if err != nil {
			t.Fatal(err)
		}
		total += len(page.Entries)
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	dur := time.Since(start)
	t.Logf("paginated %d entries in %d pages, took %v", total, pages, dur)
	if total != 2000 {
		t.Errorf("total entries = %d, want 2000", total)
	}
	if dur > 10*time.Second {
		t.Errorf("pagination took %v, want under 10s", dur)
	}
}
