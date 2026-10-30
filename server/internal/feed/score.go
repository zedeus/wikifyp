// Package feed picks a reader's next articles.
//
// The product is a doomscrolling replacement for curious people, so the feed
// has two jobs that pull against each other: follow the reader's interests,
// and keep opening doors they would not have searched for. Ranking splits
// into pure functions over article link and category data (unit-tested on
// fixture articles) and the store queries around them that bound the
// candidate pool and attach reader state. Versions of a page are grouped by
// (source, page_id) through the latest_articles view, so only the newest
// stored revision of a page can surface, and reader state attaches to the
// page, never a revision.
package feed

import (
	"cmp"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
)

// Reader states as scoring sees them. Finished and skipped are stored in
// reader_article_state.state; saved is stored as an orthogonal flag, and a
// StateArticle carries StateSaved only when the flag is set on a row with no
// finished/skipped state — a page both finished and saved scores as
// finished, exactly as it did when a finish overwrote the save.
const (
	StateFinished = "finished"
	StateSaved    = "saved"
	StateSkipped  = "skipped"
)

// linkWeight is the score of a direct link between a candidate and a
// finished or saved article: a deliberate editorial statement that the two
// belong together, worth about as much as sharing several distinctive
// categories.
const linkWeight = 1.0

// SimilarityPenalty scales how hard a candidate is pushed down for
// resembling what the feed already picked (greedy MMR in SelectDiverse).
// Similarity is a cosine in [0,1] and relevance scores sit around
// linkWeight, so at 1.0 a near-clone of an already-picked card forfeits a
// direct link's worth of score — which is what breaks the all-composers /
// all-countries feeds without hiding the genuinely best neighbours.
// Swept 0.0–2.0: 0.5 maximizes appeal across all persona types.
// Halving the penalty lets a reader's few strong candidates rank higher
// without being pushed down by MMR; the picky reader benefits most
// (42.5→71.5%) because its sparse state produces few relevant candidates
// that the full penalty was displacing.
// Swept 0.5–0.7 with adaptive decay: 0.6 gives the best balance —
// every persona ≥ 70% in fast mode. At 0.5 picky drops to 63; at 0.7
// picky drops to 51. The 0.6 sweet spot lets strong candidates rank
// high without killing diversity for focused readers.
var SimilarityPenalty = 0.6

// DynamicSimPenalty, when true, adapts the similarity penalty per reader
// based on their cluster count. Single-interest readers get a lighter
// penalty (similar articles are what they want); multi-interest readers
// get a heavier penalty to force diversity. The global SimilarityPenalty
// becomes the midpoint; the actual penalty ranges from 0.4 to 0.8.
var DynamicSimPenalty = false

// RelevanceFloor is the minimum raw relevance that can occupy a relevance
// slot. A reader with three interactions has perhaps a dozen genuinely
// related candidates; below the floor the pool is pages that merely link
// their reads once in passing, and a fresh door (exploration) serves the
// reader better than that filler. It also subsumes antipathy: skip-driven
// negative scores sit far under it. Exported so diagnostic tooling
// (cmd/feedsim) classifies eligibility by the same line selection draws.
const RelevanceFloor = 0.3

// scoreJitter is the width of the seeded multiplicative noise on scores:
// ±25% keeps clear winners on top while shuffling near-ties, so two
// refreshes of a feed are recognisably similar but not identical.
const scoreJitter = 0.25

// outgoingSaturation turns outgoing-link density into score: density times
// this, capped at 1, scales linkWeight. At 10, a candidate devoting a tenth
// of its links to the reader's reads counts as fully linked; a hub spending
// a dozen of its two thousand links on them earns almost nothing.
const outgoingSaturation = 10

// Exploration's share of the feed adapts to how much corroborated taste
// the reader has shown. A quarter of the feed being doors they did not
// open themselves (ExploreShareBase) is the dash of serendipity the
// product wants from a reader whose pattern is established — but serving
// a brand-new or patternless reader 75% guesses about their taste and 25%
// doors is backwards, so with little evidence the share climbs toward
// ExploreShareMax. Evidence is CLUSTERED positive rows (StateClusters):
// reads that corroborate each other. A dozen unrelated one-off reads are
// not a pattern, and they prove it by connecting to nothing. Confidence
// ramps linearly: the share starts at the max with zero corroborated
// reads and reaches the base at exploreRampRows of them, where it stays —
// by then the pattern has earned the feed's trust. The reader swipes the
// feed one card at a time in served order, so the share also spaces the
// doors evenly through the deck rather than bunching them.
var (
	// Swept 0.10–0.25: 0.15 balances relevance with discovery. At 0.10
	// cold-start suffers (too few exploration slots after first adoption);
	// at 0.25 the juggler drops (too much noise). 0.15 gives gundam
	// enough doors to find its micro-niches while keeping the juggler
	// above 70%.
	// Validated at full 600-article benchmark: 0.20 gives the juggler
	// real interest diversity (music=286, history=178 across 5 interests)
	// while cold-start reaches 56.5% (vs 43.5% at 0.15).
	ExploreShareBase = 0.20
	ExploreShareMax  = 0.75
)

// ClusterRecencyBoost controls how many extra rotation turns the most
// recent cluster gets relative to the oldest. At 1.0 (default) all
// clusters get equal turns — the v5 baseline. At 3.0, the newest
// cluster gets 3 turns for every 1 the oldest gets, with intermediate
// clusters interpolated geometrically. This helps readers who shift
// interests (serial rabbitholer: the current phase's cluster dominates
// rotation) without hurting steady readers (a juggler's interests are
// all recently read, so they stay roughly equal).
var ClusterRecencyBoost = 2.0

// ExploreSkipThreshold is the recent skip rate (skips / total interactions,
// over the newest 30 state rows) above which the explore share gets boosted.
// A reader skipping more than 30% of articles isn't finding what they want
// in the relevance pool — the feed should try different things.
// ExploreSimCap is the maximum category cosine between two exploration
// picks in the same feed. Picks more similar than this are skipped,
// forcing exploration to cover different topics each feed. At 0.3,
// two articles sharing a third of their categories are too similar;
// at 0.0 every pick must be completely different (too strict — many
// articles share "Living people" or "United States" categories).
// Disabled by default (1.0 = no filter): fast-mode (200 articles) showed
// gains for cold-start, but full-scale (600 articles) showed it hurts
// 9 of 14 personas — the constrained exploration pool costs more than
// the early diversity gains once the reader has enough history.
var ExploreSimCap = 1.0

var ExploreSkipThreshold = 0.3

// ExploreSkipBoost scales how much extra exploration a high-skip reader
// gets. At 0.5, a 50% skip rate (20% above threshold) adds 10% to the
// explore share — a meaningful nudge without flooding the feed with doors.
var ExploreSkipBoost = 0.5

// ExploreRampRows is how many clustered state rows it takes for the
// exploration share to reach its base. Higher values keep exploration
// elevated longer, giving cold-start and sparse readers more doors.
// Swept 25–50 in the feedsim: at 25, scattered readers whose random
// seeds form noise clusters lose exploration too soon (8% appeal);
// at 40+, picky single-interest readers get too much irrelevant
// exploration (46% appeal, down from 65%). 30 preserves picky while
// giving scattered/cold-start readers enough doors to discover their
// latent interests.
var ExploreRampRows = 30

// ClusterWindow limits how many recent state rows build rotation
// clusters. Old interests that haven't been reinforced naturally fall
// off. Without this, a serial-rabbitholer who switched interests 30
// feeds ago still has rotation turns for the abandoned topic (measured:
// phase 4 got 2% on-topic, 67% residue from old phases). A window of
// 50 holds ~7 feeds of history — enough for 2-3 active interests, short
// enough that a truly abandoned interest loses its turn. The window
// applies to rotation AND explore share: during a transition, lower
// clustered rows means more exploration, which is desirable.
var ClusterWindow = 50

