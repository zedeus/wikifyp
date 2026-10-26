package feed

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zedeus/wikifyp/server/internal/wiki"
)

// minWords is the shortest article the feed will offer. Dumps are full of
// stubs down to a single word; a card the reader opens and finishes in two
// seconds is noise, not reading.
const minWords = 50

// poolLimit bounds each candidate query. The pool only has to be wide
// enough that the real neighbours are in it; ranking decides the order.
const poolLimit = 1000

// featuredLimit bounds the featured query: a feed serves at most a few
// dozen cards, and the slack covers the odd featured stub the word floor
// then drops.
const featuredLimit = 150

// topInterestCategories bounds how many interest categories the pool
// query searches, picked by discriminative weight (rare first).
const topInterestCategories = 250

// now is the feed's clock, swappable so tests control the day seed and the
// hourly exploration rotation — For's ordering depends on both, and a test
// asserting ranked positions was flaky across wall-clock hours before this
// (exploration picks feed the similarity penalty, reordering near-ties).
var nowFn atomic.Pointer[func() time.Time]

func init() {
	f := time.Now
	nowFn.Store(&f)
}

// now is the feed's clock. It reads through an atomic pointer so PinClock
// can swap it while tests or benchmarks run concurrently with requests.
func now() time.Time { return (*nowFn.Load())() }

// PinClock fixes the feed's clock and returns a restore func. For the
// simulator's benchmark and for tests: a standardized run must not depend
// on the day or hour it happened to execute (exploration still rotates
// within a run — its seed also advances with the reader's acted-on count).
func PinClock(tm time.Time) func() {
	was := nowFn.Load()
	f := func() time.Time { return tm }
	nowFn.Store(&f)
	return func() { nowFn.Store(was) }
}

// walkSteps and walkCount shape the related-exploration walk: three
// category hops from the reader's reads, Monte Carlo over walkCount
// walkers. Depth 3 measured best (depth 2 stays too close, 4 drifts);
// link edges measured worse at every depth — citation sinks (Wayback
// Machine) and country hubs pollute them, and weighting by degree makes
// low-out-degree sinks attractors. See the walklab matrix in the worklog.
const (
	walkSteps = 3
	walkCount = 120
)

// recentStateLimit bounds how much history shapes the interest profile: the
// most recent interactions say what the reader is into now, and a tastes
// profile built from everything they ever read both drifts stale and costs
// a graph-data load proportional to a lifetime of reading (a daily reader
// passes 2,500 articles within weeks). Suppression is NOT windowed: a page
// finished or skipped years ago still never comes back. A var, not a
// const, so tests can exercise the window with a small corpus.
var recentStateLimit = 500

// Card is one feed entry, ready to serve. Exploration marks a card that
// filled an exploration slot rather than scoring in on relevance.
type Card struct {
	ArticleID   int64  `json:"id"`
	PageID      int64  `json:"-"` // not serialized; used for impression tracking
	Title       string `json:"title"`
	Source      string `json:"source"`
	Excerpt     string `json:"excerpt"`
	Sentences   int    `json:"sentences"`
	Words       int    `json:"words"`
	Attribution string `json:"attribution"`
	Exploration bool   `json:"exploration,omitempty"`
}

