package feed

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/zedeus/wikifyp/server/internal/source"
)

const fixturesDir = "../../../fixtures/articles"

// fixtureCandidate loads a committed fixture article as a ranking candidate.
// Page IDs stand in for article IDs: ranking never resolves them, it only
// needs them distinct and stable.
func fixtureCandidate(t *testing.T, slug string) Candidate {
	t.Helper()
	a := fixtureArticle(t, slug)
	return Candidate{
		ArticleID:  a.PageID,
		PageID:     a.PageID,
		Source:     a.Source,
		Title:      a.Title,
		Links:      a.Links,
		Categories: a.Categories,
	}
}

func fixtureState(t *testing.T, slug, state string) StateArticle {
	t.Helper()
	a := fixtureArticle(t, slug)
	return StateArticle{
		PageID:     a.PageID,
		Source:     a.Source,
		Title:      a.Title,
		State:      state,
		Links:      a.Links,
		Categories: a.Categories,
	}
}

func fixtureArticle(t *testing.T, slug string) *source.Article {
	t.Helper()
	src := &source.Fixtures{Dir: fixturesDir}
	a, err := src.Fetch(context.Background(), slug)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// neutralWeights gives every category the state touches the same weight, so
// ranking tests that are about links and finished/skipped mechanics are not
// entangled with document frequencies.
func neutralWeights(state []StateArticle) Weights {
	w := make(Weights)
	for _, s := range state {
		for _, c := range s.Categories {
			w[c] = 1
		}
	}
	return w
}

func rank(candidates []Candidate, state []StateArticle) []Scored {
	return RankScored(candidates, state, neutralWeights(state), nil)
}

// rankOf returns a title's position in the ranking, or -1.
func rankOf(ranked []Scored, source, title string) int {
	return slices.IndexFunc(ranked, func(c Scored) bool {
		return c.Source == source && c.Title == title
	})
}

// TestRankPromotesNeighbours: finishing Philosophy must pull up Metaphysics
// (linked from Philosophy, and linking back) over Japan, which shares
// nothing with it.
func TestRankPromotesNeighbours(t *testing.T) {
	candidates := []Candidate{
		fixtureCandidate(t, "japan"),
		fixtureCandidate(t, "metaphysics"),
		fixtureCandidate(t, "ethics"),
	}
	ranked := rank(candidates, []StateArticle{fixtureState(t, "philosophy", StateFinished)})
	if len(ranked) != 3 {
		t.Fatalf("ranked %d candidates, want 3", len(ranked))
	}
	japan := rankOf(ranked, "wikipedia", "Japan")
	for _, neighbour := range []string{"Metaphysics", "Ethics"} {
		if pos := rankOf(ranked, "wikipedia", neighbour); pos == -1 || pos > japan {
			t.Errorf("%s ranks at %d, behind unrelated Japan at %d", neighbour, pos, japan)
		}
	}
}

// TestRankSavedPromotesToo: saving counts like finishing for scoring — the
// reader declared interest either way.
func TestRankSavedPromotesToo(t *testing.T) {
	candidates := []Candidate{
		fixtureCandidate(t, "japan"),
		fixtureCandidate(t, "metaphysics"),
	}
	ranked := rank(candidates, []StateArticle{fixtureState(t, "philosophy", StateSaved)})
	if m, j := rankOf(ranked, "wikipedia", "Metaphysics"), rankOf(ranked, "wikipedia", "Japan"); m > j {
		t.Errorf("Metaphysics at %d behind Japan at %d after saving Philosophy", m, j)
	}
}

// TestRankSkipPushesCategoriesDown: skipping the Wikipedia Physics article
// must push the Simple English Physics article (sharing the Physics
// category, across sources) below an unrelated neutral article.
func TestRankSkipPushesCategoriesDown(t *testing.T) {
	candidates := []Candidate{
		fixtureCandidate(t, "simplewiki-physics"),
		fixtureCandidate(t, "japan"),
	}
	ranked := rank(candidates, []StateArticle{fixtureState(t, "physics", StateSkipped)})
	if p, j := rankOf(ranked, "simplewiki", "Physics"), rankOf(ranked, "wikipedia", "Japan"); p < j {
		t.Errorf("skipped-category Physics at %d above neutral Japan at %d", p, j)
	}
}

// TestRankFinishedNeverReturns: a finished page is dropped even though it
// scores higher than anything else.
func TestRankFinishedNeverReturns(t *testing.T) {
	candidates := []Candidate{
		fixtureCandidate(t, "philosophy"),
		fixtureCandidate(t, "metaphysics"),
	}
	ranked := rank(candidates, []StateArticle{fixtureState(t, "philosophy", StateFinished)})
	if pos := rankOf(ranked, "wikipedia", "Philosophy"); pos != -1 {
		t.Errorf("finished Philosophy still ranked, at %d", pos)
	}
	if pos := rankOf(ranked, "wikipedia", "Metaphysics"); pos != 0 {
		t.Errorf("Metaphysics not first, at %d", pos)
	}
}

// TestRankSkippedNeverServed: an article the reader acted on must not be
// offered again — a skip reappearing on the next refresh reads as the feed
// not listening (its categories still push its topic down; that is
// TestRankSkipPushesCategoriesDown), and a saved page is shelved, not a
// recommendation: it overlaps its own categories and sat pinned to the top
// of every feed (measured live, Cadillac ATS). Its signal stays positive;
// only the page itself stops serving.
func TestRankSkippedNeverServed(t *testing.T) {
	candidates := []Candidate{fixtureCandidate(t, "physics")}
	for _, state := range []string{StateSkipped, StateSaved} {
		ranked := rank(candidates, []StateArticle{fixtureState(t, "physics", state)})
		if len(ranked) != 0 {
			t.Fatalf("%s article still served: %+v", state, ranked)
		}
	}
}

// TestRankLinkMatchingStaysWithinSource: the Simple English "Chemistry"
// article links to "Physics" — the simplewiki one, not Wikipedia's.
func TestRankLinkMatchingStaysWithinSource(t *testing.T) {
	chemistry := fixtureArticle(t, "simplewiki-chemistry")
	if !slices.Contains(chemistry.Links, "Physics") {
		t.Fatal("simplewiki Chemistry no longer links to Physics; pick another pair")
	}
	state := fixtureState(t, "simplewiki-chemistry", StateFinished)
	state.Categories = nil
	enPhysics := fixtureCandidate(t, "physics")
	enPhysics.Categories = nil
	simplePhysics := fixtureCandidate(t, "simplewiki-physics")
	simplePhysics.Categories = nil

	ranked := rank([]Candidate{enPhysics, simplePhysics}, []StateArticle{state})
	if pos := rankOf(ranked, "simplewiki", "Physics"); pos != 0 {
		t.Errorf("simplewiki Physics not promoted by the link, at %d", pos)
	}
	if p := rankOf(ranked, "wikipedia", "Physics"); p != 1 {
		t.Errorf("wikipedia Physics at %d, want last", p)
	}
}

// TestRankDeterministic: same inputs, same order, across shuffled candidate
// order.
func TestRankDeterministic(t *testing.T) {
	candidates := []Candidate{
		fixtureCandidate(t, "japan"),
		fixtureCandidate(t, "calculus"),
		fixtureCandidate(t, "prime-number"),
		fixtureCandidate(t, "metaphysics"),
	}
	state := []StateArticle{fixtureState(t, "philosophy", StateFinished)}
	want := rank(candidates, state)
	shuffled := []Candidate{candidates[2], candidates[0], candidates[3], candidates[1]}
	got := rank(shuffled, state)
	for i := range want {
		if got[i].Title != want[i].Title || got[i].Source != want[i].Source {
			t.Fatalf("order differs at %d: %s vs %s", i, got[i].Title, want[i].Title)
		}
	}
}

// --- the diversity machinery ---

// testCandidate builds a synthetic candidate carrying just the categories
// the diversity tests steer by.
// TestRankInterestDecays: interest follows where the reader is heading. Two
// topics touched equally often, one at the newest end of the window and one
// at the oldest, must not weigh the same — the old one keeps roughly a
// stateDecayFloor voice, so the fresh topic outscores it by about
// sqrt(1/0.3) ~ 1.8. A flat window (ratio 1) fails this.
func TestRankInterestDecays(t *testing.T) {
	var state []StateArticle
	add := func(i int, cat string) {
		state = append(state, StateArticle{PageID: int64(5000 + i), Source: "simplewiki",
			Title: fmt.Sprintf("Read %d", i), State: StateFinished, Categories: []string{cat}})
	}
	for i := range 500 {
		switch {
		case i < 20:
			add(i, "Fresh")
		case i >= 480:
			add(i, "Stale")
		default:
			add(i, fmt.Sprintf("Filler %d", i))
		}
	}
	w := Weights{"Fresh": 0.1, "Stale": 0.1}
	ranked := RankScored([]Candidate{
		testCandidate(1, "Fresh"),
		testCandidate(2, "Stale"),
	}, state, w, nil)
	fresh, stale := ranked[0].Score, ranked[1].Score
	if ranked[0].Title != "Article 1" {
		fresh, stale = stale, fresh
	}
	if fresh <= stale {
		t.Fatalf("fresh interest %.3f did not outscore stale %.3f", fresh, stale)
	}
	if ratio := fresh / stale; ratio < 1.5 || ratio > 2.2 {
		t.Errorf("fresh/stale ratio = %.2f, want ~sqrt(1/decayFloor) = 1.8", ratio)
	}
}

// TestRankHubNominationsDiluted: a nomination is worth the nominating
// page's specificity. A year page linking a couple of hundred titles (the
// real "1975" links 227) cannot float "someone born that year" past the
// relevance floor on its own; an ordinary article inside the grace region
// still nominates in full. (The measured failure: one finished year page
// owned 4 of 15 relevance slots.)
func TestRankHubNominationsDiluted(t *testing.T) {
	hub := StateArticle{PageID: 1, Source: "simplewiki", Title: "1975", State: StateFinished}
	for i := range 226 {
		hub.Links = append(hub.Links, fmt.Sprintf("Happened in 1975 %d", i))
	}
	hub.Links = append(hub.Links, "Born That Year")
	specific := StateArticle{PageID: 2, Source: "simplewiki", Title: "Carl Wilson", State: StateFinished,
		Links: []string{"Born That Year2", "Band Mate", "B", "C", "D"}}
	candidates := []Candidate{
		{ArticleID: 1, PageID: 10, Source: "simplewiki", Title: "Born That Year"},
		{ArticleID: 2, PageID: 11, Source: "simplewiki", Title: "Band Mate"},
	}
	ranked := RankScored(candidates, []StateArticle{hub, specific}, Weights{}, nil)
	scores := map[string]float64{}
	for _, r := range ranked {
		scores[r.Title] = r.Score
	}
	if scores["Born That Year"] >= RelevanceFloor {
		t.Errorf("hub nomination = %.3f, want below the relevance floor %.1f", scores["Born That Year"], RelevanceFloor)
	}
	if scores["Band Mate"] != linkWeight {
		t.Errorf("specific nomination = %.3f, want the full linkWeight %.1f", scores["Band Mate"], linkWeight)
	}
}

func testCandidate(id int64, cats ...string) Candidate {
	return Candidate{
		ArticleID: id, PageID: id, Source: "simplewiki",
		Title:      fmt.Sprintf("Article %d", id),
		Categories: cats,
	}
}

// TestCategoryWeightsDiscountUbiquity: the weight formula must make one
// shared rare category worth more than a handful of shared huge ones —
// topical affinity over honest popularity. (Maintenance-category blocks,
// the original reason for a far steeper curve, no longer reach storage:
// ingest filters them against the wiki's own metadata.)
func TestCategoryWeightsDiscountUbiquity(t *testing.T) {
	df := map[string]int{
		"Baroque composers": 40,
		"1970s births":      8000,
		"People from Ohio":  12000,
		"Albums":            9000,
		"American singers":  15000,
	}
	w := CategoryWeights(df)

	rare := w["Baroque composers"]
	common := w["1970s births"] + w["People from Ohio"] + w["Albums"] + w["American singers"]
	if rare <= common {
		t.Errorf("1 rare category = %.3f, 4 huge categories = %.3f; popularity still dominates", rare, common)
	}
}

// TestSelectDiverseBreaksClusters: with one dominant cluster slightly
// outscoring a second topic, straight top-k would be 100% cluster; the
// similarity penalty must let the other topic through.
func TestSelectDiverseBreaksClusters(t *testing.T) {
	var scored []Scored
	for i := range 10 {
		scored = append(scored, Scored{testCandidate(int64(i), "People", "Composers"), 2.0, nil})
	}
	for i := range 10 {
		scored = append(scored, Scored{testCandidate(int64(100+i), "Rivers"), 1.0, nil})
	}
	picks := SelectDiverse(scored, nil, nil, 8, 1)
	var rivers int
	for _, p := range picks {
		if slices.Contains(p.Categories, "Rivers") {
			rivers++
		}
	}
	if rivers == 0 {
		t.Errorf("8 picks over a 2:1 cluster split contain no second-topic article: %+v", titlesOf(picks))
	}
	if rivers == 8 {
		t.Errorf("similarity penalty erased the dominant cluster entirely")
	}
}

// TestSelectDiverseExplorationSlots: at the base share — an established
// reader — every fourth slot comes from the exploration pool, flagged as
// such, and never duplicates a page.
func TestSelectDiverseExplorationSlots(t *testing.T) {
	// An established reader: plenty of corroborated rows, so exploration
	// sits at the base share — exactly every fourth slot.
	var state []StateArticle
	var scored []Scored
	for i := range 60 {
		owner := fmt.Sprintf("Read %d", i)
		state = append(state, StateArticle{PageID: int64(1000 + i), Source: "simplewiki",
			Title: owner, State: StateFinished, Categories: []string{"A", fmt.Sprintf("Sub %d", i/4)}})
	}
	for i := range 20 {
		scored = append(scored, Scored{testCandidate(int64(i), "A"), float64(20 - i), []string{fmt.Sprintf("Read %d", i)}})
	}
	explore := []Candidate{
		testCandidate(0, "B"), // duplicates a scored pick; must be skipped
		testCandidate(200, "B"),
		testCandidate(201, "B"),
		testCandidate(202, "B"),
	}
	picks := SelectDiverse(scored, explore, state, 12, 7)
	if len(picks) != 12 {
		t.Fatalf("picked %d, want 12", len(picks))
	}
	seen := make(map[int64]bool)
	var explorations []int
	for i, p := range picks {
		if seen[p.PageID] {
			t.Errorf("page %d served twice", p.PageID)
		}
		seen[p.PageID] = true
		if p.Exploration {
			explorations = append(explorations, i)
		}
	}
	if want := []int{3, 7, 11}; !slices.Equal(explorations, want) {
		t.Errorf("established reader: exploration slots at %v, want %v", explorations, want)
	}
}

// TestSelectDiverseExplorationAdapts: a reader with no corroborated
// pattern gets most of the feed as doors; the share falls back to the
// base as clustered evidence accumulates (the established reader above).
func TestSelectDiverseExplorationAdapts(t *testing.T) {
	// Twelve one-off reads, no two alike: history without a pattern.
	var state []StateArticle
	var scored []Scored
	for i := range 12 {
		owner := fmt.Sprintf("Stray %d", i)
		state = append(state, StateArticle{PageID: int64(1000 + i), Source: "simplewiki",
			Title: owner, State: StateFinished, Categories: []string{fmt.Sprintf("Lone %d", i)}})
		scored = append(scored, Scored{testCandidate(int64(i), fmt.Sprintf("N%d", i)), 1.0, []string{owner}})
	}
	var explore []Candidate
	for i := range 20 {
		explore = append(explore, testCandidate(int64(200+i), "Door"))
	}
	picks := SelectDiverse(scored, explore, state, 12, 7)
	doors := 0
	for _, p := range picks {
		if p.Exploration {
			doors++
		}
	}
	// Zero clustered evidence: the share sits at the max.
	if doors < 7 {
		t.Errorf("patternless reader got %d exploration slots of 12, want most of the feed", doors)
	}
	// The momentum slots are never converted to doors, whatever the share:
	// they are how the feed reacts to the newest read.
	if picks[0].Exploration || picks[6].Exploration {
		t.Errorf("momentum slot converted to a door: slot0=%v slot6=%v",
			picks[0].Exploration, picks[6].Exploration)
	}
}

// TestSelectDiverseSeeding: one seed is reproducible; different seeds give
// a different order somewhere in the feed (the dash of randomness).
func TestSelectDiverseSeeding(t *testing.T) {
	var scored []Scored
	for i := range 30 {
		// Flat-ish scores: plenty of near-ties for the jitter to shuffle.
		scored = append(scored, Scored{testCandidate(int64(i), "A"), 1 + float64(i%3)*0.05, nil})
	}
	a := titlesOf(SelectDiverse(scored, nil, nil, 10, 42))
	b := titlesOf(SelectDiverse(scored, nil, nil, 10, 42))
	if !slices.Equal(a, b) {
		t.Errorf("same seed, different picks:\n%v\n%v", a, b)
	}
	var differs bool
	for seed := int64(1); seed <= 5 && !differs; seed++ {
		differs = !slices.Equal(a, titlesOf(SelectDiverse(scored, nil, nil, 10, seed)))
	}
	if !differs {
		t.Error("five different seeds all produced the same feed")
	}
}

// TestSelectDiversePoolExhaustion: a tiny pool falls back to exploration
// instead of cutting the feed short.
func TestSelectDiversePoolExhaustion(t *testing.T) {
	scored := []Scored{{testCandidate(1, "A"), 1, nil}}
	explore := []Candidate{testCandidate(10, "B"), testCandidate(11, "B"), testCandidate(12, "B")}
	picks := SelectDiverse(scored, explore, nil, 4, 3)
	if len(picks) != 4 {
		t.Fatalf("picked %d, want 4 (1 scored + 3 exploration)", len(picks))
	}
}

// TestSeedStableAndVarying: the seed is a pure function of reader and day.
func TestSeedStableAndVarying(t *testing.T) {
	if DaySeed("r1", "2026-10-15") != DaySeed("r1", "2026-10-15") {
		t.Error("seed not stable")
	}
	if DaySeed("r1", "2026-10-15") == DaySeed("r2", "2026-10-15") {
		t.Error("two readers share a seed")
	}
	if DaySeed("r1", "2026-10-15") == DaySeed("r1", "2026-10-16") {
		t.Error("seed does not rotate with the day")
	}
}

func titlesOf(picks []Picked) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Title
	}
	return out
}

