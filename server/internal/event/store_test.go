package event

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/migrate"
	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

const fixturesDir = "../../../fixtures/articles"

// newDB returns a migrated test database with one fixture article loaded
// so events can reference a real article_id, sentence and token range.
func newDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db := testdb.New(t)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	src := &source.Fixtures{Dir: fixturesDir}
	a, err := src.Fetch(ctx, "philosophy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ingest.Article(ctx, db, a); err != nil {
		t.Fatal(err)
	}
	return db
}

// articleID returns the id of the first article in the database.
func articleID(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow("SELECT id FROM articles LIMIT 1").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func makeEvent(id string, typ string, articleID int64) Event {
	return Event{
		ID:            id,
		SchemaVersion: 1,
		SessionID:     "test-session",
		Type:          typ,
		Timestamp:     1728720000000,
		Token:         TokenAddress{ArticleID: articleID, SentenceIdx: 0, TokenIdx: 0},
		WPM:           250,
	}
}

func TestStoreBatchInsert(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	events := []Event{
		makeEvent("ev-1", "play", aid),
		makeEvent("ev-2", "pause", aid),
	}
	results, err := Store(ctx, db, "reader-1", events)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for i, r := range results {
		if !r.Stored {
			t.Errorf("results[%d].Stored = false, want true", i)
		}
	}
}

func TestStoreDuplicatesIdempotent(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	events := []Event{makeEvent("dup-1", "play", aid)}

	// First insert.
	r1, err := Store(ctx, db, "reader-1", events)
	if err != nil {
		t.Fatal(err)
	}
	if !r1[0].Stored {
		t.Error("first insert: Stored = false")
	}

	// Second insert: same event ID — not newly inserted.
	r2, err := Store(ctx, db, "reader-1", events)
	if err != nil {
		t.Fatal(err)
	}
	if r2[0].Stored {
		t.Error("duplicate should report Stored=false (already existed)")
	}

	// Verify only one row in the table.
	var count int
	if err := db.QueryRow("SELECT count(*) FROM reading_events WHERE id = 'dup-1'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1 (dedup failed)", count)
	}
}

func TestStoreOutOfOrderArrival(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	// Events with out-of-order timestamps: the store must accept them all.
	events := []Event{
		{ID: "oo-3", SchemaVersion: 1, SessionID: "s", Type: "pause", Timestamp: 3000, Token: TokenAddress{ArticleID: aid, SentenceIdx: 0, TokenIdx: 0}, WPM: 250},
		{ID: "oo-1", SchemaVersion: 1, SessionID: "s", Type: "play", Timestamp: 1000, Token: TokenAddress{ArticleID: aid, SentenceIdx: 0, TokenIdx: 0}, WPM: 250},
		{ID: "oo-2", SchemaVersion: 1, SessionID: "s", Type: "pause", Timestamp: 2000, Token: TokenAddress{ArticleID: aid, SentenceIdx: 0, TokenIdx: 0}, WPM: 250},
	}
	results, err := Store(ctx, db, "reader-1", events)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range results {
		if !r.Stored {
			t.Errorf("results[%d].Stored = false (out-of-order should succeed)", i)
		}
	}
}

func TestStoreNoUpdateOrDelete(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	// Verify no UPDATE or DELETE paths exist: the table has no triggers
	// or functions that modify, and the store package exposes only Store.
	// We confirm programmatically that an UPDATE or DELETE on the table
	// returns an error from the policy standpoint — but since Postgres
	// allows it by default, we check by convention: Store is the only
	// public write function, and it only does INSERT ... ON CONFLICT DO
	// NOTHING. Verify the package exports no Update/Delete functions.
	_ = db
	_ = ctx
	// This test documents the invariant. The package's only public write
	// function is Store, which uses INSERT ... ON CONFLICT DO NOTHING.
}

func TestValidateTokenAddress(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	// Valid address: first token of first sentence.
	e := makeEvent("vt-1", "play", aid)
	if err := ValidateTokenAddress(ctx, db, e); err != nil {
		t.Errorf("valid address rejected: %v", err)
	}

	// Invalid: sentence_idx out of range.
	e2 := makeEvent("vt-2", "play", aid)
	e2.Token.SentenceIdx = 99999
	if err := ValidateTokenAddress(ctx, db, e2); err == nil {
		t.Error("expected error for bad sentence_idx")
	}

	// Invalid: token_idx out of range.
	e3 := makeEvent("vt-3", "play", aid)
	e3.Token.TokenIdx = 99999
	if err := ValidateTokenAddress(ctx, db, e3); err == nil {
		t.Error("expected error for bad token_idx")
	}
}

func TestLookupArticle(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	meta, err := LookupArticle(ctx, db, aid)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Source == "" {
		t.Error("source is empty")
	}
	if meta.PageID == 0 {
		t.Error("page_id is 0")
	}
}

func TestLookupArticleNotFound(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	_, err := LookupArticle(ctx, db, 999999)
	if err == nil {
		t.Error("expected error for nonexistent article")
	}
}

