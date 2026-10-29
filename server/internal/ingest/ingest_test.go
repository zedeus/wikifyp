package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
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
	want := []SentenceRow{
		{Idx: 0, Heading: 0, Text: "Sentence one."},
		{Idx: 1, Heading: 0, Text: "Sentence two."},
		{Idx: 2, Heading: 2, Text: "History"},
		{Idx: 3, Heading: 0, Text: "Body sentence."},
	}
	var got []SentenceRow
	for rows.Next() {
		var r SentenceRow
		if err := rows.Scan(&r.Idx, &r.Heading, &r.Text); err != nil {
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

// readMeta returns the stored metadata columns for one article row,
// empty/zero for NULL.
func readMeta(t *testing.T, db *sql.DB, id int64) (imgURL string, imgW, imgH int, desc string) {
	t.Helper()
	var u, d sql.NullString
	var w, h sql.NullInt32
	if err := db.QueryRow(
		"SELECT image_url, image_width, image_height, description FROM articles WHERE id = $1", id).
		Scan(&u, &w, &h, &d); err != nil {
		t.Fatal(err)
	}
	return u.String, int(w.Int32), int(h.Int32), d.String
}

func TestIngestStoresImageAndDescription(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	a.Description = "A test subject"
	a.Image = &source.Image{URL: "https://upload.wikimedia.org/t.jpg", Width: 960, Height: 720}
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	u, w, h, d := readMeta(t, db, res.ArticleID)
	if u != a.Image.URL || w != 960 || h != 720 || d != "A test subject" {
		t.Errorf("stored metadata = (%q, %d, %d, %q), want the ingested values", u, w, h, d)
	}

	// An article without metadata stores NULLs.
	b := testArticle()
	b.Title, b.RevisionID, b.PageID = "Bare Article", 1001, 43
	resB, err := Article(context.Background(), db, &b)
	if err != nil {
		t.Fatal(err)
	}
	if u, w, h, d := readMeta(t, db, resB.ArticleID); u != "" || w != 0 || h != 0 || d != "" {
		t.Errorf("bare article stored metadata (%q, %d, %d, %q), want none", u, w, h, d)
	}
}

// TestIngestSameRevisionRefreshesMetadata: re-ingesting a stored revision
// stays a no-op for sentences, but incoming image/description metadata
// refreshes the row — and an incoming article WITHOUT metadata (a dump
// record or an old fixture) leaves stored metadata alone.
func TestIngestSameRevisionRefreshesMetadata(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	a.Description = "Old description"
	a.Image = &source.Image{URL: "https://upload.wikimedia.org/old.jpg", Width: 320, Height: 240}
	first, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}

	// Same revision, fresh metadata: the row refreshes, sentences do not.
	again := testArticle()
	again.Description = "New description"
	again.Image = &source.Image{URL: "https://upload.wikimedia.org/new.jpg", Width: 960, Height: 540}
	second, err := Article(context.Background(), db, &again)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.ArticleID != first.ArticleID {
		t.Fatalf("re-ingest = %+v, want the existing row", second)
	}
	u, w, h, d := readMeta(t, db, first.ArticleID)
	if u != again.Image.URL || w != 960 || h != 540 || d != "New description" {
		t.Errorf("metadata after refresh = (%q, %d, %d, %q), want the new values", u, w, h, d)
	}
	if n := count(t, db, "SELECT count(*) FROM sentences WHERE article_id = $1", first.ArticleID); n != 4 {
		t.Errorf("sentences = %d after metadata refresh, want 4", n)
	}

	// Same revision, no metadata: the stored values survive.
	bare := testArticle()
	if _, err := Article(context.Background(), db, &bare); err != nil {
		t.Fatal(err)
	}
	u, w, h, d = readMeta(t, db, first.ArticleID)
	if u != again.Image.URL || w != 960 || h != 540 || d != "New description" {
		t.Errorf("metadata wiped by a bare re-ingest: (%q, %d, %d, %q)", u, w, h, d)
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

func TestIngestFiltersMaintenanceCategories(t *testing.T) {
	db := newDB(t)
	// The wiki's category metadata knows one visible and one hidden
	// category; "Articles with VIAF identifiers" has no category page at
	// all, which is how most tracking categories look.
	if _, err := db.Exec(`INSERT INTO category_pages (source, category, hidden, domain) VALUES
		('wikipedia', 'Tests', false, 'Knowledge'),
		('wikipedia', 'Pages needing to be simplified', true, NULL)`); err != nil {
		t.Fatal(err)
	}
	a := testArticle()
	a.Categories = []string{"Tests", "Pages needing to be simplified", "Articles with VIAF identifiers"}
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := db.QueryRow("SELECT category FROM article_categories WHERE article_id = $1", res.ArticleID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "Tests" {
		t.Errorf("stored category = %q, want %q", got, "Tests")
	}
	var domain string
	if err := db.QueryRow("SELECT domain FROM article_domains WHERE article_id = $1", res.ArticleID).Scan(&domain); err != nil || domain != "Knowledge" {
		t.Errorf("article domain = %q, %v; want Knowledge", domain, err)
	}

	// A source with no fetched metadata keeps every category: an empty
	// table must not silently strip a corpus.
	b := testArticle()
	b.Source, b.RevisionID, b.PageID = "simplewiki", 2000, 43
	b.Categories = []string{"Tests", "Articles with VIAF identifiers"}
	resB, err := Article(context.Background(), db, &b)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "SELECT count(*) FROM article_categories WHERE article_id = $1", resB.ArticleID); n != 2 {
		t.Errorf("unfetched source: categories = %d, want 2", n)
	}
	if n := count(t, db, "SELECT count(*) FROM article_domains WHERE article_id = $1", resB.ArticleID); n != 0 {
		t.Errorf("unfetched source: article_domains rows = %d, want 0", n)
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
// bottleneck (measured by BenchmarkArticle).
func BenchmarkSentenceRows(b *testing.B) {
	raw, err := os.ReadFile(filepath.Join(fixturesDir, "chemistry.txt"))
	if err != nil {
		b.Fatal(err)
	}
	cleaned := source.Clean(string(raw))
	b.ReportAllocs()
	for b.Loop() {
		SentenceRows(cleaned)
	}
}

// BenchmarkArticle measures a whole ingest including the database writes,
// on a full real article (one transaction, one array-valued insert per
// table). This is the per-article cost of a dump import: multiply by the
// corpus (simplewiki ~250k, enwiki ~7M) for the wall-clock floor.
func BenchmarkArticle(b *testing.B) {
	db := testdb.New(b)
	if err := migrate.Apply(context.Background(), db); err != nil {
		b.Fatal(err)
	}
	src := &source.Fixtures{Dir: fixturesDir}
	a, err := src.Fetch(context.Background(), "Chemistry")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		// A fresh revision each round, so every ingest takes the full
		// insert path rather than the already-stored short circuit.
		a.RevisionID = int64(i + 1)
		if _, err := Article(context.Background(), db, a); err != nil {
			b.Fatal(err)
		}
	}
}

// TestSentenceRowsDropEmptySections: a section whose only content was
// stripped (a gallery, a reference list) loses its heading too, and the
// indices are assigned after the drop, with no holes. A parent heading
// stays when a subsection has prose.
func TestSentenceRowsDropEmptySections(t *testing.T) {
	rows := SentenceRows(strings.Join([]string{
		"Intro prose.",
		"== Gallery ==", // nothing left in it
		"== History ==", // empty itself, but a subsection has prose
		"=== Early ===",
		"Old prose.",
		"== Notes and references ==", // trailing empty section
	}, "\n"))
	want := []SentenceRow{
		{Idx: 0, Heading: 0, Text: "Intro prose."},
		{Idx: 1, Heading: 2, Text: "History"},
		{Idx: 2, Heading: 3, Text: "Early"},
		{Idx: 3, Heading: 0, Text: "Old prose."},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

// storedCategories reads an article's stored categories, sorted.
func storedCategories(t *testing.T, db *sql.DB, articleID int64) []string {
	t.Helper()
	rows, err := db.Query("SELECT category FROM article_categories WHERE article_id = $1 ORDER BY category", articleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

// TestIngestKeepsFeaturedDespiteHidden: the wikis flag their featured
// markers ("Good articles", "Featured articles") hidden, exactly like
// maintenance plumbing — but they are what seeds a brand-new reader's
// feed, so the hidden-category filter must exempt them. Pruning them
// emptied the featured pool unnoticed: a zero-history reader got an
// empty feed.
func TestIngestKeepsFeaturedDespiteHidden(t *testing.T) {
	db := newDB(t)
	if _, err := db.Exec(`INSERT INTO category_pages (source, category, hidden, domain) VALUES
		('wikipedia', 'Tests', false, 'Knowledge'),
		('wikipedia', 'Featured articles', true, NULL),
		('wikipedia', 'Pages needing cleanup', true, NULL)`); err != nil {
		t.Fatal(err)
	}
	a := testArticle()
	a.Categories = []string{"Tests", "Featured articles", "Pages needing cleanup"}
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	got := storedCategories(t, db, res.ArticleID)
	want := []string{"Featured articles", "Tests"}
	if !slices.Equal(got, want) {
		t.Errorf("stored categories = %v, want %v", got, want)
	}
}

// TestPruneCategoriesKeepsFeatured: the prune that reconciles stored
// categories with freshly fetched metadata must exempt the featured
// markers the same way ingest does — one rule, both doors.
func TestPruneCategoriesKeepsFeatured(t *testing.T) {
	db := newDB(t)
	a := testArticle()
	a.Categories = []string{"Tests", "Featured articles", "Pages needing cleanup"}
	// Ingested before any metadata existed: everything stored.
	res, err := Article(context.Background(), db, &a)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "SELECT count(*) FROM article_categories WHERE article_id = $1", res.ArticleID); n != 3 {
		t.Fatalf("pre-prune categories = %d, want 3", n)
	}
	if _, err := db.Exec(`INSERT INTO category_pages (source, category, hidden, domain) VALUES
		('wikipedia', 'Tests', false, 'Knowledge'),
		('wikipedia', 'Featured articles', true, NULL),
		('wikipedia', 'Pages needing cleanup', true, NULL)`); err != nil {
		t.Fatal(err)
	}
	pruned, err := PruneCategories(context.Background(), db, "wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1 (only the cleanup category)", pruned)
	}
	got := storedCategories(t, db, res.ArticleID)
	want := []string{"Featured articles", "Tests"}
	if !slices.Equal(got, want) {
		t.Errorf("post-prune categories = %v, want %v", got, want)
	}
}