// TestSelectDiverseNeverServesAntipathy: a pool where everything scores
// negative (e.g. a reader who has only skipped) yields exploration picks,
// not the least-disliked candidates.
func TestSelectDiverseNeverServesAntipathy(t *testing.T) {
	scored := []Scored{
		{testCandidate(1, "A"), -0.2, nil},
		{testCandidate(2, "A"), -0.5, nil},
	}
	explore := []Candidate{testCandidate(10, "B"), testCandidate(11, "B"), testCandidate(12, "B")}
	picks := SelectDiverse(scored, explore, nil, 3, 5)
	for _, p := range picks {
		if !p.Exploration {
			t.Errorf("served net-negative candidate %q over exploration", p.Title)
		}
	}
	if len(picks) != 3 {
		t.Errorf("picked %d, want 3 exploration picks", len(picks))
	}
}

// TestSelectDiverseExplorationCountsForSimilarity: a scored candidate that
// duplicates an exploration pick's categories pays the similarity penalty,
// as if a relevance pick had taken the slot. Slots 0-2 go to three strong
// unrelated picks, slot 3 explores a composer; the scored composer (1.2)
// must then fall behind the river (1.0) for every seed — its net score
// after the penalty (at most 1.2·1.25 − 1.0 = 0.5) can never reach the
// river's minimum (1.0·0.75).
func TestSelectDiverseExplorationCountsForSimilarity(t *testing.T) {
	scored := []Scored{
		{testCandidate(1, "X1"), 3.0, nil},
		{testCandidate(2, "X2"), 3.0, nil},
		{testCandidate(3, "X3"), 3.0, nil},
		{testCandidate(4, "Composers", "People"), 1.2, nil},
		{testCandidate(5, "Rivers"), 1.0, nil},
	}
	explore := []Candidate{testCandidate(100, "Composers", "People")}
	for seed := int64(0); seed < 20; seed++ {
		picks := SelectDiverse(scored, explore, nil, 6, seed)
		composerAt, riverAt := -1, -1
		for i, p := range picks {
			switch p.PageID {
			case 4:
				composerAt = i
			case 5:
				riverAt = i
			}
		}
		if riverAt == -1 {
			t.Fatalf("seed %d: river missing from %v", seed, titlesOf(picks))
		}
		if composerAt != -1 && composerAt < riverAt {
			t.Fatalf("seed %d: composer at %d before river at %d despite an explored composer",
				seed, composerAt, riverAt)
		}
	}
}