// SetState records what a reader did with a page, replacing any earlier
// state for the same page: saving then finishing leaves finished.
func SetState(ctx context.Context, db *sql.DB, readerID, source string, pageID int64, state string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO reader_article_state (reader_id, source, page_id, state)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (reader_id, source, page_id)
		DO UPDATE SET state = excluded.state, updated_at = now()`,
		readerID, source, pageID, state)
	if err != nil {
		return fmt.Errorf("feed: set state: %w", err)
	}
	return nil
}

// For assembles a reader's feed: score the bounded candidate pool around
// what they finished, saved and skipped, then pick diversely — seeded
// jitter, a similarity penalty against monotone runs, and exploration slots
// drawn from featured articles and the corpus at large. A reader with no
// history gets featured articles interleaved with random corpus samples.
func For(ctx context.Context, db *sql.DB, readerID string, limit int) ([]Card, error) {
	cards, _, _, _, err := ForExplained(ctx, db, readerID, limit)
	return cards, err
}

// ForExplained is For plus the intermediate products a diagnostic surface
// wants to show next to the feed (the state window, interest weights and
// scored pool) — in one pipeline run, because the pool build is the most
// expensive computation in the system and the dev UI used to pay it twice.
func ForExplained(ctx context.Context, db *sql.DB, readerID string, limit int) ([]Card, []StateArticle, Weights, []Scored, error) {
	// One clock read per request: the day seed and the exploration hour
	// must agree even when the request straddles midnight.
	at := now().UTC()
	day := at.Format(time.DateOnly)
	seed := DaySeed(readerID, day)
	state, weights, scored, err := Explain(ctx, db, readerID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// Suppression spans the whole history, not the interest window.
	suppressed, suppressedTitles, err := suppressedPagesAll(ctx, db, readerID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// Exploration draws from its own seed, advanced by how much the reader
	// has acted on AND by the hour: the day seed alone made the day's
	// exploration a finite pool (one featured slice plus one fixed random
	// sample) that a heavy reader exhausted by evening — a simulated
	// 159-feed day ran it dry — and a reader who only reads, never acting,
	// saw the same doors all day (a simulated picky reader repeated a
	// third of their feed). The ranked slots stay put between refreshes;
	// only the doors rotate.
	hour := at.Format("2006-01-02T15")
	exploreSeed := DaySeed(fmt.Sprintf("%s|%d|%s", readerID, len(suppressed), hour), day)

	// The exploration pool: featured articles alternating with seeded
	// random picks from the whole corpus, minus anything finished. The
	// feed consumes the pool in order, so the interleave is what makes
	// corpus breadth actually reach the exploration slots rather than
	// queue behind a hundred featured articles.
	explore, err := featured(ctx, db, strconv.FormatInt(exploreSeed, 10), readerID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// Corpus breadth: stratified random articles from across the wiki's
	// topic branches, or uniform random as a fallback. These give a
	// cold-start reader (no state, no seeds) corpus diversity that
	// featured articles alone can't provide — featured content skews
	// toward biography and history, so a reader whose latent interests
	// are computing or animals may never find them without random doors.
	random, err := stratifiedArticles(ctx, db, readerID, limit, exploreSeed)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(random) == 0 {
		if random, err = randomArticles(ctx, db, readerID, limit, exploreSeed); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	explore = interleave(explore, random)
	if len(state) > 0 {
		// The middle distance: category random walks from the reader's
		// reads land on "related but unseen" (measured: ~30-85% on-interest
		// depending on how focused the reader is, ~half absent from the
		// one-hop pool, ~zero junk), so they take every other door, with
		// far-away stratified picks and featured curation sharing the rest.
		walked, err := Walk(ctx, db, state, WalkConfig{Steps: walkSteps, Cats: true, Walks: walkCount, Top: 2 * limit}, exploreSeed)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		related := make([]Candidate, 0, len(walked))
		for _, h := range walked {
			related = append(related, h.Candidate)
		}
		// The same readability floor every other exploration lane applies:
		// a walk lands on stubs as happily as on articles.
		if related, err = keepReadable(ctx, db, related); err != nil {
			return nil, nil, nil, nil, err
		}
		explore = interleave(related, explore)
	}
	explore = slices.DeleteFunc(explore, func(c Candidate) bool {
		return suppressed[pageKey{c.Source, c.PageID}] || suppressedTitles[c.Title]
	})
	// Only the head of the pool can ever be consumed (a handful of slots
	// per feed); trim it, then load those articles' categories so the
	// similarity penalty sees exploration picks too — without this, an
	// explored composer biography licenses a run of near-identical
	// composers right after it.
	explore = explore[:min(len(explore), 3*limit)]
	if err := fillCategories(ctx, db, explore); err != nil {
		return nil, nil, nil, nil, err
	}

	picks := SelectDiverse(scored, explore, state, limit, seed)
	chosen := make([]Candidate, len(picks))
	for i, p := range picks {
		chosen[i] = p.Candidate
	}
	// cards keeps the chosen order, so picks and cards line up by index.
	cards, err := cards(ctx, db, chosen)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for i := range cards {
		// A reader with no history has no neighbourhood to explore out of:
		// their whole feed is a fresh start, and flagging all of it would
		// drain the flag's meaning.
		cards[i].Exploration = picks[i].Exploration && len(state) > 0
	}
	return cards, state, weights, scored, nil
}

// fillCategories loads each candidate's categories in place. Categories
// only: the explore pool never needs links, and loading every outgoing
// link row for sixty articles per feed just to drop them was measurable
// wasted I/O on the request path.
func fillCategories(ctx context.Context, db *sql.DB, cs []Candidate) error {
	ids := make([]int64, len(cs))
	for i, c := range cs {
		ids[i] = c.ArticleID
	}
	rows, err := db.QueryContext(ctx,
		"SELECT article_id, category FROM article_categories WHERE article_id = ANY($1)", ids)
	if err != nil {
		return fmt.Errorf("feed: fill categories: %w", err)
	}
	defer rows.Close()
	cats := make(map[int64][]string)
	for rows.Next() {
		var id int64
		var c string
		if err := rows.Scan(&id, &c); err != nil {
			return fmt.Errorf("feed: fill categories: %w", err)
		}
		cats[id] = append(cats[id], c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("feed: fill categories: %w", err)
	}
	for i := range cs {
		cs[i].Categories = cats[cs[i].ArticleID]
	}
	return nil
}

// interleave alternates two slices for as long as both last, then appends
// the rest.
func interleave(a, b []Candidate) []Candidate {
	out := make([]Candidate, 0, len(a)+len(b))
	for i := 0; i < len(a) || i < len(b); i++ {
		if i < len(a) {
			out = append(out, a[i])
		}
		if i < len(b) {
			out = append(out, b[i])
		}
	}
	return out
}

// Explain returns the ranking's working for a reader: their state articles,
// the category weights, and the scored candidate pool before selection (the
// dev UI shows all three).
func Explain(ctx context.Context, db *sql.DB, readerID string) ([]StateArticle, Weights, []Scored, error) {
	state, err := readerState(ctx, db, readerID)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(state) == 0 {
		return state, nil, nil, nil
	}
	w, err := interestWeights(ctx, db, state)
	if err != nil {
		return nil, nil, nil, err
	}
	pool, err := candidates(ctx, db, readerID, state, w)
	if err != nil {
		return nil, nil, nil, err
	}
	imps, err := impressionCategoryCounts(ctx, db, readerID)
	if err != nil {
		return nil, nil, nil, err
	}
	return state, w, RankScored(pool, state, w, imps), nil
}

// stateCategories is the deduplicated list of categories the reader's state
// touches: what the candidate pool query matches on and what the weights
// cover.
func stateCategories(state []StateArticle) []string {
	seen := make(map[string]bool)
	var cats []string
	for _, s := range state {
		for _, c := range s.Categories {
			if !seen[c] {
				seen[c] = true
				cats = append(cats, c)
			}
		}
	}
	return cats
}

// impressionCategoryCounts counts how many times articles in each category
// have been served to this reader. The feed records one impression per card
// per serve, so a page shown three times counts three times for each of
// its categories. The result drives impression-normalized interest: a
// category the reader sees all the time but never clicks is weaker evidence
// than one they click every time it appears.
func impressionCategoryCounts(ctx context.Context, db *sql.DB, readerID string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ac.category, count(*)
		FROM reader_impressions ri
		JOIN latest_articles a ON a.source = ri.source AND a.page_id = ri.page_id
		JOIN article_categories ac ON ac.article_id = a.id
		WHERE ri.reader_id = $1
		GROUP BY ac.category`, readerID)
	if err != nil {
		return nil, fmt.Errorf("feed: impression category counts: %w", err)
	}
	defer rows.Close()
	counts := make(map[string]int)
	for rows.Next() {
		var c string
		var n int
		if err := rows.Scan(&c, &n); err != nil {
			return nil, fmt.Errorf("feed: impression category counts: %w", err)
		}
		counts[c] = n
	}
	return counts, rows.Err()
}