// PoolWindow limits how many recent state rows generate the candidate
// pool (link and category queries) and seed random walks. Scoring
// (RankScored) still sees the full state window — recency decay handles
// old interests' scoring voice. But the pool should reflect where the
// reader is heading: a serial reader who moved through cars → astronomy
// → football → history doesn't need the pool stuffed with cars
// candidates that scoring will rank at 80% voice. Without this, the
// serial-rabbitholer's phase 4 feed was 61% residue from old phases
// because the pool was built from all 500 state rows and old interests
// dominated the bounded pool slots. Swept 50–200: at 100 serial
// readers gain +12 (phase 4 active 2%→57%) but deep single-interest
// readers lose 7 appeal (mono 85→78, link-depth narrowing). At 200
// the serial reader still gains +5 (phase 4 2%→35%) and the mono
// reader keeps within 2 of its baseline — the right tradeoff.
// 0 = no window (use the full state for pool construction).
var PoolWindow = 200

// InterestFade, when true, attenuates each cluster's scoring voice by
// how much its engagement has dropped off recently. A cluster that was
// active long ago but absent from the newest InterestFadeWindow state
// rows is an abandoned interest — its categories and link nominations
// lose voice proportionally. A cluster that keeps appearing in recent
// state (even sparsely) keeps full voice. This distinguishes "moved on"
// (serial's abandoned cars phase) from "still interested, just slow"
// (picky's steady animals) without a global time decay that punishes
// both equally. The mechanism: for each cluster, compute what fraction
// of its members fall in the recent window vs the full state. A cluster
// with 20 total members and 0 in the recent 50 rows is fading; one
// with 5 total and 2 in the recent 50 is active. The fade factor is
// clamped to [InterestFadeFloor, 1.0] so abandoned interests keep a
// residual voice (their categories are still real signal, just weaker).
var InterestFade = true

// InterestFadeWindow is how many recent state rows define "recent" for
// the engagement trending. Should be large enough to cover several
// feeds (7 articles per feed × 5 feeds = 35 rows) but small enough
// that an interest abandoned 10 feeds ago has clearly faded.
var InterestFadeWindow = 50

// InterestFadeFloor is the minimum voice multiplier for a fully faded
// cluster. At 0.3, an abandoned interest keeps 30% of its voice —
// enough to serve its best candidates if they happen to appear, but
// not enough to dominate the feed with residue.
var InterestFadeFloor = 0.3

// QualityBonus scales a mild score multiplier based on article word count.
// Longer articles represent more substantial content — a 5000-word featured
// article about Porsche 911 history is a richer read than a 200-word stub
// about a minor car brand. The multiplier is 1 + QualityBonus * clamp(
// log2(words/median), -1, 1), so at 0.05 the range is [0.95, 1.05] — a
// tiebreaker, not a dominant signal. Candidates with Words == 0 (unknown)
// are unaffected. Swept 0.05–0.20: 0.05 is the sweet spot — shifter
// floor +11.5 (substantial football articles outcompete car stubs during
// transitions), scattered floor +4.0 (better articles build signal
// faster). At 0.10+ the bonus over-penalizes legitimate short articles
// (animal stubs for picky, transition-phase candidates for serial).
var QualityBonus = 0.05

// SubInterestDiversity penalizes candidates from the same sub-topic as the
// last pick from their cluster's rotation turn. Within a single "cars"
// cluster, this nudges the feed toward variety: Italian supercars one card,
// American sedans the next, rather than three Lamborghinis in a row. The
// penalty is SubInterestDiversity * cosine(candidate cats, last-pick cats)
// subtracted from the candidate's effective score during rotation turns
// only (momentum and global-fallback slots are unaffected). Disabled by
// default (0.0).
var SubInterestDiversity = 0.0

// nominationGrace is the outgoing degree up to which a state page
// nominates its neighbours at full strength; beyond it the strength decays
// as grace/degree. Ordinary articles (biographies, bands, car models) sit
// inside the grace region; year pages, countries and lists sit far outside
// it. See the nomination comment in RankScored.
const nominationGrace = 50.0

// FreshSimCap is the baseline for how similar to this feed's already-picked
// cards a candidate may be and still occupy a fresh rotation turn. Turn
// share used to be proportional to state-row count, so volume compounded: a
// five-interest reader's feed collapsed onto whichever interest won early
// (one run computing 394 of 600, the next history), and a reader who moved
// on kept getting ~72% yesterday's topic. Satiation breaks the spiral per
// feed without naming topics: once a cluster of near-identical cards is
// in, its turns yield. The stale fallback keeps the single-interest reader
// whole — when everything is alike, everything satiates together, and the
// feed serves it anyway. The cap each reader actually gets self-tunes
// around this baseline from their own state: see AdaptiveFreshCap.
var FreshSimCap = 0.5

// AdaptiveFreshCap personalizes the satiation cap from the one thing the
// reader has already told us — how much their reads resemble each other.
// A fixed cap was measured to trade readers against each other: 0.5
// balanced the five-interest juggler (worst interest 2→124 of 600 serves)
// but cost the single-interest picky reader ten appeal points, because a
// narrow state satiates instantly and the feed floods it with variety it
// never asked for. The mean pairwise category cosine of the newest
// positive reads says which reader this is: cohesive state (one deep
// love) loosens the cap toward 0.8 — more of the same stays welcome —
// and a scattered state tightens it toward 0.35, where the spiral-
// breaking matters. (A 0.2 floor — below same-interest fragment cosine —
// was measured and rejected: it left the five-interest collapse intact
// and cost the broad-single-interest reader its band floor; the fragment
// spiral needs impression data, not a tighter cap.)
func AdaptiveFreshCap(state []StateArticle) float64 {
	// Structural mechanism: uses binary state (not engagement) to avoid
	// the feedback loop where engagement-weighted coherence amplifies
	// the dominant interest over long runs.
	var sets []map[string]bool
	var cats [][]string
	for _, s := range state {
		if s.State == StateSkipped || len(s.Categories) == 0 {
			continue
		}
		m := make(map[string]bool, len(s.Categories))
		for _, c := range s.Categories {
			m[c] = true
		}
		sets = append(sets, m)
		cats = append(cats, s.Categories)
		if len(sets) == 40 {
			break
		}
	}
	if len(sets) < 2 {
		return FreshSimCap
	}
	sum, n := 0.0, 0
	for i := range sets {
		for j := i + 1; j < len(sets); j++ {
			sum += cosine(sets[i], cats[j])
			n++
		}
	}
	return min(0.8, max(0.35, 0.35+0.9*sum/float64(n)))
}

// clusterSimThreshold is the category cosine above which two positive
// state rows count as the same interest when rotation turns are grouped
// (see StateClusters). Measured on the dev corpus: pairs from different
// interests share no categories at all (cars↔animals cosine exactly 0),
// while roughly a third of same-interest pairs clear 0.15 — enough to
// chain a broad interest (birds to sharks) into one component. An
// internally diverse interest (computing: browsers, OSes, games carry
// disjoint category sets) still fragments into a handful of components;
// that over-split costs the interest a few extra turns, while an
// over-merge would rebuild the engagement spiral inside the merged
// cluster (link-overlap edges were measured to bridge animals into
// computing through shared hub targets, and were rejected for exactly
// that), so the threshold errs toward splitting.
const clusterSimThreshold = 0.15