// TestRankHubsDoNotOutscoreSpecificity: a hub (a year page, a "List of …")
// links to thousands of titles, so a long-term reader's state matches many
// of them by volume alone; a focused article dedicating two of its forty
// links to the reader's reads must outrank it.
func TestRankHubsDoNotOutscoreSpecificity(t *testing.T) {
	state := make([]StateArticle, 60)
	for i := range state {
		state[i] = StateArticle{
			PageID: int64(1000 + i), Source: "simplewiki",
			Title: fmt.Sprintf("Read %d", i), State: StateFinished,
		}
	}
	hub := testCandidate(1)
	hub.Title = "2022"
	for i := range 2000 {
		hub.Links = append(hub.Links, fmt.Sprintf("Other %d", i))
	}
	for i := range 12 { // the hub hits a dozen of the reader's reads by volume
		hub.Links = append(hub.Links, fmt.Sprintf("Read %d", i))
	}
	focused := testCandidate(2)
	focused.Title = "Focused"
	for i := range 38 {
		focused.Links = append(focused.Links, fmt.Sprintf("Elsewhere %d", i))
	}
	focused.Links = append(focused.Links, "Read 0", "Read 1")

	ranked := rank([]Candidate{hub, focused}, state)
	if ranked[0].Title != "Focused" {
		t.Errorf("hub outranks the focused article: %+v", ranked)
	}
}