// interestWeights looks up how discriminative each category the reader's
// state touches is across the corpus.
func interestWeights(ctx context.Context, db *sql.DB, state []StateArticle) (Weights, error) {
	cats := stateCategories(state)
	df := make(map[string]int, len(cats))
	if len(cats) == 0 {
		return CategoryWeights(df), nil
	}
	rows, err := db.QueryContext(ctx,
		"SELECT category, count(*) FROM article_categories WHERE category = ANY($1) GROUP BY category", cats)
	if err != nil {
		return nil, fmt.Errorf("feed: category weights: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		var n int
		if err := rows.Scan(&c, &n); err != nil {
			return nil, fmt.Errorf("feed: category weights: %w", err)
		}
		df[c] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: category weights: %w", err)
	}
	return CategoryWeights(df), nil
}

// randomArticles draws a seeded sample of readable articles from the whole
// corpus: the breadth half of the exploration pool. Article IDs are dense
// (one identity sequence), so guessing IDs and keeping the hits is a cheap
// uniform-ish sample that stays deterministic per seed.
func randomArticles(ctx context.Context, db *sql.DB, readerID string, n int, seed int64) ([]Candidate, error) {
	var lo, hi sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT min(id), max(id) FROM articles").Scan(&lo, &hi); err != nil {
		return nil, fmt.Errorf("feed: random articles: %w", err)
	}
	if !hi.Valid {
		return nil, nil
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0x72616e64)) // "rand"
	ids := make([]int64, 8*n)
	for i := range ids {
		ids[i] = lo.Int64 + rng.Int64N(hi.Int64-lo.Int64+1)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.page_id, a.source, a.title
		FROM latest_articles a
		WHERE a.id = ANY($1) AND NOT EXISTS (
			SELECT 1 FROM reader_article_state r
			WHERE r.reader_id = $2 AND r.source = a.source AND r.page_id = a.page_id)`, ids, readerID)
	if err != nil {
		return nil, fmt.Errorf("feed: random articles: %w", err)
	}
	defer rows.Close()
	byID := make(map[int64]Candidate)
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ArticleID, &c.PageID, &c.Source, &c.Title); err != nil {
			return nil, fmt.Errorf("feed: random articles: %w", err)
		}
		byID[c.ArticleID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: random articles: %w", err)
	}
	// Keep the seeded guess order, drop misses and duplicates.
	out := make([]Candidate, 0, n)
	seen := make(map[int64]bool)
	for _, id := range ids {
		if c, ok := byID[id]; ok && !seen[id] {
			seen[id] = true
			out = append(out, c)
		}
	}
	kept, err := keepReadable(ctx, db, out)
	if err != nil {
		return nil, err
	}
	// The 8x oversample only compensates for misses and unreadable hits;
	// the sample itself stays at n.
	if len(kept) > n {
		kept = kept[:n]
	}
	return kept, nil
}

// stratifiedArticles samples exploration candidates across the wiki's own
// topic branches (article_domains, rebuilt by fetch-categories): the seeded
// rotation visits domains in shuffled order and each pick is a random-id
// probe within one domain — a single descent of the (domain, article_id)
// index, like randomArticles' id guessing but domain-scoped and never
// missing. Uniform sampling mirrors the corpus's demographics: three
// mega-domains own 71% of the draws, five uniform picks span 3.4 domains on
// average and Science gets a door once per ~16 feeds; stratified picks span
// one domain each by construction. An unfetched topic tree (empty table)
// returns nothing and the caller falls back to uniform sampling.
func stratifiedArticles(ctx context.Context, db *sql.DB, readerID string, n int, seed int64) ([]Candidate, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT domain, min(article_id), max(article_id) FROM article_domains GROUP BY domain ORDER BY domain")
	if err != nil {
		return nil, fmt.Errorf("feed: stratified articles: %w", err)
	}
	defer rows.Close()
	type span struct {
		domain string
		lo, hi int64
	}
	var spans []span
	for rows.Next() {
		var s span
		if err := rows.Scan(&s.domain, &s.lo, &s.hi); err != nil {
			return nil, fmt.Errorf("feed: stratified articles: %w", err)
		}
		spans = append(spans, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: stratified articles: %w", err)
	}
	if len(spans) == 0 {
		return nil, nil
	}

	rng := rand.New(rand.NewPCG(uint64(seed), 0x646f6d73)) // "doms"
	rng.Shuffle(len(spans), func(i, j int) { spans[i], spans[j] = spans[j], spans[i] })
	// Oversample 3x: a probe can land on an old revision, a suppressed
	// page, an unreadable stub, or a duplicate of an earlier probe.
	domains := make([]string, 3*n)
	probes := make([]int64, 3*n)
	for i := range domains {
		s := spans[i%len(spans)]
		domains[i] = s.domain
		probes[i] = s.lo + rng.Int64N(s.hi-s.lo+1)
	}

	picked, err := db.QueryContext(ctx, `
		SELECT a.id, a.page_id, a.source, a.title
		FROM unnest($1::text[], $2::bigint[]) WITH ORDINALITY AS o (domain, probe, ord)
		CROSS JOIN LATERAL (
			SELECT ad.article_id FROM article_domains ad
			WHERE ad.domain = o.domain AND ad.article_id >= o.probe
			ORDER BY ad.article_id LIMIT 1) pick
		JOIN latest_articles a ON a.id = pick.article_id
		WHERE NOT EXISTS (
			SELECT 1 FROM reader_article_state r
			WHERE r.reader_id = $3 AND r.source = a.source AND r.page_id = a.page_id)
		ORDER BY o.ord`, domains, probes, readerID)
	if err != nil {
		return nil, fmt.Errorf("feed: stratified articles: %w", err)
	}
	defer picked.Close()
	out := make([]Candidate, 0, n)
	seen := make(map[int64]bool)
	for picked.Next() {
		var c Candidate
		if err := picked.Scan(&c.ArticleID, &c.PageID, &c.Source, &c.Title); err != nil {
			return nil, fmt.Errorf("feed: stratified articles: %w", err)
		}
		if !seen[c.ArticleID] {
			seen[c.ArticleID] = true
			out = append(out, c)
		}
	}
	if err := picked.Err(); err != nil {
		return nil, fmt.Errorf("feed: stratified articles: %w", err)
	}
	kept, err := keepReadable(ctx, db, out)
	if err != nil {
		return nil, err
	}
	if len(kept) > n {
		kept = kept[:n]
	}
	return kept, nil
}

// suppressedPagesAll loads every page the reader's full history keeps out
// of the feed, plus titles of skipped articles — a skipped "Physics" from
// one source should not reappear from another source through exploration
// (the feed would feel like it was not listening). Finished articles are
// not title-suppressed: the simplewiki and wikipedia versions of a topic
// are genuinely different articles, and finishing one should not hide the
// other from exploration.
func suppressedPagesAll(ctx context.Context, db *sql.DB, readerID string) (map[pageKey]bool, map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.source, s.page_id, s.state, a.title
		FROM reader_article_state s
		JOIN latest_articles a USING (source, page_id)
		WHERE s.reader_id = $1`,
		readerID)
	if err != nil {
		return nil, nil, fmt.Errorf("feed: suppressed pages: %w", err)
	}
	defer rows.Close()
	suppressed := make(map[pageKey]bool)
	skippedTitles := make(map[string]bool)
	for rows.Next() {
		var k pageKey
		var state, title string
		if err := rows.Scan(&k.source, &k.pageID, &state, &title); err != nil {
			return nil, nil, fmt.Errorf("feed: suppressed pages: %w", err)
		}
		suppressed[k] = true
		if state == StateSkipped {
			skippedTitles[title] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("feed: suppressed pages: %w", err)
	}
	return suppressed, skippedTitles, nil
}