// StateClusters groups the reader's positive state rows into interest
// clusters: connected components under "category cosine above
// clusterSimThreshold". Rows that connect to nothing — one-off reads,
// uncategorized pages — come back separately as misc. Order is
// deterministic: rows stay newest-first within a cluster, clusters sort
// by their newest row, misc stays newest-first.
func StateClusters(state []StateArticle) (clusters [][]string, misc []string) {
	type row struct {
		title string
		set   map[string]bool
		cats  []string
	}
	var rows []row
	for _, s := range state {
		// Structural mechanism: uses binary state to avoid the feedback
		// loop. Engagement only modulates voice in the scoring loop.
		if s.State == StateSkipped {
			continue
		}
		set := make(map[string]bool, len(s.Categories))
		for _, c := range s.Categories {
			set[c] = true
		}
		rows = append(rows, row{s.Title, set, s.Categories})
	}
	parent := make([]int, len(rows))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			if cosine(rows[i].set, rows[j].cats) > clusterSimThreshold {
				parent[find(i)] = find(j)
			}
		}
	}
	members := make(map[int][]string)
	var order []int // roots by newest member
	for i := range rows {
		r := find(i)
		if len(members[r]) == 0 {
			order = append(order, r)
		}
		members[r] = append(members[r], rows[i].title)
	}
	for _, r := range order {
		if len(members[r]) >= 2 {
			clusters = append(clusters, members[r])
		} else {
			misc = append(misc, members[r][0])
		}
	}
	return clusters, misc
}

// ClusteredRows is how much corroborated evidence a reader's state holds:
// the number of positive rows that sit inside StateClusters clusters. The
// adaptive exploration share keys on it, and diagnostic tooling reports
// the same number so the two can never drift apart.
func ClusteredRows(state []StateArticle) int {
	clusters, _ := StateClusters(state)
	n := 0
	for _, cl := range clusters {
		n += len(cl)
	}
	return n
}

// The decay floor is the voice the oldest interaction inside the interest
// window keeps relative to the newest. Interest counting weighs each state
// article by the exponential decay of its recency rank — 1.0 just read,
// this at the window's edge (recentStateLimit) — so the profile follows
// where the reader is heading rather than where they once binged, and a
// state sliding off the window's cliff loses a 0.3 voice, not a full one.
// Decay applies to category interest only: links are structural statements
// that two pages belong together, and slot rotation already gives every
// read an equal turn — returning to a topic refreshes its updated_at, so a
// kept-alive interest never fades.
// Adaptive decay floor: new readers get DecayFloorMin (interests must
// earn their place from evidence, not from being first adopted);
// established readers get DecayFloorMax (trust the pattern, let weaker
// interests persist). Ramps linearly by state size.
//
// Without adaptation, a fixed 0.7 killed cold-start (first adopted
// interest dominated forever) and a fixed 0.3 killed picky (sparse
// signals faded too fast). The adaptive floor gives each reader type
// the decay that serves them.
var (
	DecayFloorMin      = 0.3 // for new readers (0-5 state rows)
	DecayFloorMax      = 0.6 // for established readers (25+ rows)
	DecayFloorRampRows = 25  // rows where the ramp reaches the max
)

// effectiveDecayFloor computes the decay floor for a reader with the
// given number of state rows.
func effectiveDecayFloor(stateRows int) float64 {
	if DecayFloorMin == DecayFloorMax {
		return DecayFloorMin
	}
	t := min(1.0, float64(stateRows)/float64(DecayFloorRampRows))
	return DecayFloorMin + (DecayFloorMax-DecayFloorMin)*t
}

// Candidate is one article the ranking considers, with the graph data
// scoring reads: its outgoing links and its categories.
type Candidate struct {
	ArticleID  int64 // latest stored revision of the page
	PageID     int64
	Source     string
	Title      string
	Links      []string
	Categories []string
	Words      int // total word count; 0 = unknown (quality bonus skipped)
}

// StateArticle is an article the reader finished, saved or skipped, carrying
// the same graph data as a Candidate.
type StateArticle struct {
	PageID     int64
	Source     string
	Title      string
	State      string
	Completion float64 // 0–1: how far through the article the reader got
	Words      int     // total tokens in the article (for absolute engagement thresholds)
	Links      []string
	Categories []string
}

// Weights is the discriminative weight of each category the reader's state
// touches. See CategoryWeights.
type Weights map[string]float64

// CategoryWeights turns document frequencies (how many corpus articles carry
// a category) into weights: w = 1/sqrt(df). Sharing "Baroque composers"
// (dozens of articles) means something; sharing "1970s births" (thousands)
// means little. The curve used to be a deliberately steep 1/df, built to
// mute the blocks of thirty or forty correlated authority-control
// categories every well-sourced biography carried — but those are
// maintenance categories the wiki itself marks hidden, and since ingest
// filters them against the wiki's metadata (fetch-categories) they no
// longer exist in stored data; keeping the steep curve on top was a second
// mechanism for the same junk, and it muted categories as a ranking signal
// entirely (a rare shared category scored ~0.03 against a link's 1.0).
// Reopened at 1/sqrt, categories became a real second channel: measured,
// persona appeal rose across the board (cars 45→72%, adopter 35→64%) and
// the equal-love multi-interest reader's starved interest finally serves
// (astronomy 0→20) — while the canonical feeds kept their link-driven
// tops and MMR still breaks same-topic runs. The curve only guards
// honest popularity now; junk is the ingest filter's job.
func CategoryWeights(df map[string]int) Weights {
	w := make(Weights, len(df))
	for c, n := range df {
		if n < 1 {
			n = 1
		}
		w[c] = 1 / math.Sqrt(float64(n))
	}
	return w
}

// impressionDiscount computes a per-category multiplier that gently
// dampens categories the feed over-serves relative to the reader's
// engagement. The discount only applies to the EXCESS: if a category
// receives more impression share than its interaction share, the
// surplus is damped. Categories at or below their fair share are
// untouched (factor = 1.0).
//
// Example: a category with 15% impression share and 5% interaction
// share has 10% excess, giving factor = 1 - 0.10 * damping ≈ 0.80.
// A category with 20% impression share and 36% interaction share has
// no excess: factor = 1.0.
//
// This design avoids the juggler problem: a five-interest reader's
// less-clicked categories (real interests with fewer opportunities)
// get a mild ~20% discount instead of a harsh 65-90% one. The
// categories that genuinely dominate the feed without clicks are the
// ones that get pushed down.
func impressionDiscount(liked map[string]float64, impressions map[string]int) map[string]float64 {
	var totalImp float64
	for _, n := range impressions {
		totalImp += float64(n)
	}
	var totalLiked float64
	for _, n := range liked {
		totalLiked += n
	}
	if totalImp == 0 || totalLiked == 0 {
		return nil
	}
	// Graduated warmup: the discount ramps from zero to full strength
	// over ImpressionWarmup total impressions, so early noise gets a
	// gentle discount and the reader's stabilized pattern gets the full
	// one. A hard cutoff at 5000 meant the discount never activated in
	// practice (60 feeds × 20 cards = 1200 impressions). The ramp
	// avoids the winner-take-all loop that a full-strength discount on
	// noisy early data caused (juggler collapse, measured at damping=2).
	ramp := min(1.0, totalImp/ImpressionWarmup)
	if ramp < 0.01 {
		return nil
	}
	factors := make(map[string]float64, len(liked))
	for c := range liked {
		impShare := float64(impressions[c]) / totalImp
		if impShare < 1e-9 {
			continue // never shown — no discount
		}
		likedShare := liked[c] / totalLiked
		excess := impShare - likedShare
		if excess <= 0 {
			continue
		}
		f := 1 - excess*ImpressionDamping*ramp
		factors[c] = max(impressionDiscountFloor, f)
	}
	return factors
}