// TestSelectDiverseRotationFairness: relevance slots rotate across the
// reader's interest clusters, so a high-degree interest (a country
// neighbourhood producing a hundred candidates) cannot crowd out a sparse
// one (two car reads producing three) — every interest gets its turn,
// whatever the score or volume gap.
func TestSelectDiverseRotationFairness(t *testing.T) {
	state := []StateArticle{
		{PageID: 1, Source: "simplewiki", Title: "China", State: StateFinished,
			Categories: []string{"Countries", "Asia"}},
		{PageID: 4, Source: "simplewiki", Title: "Great Wall", State: StateFinished,
			Categories: []string{"Asia", "Landmarks"}},
		{PageID: 2, Source: "simplewiki", Title: "BMW 3 Series", State: StateFinished,
			Categories: []string{"Cars", "BMW"}},
		{PageID: 3, Source: "simplewiki", Title: "Cadillac ATS", State: StateSaved,
			Categories: []string{"Cars", "Cadillac"}},
	}
	var scored []Scored
	for i := range 100 {
		c := testCandidate(int64(i), fmt.Sprintf("CN%d", i))
		scored = append(scored, Scored{c, 1.4, []string{"China"}})
	}
	for i := range 3 {
		c := testCandidate(int64(500+i), fmt.Sprintf("CAR%d", i))
		owner := "BMW 3 Series"
		if i == 2 {
			owner = "Cadillac ATS"
		}
		scored = append(scored, Scored{c, 0.6, []string{owner}})
	}
	for seed := int64(0); seed < 10; seed++ {
		picks := SelectDiverse(scored, nil, state, 12, seed)
		var cars int
		for _, p := range picks {
			if p.PageID >= 500 {
				cars++
			}
		}
		if cars < 3 {
			t.Fatalf("seed %d: 12 picks, only %d from the car reads: %v", seed, cars, titlesOf(picks))
		}
	}
}

