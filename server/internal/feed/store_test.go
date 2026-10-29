package feed

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/migrate"
	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

// newDB returns a migrated test database loaded with a mix of real content:
// fixture articles from both wikis and the committed dump sample (which
// includes a featured article and two one-word stubs).
// fixedClock pins the feed's clock for one test: day-seeded jitter and the
// hourly exploration rotation must not make ordering assertions flaky.
func fixedClock(t *testing.T) {
	t.Helper()
	t.Cleanup(PinClock(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)))
}

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db := testdb.New(t)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	src := &source.Fixtures{Dir: fixturesDir}
	for _, slug := range []string{
		"philosophy", "metaphysics", "ethics", "japan", "physics",
		"simplewiki-physics", "simplewiki-chemistry",
	} {
		a, err := src.Fetch(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ingest.Article(ctx, db, a); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open("../../../fixtures/dumps/simplewiki-sample.json.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _, err = source.ReadDump(ctx, f, time.Now(), func(a *source.Article) error {
		_, err := ingest.Article(ctx, db, a)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func titles(cards []Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.Title
	}
	return out
}

func position(cards []Card, title string) int {
	for i, c := range cards {
		if c.Title == title {
			return i
		}
	}
	return -1
}

// checkCards asserts the invariants every feed must hold: no stubs, no
// duplicate pages, populated card fields.
func checkCards(t *testing.T, cards []Card) {
	t.Helper()
	seen := make(map[string]bool)
	for _, c := range cards {
		if c.Words < minWords {
			t.Errorf("card %q has %d words, below the %d-word floor", c.Title, c.Words, minWords)
		}
		key := c.Source + "/" + c.Title
		if seen[key] {
			t.Errorf("card %q served twice", key)
		}
		seen[key] = true
		if c.ArticleID == 0 || c.Excerpt == "" || c.Attribution == "" || c.Sentences == 0 {
			t.Errorf("card %q incomplete: %+v", c.Title, c)
		}
	}
}

// TestForNewReader: a reader with no state gets featured articles
// interleaved with random corpus samples — in the loaded data the
// featured article is the dump sample's Red Hot Chili Peppers
// (Category:Very good articles).
func TestForNewReader(t *testing.T) {
	db := newDB(t)
	cards, err := For(context.Background(), db, "reader-new", 10)
	if err != nil {
		t.Fatal(err)
	}
	checkCards(t, cards)
	got := titles(cards)
	if len(got) == 0 {
		t.Fatal("new reader feed is empty")
	}
	hasFeatured := false
	for _, title := range got {
		if title == "Red Hot Chili Peppers" {
			hasFeatured = true
		}
	}
	if !hasFeatured {
		t.Errorf("new reader feed = %q, want Red Hot Chili Peppers among them", got)
	}
}

// TestForRankedWithTopUp: finishing Philosophy fills the feed with its
// neighbours, never Philosophy itself, and featured articles top up the
// remainder.
func TestForRankedWithTopUp(t *testing.T) {
	fixedClock(t)
	db := newDB(t)
	ctx := context.Background()
	philosophy := fixtureArticle(t, "philosophy")
	if err := SetState(ctx, db, "r1", philosophy.Source, philosophy.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r1", 10)
	if err != nil {
		t.Fatal(err)
	}
	checkCards(t, cards)
	if position(cards, "Philosophy") != -1 {
		t.Errorf("finished Philosophy served again: %q", titles(cards))
	}
	meta, eth := position(cards, "Metaphysics"), position(cards, "Ethics")
	if meta == -1 || eth == -1 {
		t.Fatalf("neighbours missing from feed: %q", titles(cards))
	}
	japan := position(cards, "Japan")
	if japan != -1 && japan < meta {
		t.Errorf("unrelated Japan at %d above neighbour Metaphysics at %d", japan, meta)
	}
	// The featured top-up runs because ranking alone cannot fill 10 slots.
	if position(cards, "Red Hot Chili Peppers") == -1 {
		t.Errorf("featured top-up missing: %q", titles(cards))
	}
}

// TestForSkipDemotes: skipping an article demotes its categories — a
// candidate sharing those categories scores lower than a pure neighbour.
// Cross-source title suppression means skipping wikipedia Physics also
// removes simplewiki Physics from the feed (same title = same article
// to the reader), so the demotion is tested through other articles that
// share the skipped article's categories without sharing its title.
func TestForSkipDemotes(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	physics := fixtureArticle(t, "physics")
	if err := SetState(ctx, db, "r2", physics.Source, physics.PageID, StateSkipped); err != nil {
		t.Fatal(err)
	}
	chem := fixtureArticle(t, "simplewiki-chemistry")
	if err := SetState(ctx, db, "r2", chem.Source, chem.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r2", 10)
	if err != nil {
		t.Fatal(err)
	}
	checkCards(t, cards)
	// Physics is title-suppressed across sources (skipping = "not this").
	if position(cards, "Physics") != -1 {
		t.Errorf("skipped title 'Physics' came back: %q", titles(cards))
	}
	// The skipped Wikipedia Physics page must not appear by page key either.
	for _, c := range cards {
		if c.Source == physics.Source && c.Title == physics.Title {
			t.Errorf("the skipped article itself came back: %q", titles(cards))
		}
	}
}

// TestForStubsNeverServed: the dump sample's one-word pages must not appear
// even when a reader's interests point straight at them (History of Canada's
// incoming link neighbourhood is empty here, so drive it via featured and
// category pools and check the floor holds everywhere).
func TestForStubsNeverServed(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	// Finish the two stub-adjacent sample articles: Fossil word shares
	// Category:Stubs with nothing ingested, but Cloverleaf quasar and the
	// stubs share nothing either — the floor is what keeps them out of the
	// featured and category pools regardless of state.
	fw := articleByTitle(t, db, "Fossil word")
	if err := SetState(ctx, db, "r3", "simplewiki", fw, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r3", 20)
	if err != nil {
		t.Fatal(err)
	}
	checkCards(t, cards)
	for _, title := range []string{"Spraycan", "History of Canada", "Fossil word"} {
		if position(cards, title) != -1 {
			t.Errorf("stub %q served: %q", title, titles(cards))
		}
	}
}

// TestSetStateReplaces: one state per page; saving then finishing leaves
// finished. A saved page is already off the feed (shelved, not a
// recommendation), and it stays off after finishing.
func TestSetStateReplaces(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	rhcp := articleByTitle(t, db, "Red Hot Chili Peppers")
	if err := SetState(ctx, db, "r4", "simplewiki", rhcp, StateSaved); err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r4", 10)
	if err != nil {
		t.Fatal(err)
	}
	if position(cards, "Red Hot Chili Peppers") != -1 {
		t.Errorf("saved article still served: %q", titles(cards))
	}
	if err := SetState(ctx, db, "r4", "simplewiki", rhcp, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err = For(ctx, db, "r4", 10)
	if err != nil {
		t.Fatal(err)
	}
	if position(cards, "Red Hot Chili Peppers") != -1 {
		t.Errorf("finished article still served: %q", titles(cards))
	}
	var state string
	if err := db.QueryRowContext(ctx,
		"SELECT state FROM reader_article_state WHERE reader_id = 'r4'").Scan(&state); err != nil || state != StateFinished {
		t.Errorf("state = %q, %v; want one finished row", state, err)
	}
}

// TestForNewRevisionStaysSuppressed: state attaches to the page, so storing
// a newer revision of a finished article must not resurface it — and the
// feed serves only the latest revision of a page.
func TestForNewRevisionStaysSuppressed(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	src := &source.Fixtures{Dir: fixturesDir}
	meta, err := src.Fetch(ctx, "metaphysics")
	if err != nil {
		t.Fatal(err)
	}
	philosophy := fixtureArticle(t, "philosophy")
	if err := SetState(ctx, db, "r5", philosophy.Source, philosophy.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}

	// A "newer revision" of Metaphysics lands (same page, bumped revision).
	meta.RevisionID++
	res, err := ingest.Article(ctx, db, meta)
	if err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r5", 20)
	if err != nil {
		t.Fatal(err)
	}
	checkCards(t, cards) // no duplicate pages: only one Metaphysics
	pos := position(cards, "Metaphysics")
	if pos == -1 {
		t.Fatalf("Metaphysics missing: %q", titles(cards))
	}
	if cards[pos].ArticleID != res.ArticleID {
		t.Errorf("feed serves article %d, want the newest revision %d", cards[pos].ArticleID, res.ArticleID)
	}

	// Finish the page via the new revision's page id, then the older
	// revision must not come back either.
	if err := SetState(ctx, db, "r5", meta.Source, meta.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err = For(ctx, db, "r5", 20)
	if err != nil {
		t.Fatal(err)
	}
	if position(cards, "Metaphysics") != -1 {
		t.Errorf("finished page resurfaced via another revision: %q", titles(cards))
	}
}

// articleByTitle resolves a stored article's page id.
func articleByTitle(t *testing.T, db *sql.DB, title string) int64 {
	t.Helper()
	var pageID int64
	if err := db.QueryRowContext(context.Background(),
		"SELECT page_id FROM articles WHERE title = $1", title).Scan(&pageID); err != nil {
		t.Fatalf("page id of %q: %v", title, err)
	}
	return pageID
}

// TestSetStateRejectsUnknownState: the state column's CHECK constraint is
// the last line of defense against a typo'd state string.
func TestSetStateRejectsUnknownState(t *testing.T) {
	db := newDB(t)
	err := SetState(context.Background(), db, "r6", "simplewiki", 1, "liked")
	if err == nil {
		t.Fatal("unknown state accepted")
	}
}

// TestInterleave: the exploration pool must alternate featured and random
// entries — appending random after ~150 featured made corpus breadth
// unreachable (at most a handful of exploration slots are ever consumed).
func TestInterleave(t *testing.T) {
	a := []Candidate{{PageID: 1}, {PageID: 2}, {PageID: 3}}
	b := []Candidate{{PageID: 10}}
	got := interleave(a, b)
	want := []int64{1, 10, 2, 3}
	for i, c := range got {
		if c.PageID != want[i] {
			t.Fatalf("interleave order = %v at %d, want %v", c.PageID, i, want)
		}
	}
}

// TestForSkippedDisappears: skipping anything the feed served — relevance
// pick or exploration pick — removes it from the next refresh. The original
// bug: exploration picks were only filtered against finished pages, and the
// within-day seed regenerates the same exploration picks, so a skipped
// explore card came straight back.
func TestForSkippedDisappears(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	philosophy := fixtureArticle(t, "philosophy")
	if err := SetState(ctx, db, "r7", philosophy.Source, philosophy.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r7", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) == 0 {
		t.Fatal("empty feed")
	}
	// Skip every served card, then none of them may return.
	for _, c := range cards {
		var pageID int64
		var source string
		if err := db.QueryRowContext(ctx,
			"SELECT page_id, source FROM articles WHERE id = $1", c.ArticleID).Scan(&pageID, &source); err != nil {
			t.Fatal(err)
		}
		if err := SetState(ctx, db, "r7", source, pageID, StateSkipped); err != nil {
			t.Fatal(err)
		}
	}
	again, err := For(ctx, db, "r7", 10)
	if err != nil {
		t.Fatal(err)
	}
	skipped := make(map[string]bool)
	for _, c := range cards {
		skipped[c.Title] = true
	}
	for _, c := range again {
		if skipped[c.Title] {
			t.Errorf("skipped %q served again (exploration=%v)", c.Title, c.Exploration)
		}
	}
}

// TestSuppressionOutlivesTheInterestWindow: the interest profile is built
// from recent history only, but a page finished long ago must still never
// be served — by ranking or exploration.
func TestSuppressionOutlivesTheInterestWindow(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	old := recentStateLimit
	recentStateLimit = 1
	defer func() { recentStateLimit = old }()

	// Finished long ago: Metaphysics. Finished just now: Philosophy.
	// The window of 1 keeps only Philosophy in the interest profile, and
	// Metaphysics is Philosophy's strongest neighbour — the first thing the
	// feed would serve if suppression were windowed too.
	meta := fixtureArticle(t, "metaphysics")
	if err := SetState(ctx, db, "r8", meta.Source, meta.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		"UPDATE reader_article_state SET updated_at = now() - interval '90 days' WHERE reader_id = 'r8'"); err != nil {
		t.Fatal(err)
	}
	philosophy := fixtureArticle(t, "philosophy")
	if err := SetState(ctx, db, "r8", philosophy.Source, philosophy.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}

	cards, err := For(ctx, db, "r8", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cards {
		if c.Title == "Metaphysics" || c.Title == "Philosophy" {
			t.Errorf("finished %q served (suppression must outlive the interest window)", c.Title)
		}
	}
	state, _, _, err := Explain(ctx, db, "r8")
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 1 || state[0].Title != "Philosophy" {
		t.Errorf("interest window = %+v, want just the recent Philosophy", state)
	}
}

// TestExplorationSurvivesFeaturedExhaustion: a long-term reader works
// through the featured pool; exploration must then come from the corpus at
// large instead of dying. (The original bug: featured LIMITed its day-slice
// before suppression filtering, so a reader who had finished those
// articles got zero exploration for the rest of the day.)
// TestWalkFindsNeighbourhood: category walks from a finished article reach
// readable two-hop neighbours, never the reader's own pages, and are
// deterministic for a seed.
func TestWalkFindsNeighbourhood(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	// The test corpus's only walkable categories are "Physics" (both
	// Physics fixtures) and "Stubs"; walking from the Wikipedia Physics
	// reaches its Simple English twin and the stub neighbours, two to
	// three hops out. 400 walkers make the stochastic survival certain.
	physics := fixtureArticle(t, "physics")
	if err := SetState(ctx, db, "r11", physics.Source, physics.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	state, err := readerState(ctx, db, "r11")
	if err != nil {
		t.Fatal(err)
	}
	cfg := WalkConfig{Steps: 3, Cats: true, Walks: 400}
	hits, err := Walk(ctx, db, state, cfg, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("walk found nothing in the fixture corpus")
	}
	for _, h := range hits {
		if h.PageID == physics.PageID && h.Source == physics.Source {
			t.Errorf("walk returned the reader's own page %q", h.Title)
		}
		if h.Depth < 2 {
			t.Errorf("hit %q at depth %d, want >= 2", h.Title, h.Depth)
		}
		if h.Visits < 1 {
			t.Errorf("hit %q with %d visits", h.Title, h.Visits)
		}
	}
	again, err := Walk(ctx, db, state, cfg, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(hits) || again[0].ArticleID != hits[0].ArticleID {
		t.Errorf("same seed walked differently: %d vs %d hits", len(again), len(hits))
	}
}

// TestStratifiedArticlesSpanDomains: with a fetched topic tree, each
// exploration probe stays inside its domain and the rotation visits every
// domain before repeating one; suppressed pages never surface; an empty
// article_domains table yields nothing (the uniform fallback's cue).
func TestStratifiedArticlesSpanDomains(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	if got, err := stratifiedArticles(ctx, db, "r10", 6, 1); err != nil || got != nil {
		t.Fatalf("unfetched tree: got %v, %v; want none", got, err)
	}
	// Assign every stored article one of three domains, round-robin by id.
	if _, err := db.Exec(`
		INSERT INTO article_domains (article_id, domain)
		SELECT id, (ARRAY['Nature','People','Technology'])[1 + id % 3] FROM articles`); err != nil {
		t.Fatal(err)
	}
	var suppressedPage *pageKey
	for seed := int64(1); seed <= 5; seed++ {
		got, err := stratifiedArticles(ctx, db, "r10", 6, seed)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) < 3 {
			t.Fatalf("seed %d: only %d picks", seed, len(got))
		}
		domains := make(map[string]int)
		for _, c := range got {
			var d string
			if err := db.QueryRow("SELECT domain FROM article_domains WHERE article_id = $1", c.ArticleID).Scan(&d); err != nil {
				t.Fatal(err)
			}
			domains[d]++
			if suppressedPage != nil && (pageKey{c.Source, c.PageID}) == *suppressedPage {
				t.Errorf("seed %d: suppressed article %q served", seed, c.Title)
			}
		}
		if len(domains) != 3 {
			t.Errorf("seed %d: picks span %d domains (%v), want 3", seed, len(domains), domains)
		}
		if suppressedPage == nil {
			first := got[0]
			suppressedPage = &pageKey{first.Source, first.PageID}
			if err := SetState(ctx, db, "r10", first.Source, first.PageID, StateSkipped); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestExplorationSurvivesFeaturedExhaustion(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	// Finish the only featured article in the corpus, plus a seed interest.
	rhcp := articleByTitle(t, db, "Red Hot Chili Peppers")
	if err := SetState(ctx, db, "r9", "simplewiki", rhcp, StateFinished); err != nil {
		t.Fatal(err)
	}
	philosophy := fixtureArticle(t, "philosophy")
	if err := SetState(ctx, db, "r9", philosophy.Source, philosophy.PageID, StateFinished); err != nil {
		t.Fatal(err)
	}
	cards, err := For(ctx, db, "r9", 12)
	if err != nil {
		t.Fatal(err)
	}
	var explore int
	for _, c := range cards {
		if c.Exploration {
			explore++
		}
		if c.Title == "Red Hot Chili Peppers" {
			t.Error("finished featured article served")
		}
	}
	if explore == 0 {
		t.Errorf("no exploration slots after the featured pool was exhausted: %v", titles(cards))
	}
}