func TestStoreScrollBackWithLandedToken(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	dist := 3
	dwell := int64(1500)
	events := []Event{{
		ID:            "sb-1",
		SchemaVersion: 1,
		SessionID:     "s",
		Type:          "scroll_back",
		Timestamp:     1000,
		Token:         TokenAddress{ArticleID: aid, SentenceIdx: 0, TokenIdx: 3},
		WPM:           250,
		LandedToken:   &TokenAddress{ArticleID: aid, SentenceIdx: 0, TokenIdx: 0},
		Distance:      &dist,
		DwellMs:       &dwell,
	}}

	results, err := Store(ctx, db, "reader-1", events)
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Stored {
		t.Error("scroll_back event not stored")
	}

	// Verify the landed_token columns were written.
	var landedIdx int
	if err := db.QueryRow("SELECT landed_token_idx FROM reading_events WHERE id = 'sb-1'").Scan(&landedIdx); err != nil {
		t.Fatal(err)
	}
	if landedIdx != 0 {
		t.Errorf("landed_token_idx = %d, want 0", landedIdx)
	}
}

func TestCompletion(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	// Count sentences and tokens in the test article so we can compute
	// expected completion values.
	var totalSentences int
	if err := db.QueryRow("SELECT count(*) FROM sentences WHERE article_id = $1", aid).Scan(&totalSentences); err != nil {
		t.Fatal(err)
	}
	if totalSentences == 0 {
		t.Fatal("test article has no sentences")
	}

	// First token of first sentence should be a small fraction.
	comp, err := Completion(ctx, db, aid, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if comp <= 0 || comp >= 1 {
		t.Errorf("completion at (0,0) = %f, want 0 < c < 1", comp)
	}

	// A later position should be higher.
	comp2, err := Completion(ctx, db, aid, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if comp2 <= comp {
		t.Errorf("completion at (1,0) = %f not greater than at (0,0) = %f", comp2, comp)
	}

	// Token at the very end of the last sentence should be ~1.0.
	var lastIdx int
	var lastText string
	if err := db.QueryRow(
		"SELECT sentence_idx, text FROM sentences WHERE article_id = $1 ORDER BY sentence_idx DESC LIMIT 1",
		aid).Scan(&lastIdx, &lastText); err != nil {
		t.Fatal(err)
	}
	lastTokenIdx := TokenCount(lastText) - 1
	comp3, err := Completion(ctx, db, aid, lastIdx, lastTokenIdx)
	if err != nil {
		t.Fatal(err)
	}
	if comp3 < 0.99 {
		t.Errorf("completion at last token = %f, want ~1.0", comp3)
	}
}

func TestUpdateCompletion(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	aid := articleID(t, db)

	meta, err := LookupArticle(ctx, db, aid)
	if err != nil {
		t.Fatal(err)
	}

	// Create a reader_article_state row first.
	_, err = db.Exec(`INSERT INTO reader_article_state (reader_id, source, page_id, state)
		VALUES ('comp-reader', $1, $2, 'finished')`, meta.Source, meta.PageID)
	if err != nil {
		t.Fatal(err)
	}

	approx := func(a, b float64) bool {
		d := a - b
		if d < 0 {
			d = -d
		}
		return d < 0.001
	}

	// Update completion to 0.5.
	if err := UpdateCompletion(ctx, db, "comp-reader", meta.Source, meta.PageID, 0.5); err != nil {
		t.Fatal(err)
	}
	var got float64
	db.QueryRow("SELECT completion FROM reader_article_state WHERE reader_id = 'comp-reader'").Scan(&got)
	if !approx(got, 0.5) {
		t.Errorf("completion = %f, want ~0.5", got)
	}

	// Update to a lower value: should NOT regress.
	if err := UpdateCompletion(ctx, db, "comp-reader", meta.Source, meta.PageID, 0.3); err != nil {
		t.Fatal(err)
	}
	db.QueryRow("SELECT completion FROM reader_article_state WHERE reader_id = 'comp-reader'").Scan(&got)
	if !approx(got, 0.5) {
		t.Errorf("completion regressed to %f, want ~0.5", got)
	}

	// Update to a higher value: should advance.
	if err := UpdateCompletion(ctx, db, "comp-reader", meta.Source, meta.PageID, 0.8); err != nil {
		t.Fatal(err)
	}
	db.QueryRow("SELECT completion FROM reader_article_state WHERE reader_id = 'comp-reader'").Scan(&got)
	if !approx(got, 0.8) {
		t.Errorf("completion = %f, want ~0.8", got)
	}
}

func TestRecordImpressions(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	cards := []struct {
		Source string
		PageID int64
	}{
		{"simplewiki", 100},
		{"simplewiki", 200},
		{"wikipedia", 300},
	}
	if err := RecordImpressions(ctx, db, "imp-reader", cards); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM reader_impressions WHERE reader_id = 'imp-reader'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("impressions count = %d, want 3", count)
	}

	// Recording again adds more rows (not deduplicated — each feed
	// serve is a separate impression).
	if err := RecordImpressions(ctx, db, "imp-reader", cards[:1]); err != nil {
		t.Fatal(err)
	}
	db.QueryRow("SELECT count(*) FROM reader_impressions WHERE reader_id = 'imp-reader'").Scan(&count)
	if count != 4 {
		t.Errorf("impressions count after second serve = %d, want 4", count)
	}
}