// impressionDamping controls how strongly over-served categories are
// discounted. At 1.0, a category with 10% excess impression share
// gets factor ≈ 0.90 — a nudge, not a shove. Higher values (2.0+)
// were measured to cause a collapse feedback loop in multi-interest
// readers: the discount reduces serving → reduces clicks → reduces
// CTR → increases the discount further, until one interest cannibalizes
// the rest (the juggler's music dropped from 143 to 11 interactions
// at damping=2.0). Gentle damping avoids the spiral while still
// signaling genuinely over-served categories to the ranker.
var ImpressionDamping = 1.0

// impressionDiscountFloor prevents a category from being discounted
// below this fraction even at extreme over-serving. At 0.5 no category
// loses more than half its raw interest — enough to rebalance gently,
// too little to starve a real interest out of the feed.
const impressionDiscountFloor = 0.5

// ImpressionWarmup is the total impression count where the graduated
// discount reaches full strength. The discount ramps linearly from zero:
// at 100 impressions (5 feeds) it's at 50% strength, at 200 (10 feeds)
// full. This replaces the old hard cutoff at 5000 that never activated
// in practice (60 feeds × 20 cards = 1200 max).
var ImpressionWarmup = 200.0

// Engagement thresholds. Exported so the feedsim can sweep them.
var (
	// EngageThreshold: completion fraction for zone 1→2 (bounce→interest).
	// Swept 0.10–0.40 in the feedsim fast benchmark; 0.20 maximizes
	// the average across mono/juggler/gundam/picky (64.1% avg appeal).
	EngageThreshold = 0.20
	// EngageTokens: absolute token count for zone 1→2.
	EngageTokens = 200
	// DeepThreshold: completion fraction for zone 2→3 (interest→deep).
	DeepThreshold = 0.80
	// DeepTokens: absolute tokens for zone 2→3.
	DeepTokens = 500
	// DeepBoost: extra weight above 1.0 for deep reads (superlinear).
	DeepBoost = 0.0
	// EngageOff disables completion-driven engagement: finished/saved → +1,
	// skipped → -1. When off (the default), the engagement model uses
	// continuous completion for positive signal (95% > 50%) with mean-
	// normalization to prevent topic-length bias from compounding, and
	// keeps the skip penalty at -1.0 (a skip is a skip). Toggle with
	// -engage-off in feedsim to compare against the binary baseline.
	EngageOff = false
	// TokenSaturation: absolute tokens where token normalization tops out.
	// Reading this many tokens gives full credit; fewer gives sqrt-scaled.
	TokenSaturation = 500.0
)

// engagementWeight converts completion, article size and state into a
// signed, token-normalized weight.
//
// Three zones, no cliff edges:
//  1. Bounce (below BOTH low thresholds): negative, scales with how
//     early the reader left. Reading 5% of a stub is full-force negative.
//  2. Interest (above EITHER low threshold): positive, moderate. The
//     reader spent real attention — either a meaningful fraction of the
//     article or a meaningful number of tokens.
//  3. Deep (above BOTH high thresholds): superlinear boost. The reader
//     consumed most of a substantial article — strong evidence.
//
// Token normalization: the weight is scaled by how many actual tokens
// were consumed, so 100% of a 60-word stub counts less than 50% of a
// 3000-word article. The scale saturates at TokenSaturation tokens.
func engagementWeight(completion float64, words int, state string) float64 {
	if state == StateSaved {
		// Saving means "come back to this later" — intent to engage, not
		// confirmed deep engagement. A moderate positive: stronger than a
		// bounce, weaker than a deep read. If completion data is available
		// (the reader read some before saving), use that; otherwise assume
		// moderate interest (0.7 — roughly "read some, liked enough to
		// bookmark").
		if completion <= 0 {
			completion = 0.7
		}
		// Fall through to the normal positive formula rather than
		// returning a flat 1.0 — the reader hasn't proven they love the
		// topic, just that it caught their eye.
	}
	if EngageOff {
		if state == StateSkipped {
			return -1.0
		}
		return 1.0
	}
	tokensRead := completion * float64(words)
	if state == StateSkipped {
		// Nuanced skip: both completion AND absolute tokens matter.
		// A 0% skip (instant bounce) = total rejection (-1.0).
		// A 10% skip on a short article (60 tokens) = barely tried (-0.85).
		// A 30% skip on a long article (3000 tokens read) = gave it a
		// real try, mild negative (-0.21). Reading 3000 words before
		// deciding is 10 minutes of active attention — that's not
		// rejection of the topic, just this article.
		pctFactor := 1 - completion // 0% → 1.0, 30% → 0.7
		// More tokens read → less negative (they genuinely tried)
		tokenDampen := min(0.7, tokensRead/1000)
		return -pctFactor * (1 - tokenDampen)
	}
	// Positive engagement: completion × token scale. 95% of a long
	// article (9500 tokens) is much stronger evidence than 95% of a
	// stub (57 tokens). The completion sets the base in [0.5, 1.0];
	// the token factor scales it by how much actual reading happened,
	// saturating at TokenSaturation tokens.
	//
	// Examples at TokenSaturation=500:
	//   95% of 1000w (950 tokens): 0.975 × 1.0  = 0.975 (strong love)
	//   60% of 1000w (600 tokens): 0.80  × 1.0  = 0.80  (solid interest)
	//   95% of 100w  (95 tokens):  0.975 × 0.72 = 0.70  (stub, less signal)
	//   30% of 100w  (30 tokens):  0.65  × 0.55 = 0.36  (minimal reading)
	pctWeight := 0.5 + 0.5*completion
	tokenScale := min(1.0, math.Sqrt(tokensRead/TokenSaturation))
	// Blend: 60% completion signal, 40% token signal. Completion
	// dominates because it's the reader's proportion of attention;
	// tokens prevent stubs from counting as much as real reads.
	return pctWeight * (0.6 + 0.4*tokenScale)
}

// Scored is one ranked candidate with the relevance score that placed it
// and the state articles (owners, by title) whose links or categories
// produced that score — selection rotates the feed's slots across the
// reader's state, and a slot can only be filled by a candidate its owner
// produced.
type Scored struct {
	Candidate
	Score  float64
	Owners []string
}