// TestSelectDiverseMomentum: the first slot of every feed belongs to the
// newest read — what the reader just finished steers the very next feed,
// for any seed, even when older reads own far higher-scoring candidates.
func TestSelectDiverseMomentum(t *testing.T) {
	state := []StateArticle{
		{PageID: 1, Source: "simplewiki", Title: "Lionel Messi", State: StateFinished},
	}
	var scored []Scored
	scored = append(scored, Scored{testCandidate(500, "Football"), 0.4, []string{"Lionel Messi"}})
	for i := range 30 {
		title := fmt.Sprintf("Car %d", i)
		state = append(state, StateArticle{PageID: int64(2 + i), Source: "simplewiki",
			Title: title, State: StateFinished})
		scored = append(scored, Scored{testCandidate(int64(i), "Cars"), 2.0, []string{title}})
	}
	for seed := int64(0); seed < 20; seed++ {
		picks := SelectDiverse(scored, nil, state, 12, seed)
		if len(picks) == 0 || picks[0].PageID != 500 {
			t.Fatalf("seed %d: first slot = %v, want the newest read's candidate", seed, titlesOf(picks))
		}
	}
}

// TestSelectDiverseTopicSatiation: a binge topic with ten times the state
// rows must not own the feed — once its near-identical cards are in, its
// turns yield to the waiting side interest, however few rows that has.
// And when there is no side interest (every candidate alike), the stale
// fallback serves the binge exactly as before: satiation only
// redistributes, it never starves.
func TestSelectDiverseTopicSatiation(t *testing.T) {
	var state []StateArticle
	var scored []Scored
	for i := range 10 {
		owner := fmt.Sprintf("Binge %d", i)
		state = append(state, StateArticle{PageID: int64(1 + i), Source: "simplewiki",
			Title: owner, State: StateFinished})
		scored = append(scored, Scored{testCandidate(int64(i), "Binge Topic"), 2.0, []string{owner}})
	}
	state = append(state, StateArticle{PageID: 100, Source: "simplewiki", Title: "Side", State: StateFinished})
	for i := range 4 {
		scored = append(scored, Scored{testCandidate(int64(100+i), fmt.Sprintf("Side %d", i)), 0.5, []string{"Side"}})
	}
	for seed := int64(0); seed < 5; seed++ {
		picks := SelectDiverse(scored, nil, state, 8, seed)
		side := 0
		for _, p := range picks {
			if p.PageID >= 100 {
				side++
			}
		}
		if side < 3 {
			t.Errorf("seed %d: side interest got %d of 8 slots against the binge, want >= 3", seed, side)
		}
	}

	// No side interest: the binge must still fill the feed via the stale
	// fallback, not collapse to nothing.
	picks := SelectDiverse(scored[:10], nil, state[:10], 8, 1)
	if len(picks) != 8 {
		t.Errorf("single-topic reader got %d of 8 cards, want all (stale fallback)", len(picks))
	}
}

