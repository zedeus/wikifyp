package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zedeus/wikifyp/server/internal/migrate"
	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testdb.New(t)
	if err := migrate.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

const fixturesDir = "../../../fixtures/articles"

func testArticle() source.Article {
	return source.Article{
		Source:      "wikipedia",
		Title:       "Test Article",
		URL:         "https://en.wikipedia.org/wiki/Test_Article",
		PageID:      42,
		RevisionID:  1000,
		License:     "CC BY-SA 4.0",
		Attribution: `Text from the Wikipedia article "Test Article"`,
		Fetched:     "2026-10-12",
		Text:        "Sentence one. Sentence two.\n\n== History ==\n\nBody sentence.",
		Links:       []string{"Other Article", "Another"},
		Categories:  []string{"Tests"},
	}
}

func count(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIngestStoresSentencesInOrder(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created {
		t.Error("Created = false on first ingest")
	}

	rows, err := db.Query(
		"SELECT sentence_idx, heading_level, text FROM sentences WHERE article_id = $1 ORDER BY sentence_idx",
		res.ArticleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []sentenceRow{
		{idx: 0, heading: 0, text: "Sentence one."},
		{idx: 1, heading: 0, text: "Sentence two."},
		{idx: 2, heading: 2, text: "History"},
		{idx: 3, heading: 0, text: "Body sentence."},
	}
	var got []sentenceRow
	for rows.Next() {
		var r sentenceRow
		if err := rows.Scan(&r.idx, &r.heading, &r.text); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("stored %d sentences, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sentence %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if res.Sentences != len(want) {
		t.Errorf("Result.Sentences = %d, want %d", res.Sentences, len(want))
	}

	if n := count(t, db, "SELECT count(*) FROM article_links WHERE article_id = $1", res.ArticleID); n != 2 {
		t.Errorf("links stored = %d, want 2", n)
	}
	if n := count(t, db, "SELECT count(*) FROM article_categories WHERE article_id = $1", res.ArticleID); n != 1 {
		t.Errorf("categories stored = %d, want 1", n)
	}
}

func TestIngestRequiresProvenance(t *testing.T) {
	db := newDB(t)
	for _, corrupt := range []func(*source.Article){
		func(a *source.Article) { a.RevisionID = 0 },
		func(a *source.Article) { a.License = "" },
		func(a *source.Article) { a.Attribution = "" },
	} {
		a := testArticle()
		corrupt(&a)
		if _, err := Article(context.Background(), db, &a); err == nil {
			t.Errorf("article %+v accepted without provenance", a)
		}
	}
	if n := count(t, db, "SELECT count(*) FROM articles"); n != 0 {
		t.Fatalf("rejected articles left %d rows", n)
	}
}

func TestIngestSameRevisionChangesNothing(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	first, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}

	// Even a differing text for the same revision must change nothing:
	// the stored revision is the truth.
	again := testArticle()
	again.Text = "Completely different text."
	second, err := Article(context.Background(), db, &again)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created {
		t.Error("second ingest of the same revision reports Created")
	}
	if second.ArticleID != first.ArticleID {
		t.Errorf("second ingest returned article %d, want %d", second.ArticleID, first.ArticleID)
	}
	if n := count(t, db, "SELECT count(*) FROM articles"); n != 1 {
		t.Errorf("articles = %d, want 1", n)
	}
	if n := count(t, db, "SELECT count(*) FROM sentences"); n != 4 {
		t.Errorf("sentences = %d, want 4", n)
	}
	var text string
	if err := db.QueryRow("SELECT text FROM sentences WHERE article_id = $1 AND sentence_idx = 0", first.ArticleID).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != "Sentence one." {
		t.Errorf("stored sentence changed to %q", text)
	}
}

func TestIngestNewRevisionIsANewVersion(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	first, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}

	b := testArticle()
	b.RevisionID = 1001
	b.Text = "Revised sentence."
	second, err := Article(context.Background(), db, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Created {
		t.Error("new revision not stored")
	}
	if second.ArticleID == first.ArticleID {
		t.Error("new revision reused the old article id")
	}
	if n := count(t, db, "SELECT count(*) FROM articles WHERE title = 'Test Article'"); n != 2 {
		t.Errorf("articles = %d, want 2", n)
	}
	// The old version's sentences are untouched.
	if n := count(t, db, "SELECT count(*) FROM sentences WHERE article_id = $1", first.ArticleID); n != 4 {
		t.Errorf("old version sentences = %d, want 4", n)
	}
	if n := count(t, db, "SELECT count(*) FROM sentences WHERE article_id = $1", second.ArticleID); n != 1 {
		t.Errorf("new version sentences = %d, want 1", n)
	}
}

func TestIngestRejectsEmptyAfterCleaning(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	a.Text = "== References ==\n\nOnly apparatus here.\n"
	if _, err := Article(context.Background(), db, &a); err == nil || !strings.Contains(err.Error(), "no sentences") {
		t.Fatalf("err = %v, want no-sentences error", err)
	}
	if n := count(t, db, "SELECT count(*) FROM articles"); n != 0 {
		t.Fatalf("empty article stored anyway (%d rows)", n)
	}
}

// TestIngestChemistryFixture ingests a committed real fixture end to end and
// checks hand-read facts about it: its first sentence, a section heading,
// and the absence of everything cleaning strips.
func TestIngestChemistryFixture(t *testing.T) {
	db := newDB(t)
	f := &source.Fixtures{Dir: fixturesDir}
	a, err := f.Fetch(context.Background(), "Chemistry")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Article(context.Background(), db, a)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sentences < 100 {
		t.Errorf("chemistry stored only %d sentences", res.Sentences)
	}

	var first string
	if err := db.QueryRow("SELECT text FROM sentences WHERE article_id = $1 AND sentence_idx = 0", res.ArticleID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if first != "Chemistry is the scientific study of the properties and behavior of matter." {
		t.Errorf("first sentence = %q", first)
	}

	if n := count(t, db,
		"SELECT count(*) FROM sentences WHERE article_id = $1 AND heading_level = 2 AND text = 'Etymology'",
		res.ArticleID); n != 1 {
		t.Errorf("Etymology heading rows = %d, want 1", n)
	}
	for _, gone := range []string{`%displaystyle%`, `%== %`, `%Atkins, P.W., Overton%`} {
		if n := count(t, db, "SELECT count(*) FROM sentences WHERE article_id = $1 AND text LIKE $2", res.ArticleID, gone); n != 0 {
			t.Errorf("%d stored sentences still match %s", n, gone)
		}
	}
}

// fixtureSlugs lists every committed fixture by its json file.
func fixtureSlugs(t *testing.T) []string {
	t.Helper()
	metas, err := filepath.Glob(filepath.Join(fixturesDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	slugs := make([]string, 0, len(metas))
	for _, m := range metas {
		slugs = append(slugs, strings.TrimSuffix(filepath.Base(m), ".json"))
	}
	return slugs
}

// TestIngestAllFixtures ingests every committed fixture: there must be at
// least 20, and each must carry its provenance (ingest fails without
// revision, license or attribution) and produce sentences.
func TestIngestAllFixtures(t *testing.T) {
	slugs := fixtureSlugs(t)
	if len(slugs) < 20 {
		t.Fatalf("only %d committed fixtures, want at least 20", len(slugs))
	}
	db := newDB(t)
	f := &source.Fixtures{Dir: fixturesDir}
	for _, slug := range slugs {
		a, err := f.Fetch(context.Background(), slug)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		res, err := Article(context.Background(), db, a)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if !res.Created || res.Sentences == 0 {
			t.Errorf("%s: created=%v sentences=%d", slug, res.Created, res.Sentences)
		}
	}
}

// TestIngestNestingSentences pins that the two deeply nested sentences the
// sentence-feature work (issue #13) depends on survive ingest as single
// stored sentences, exactly as they read in the fixtures.
func TestIngestNestingSentences(t *testing.T) {
	sentences := []struct{ slug, text string }{
		{"metaphysics", "Concrete particulars encountered in everyday life, like rocks, tables, and organisms, are complex entities composed of various parts."},
		{"ethics", "Obligation and permission are contrasting terms that can be defined through each other: to be obligated to do something means that one is not permitted not to do it and to be permitted to do something means that one is not obligated not to do it."},
	}
	db := newDB(t)
	f := &source.Fixtures{Dir: fixturesDir}
	for _, tt := range sentences {
		a, err := f.Fetch(context.Background(), tt.slug)
		if err != nil {
			t.Fatalf("%s: %v", tt.slug, err)
		}
		res, err := Article(context.Background(), db, a)
		if err != nil {
			t.Fatalf("%s: %v", tt.slug, err)
		}
		if n := count(t, db, "SELECT count(*) FROM sentences WHERE article_id = $1 AND text = $2", res.ArticleID, tt.text); n != 1 {
			t.Errorf("%s: stored rows with the nesting sentence = %d, want 1", tt.slug, n)
		}
	}
}

func TestIngestDuplicateLinksAndCategories(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	// The API can repeat a target across continuation pages.
	a.Links = []string{"Same", "Same", "Other"}
	a.Categories = []string{"Dup", "Dup"}
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "SELECT count(*) FROM article_links WHERE article_id = $1", res.ArticleID); n != 2 {
		t.Errorf("links = %d, want 2", n)
	}
	if n := count(t, db, "SELECT count(*) FROM article_categories WHERE article_id = $1", res.ArticleID); n != 1 {
		t.Errorf("categories = %d, want 1", n)
	}
}

func TestIngestCRLFText(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	a.Text = "One sentence.\r\nTwo here.\r\n\r\n== History ==\r\n\r\nThree.\r\n"
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT text FROM sentences WHERE article_id = $1 ORDER BY sentence_idx", res.ArticleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsRune(s, '\r') {
			t.Errorf("stored sentence keeps a carriage return: %q", s)
		}
		got = append(got, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"One sentence.", "Two here.", "History", "Three."}
	if len(got) != len(want) {
		t.Fatalf("sentences = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sentence %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// BenchmarkSentenceRows measures the CPU side of ingest (clean + split +
// row assembly) on a full real article. At ~7M articles a dump import does
// this once per article; the database inserts, not this, are the known
// bottleneck (see the comment at the prepared insert in Article).
func BenchmarkSentenceRows(b *testing.B) {
	raw, err := os.ReadFile(filepath.Join(fixturesDir, "chemistry.txt"))
	if err != nil {
		b.Fatal(err)
	}
	cleaned := source.Clean(string(raw))
	b.ReportAllocs()
	for b.Loop() {
		sentenceRows(cleaned)
	}
}