// RankScored scores candidates against the reader's state: a direct link to
// or from a finished or saved article counts linkWeight, shared categories
// count their weight (per state article sharing them), and the categories
// of skipped articles count against. State is ordered newest interaction
// first (as readerState loads it); category interest decays with that rank
// (see effectiveDecayFloor). No acted-on page comes back (see suppressedPages);
// a saved page keeps promoting its neighbourhood, not itself. The order is
// deterministic: score, then source and page ID.
//
// impressions (optional, nil OK) is the per-category count of how many
// times articles in each category have appeared in the reader's feeds.
// When present, category interest is discounted for categories the reader
// sees often but never clicks — impression-normalized interest.
func RankScored(candidates []Candidate, state []StateArticle, w Weights, impressions map[string]int) []Scored {
	suppressed := suppressedPages(state)
	// Every structure remembers which state article (owner) it came from,
	// so each candidate can say whose reading it serves.
	neighbour := make(map[titleKey][]string) // titles linked from finished or saved
	nomination := make(map[string]float64)   // nominating strength per state title
	stateTitle := make(map[titleKey]string)
	catOwners := make(map[string][]string)
	liked := make(map[string]float64)
	disliked := make(map[string]float64)
	decayFloor := effectiveDecayFloor(len(state))
	decay := math.Log(decayFloor) / float64(max(recentStateLimit, 1))
	// Mean-normalized engagement: compute the reader's average positive
	// engagement, then express each row's engagement relative to that
	// average. This breaks the feedback loop where topics with
	// systematically higher completion (longer articles, easier content)
	// compound into voice dominance over 600+ articles. A reader who
	// averages 80% completion: a 95% read is 1.19× average (slight
	// boost), a 60% read is 0.75× (slight dampen). The sqrt saturation
	// on top means even a 2:1 normalized advantage saturates quickly.
	// Only applied when engagement is enabled (EngageOff=false).
	meanEng := 1.0
	if !EngageOff {
		var engSum float64
		var engN int
		for _, s := range state {
			e := engagementWeight(s.Completion, s.Words, s.State)
			if e > 0 {
				engSum += e
				engN++
			}
		}
		if engN > 0 {
			meanEng = engSum / float64(engN)
		}
	}
	// InterestFade: compute per-title fade factors from cluster engagement
	// trending. A cluster with recent activity keeps full voice; one that
	// stopped appearing in the recent window fades toward InterestFadeFloor.
	titleFade := make(map[string]float64)
	if InterestFade {
		cls, _ := StateClusters(state)
		// Build a set of titles in the recent window.
		recentTitles := make(map[string]bool)
		for i, s := range state {
			if i >= InterestFadeWindow {
				break
			}
			if s.State != StateSkipped {
				recentTitles[s.Title] = true
			}
		}
		for _, cl := range cls {
			// Activity ratio: what fraction of this cluster's members
			// have appeared in the recent window. A cluster with 20
			// members and 0 in recent is fully faded; 5 members and
			// 2 in recent (0.4) is moderately active.
			recent := 0
			for _, t := range cl {
				if recentTitles[t] {
					recent++
				}
			}
			ratio := float64(recent) / float64(len(cl))
			// The fade curve: sqrt gives a gentle ramp — a cluster
			// with 25% of members recent keeps 50% voice (not 25%).
			// A fully active cluster (ratio=1) keeps 100%.
			fade := InterestFadeFloor + (1-InterestFadeFloor)*math.Sqrt(ratio)
			for _, t := range cl {
				titleFade[t] = fade
			}
		}
	}
	for i, s := range state {
		voice := math.Exp(decay * float64(i))

		// Completion is the primary engagement signal: how much of the
		// article the reader actively consumed (RSVP = thumb down = honest
		// attention). The engageThreshold is where the signal flips from
		// negative (bounced) to positive (engaged). Saved articles are
		// always positive regardless of completion.
		eng := engagementWeight(s.Completion, s.Words, s.State)
		if eng < 0 {
			for _, c := range s.Categories {
				disliked[c] += voice * (-eng)
			}
			continue
		}
		// Normalize positive engagement relative to the reader's mean:
		// above-average reads still boost, below-average dampen, but the
		// absolute level that caused compounding is removed.
		if !EngageOff && meanEng > 0 {
			eng = max(0.2, eng/meanEng)
		}
		voice *= eng
		// InterestFade: attenuate voice by the cluster's recent-activity
		// ratio so abandoned interests lose weight while still-active
		// sparse interests keep theirs.
		if fade, ok := titleFade[s.Title]; ok {
			voice *= fade
		}
		stateTitle[titleKey{s.Source, s.Title}] = s.Title
		// A nomination's strength is the nominating page's specificity:
		// full inside the grace region (nominationGrace links), decaying
		// as grace/degree beyond it — the state-side mirror of the
		// candidate-side density rule below. A link from an ordinary
		// article is an editorial statement; a link from a 227-link year
		// page is a calendar entry. Measured: one finished year page
		// nominated 4 of 15 relevance cards (people born that year, a car
		// launched that year) at the flat weight; diluted, a page with
		// more than nominationGrace/RelevanceFloor ≈ 167 links cannot
		// float a candidate past the relevance floor on its own. (A
		// steeper sqrt curve also halved ordinary 60-120-link pages and
		// starved the small-state picky reader — appeal 27→16%.)
		strength := min(1, nominationGrace/float64(max(len(s.Links), 1)))
		// Fade nominations from abandoned clusters too — this is the
		// targeted version of what TimedDecay did globally.
		if fade, ok := titleFade[s.Title]; ok {
			strength *= fade
		}
		nomination[s.Title] = strength
		for _, l := range s.Links {
			k := titleKey{s.Source, l}
			neighbour[k] = append(neighbour[k], s.Title)
		}
		for _, c := range s.Categories {
			liked[c] += voice
			catOwners[c] = append(catOwners[c], s.Title)
		}
	}
	// Interest in a category saturates with the square root of how often
	// the reader touched it (each touch at its recency voice). Linear
	// counting let one binge topic outvote
	// every other interest by its raw volume — four hundred computing
	// finishes next to twenty football ones made feeds 95% computing (a
	// simulated two-interest reader measured it); with the square root the
	// same history argues 4.5:1, which selection then mixes. Skips pull
	// the same way downward.
	//
	// Impression normalization (when impression counts are available):
	// a category the reader sees proportionally more than they click is
	// weaker evidence than one they click proportionally to how often
	// they see it. The discount compares each category's impression
	// share to its interaction share: a category that gets 30% of all
	// impressions but only 10% of interactions is over-served and
	// discounted by 10/30 ≈ 0.33; one at 10%/10% gets no discount.
	// The floor of 0.1 prevents a zero-interaction category from being
	// completely silenced (it may simply need more exposure). Categories
	// at or below their fair share are not boosted (capped at 1.0).
	interest := make(map[string]float64, len(liked)+len(disliked))
	var impFactor map[string]float64
	if impressions != nil {
		impFactor = impressionDiscount(liked, impressions)
	}
	for c, n := range liked {
		v := math.Sqrt(n) * w[c]
		if f, ok := impFactor[c]; ok {
			v *= f
		}
		interest[c] = v
	}
	for c, n := range disliked {
		interest[c] -= math.Sqrt(n) * w[c]
	}

	// Precompute median word count for the quality bonus. Only
	// candidates with Words > 0 participate; the rest are unaffected.
	var medianWords float64
	if QualityBonus > 0 {
		var ww []int
		for _, c := range candidates {
			if c.Words > 0 {
				ww = append(ww, c.Words)
			}
		}
		if len(ww) > 0 {
			slices.Sort(ww)
			mid := len(ww) / 2
			if len(ww)%2 == 0 {
				medianWords = float64(ww[mid-1]+ww[mid]) / 2
			} else {
				medianWords = float64(ww[mid])
			}
		}
	}

	ranked := make([]Scored, 0, len(candidates))
	for _, c := range candidates {
		if suppressed[pageKey{c.Source, c.PageID}] {
			continue
		}
		score := 0.0
		owners := make(map[string]bool)
		// An incoming nomination scores once, at the strength of the most
		// specific page that made it — several nominators never stacked
		// before (the weight was flat) and still don't. The candidate's own
		// hubness deliberately does NOT dilute it: a sparse page's link to
		// a hub can be a fine door (a saved car's market, China), and no
		// cheap structural signal separates a calendar hub from a legit
		// dense page — candidate out-degree demoted Metaphysics under
		// Japan (a fixture test caught it) and words-per-link rates Queens
		// below the 1975 year page. The stray year card this permits is
		// bounded to its nominator's rotation turn.
		nominated := 0.0
		for _, t := range neighbour[titleKey{c.Source, c.Title}] {
			owners[t] = true
			nominated = max(nominated, nomination[t])
		}
		score += linkWeight * nominated
		// Outgoing links score by density, not volume: the share of the
		// candidate's links that point at the reader's reads. A year page
		// or "List of …" links to thousands of titles, so for any
		// long-term reader it matches dozens of them — unnormalized, hubs
		// owned every feed once the history grew past ~500 rows (measured).
		// Density is also what the statistics say: for one reader, the
		// surprise of m matches out of d links (its hypergeometric lift)
		// orders candidates exactly by m/d. The saturation cap makes a
		// small article that devotes a tenth or more of its links to the
		// reader's reads count as fully linked. Each match counts at the
		// matched page's specificity: linking to a hub read (a year page,
		// a country) is not the affinity statement linking to a 30-link
		// biography is — unweighted, any sparse article mentioning a
		// finished year page scored a saturated 1.0 (measured on the
		// Cincinnati Reds via "1975").
		matches := 0.0
		for _, l := range c.Links {
			if t, ok := stateTitle[titleKey{c.Source, l}]; ok {
				matches += nomination[t]
				owners[t] = true
			}
		}
		if matches > 0 {
			density := matches / float64(len(c.Links))
			score += linkWeight * min(1, outgoingSaturation*density)
		}
		for _, cat := range c.Categories {
			score += interest[cat]
			if interest[cat] > 0 {
				for _, t := range catOwners[cat] {
					owners[t] = true
				}
			}
		}
		list := make([]string, 0, len(owners))
		for t := range owners {
			list = append(list, t)
		}
		slices.Sort(list)
		// Quality bonus: longer articles get a mild score boost as a
		// tiebreaker. The multiplier is 1 ± QualityBonus, scaled by
		// log2(words/median) clamped to [-1, 1].
		if QualityBonus > 0 && c.Words > 0 && medianWords > 0 {
			ratio := math.Log2(float64(c.Words) / medianWords)
			ratio = max(-1, min(1, ratio))
			score *= 1 + QualityBonus*ratio
		}
		ranked = append(ranked, Scored{c, score, list})
	}
	slices.SortFunc(ranked, compareScored)
	return ranked
}