// TestSelectDiverseStrayBounded: an accidental read, connected to nothing,
// holds no rotation turn — however high its neighbourhood scores, it stays
// a sliver of the feed while a corroborated interest has fresh candidates.
func TestSelectDiverseStrayBounded(t *testing.T) {
	var state []StateArticle
	for i := range 11 {
		state = append(state, StateArticle{PageID: int64(1 + i), Source: "simplewiki",
			Title: fmt.Sprintf("Composer %d", i), State: StateFinished,
			Categories: []string{"Composers", fmt.Sprintf("Era %d", i/4)}})
	}
	state = append(state, StateArticle{PageID: 500, Source: "simplewiki",
		Title: "Lawrence, Kansas", State: StateFinished})
	var scored []Scored
	for i := range 10 {
		scored = append(scored, Scored{testCandidate(int64(i), fmt.Sprintf("M%d", i)), 1.0, []string{"Composer 0"}})
	}
	for i := range 10 {
		scored = append(scored, Scored{testCandidate(int64(100+i), fmt.Sprintf("L%d", i)), 2.0, []string{"Lawrence, Kansas"}})
	}
	for seed := int64(0); seed < 10; seed++ {
		picks := SelectDiverse(scored, nil, state, 8, seed)
		var stray int
		for _, p := range picks {
			if p.PageID >= 100 {
				stray++
			}
		}
		if stray > 2 {
			t.Fatalf("seed %d: stray read took %d of 8 slots: %v", seed, stray, titlesOf(picks))
		}
	}
}

// TestRankInterestSaturates: interest in a category grows with the square
// root of how often the reader touched it, so a binge topic cannot outvote
// a smaller steady interest by raw volume — one hundred reads against four
// argue 5:1, not 25:1.
func TestRankInterestSaturates(t *testing.T) {
	var state []StateArticle
	for i := range 100 {
		state = append(state, StateArticle{PageID: int64(1000 + i), Source: "simplewiki",
			Title: fmt.Sprintf("Big %d", i), State: StateFinished, Categories: []string{"Big"}})
	}
	for i := range 4 {
		state = append(state, StateArticle{PageID: int64(2000 + i), Source: "simplewiki",
			Title: fmt.Sprintf("Small %d", i), State: StateFinished, Categories: []string{"Small"}})
	}
	w := Weights{"Big": 0.1, "Small": 0.1}
	ranked := RankScored([]Candidate{
		testCandidate(1, "Big"),
		testCandidate(2, "Small"),
	}, state, w, nil)
	big, small := ranked[0].Score, ranked[1].Score
	if ranked[0].Title != "Article 1" {
		big, small = small, big
	}
	if ratio := big / small; ratio > 6 || ratio < 4 {
		t.Errorf("binge/steady interest ratio = %.1f, want ~sqrt(100/4) = 5", ratio)
	}
}