// readerState loads the reader's most recent recentStateLimit interactions
// with each page's links and categories (from its latest stored revision):
// the interest profile the ranking works from.
func readerState(ctx context.Context, db *sql.DB, readerID string) ([]StateArticle, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.page_id, a.source, a.title, s.state, s.completion,
			(SELECT coalesce(sum(array_length(string_to_array(se.text, ' '), 1)), 0)
			 FROM sentences se WHERE se.article_id = a.id) AS words
		FROM reader_article_state s
		JOIN latest_articles a USING (source, page_id)
		WHERE s.reader_id = $1
		ORDER BY s.updated_at DESC, s.source, s.page_id
		LIMIT $2`, readerID, recentStateLimit)
	if err != nil {
		return nil, fmt.Errorf("feed: reader state: %w", err)
	}
	defer rows.Close()
	var state []StateArticle
	var ids []int64
	for rows.Next() {
		var s StateArticle
		var id int64
		if err := rows.Scan(&id, &s.PageID, &s.Source, &s.Title, &s.State, &s.Completion, &s.Words); err != nil {
			return nil, fmt.Errorf("feed: reader state: %w", err)
		}
		state = append(state, s)
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: reader state: %w", err)
	}
	links, cats, err := graphData(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for i, id := range ids {
		state[i].Links = links[id]
		state[i].Categories = cats[id]
	}
	return state, nil
}

// candidates gathers the bounded pool the ranking orders: articles linked
// from the reader's finished and saved pages, articles linking back to
// them, and articles sharing their categories. Pages the reader's full
// history suppresses are excluded in SQL — a long-term reader has finished
// much of their own neighbourhood, and without the exclusion the bounded
// pool fills with pages that can never be served. Every query reads
// latest_articles, filters stubs, and is bounded and deterministic.
func candidates(ctx context.Context, db *sql.DB, readerID string, state []StateArticle, w Weights) ([]Candidate, error) {
	// The category pool only searches the most discriminative interest
	// categories: a long-term reader's window touches thousands of
	// categories, most of them plumbing ("Articles with VIAF identifiers")
	// whose weight rounds to zero — scanning their huge membership lists
	// cost more than the rest of the pool build combined and only pulled
	// in noise the scoring would discard anyway.
	interest := stateCategories(state)
	slices.SortFunc(interest, func(a, b string) int {
		if w[a] != w[b] {
			return cmp.Compare(w[b], w[a])
		}
		return cmp.Compare(a, b)
	})
	interest = interest[:min(len(interest), topInterestCategories)]
	var sources, titles []string
	for _, s := range state {
		if s.State == StateSkipped {
			continue
		}
		sources = append(sources, s.Source)
		titles = append(titles, s.Title)
	}

	byPage := make(map[pageKey]*Candidate)
	add := func(query string, args ...any) error {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("feed: candidates: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var c Candidate
			if err := rows.Scan(&c.ArticleID, &c.PageID, &c.Source, &c.Title); err != nil {
				return fmt.Errorf("feed: candidates: %w", err)
			}
			k := pageKey{c.Source, c.PageID}
			if _, ok := byPage[k]; !ok {
				byPage[k] = &c
			}
		}
		return rows.Err()
	}

	// Every pool query ends with the same guard: no page the reader's whole
	// history has acted on can ever be served, so none may occupy pool slots.
	const notSuppressed = ` AND NOT EXISTS (
			SELECT 1 FROM reader_article_state r
			WHERE r.reader_id = $4 AND r.source = a.source AND r.page_id = a.page_id)`

	// Articles the reader's finished and saved pages link to, best-connected
	// first.
	if len(titles) > 0 {
		if err := add(`
			SELECT a.id, a.page_id, a.source, a.title
			FROM latest_articles a
			JOIN article_links l ON l.target = a.title
			JOIN latest_articles s ON l.article_id = s.id
			JOIN unnest($1::text[], $2::text[]) AS st (source, title)
			  ON s.source = st.source AND s.title = st.title
			WHERE a.source = s.source`+notSuppressed+`
			GROUP BY a.id, a.page_id, a.source, a.title
			ORDER BY count(*) DESC, a.page_id
			LIMIT $3`, sources, titles, poolLimit, readerID); err != nil {
			return nil, err
		}
		// Articles linking back to them.
		if err := add(`
			SELECT a.id, a.page_id, a.source, a.title
			FROM latest_articles a
			JOIN article_links l ON l.article_id = a.id
			JOIN unnest($1::text[], $2::text[]) AS st (source, title)
			  ON a.source = st.source AND l.target = st.title
			WHERE true`+notSuppressed+`
			GROUP BY a.id, a.page_id, a.source, a.title
			ORDER BY count(*) DESC, a.page_id
			LIMIT $3`, sources, titles, poolLimit, readerID); err != nil {
			return nil, err
		}
	}
	// Articles sharing categories with anything the reader acted on,
	// most shared categories first.
	if len(interest) > 0 {
		if err := add(`
			SELECT a.id, a.page_id, a.source, a.title
			FROM latest_articles a
			JOIN article_categories c ON c.article_id = a.id
			WHERE c.category = ANY($1)`+strings.ReplaceAll(notSuppressed, "$4", "$3")+`
			GROUP BY a.id, a.page_id, a.source, a.title
			ORDER BY count(*) DESC, a.page_id
			LIMIT $2`, interest, poolLimit, readerID); err != nil {
			return nil, err
		}
	}

	pool := make([]Candidate, 0, len(byPage))
	for _, c := range byPage {
		pool = append(pool, *c)
	}
	pool, err := keepReadable(ctx, db, pool)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(pool))
	for i, c := range pool {
		ids[i] = c.ArticleID
	}
	links, cats, err := graphData(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for i := range pool {
		pool[i].Links = links[pool[i].ArticleID]
		pool[i].Categories = cats[pool[i].ArticleID]
	}
	return pool, nil
}

// keepReadable drops candidates below the word floor, keeping order.
func keepReadable(ctx context.Context, db *sql.DB, pool []Candidate) ([]Candidate, error) {
	ids := make([]int64, len(pool))
	for i, c := range pool {
		ids[i] = c.ArticleID
	}
	words, err := wordCounts(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	kept := pool[:0]
	for _, c := range pool {
		if words[c.ArticleID] >= minWords {
			kept = append(kept, c)
		}
	}
	return kept, nil
}

// featured lists featured articles the reader has not finished or skipped,
// shuffled deterministically by salt (derived from reader and day) so two
// new readers start in different corners, one reader's feed is stable
// across refreshes, and tomorrow brings new doors. Suppression sits inside
// the query: the limit must yield servable articles, or a reader who has
// worked through the day's slice of the featured pool loses exploration
// entirely (measured on a simulated long-term reader).
func featured(ctx context.Context, db *sql.DB, salt, readerID string) ([]Candidate, error) {
	var sources, categories []string
	for source, cats := range wiki.FeaturedCategories {
		for _, c := range cats {
			sources = append(sources, source)
			categories = append(categories, c)
		}
	}
	// Bounded: enwiki has tens of thousands of featured and good articles.
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.page_id, a.source, a.title
		FROM latest_articles a
		JOIN article_categories c ON c.article_id = a.id
		JOIN unnest($1::text[], $2::text[]) AS f (source, category)
		  ON a.source = f.source AND c.category = f.category
		WHERE NOT EXISTS (
			SELECT 1 FROM reader_article_state r
			WHERE r.reader_id = $5 AND r.source = a.source AND r.page_id = a.page_id)
		GROUP BY a.id, a.page_id, a.source, a.title
		ORDER BY md5($3 || a.id::text), a.id
		LIMIT $4`, sources, categories, salt, featuredLimit, readerID)
	if err != nil {
		return nil, fmt.Errorf("feed: featured: %w", err)
	}
	defer rows.Close()
	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ArticleID, &c.PageID, &c.Source, &c.Title); err != nil {
			return nil, fmt.Errorf("feed: featured: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: featured: %w", err)
	}
	return keepReadable(ctx, db, out)
}