func compareScored(a, b Scored) int {
	if a.Score != b.Score {
		return cmp.Compare(b.Score, a.Score)
	}
	if a.Source != b.Source {
		return cmp.Compare(a.Source, b.Source)
	}
	return cmp.Compare(a.PageID, b.PageID)
}

// Reason explains why a card was recommended. Produced at pick time in
// SelectDiverse, never reconstructed — the feed's ordering depends on the
// day seed, hour and state window, so a reason computed later would disagree
// with the actual selection.
type Reason struct {
	Kind           string   `json:"kind"`                      // "relevance", "exploration", "momentum"
	SeedTitle      string   `json:"seed_title,omitempty"`      // state article whose rotation turn produced this pick
	SeedCategories []string `json:"seed_categories,omitempty"` // top shared categories between seed and pick
}

// Picked is one chosen feed entry: its final score after noise and
// similarity penalties, and whether it filled an exploration slot.
type Picked struct {
	Candidate
	Score       float64
	Exploration bool
	Reason      Reason
}

// SelectDiverse fills k feed slots from the scored pool, keeping the feed
// varied three ways:
//
//   - Seeded jitter: every score is multiplied by 1 ± scoreJitter, so
//     near-ties land in a different order for every reader and every day
//     while clear favourites stay on top.
//   - Similarity penalty (greedy MMR): each pick is the best remaining
//     score minus SimilarityPenalty times its category-set cosine with the
//     most similar article already picked. A run of composers (or of
//     anything else that shares most of its categories) prices the next
//     one out, letting the rest of the reader's neighbourhood through.
//   - Slot rotation: relevance slots belong to the reader's state
//     articles in a seeded rotation, and a slot is filled by the best
//     candidate its owner produced. Every read gets a voice regardless of
//     its graph degree — a country with three thousand neighbours and a
//     three-link car article command the same share — and a one-off
//     accidental read is bounded to its turn in the rotation. When an
//     owner has nothing left above the relevance floor, its turn passes;
//     when no owner has anything, the slot falls back to the global best,
//     then to exploration.
//   - Momentum slots: the first relevance slot and the one past mid-feed
//     go to the newest reads (in recency order) instead of the rotation,
//     so the last thing the reader finished steers the very next feed.
//   - Exploration slots: an adaptive share of slots (see
//     ExploreShareBase) is filled from the
//     explore pool (in its given order, already shuffled by the caller) —
//     articles from outside the reader's neighbourhood entirely.
//
// state must be the same slice RankScored saw (its order, newest reads
// first, drives the momentum slots and the interest decay). Candidates and
// explore entries are deduplicated by page; results are deterministic for a
// given seed.
func SelectDiverse(scored []Scored, explore []Candidate, state []StateArticle, k int, seed int64) []Picked {
	rng := rand.New(rand.NewPCG(uint64(seed), 0x77696b69)) // "wiki"
	freshCap := AdaptiveFreshCap(state)
	// Window the state for rotation clusters: only recent interactions
	// get turns. An interest abandoned many feeds ago falls off rather
	// than holding turns forever. The full state still drives scoring
	// (RankScored sees it all); only the rotation and explore share
	// reflect the window.
	clusterState := state
	if ClusterWindow > 0 && len(clusterState) > ClusterWindow {
		clusterState = clusterState[:ClusterWindow]
	}
	clusters, _ := StateClusters(clusterState)
	// Dynamic similarity penalty: a reader with one deep interest needs
	// a lighter penalty (similar articles ARE what they want), while a
	// multi-interest reader needs heavier penalty to force diversity
	// across their interests. Keyed on cluster count: 1 cluster → 0.4
	// (let similar through), 4+ clusters → 0.8 (force diversity).
	simPenalty := SimilarityPenalty
	if DynamicSimPenalty {
		nc := len(clusters)
		switch {
		case nc == 0:
			simPenalty = SimilarityPenalty // cold-start: use baseline, needs diversity in exploration
		case nc == 1:
			simPenalty = 0.4 // single deep interest: let similar articles through
		case nc == 2:
			simPenalty = 0.5
		case nc == 3:
			simPenalty = 0.65
		default:
			simPenalty = 0.8 // 4+ interests: force diversity
		}
	}
	// The incremental MMR state per remaining candidate: its category set
	// (built once) and its running similarity to the closest pick so far,
	// updated against only the newest pick each round.
	type entry struct {
		Scored
		raw    float64 // relevance before jitter: what the floor gates on
		cats   map[string]bool
		maxSim float64
		picked bool
	}
	remaining := make([]entry, len(scored))
	for i, s := range scored {
		raw := s.Score
		s.Score *= 1 + scoreJitter*(2*rng.Float64()-1)
		cats := make(map[string]bool, len(s.Categories))
		for _, c := range s.Categories {
			cats[c] = true
		}
		remaining[i] = entry{Scored: s, raw: raw, cats: cats}
	}

	// The rotation: turns rotate across INTEREST CLUSTERS of the reader's
	// positive state (category-cosine connected components, StateClusters),
	// not across individual rows. Row-level rotation made turn share
	// proportional to row count, so engagement volume compounded: a
	// five-interest reader's dominant topic took 300 of 600 serves while
	// another served 2, and a reader who moved on watched the new interest's
	// share decay (59→21→17→7% across four scripted hole switches) as every
	// retired topic's rows kept their turns. A cluster gets one turn per
	// cycle whatever its row count — 300 computing reads and 5 music reads
	// argue as equals — and a cluster's turn takes the best candidate any
	// of its rows produced. Singleton rows (one-off reads, uncategorized
	// pages) hold no turn: rotation is for corroborated interests. A shared
	// misc turn for the strays was measured to crater the single-interest
	// reader — one cluster plus strays alternates cluster/misc, so junk
	// neighbourhoods took near half the feed (cars appeal 80→44, repeats
	// 9→20). A stray still shapes scores, and the moment it is read it IS
	// the newest read, so momentum serves its neighbourhood on the very
	// next feed — a second bite forms a two-row cluster and earns a turn.
	// recent keeps the raw order (newest read first) for the momentum slots.
	var recent []string
	ownerCandidates := make(map[string][]int)
	for _, st := range state {
		if st.State != StateSkipped {
			recent = append(recent, st.Title)
		}
	}
	for i, c := range remaining {
		for _, o := range c.Owners {
			ownerCandidates[o] = append(ownerCandidates[o], i)
		}
	}
	// Each cluster is one rotation turn. The turn anchors on a rotating
	// member row rather than merging every member's candidates into one
	// best-of: a merged turn re-served the same cluster-wide best cards
	// every fetch, and the low-engagement single-interest reader — whose
	// state barely changes between fetches — watched their feed freeze
	// (picky appeal 45→29, repeats 22→37 measured). Anchoring restores the
	// old row rotation's depth sampling inside the cluster's single turn.
	type turnGroup struct {
		members []string
		next    int
	}
	// When ClusterRecencyBoost > 1, recent clusters get proportionally
	// more rotation turns. Each cluster's recency is the position of its
	// newest member in state (newest-first). The boost is geometric:
	// the newest cluster gets ClusterRecencyBoost turns for every 1 turn
	// the oldest gets, with intermediate clusters interpolated.
	stateRank := make(map[string]int)
	for i, s := range state {
		if s.State != StateSkipped {
			stateRank[s.Title] = i
		}
	}
	clusterRecency := func(cl []string) int {
		best := len(state)
		for _, t := range cl {
			if r, ok := stateRank[t]; ok && r < best {
				best = r
			}
		}
		return best
	}
	// Sort clusters by recency (newest member first) for deterministic
	// weighted rotation.
	type rankedCluster struct {
		cluster []string
		rank    int
	}
	ranked := make([]rankedCluster, len(clusters))
	for i, cl := range clusters {
		ranked[i] = rankedCluster{cl, clusterRecency(cl)}
	}
	slices.SortFunc(ranked, func(a, b rankedCluster) int {
		return cmp.Compare(a.rank, b.rank)
	})
	groups := make([]*turnGroup, 0, len(clusters)*2)
	if ClusterRecencyBoost <= 1 || len(ranked) <= 1 {
		for _, rc := range ranked {
			groups = append(groups, &turnGroup{members: rc.cluster})
		}
	} else {
		// Geometric interpolation: newest gets boost turns, oldest gets 1.
		for i, rc := range ranked {
			t := float64(i) / float64(max(1, len(ranked)-1))
			weight := math.Pow(ClusterRecencyBoost, 1-t)
			repeats := max(1, int(math.Round(weight)))
			g := &turnGroup{members: rc.cluster}
			for range repeats {
				groups = append(groups, g)
			}
		}
	}
	rng.Shuffle(len(groups), func(i, j int) { groups[i], groups[j] = groups[j], groups[i] })
	turn := 0
	clustered := 0
	for _, cl := range clusters {
		clustered += len(cl) // = ClusteredRows, on the clusters already built
	}
	exploreShare := ExploreShareBase +
		(ExploreShareMax-ExploreShareBase)*max(0, 1-float64(clustered)/float64(ExploreRampRows))
	// Skip-rate-reactive exploration: when the reader's recent
	// interactions are mostly skips, the relevance pool isn't serving
	// what they want — boost exploration to cycle through different
	// topics faster. This makes the feed adapt to the reader's
	// disengagement rather than waiting for time-based cluster growth.
	if len(state) > 5 {
		recentSkips, recentPositive := 0, 0
		for _, s := range state[:min(len(state), 30)] {
			if s.State == StateSkipped {
				recentSkips++
			} else {
				recentPositive++
			}
		}
		skipRate := float64(recentSkips) / float64(recentSkips+recentPositive)
		if skipRate > ExploreSkipThreshold {
			boost := (skipRate - ExploreSkipThreshold) * ExploreSkipBoost
			exploreShare = min(ExploreShareMax, exploreShare+boost)
		}
	}

	picked := make([]Picked, 0, k)
	seen := make(map[pageKey]bool)
	// Intra-exploration diversity: track categories of exploration picks
	// so subsequent exploration cards cover genuinely different territory.
	// Without this, a cold-start reader's exploration could draw 5 animal
	// articles (the corpus's largest cluster) and never reach computing.
	// The filter prefers diverse picks but falls back to the next eligible
	// candidate when no diverse one is available (the pool must not run dry).
	var exploredCats []map[string]bool
	takeExplore := func(c Candidate) {
		seen[pageKey{c.Source, c.PageID}] = true
		picked = append(picked, Picked{Candidate: c, Exploration: true, Reason: Reason{Kind: "exploration"}})
		if len(c.Categories) > 0 {
			cSet := make(map[string]bool, len(c.Categories))
			for _, cat := range c.Categories {
				cSet[cat] = true
			}
			exploredCats = append(exploredCats, cSet)
		}
		for i := range remaining {
			if !remaining[i].picked {
				remaining[i].maxSim = max(remaining[i].maxSim, cosine(remaining[i].cats, c.Categories))
			}
		}
	}
	pickExplore := func() bool {
		bestFallback := -1
		for i := 0; i < len(explore); i++ {
			c := explore[i]
			if seen[pageKey{c.Source, c.PageID}] {
				continue
			}
			tooSimilar := false
			if len(exploredCats) > 0 && len(c.Categories) > 0 {
				cSet := make(map[string]bool, len(c.Categories))
				for _, cat := range c.Categories {
					cSet[cat] = true
				}
				for _, prev := range exploredCats {
					if cosine(cSet, catSlice(prev)) > ExploreSimCap {
						tooSimilar = true
						break
					}
				}
			}
			if tooSimilar {
				if bestFallback < 0 {
					bestFallback = i
				}
				continue
			}
			explore = append(explore[:i], explore[i+1:]...)
			takeExplore(c)
			return true
		}
		if bestFallback >= 0 {
			c := explore[bestFallback]
			explore = append(explore[:bestFallback], explore[bestFallback+1:]...)
			takeExplore(c)
			return true
		}
		return false
	}

	// bestOf picks the strongest eligible candidate among indices (or all
	// of remaining when indices is nil): raw score at or above the floor to
	// hold a relevance slot — the similarity penalty merely reorders — with
	// the penalized score deciding.
	// bestOf picks an owner's strongest eligible candidate. fresh turns
	// additionally refuse candidates too similar to this feed's picks
	// (maxSim past FreshSimCap): a topic the feed already shows yields its
	// turn to someone else's. The caller retries every turn without the
	// cap when no owner has anything fresh, so a reader with one deep love
	// (every candidate alike) gets exactly the old behavior — the cap
	// redistributes slots only when another interest is waiting.
	// Sub-interest diversity: track the categories of the last pick from
	// each cluster group so rotation turns prefer different sub-topics.
	lastClusterCats := make(map[*turnGroup]map[string]bool)

	bestOf := func(indices []int, fresh bool, minRaw float64, diverseFrom map[string]bool) int {
		best, bestScore := -1, math.Inf(-1)
		consider := func(i int) {
			c := remaining[i]
			// The floor gates on the unjittered score: eligibility is the
			// stable line RelevanceFloor documents (and feedsim's pool
			// diagnostics measure); jitter only reorders the eligible.
			if c.picked || seen[pageKey{c.Source, c.PageID}] || c.raw < minRaw {
				return
			}
			if fresh && c.maxSim > freshCap {
				return
			}
			score := c.Score - simPenalty*c.maxSim
			// Sub-interest diversity: penalize candidates that share
			// sub-topics with the last pick from this cluster.
			if SubInterestDiversity > 0 && diverseFrom != nil {
				score -= SubInterestDiversity * cosine(c.cats, catSlice(diverseFrom))
			}
			if score > bestScore {
				best, bestScore = i, score
			}
		}
		if indices == nil {
			for i := range remaining {
				consider(i)
			}
		} else {
			for _, i := range indices {
				consider(i)
			}
		}
		return best
	}
	take := func(i int, reason Reason) {
		c := &remaining[i]
		c.picked = true
		seen[pageKey{c.Source, c.PageID}] = true
		picked = append(picked, Picked{Candidate: c.Candidate, Score: c.Score - simPenalty*c.maxSim, Reason: reason})
		for j := range remaining {
			if !remaining[j].picked {
				remaining[j].maxSim = max(remaining[j].maxSim, cosine(remaining[j].cats, c.Candidate.Categories))
			}
		}
	}

	// seedReason builds a Reason for a relevance or momentum pick,
	// recording which state article's rotation turn produced it and the
	// top shared categories between seed and candidate.
	seedReason := func(kind, seedTitle string, seedCats []string, pickCats []string) Reason {
		shared := sharedCategories(seedCats, pickCats, 3)
		return Reason{Kind: kind, SeedTitle: seedTitle, SeedCategories: shared}
	}
	// stateByTitle indexes the state window for category lookups.
	stateIdx := make(map[string][]string, len(state))
	for _, s := range state {
		stateIdx[s.Title] = s.Categories
	}

	for len(picked) < k {
		// A slot is exploration when the running share crosses the target
		// (Bresenham): at the 0.25 base this is exactly every fourth slot.
		// The two momentum slots are never converted: they are how the
		// feed reacts to what was just read, and at a high share the door
		// cadence would land on k/2 and silence the mid-feed one — a
		// just-adopted interest lost half its service exactly when the
		// reader was signalling it.
		momentum := len(picked) == 0 || len(picked) == k/2
		if !momentum && int(float64(len(picked)+1)*exploreShare) > int(float64(len(picked))*exploreShare) && pickExplore() {
			continue
		}
		best := -1
		var pickedGroup *turnGroup // set when rotation produced the pick
		var seedTitle string       // state article whose turn produced this pick
		// Momentum slots: the first relevance slot and the one past
		// mid-feed belong to the newest reads in order, so what the reader
		// just finished visibly steers the next feed. Bounding momentum to
		// two slots keeps session continuity from becoming a rich-get-richer
		// loop — a recency-weighted rotation was measured to starve the
		// older, richer neighbourhoods (single-interest appeal 35%→27%,
		// churn 2→9); every other slot still rotates fairly.
		if momentum {
			for _, o := range recent {
				// Same nil guard as the rotation: a read with no candidates
				// passes its momentum chance instead of searching everyone.
				if len(ownerCandidates[o]) == 0 {
					continue
				}
				if best = bestOf(ownerCandidates[o], true, RelevanceFloor, nil); best >= 0 {
					seedTitle = o
					break
				}
			}
		}
		// Slot order, every step measured in the reader benchmark: fresh
		// rotation turns; then any fresh candidate regardless of whose
		// turn it is — the escape hatch that lets an interest with few
		// state rows jump the row-count queue (a five-interest reader's
		// worst-served interest went 2 of 600 without it; raising this
		// step's bar to admit only strong candidates re-collapsed the
		// balance, because a thin interest's nominations arrive
		// degree-diluted below any bar that still excludes filler); then
		// the same two without the freshness cap, so a reader whose every
		// candidate is alike gets their strong similar cards back rather
		// than nothing. The focused reader pays a few appeal points for
		// the balance (74→69 measured) — the doomscrolling product reads
		// across interests far more than it burrows, so that is the right
		// side of the trade.
		for _, fresh := range []bool{true, false} {
			for tried := 0; best < 0 && tried < len(groups); tried++ {
				g := groups[turn%len(groups)]
				turn++
				// One turn, rotating anchor: later anchors only fill in
				// when the pointed-at member has nothing eligible.
				for range g.members {
					o := g.members[g.next%len(g.members)]
					g.next++
					// A missing owner must pass the anchor, not fall through
					// to a global search: bestOf reads nil as "everyone",
					// and an owner map lookup returns exactly that nil.
					if len(ownerCandidates[o]) == 0 {
						continue
					}
					if best = bestOf(ownerCandidates[o], fresh, RelevanceFloor, lastClusterCats[g]); best >= 0 {
						pickedGroup = g
						seedTitle = o
						break
					}
				}
			}
			if best < 0 {
				best = bestOf(nil, fresh, RelevanceFloor, nil)
			}
			if best >= 0 {
				break
			}
		}
		if best < 0 {
			if !pickExplore() {
				break
			}
			continue
		}
		// Record the picked candidate's categories for sub-interest
		// diversity before take() marks it picked.
		if pickedGroup != nil && SubInterestDiversity > 0 {
			lastClusterCats[pickedGroup] = remaining[best].cats
		}
		kind := "relevance"
		if momentum && seedTitle != "" {
			kind = "momentum"
		}
		reason := seedReason(kind, seedTitle, stateIdx[seedTitle], remaining[best].Candidate.Categories)
		take(best, reason)
	}
	return picked
}