// TestImpressionNormalizedInterest: a category the reader sees often but
// never clicks is downweighted relative to one they click every time.
func TestImpressionNormalizedInterest(t *testing.T) {
	state := []StateArticle{
		{Title: "Cars article", State: StateFinished,
			Links: []string{}, Categories: []string{"Cars", "Shared"}},
		{Title: "Music article", State: StateFinished,
			Links: []string{}, Categories: []string{"Music", "Shared"}},
	}
	candidates := []Candidate{
		{ArticleID: 10, PageID: 10, Source: "simplewiki", Title: "New car",
			Links: []string{}, Categories: []string{"Cars"}},
		{ArticleID: 11, PageID: 11, Source: "simplewiki", Title: "New song",
			Links: []string{}, Categories: []string{"Music"}},
	}
	w := Weights{"Cars": 0.1, "Music": 0.1, "Shared": 0.05}

	// Without impressions: both should score similarly.
	noImps := RankScored(candidates, state, w, nil)
	var carNoImp, musicNoImp float64
	for _, s := range noImps {
		if s.Title == "New car" {
			carNoImp = s.Score
		}
		if s.Title == "New song" {
			musicNoImp = s.Score
		}
	}
	if carNoImp == 0 || musicNoImp == 0 {
		t.Fatal("candidates not scored")
	}

	// With impressions above the warmup threshold: Cars shown far more
	// than Music. Cars impression share > its interaction share, so the
	// excess-damping discount kicks in.
	imps := map[string]int{"Cars": 4000, "Shared": 2000, "Music": 100}
	withImps := RankScored(candidates, state, w, imps)
	var carImp, musicImp float64
	for _, s := range withImps {
		if s.Title == "New car" {
			carImp = s.Score
		}
		if s.Title == "New song" {
			musicImp = s.Score
		}
	}
	if musicImp <= carImp {
		t.Errorf("impression-normalized: music (%.3f) should outscore cars (%.3f) since cars were shown 100x", musicImp, carImp)
	}
	if carImp >= carNoImp {
		t.Errorf("heavy impressions should discount cars: with=%.3f should be < without=%.3f", carImp, carNoImp)
	}
}

// TestStateClusters: positive rows chain into interest clusters through
// pairwise category overlap; rows that connect to nothing are misc.
func TestStateClusters(t *testing.T) {
	state := []StateArticle{
		{Title: "Corolla", State: StateFinished, Categories: []string{"Toyota", "Sedans"}},
		{Title: "Elvis", State: StateFinished, Categories: []string{"Singers", "Rock"}},
		{Title: "Camry", State: StateFinished, Categories: []string{"Toyota", "Hybrids"}},
		// Chains to the cars cluster through Camry only (no Toyota overlap
		// with Corolla): components, not pairwise similarity.
		{Title: "Prius", State: StateFinished, Categories: []string{"Hybrids", "Hatchbacks"}},
		{Title: "Presley discography", State: StateFinished, Categories: []string{"Rock", "Discographies"}},
		{Title: "Skip me", State: StateSkipped, Categories: []string{"Toyota"}},
		{Title: "Stray", State: StateFinished, Categories: []string{"Bridges"}},
		{Title: "No cats", State: StateFinished},
	}
	clusters, misc := StateClusters(state)
	want := [][]string{
		{"Corolla", "Camry", "Prius"},
		{"Elvis", "Presley discography"},
	}
	if !slices.EqualFunc(clusters, want, slices.Equal) {
		t.Errorf("clusters = %v, want %v", clusters, want)
	}
	if !slices.Equal(misc, []string{"Stray", "No cats"}) {
		t.Errorf("misc = %v, want [Stray, No cats]", misc)
	}
}

// TestSelectDiverseClusterBalance: a 30-read interest and a 3-read interest
// get comparable feed shares. Row-level rotation gave the big interest 10x
// the turns (engagement volume compounded); cluster-level turns argue as
// equals. The big interest's candidates carry distinct categories so
// satiation cannot be what bounds it — only the rotation.
func TestSelectDiverseClusterBalance(t *testing.T) {
	var state []StateArticle
	var scored []Scored
	for i := range 30 {
		owner := fmt.Sprintf("Big %d", i)
		state = append(state, StateArticle{PageID: int64(1 + i), Source: "simplewiki",
			Title: owner, State: StateFinished, Categories: []string{"Big topic", fmt.Sprintf("Big sub %d", i/3)}})
		scored = append(scored, Scored{testCandidate(int64(i), fmt.Sprintf("Big kind %d", i)), 2.0, []string{owner}})
	}
	for i := range 3 {
		owner := fmt.Sprintf("Small %d", i)
		state = append(state, StateArticle{PageID: int64(100 + i), Source: "simplewiki",
			Title: owner, State: StateFinished, Categories: []string{"Small topic", fmt.Sprintf("Small sub %d", i)}})
		scored = append(scored, Scored{testCandidate(int64(100+i), fmt.Sprintf("Small kind %d", i)), 0.6, []string{owner}})
	}
	for seed := int64(0); seed < 5; seed++ {
		picks := SelectDiverse(scored, nil, state, 8, seed)
		small := 0
		for _, p := range picks {
			if p.PageID >= 100 {
				small++
			}
		}
		if small < 2 {
			t.Errorf("seed %d: small interest got %d of 8 slots against a 30-read interest, want >= 2", seed, small)
		}
	}
}