// graphData loads outgoing links and categories for a set of article rows.
func graphData(ctx context.Context, db *sql.DB, ids []int64) (links, cats map[int64][]string, err error) {
	links = make(map[int64][]string)
	cats = make(map[int64][]string)
	if len(ids) == 0 {
		return links, cats, nil
	}
	for _, q := range []struct {
		query string
		into  map[int64][]string
	}{
		{"SELECT article_id, target FROM article_links WHERE article_id = ANY($1)", links},
		{"SELECT article_id, category FROM article_categories WHERE article_id = ANY($1)", cats},
	} {
		rows, err := db.QueryContext(ctx, q.query, ids)
		if err != nil {
			return nil, nil, fmt.Errorf("feed: graph data: %w", err)
		}
		for rows.Next() {
			var id int64
			var v string
			if err := rows.Scan(&id, &v); err != nil {
				rows.Close()
				return nil, nil, fmt.Errorf("feed: graph data: %w", err)
			}
			q.into[id] = append(q.into[id], v)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("feed: graph data: %w", err)
		}
		rows.Close()
	}
	return links, cats, nil
}

// wordCounts sums each article's token counts. Tokens are derived from
// sentence text by splitting on spaces (the system-wide rule), so the word
// count is too.
func wordCounts(ctx context.Context, db *sql.DB, ids []int64) (map[int64]int, error) {
	counts := make(map[int64]int)
	if len(ids) == 0 {
		return counts, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT article_id, sum(array_length(string_to_array(text, ' '), 1))
		FROM sentences WHERE article_id = ANY($1) GROUP BY article_id`, ids)
	if err != nil {
		return nil, fmt.Errorf("feed: word counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("feed: word counts: %w", err)
		}
		counts[id] = n
	}
	return counts, rows.Err()
}

// cards fills in what the feed shows for the chosen articles, keeping their
// order: first prose sentence as the excerpt, sentence and word counts,
// attribution.
func cards(ctx context.Context, db *sql.DB, chosen []Candidate) ([]Card, error) {
	cards := make([]Card, 0, len(chosen))
	if len(chosen) == 0 {
		return cards, nil
	}
	ids := make([]int64, len(chosen))
	for i, c := range chosen {
		ids[i] = c.ArticleID
	}
	type detail struct {
		excerpt     string
		attribution string
		sentences   int
		words       int
	}
	details := make(map[int64]detail, len(ids))
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.attribution,
		       count(*),
		       sum(array_length(string_to_array(s.text, ' '), 1)),
		       (SELECT text FROM sentences
		        WHERE article_id = a.id AND heading_level = 0
		        ORDER BY sentence_idx LIMIT 1)
		FROM articles a
		JOIN sentences s ON s.article_id = a.id
		WHERE a.id = ANY($1)
		GROUP BY a.id`, ids)
	if err != nil {
		return nil, fmt.Errorf("feed: cards: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var d detail
		var excerpt sql.NullString
		if err := rows.Scan(&id, &d.attribution, &d.sentences, &d.words, &excerpt); err != nil {
			return nil, fmt.Errorf("feed: cards: %w", err)
		}
		d.excerpt = excerpt.String
		details[id] = d
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: cards: %w", err)
	}
	for _, c := range chosen {
		d := details[c.ArticleID]
		cards = append(cards, Card{
			ArticleID:   c.ArticleID,
			PageID:      c.PageID,
			Title:       c.Title,
			Source:      c.Source,
			Excerpt:     d.excerpt,
			Sentences:   d.sentences,
			Words:       d.words,
			Attribution: d.attribution,
		})
	}
	return cards, nil
}