// cosine is the category-set cosine between a remaining candidate and one
// picked article: how much of the same kind of thing another pick would be.
// It deliberately uses raw category sets, not the 1/df weights — the
// authority-control block two biographies share is exactly what makes them
// "another famous person", which is the sameness being penalised.
// catSlice converts a category set to a slice for cosine.
func catSlice(cats map[string]bool) []string {
	s := make([]string, 0, len(cats))
	for c := range cats {
		s = append(s, c)
	}
	return s
}

// sharedCategories returns up to n categories present in both slices,
// preferring shorter (more specific) names. Used to build the "because
// you read X — topics in common" part of a recommendation reason.
func sharedCategories(a, b []string, n int) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := make(map[string]bool, len(a))
	for _, c := range a {
		set[c] = true
	}
	var shared []string
	for _, c := range b {
		if set[c] {
			shared = append(shared, c)
		}
	}
	slices.SortFunc(shared, func(a, b string) int {
		return cmp.Compare(len(a), len(b))
	})
	if len(shared) > n {
		shared = shared[:n]
	}
	return shared
}

func cosine(cats map[string]bool, picked []string) float64 {
	if len(cats) == 0 || len(picked) == 0 {
		return 0
	}
	shared := 0
	for _, c := range picked {
		if cats[c] {
			shared++
		}
	}
	return float64(shared) / math.Sqrt(float64(len(cats))*float64(len(picked)))
}