// BenchmarkStateClusters measures the per-feed cost of clustering a full
// interest window (recentStateLimit rows, realistic category counts) —
// the quadratic pairwise pass is the concern at 7M-article scale, where
// state size, not corpus size, is the driver (it is bounded by the
// window, so the cost is constant per feed).
func BenchmarkStateClusters(b *testing.B) {
	var state []StateArticle
	for i := range 500 {
		cats := make([]string, 15)
		for j := range cats {
			cats[j] = fmt.Sprintf("Interest %d sub %d", i%5, (i+j)%40)
		}
		state = append(state, StateArticle{Title: fmt.Sprintf("Row %d", i),
			State: StateFinished, Categories: cats})
	}
	b.ResetTimer()
	for range b.N {
		StateClusters(state)
	}
}

// TestSelectDiverseEmptyOwnerPasses: a state row that produced no
// candidates passes its turn and its momentum chance. ownerCandidates of
// a missing owner is nil, and bestOf reads nil as "search everyone" — so
// without the guard, the empty owner's slot silently went to the global
// best score, feeding exactly the dominant interest the rotation exists
// to bound (a latent bug that predated cluster rotation).
func TestSelectDiverseEmptyOwnerPasses(t *testing.T) {
	state := []StateArticle{
		{PageID: 1, Source: "simplewiki", Title: "Quiet read", State: StateFinished,
			Categories: []string{"Topic", "Quiet"}},
		{PageID: 2, Source: "simplewiki", Title: "Busy read", State: StateFinished,
			Categories: []string{"Topic", "Busy"}},
	}
	var scored []Scored
	for i := range 8 {
		scored = append(scored, Scored{testCandidate(int64(i), fmt.Sprintf("B%d", i)), 0.5, []string{"Busy read"}})
	}
	// Strong candidates nobody's reads produced: only a global search
	// (the end-of-rotation fallback) should ever reach them.
	for i := range 8 {
		scored = append(scored, Scored{testCandidate(int64(100+i), fmt.Sprintf("G%d", i)), 3.0, nil})
	}
	for seed := int64(0); seed < 5; seed++ {
		picks := SelectDiverse(scored, nil, state, 6, seed)
		for _, p := range picks {
			if p.PageID >= 100 {
				t.Errorf("seed %d: unowned candidate %d served while an owner still had cards", seed, p.PageID)
			}
		}
	}
}

// TestSelectDiverseHostileShapes: the selection must stay sane at the
// shapes the real client produces — it preloads tiny feeds (3-5 cards),
// readers exist whose every interaction is a skip, and brand-new readers
// have no state at all. No panics, no empty feeds while any pool has
// cards, and the first slot stays a relevance card whenever one exists.
func TestSelectDiverseHostileShapes(t *testing.T) {
	positive := []StateArticle{
		{PageID: 1, Source: "simplewiki", Title: "Lion", State: StateFinished,
			Categories: []string{"Mammals", "Cats"}},
		{PageID: 2, Source: "simplewiki", Title: "Tiger", State: StateFinished,
			Categories: []string{"Mammals", "Asia"}},
	}
	allSkipped := []StateArticle{
		{PageID: 3, Source: "simplewiki", Title: "Golf", State: StateSkipped, Categories: []string{"Sports"}},
		{PageID: 4, Source: "simplewiki", Title: "Opera", State: StateSkipped, Categories: []string{"Music"}},
	}
	scored := []Scored{
		{testCandidate(10, "Mammals"), 1.2, []string{"Lion"}},
		{testCandidate(11, "Cats"), 1.1, []string{"Lion"}},
		{testCandidate(12, "Asia"), 1.0, []string{"Tiger"}},
	}
	var explore []Candidate
	for i := range 10 {
		explore = append(explore, testCandidate(int64(100+i), "Door"))
	}
	for _, tc := range []struct {
		name   string
		scored []Scored
		state  []StateArticle
		k      int
	}{
		{"preload of one", scored, positive, 1},
		{"preload of two", scored, positive, 2},
		{"preload of three", scored, positive, 3},
		{"preload of five", scored, positive, 5},
		{"all interactions are skips", nil, allSkipped, 5},
		{"no state at all", nil, nil, 5},
		{"state but nothing scored", nil, positive, 5},
	} {
		for seed := int64(0); seed < 5; seed++ {
			picks := SelectDiverse(tc.scored, slices.Clone(explore), tc.state, tc.k, seed)
			if len(picks) != tc.k {
				t.Errorf("%s seed %d: got %d cards, want %d", tc.name, seed, len(picks), tc.k)
			}
			if len(tc.scored) > 0 && len(picks) > 0 && picks[0].Exploration {
				t.Errorf("%s seed %d: slot 0 went to a door while relevance cards existed", tc.name, seed)
			}
		}
	}
}