// DaySeed derives the selection seed from the reader and the day: one reader's
// feed is stable across refreshes within a day (no slot-machine feel),
// different between readers, and fresh again tomorrow. Everything that
// varies the feed (jitter, exploration, the featured shuffle) derives from
// this one value.
func DaySeed(readerID string, day string) int64 {
	h := fnv.New64a()
	h.Write([]byte(readerID))
	h.Write([]byte{0})
	h.Write([]byte(day))
	return int64(h.Sum64())
}

// suppressedPages is the one definition of which pages a reader's state
// keeps out of the feed — both the ranked pool and the exploration pool
// filter through it, so a page can never resurface via one path after the
// other dropped it. Every acted-on page suppresses: finished pages are
// done; skipped pages were refused, and an article the reader just waved
// away reappearing on the next refresh reads as the feed not listening;
// saved pages are shelved — the page overlaps its own categories, so left
// servable it pinned itself to the top of every feed (measured live), and
// a bookmark is the reader's to reopen, not a recommendation. States still
// shape the feed beyond their own page: finished and saved count toward
// interest, skips count against.
func suppressedPages(state []StateArticle) map[pageKey]bool {
	suppressed := make(map[pageKey]bool)
	for _, s := range state {
		suppressed[pageKey{s.Source, s.PageID}] = true
	}
	return suppressed
}

type pageKey struct {
	source string
	pageID int64
}

type titleKey struct {
	source string
	title  string
}
