// Command feedsim plays simulated readers against the feed on a real,
// imported corpus and scores what they consumed. It is the feed's wind
// tunnel: each persona has interests (matched against article categories)
// and consumes articles the way the app serves them — one at a time, drawn
// from the current ranked feed weighted toward its top (the reader never
// sees a list) — finishing what appeals, swiping past what does not, and
// sometimes adopting a latent interest when an exploration card reveals
// it. The summary says whether the feed would have kept that reader
// reading: a churn event is quitThreshold consecutive boring articles, a
// session death.
//
// It needs WIKIFYP_DATABASE_URL pointing at a corpus (the dev database,
// never the test suite) and writes reader state under the sim- prefix:
//
//	WIKIFYP_DATABASE_URL=postgres://… go run ./cmd/feedsim [-iterations 60] [-run cars] [-v]
package main

import (
	"cmp"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zedeus/wikifyp/server/internal/config"
	"github.com/zedeus/wikifyp/server/internal/event"
	"github.com/zedeus/wikifyp/server/internal/feed"
)

// interest is one thing a persona cares about, recognized by category name.
type interest struct {
	name  string
	match *regexp.Regexp
}

func in(name, pattern string) interest {
	return interest{name, regexp.MustCompile(`(?i)` + pattern)}
}

// The matchers are each persona's ground truth: written by reading the
// actual category inventory of every seed article's link neighbourhood
// (benchmark v2 survey), because the feed's first hole-entry serves are
// entity-specific pages ("Great conjunction": categories Jupiter, Saturn)
// whose categories carry no topic word. v1's topic-generic patterns were
// blind to exactly those cards, so phased personas measured hole-entry as
// 0% while the feed was serving the right doors at slot 0. Known accepted
// false positives: bare "football" admits American football; "wars?\b"
// admits Star Wars — both rare in practice and cheaper than the blindness.
var (
	cars       = in("cars", `automobiles|motorsport|racing drivers|vehicle|car companies|car manufactur|trucks|buses|sedans|sports cars|coupes|convertibles`)
	computing  = in("computing", `computer|computing|software|programming|operating system|internet|linux|unix|web browsers|desktop environments|word processors|laptops`)
	football   = in("football", `football|soccer|fifa|uefa|world cup|premier league|la liga|bundesliga|serie a|ligue 1|ballon d'or|^f[ck] |f\.c\.`)
	animals    = in("animals", `mammals|birds|reptiles|amphibians|fish|insects|dogs|cats|sharks|animals|poultry|livestock|rodents|primates|whales|species$|zoolog|panthera|bovids|carnivor|herbivor|iucn`)
	astronomy  = in("astronomy", `astro|planet|constellation|galax|comets|asteroid|moons|nebula|stars$|stellar|supernova|black holes|quasar|solar system|spacecraft|space agency|gas giants|supergiants|red giants|(white|red|brown) dwarfs|^(mercury|venus|earth|mars|jupiter|saturn|uranus|neptune|pluto|sun|moon)$`)
	presidents = in("presidents", `presidents of|vice presidents|first ladies`)
	history    = in("history", `history|wars?\b|battles|empires?|ancient|medieval|revolutions|archaeolog|dynasty|emperors|kings|queens|rulers|pharaoh|caliph|popes|byzantine|ottoman|persia|mesopotam|antiquity|crusade|viking|prehistor|monarch|middle ages|roman |romans$|former countries`)
	mecha      = in("mecha", `gundam|anime|manga|robots?$|robots? |science fiction`)
	movies     = in("movies", `movies|films?$|films |actors|actresses|movie director|cinema`)
	music      = in("music", `albums|singers|songs$|songs |musical groups|bands$|rappers|musicians|singles$|guitarists|pianists|drummers|ballads|grammy|record producers|music$`)
	dogs       = in("dogs", `dog breeds|dogs$|^dogs |terriers|spaniels|retrievers|hounds|sheepdogs|mastiffs`)
	fungi      = in("fungi", `fungi|mushroom`)
)

// persona is a simulated reader: what they start from, what appeals, and
// what they could come to love if the feed ever shows it to them.
type persona struct {
	name      string
	seeds     []string // simplewiki titles finished before the first feed
	interests []interest
	latent    []interest
	// phases script a reader whose tastes move: each phase replaces the
	// active interests for a stretch of fetches and finishes its seeds on
	// entry (a taste shift shows up as reading, not just swiping — without
	// seeds the feed has no new signal and the run only measures
	// exploration luck). The report adds one line per phase: how much of
	// its stretch served the now-active interests, how much still served
	// the previous phase's (residue), and the fetch the active interest
	// first reached the reader.
	phases []phase
	// pickiness scales the finish probability; 1 is the default reader.
	pickiness float64
	// finish and skip override the base action probabilities (0 keeps the
	// defaults, 0.75 and 0.15): finish is the chance an appealing card is
	// read, skip the chance an unappealing one is deliberately waved away
	// rather than swiped past.
	finish, skip float64
}

// phase is one stretch of a scripted reader's life.
type phase struct {
	fetches   int
	interests []interest
	seeds     []string
}

var personas = []persona{
	{name: "cars", seeds: []string{"Porsche 911", "Toyota Corolla"},
		interests: []interest{cars}, pickiness: 1},
	{name: "computers-football", seeds: []string{"Linux", "Lionel Messi"},
		interests: []interest{computing, football}, pickiness: 1},
	{name: "animals-astronomy-presidents", seeds: []string{"Lion", "Jupiter", "Abraham Lincoln"},
		interests: []interest{animals, astronomy, presidents}, pickiness: 1},
	{name: "history-broad", seeds: []string{"Roman Empire", "Middle Ages"},
		interests: []interest{history}, pickiness: 1},
	{name: "adopter", seeds: []string{"Porsche 911", "Toyota Corolla"},
		interests: []interest{cars}, latent: []interest{astronomy}, pickiness: 1},
	{name: "picky", seeds: []string{"Lion", "Penguin"},
		interests: []interest{animals}, pickiness: 0.35},
	// Discovery stress test: the profile starts all-mecha, and the other
	// two loves are tiny slices of the corpus (~0.08% dog breeds, ~0.03%
	// fungi) reachable only through exploration — how long until first
	// contact, and does the feed run with it after?
	{name: "gundam-dogs-fungi", seeds: []string{"Gundam", "Mobile Suit Gundam 00", "Anime", "Robot"},
		interests: []interest{mecha, dogs, fungi}, pickiness: 1},
	{name: "shifter", seeds: []string{"Porsche 911", "Toyota Corolla"},
		interests: []interest{cars}, pickiness: 1,
		phases: []phase{
			{fetches: 30, interests: []interest{cars}},
			{fetches: 30, interests: []interest{football}, seeds: []string{"Lionel Messi", "FC Bayern Munich"}},
		}},
	// Only cares about football, really: reads almost everything on topic,
	// deliberately skips what is not. Precision and rabbit-hole depth test.
	{name: "monomaniac-football", seeds: []string{"Lionel Messi", "FC Bayern Munich"},
		interests: []interest{football}, pickiness: 1, finish: 0.9, skip: 0.3},
	// Deep but fleeting: an obsession a day, self-seeded, four in a row.
	// Tests hole-entry latency, dive depth and the previous hole's residue.
	{name: "serial-rabbitholer", seeds: []string{"Porsche 911"},
		interests: []interest{cars}, pickiness: 1, finish: 0.85,
		phases: []phase{
			{fetches: 15, interests: []interest{cars}, seeds: []string{"Toyota Corolla"}},
			{fetches: 15, interests: []interest{astronomy}, seeds: []string{"Jupiter", "Saturn"}},
			{fetches: 15, interests: []interest{football}, seeds: []string{"Lionel Messi", "FC Bayern Munich"}},
			{fetches: 15, interests: []interest{history}, seeds: []string{"Roman Empire", "Middle Ages"}},
		}},
	// Five long-running interests at once: the juggling test.
	{name: "juggler-five", seeds: []string{"Porsche 911", "Lion", "Linux", "Elvis Presley", "Roman Empire"},
		interests: []interest{cars, animals, computing, music, history}, pickiness: 1},
	// v3 personas: the signal-poor readers the adaptive exploration work
	// is for. A brand-new reader: no history, no declared tastes, only
	// what they would love if ever shown it — measures discovery speed
	// from zero (quits before first adoption ARE the metric).
	{name: "cold-start", latent: []interest{animals, computing}, pickiness: 1},
	// A pattern is forming but thin: three reads of one interest.
	{name: "sparse-cars", seeds: []string{"Porsche 911", "Toyota Corolla", "Ford Mustang"},
		interests: []interest{cars}, pickiness: 1},
	// A dozen unrelated reads: history with no pattern at all. What they
	// would actually love is reachable only through discovery.
	{name: "scattered", seeds: []string{"Eiffel Tower", "Photosynthesis", "Jazz", "Mount Everest",
		"Sushi", "Basketball", "Leonardo da Vinci", "Volcano", "Bitcoin", "William Shakespeare",
		"Great Barrier Reef", "Chess"},
		latent: []interest{astronomy, dogs}, pickiness: 1},
}

// walkLab runs the random-walk configuration matrix — depth, edge types,
// weighting — against one reader's live state and scores each config's
// hits three ways: how many land on the reader's known interests (the
// "related" band: well above corpus random, below the 1-hop pool), how
// many are novel (absent from the 1-hop candidate pool), and how many are
// junk. Top titles print for eyeballing; numbers pick the integration.
func walkLab(ctx context.Context, db *sql.DB, reader string, behaviorSeed uint64) error {
	state, _, scored, err := feed.Explain(ctx, db, reader)
	if err != nil {
		return err
	}
	if len(state) == 0 {
		return fmt.Errorf("walklab: reader %q has no state", reader)
	}
	inPool := make(map[int64]bool, len(scored))
	for _, s := range scored {
		inPool[s.ArticleID] = true
	}
	var matchers []interest
	for _, p := range personas {
		if strings.Contains(reader, p.name) || strings.HasSuffix(reader, p.name) {
			matchers = append(matchers, p.interests...)
			matchers = append(matchers, p.latent...)
		}
	}
	if len(matchers) == 0 {
		// Unknown reader: characterize against every known interest.
		matchers = []interest{cars, computing, football, animals, astronomy, presidents, history, mecha, dogs, fungi}
	}

	fmt.Printf("walklab %s: %d state articles, %d in 1-hop pool\n", reader, len(state), len(scored))
	fmt.Printf("%-22s %5s %5s %7s %7s %5s  %s\n", "config", "hits", "uniq", "match%", "novel%", "junk%", "top hits")
	for _, steps := range []int{2, 3, 4} {
		for _, edges := range []string{"links", "cats", "mixed"} {
			for _, weighted := range []bool{false, true} {
				cfg := feed.WalkConfig{Steps: steps, Walks: 300,
					Links: edges != "cats", Cats: edges != "links", Weighted: weighted}
				hits, err := feed.Walk(ctx, db, state, cfg, int64(behaviorSeed)+int64(steps))
				if err != nil {
					return err
				}
				cats, err := walkCategories(ctx, db, hits)
				if err != nil {
					return err
				}
				total, match, novel, junk := 0, 0, 0, 0
				var top []string
				for _, h := range hits {
					total += h.Visits
					matched := len(matching(matchers, cats[h.ArticleID])) > 0
					if matched {
						match += h.Visits
					}
					if !inPool[h.ArticleID] {
						novel += h.Visits
					}
					if junkTitle.MatchString(h.Title) {
						junk += h.Visits
					}
					if len(top) < 7 {
						mark := ""
						if matched {
							mark = "*"
						}
						top = append(top, fmt.Sprintf("%s%s(%d)", mark, h.Title, h.Visits))
					}
				}
				name := fmt.Sprintf("n=%d %s", steps, edges)
				if weighted {
					name += " wgt"
				}
				if total == 0 {
					fmt.Printf("%-22s all walkers died\n", name)
					continue
				}
				fmt.Printf("%-22s %5d %5d %6.1f%% %6.1f%% %4.1f%%  %s\n",
					name, total, len(hits),
					100*float64(match)/float64(total),
					100*float64(novel)/float64(total),
					100*float64(junk)/float64(total),
					strings.Join(top, " "))
			}
		}
	}
	return nil
}

// walkCategories loads hit categories for interest matching.
func walkCategories(ctx context.Context, db *sql.DB, hits []feed.WalkHit) (map[int64][]string, error) {
	ids := make([]int64, len(hits))
	for i, h := range hits {
		ids[i] = h.ArticleID
	}
	rows, err := db.QueryContext(ctx,
		"SELECT article_id, category FROM article_categories WHERE article_id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]string)
	for rows.Next() {
		var id int64
		var c string
		if err := rows.Scan(&id, &c); err != nil {
			return nil, err
		}
		out[id] = append(out[id], c)
	}
	return out, rows.Err()
}

// capInfo dumps the coherence structure of a reader's state, the raw
// material of the adaptive satiation cap: the mean pairwise category cosine
// (what the cap keys on today) and, at several thresholds, the connected
// components of the same rows under "cosine above threshold" edges — a
// candidate replacement signal that would read a broad-but-single interest
// (lion~tiger~shark chain into one component) differently from several
// interests (disconnected clusters), which mean pairwise cosine cannot.
func capInfo(ctx context.Context, db *sql.DB, reader string) error {
	state, _, scored, err := feed.Explain(ctx, db, reader)
	if err != nil {
		return err
	}
	// Candidate co-ownership: how many scored candidates each pair of state
	// rows nominates jointly. Two reads of one interest recommend the same
	// neighbourhood even when their category sets are disjoint.
	coOwned := make(map[[2]string]int)
	owned := make(map[string]int)
	for _, c := range scored {
		for i, a := range c.Owners {
			owned[a]++
			for _, b := range c.Owners[i+1:] {
				k := [2]string{a, b}
				if b < a {
					k = [2]string{b, a}
				}
				coOwned[k]++
			}
		}
	}
	containment := func(a, b string) float64 {
		k := [2]string{a, b}
		if b < a {
			k = [2]string{b, a}
		}
		m := min(owned[a], owned[b])
		if m == 0 {
			return 0
		}
		return float64(coOwned[k]) / float64(m)
	}
	// Mirror adaptiveFreshCap's row selection: newest positive rows with
	// categories, at most 40.
	type row struct {
		title string
		set   map[string]bool
		cats  []string
		links map[string]bool
	}
	var rows []row
	for _, s := range state {
		if s.State == feed.StateSkipped || len(s.Categories) == 0 {
			continue
		}
		set := make(map[string]bool, len(s.Categories))
		for _, c := range s.Categories {
			set[c] = true
		}
		links := make(map[string]bool, len(s.Links))
		for _, l := range s.Links {
			links[l] = true
		}
		rows = append(rows, row{s.Title, set, s.Categories, links})
		if len(rows) == 40 {
			break
		}
	}
	if len(rows) < 2 {
		fmt.Printf("%s: %d usable state rows — cap falls back to the baseline\n", reader, len(rows))
		return nil
	}
	cos := func(a, b row) float64 {
		shared := 0
		for _, c := range b.cats {
			if a.set[c] {
				shared++
			}
		}
		return float64(shared) / math.Sqrt(float64(len(a.set))*float64(len(b.set)))
	}
	// linkCos is the co-citation cosine over outgoing link targets: two
	// reads about the same thing cite the same pages even when their
	// category sets are disjoint (computing subtopics fragment on
	// categories but share Microsoft/Windows/Internet links).
	linkCos := func(a, b row) float64 {
		if len(a.links) == 0 || len(b.links) == 0 {
			return 0
		}
		shared := 0
		for l := range b.links {
			if a.links[l] {
				shared++
			}
		}
		return float64(shared) / math.Sqrt(float64(len(a.links))*float64(len(b.links)))
	}
	sum, n := 0.0, 0
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			sum += cos(rows[i], rows[j])
			n++
		}
	}
	mean := sum / float64(n)
	fmt.Printf("%s: %d rows | mean pairwise cosine %.3f | current cap %.2f\n",
		reader, len(rows), mean, feed.AdaptiveFreshCap(state))
	// Full-window cluster structure: the evidence the adaptive exploration
	// share keys on (feed.ClusteredRows is the same number the feed uses).
	clusters, misc := feed.StateClusters(state)
	clustered := feed.ClusteredRows(state)
	fmt.Printf(" full window: %d positive rows | %d in %d clusters | %d singletons\n",
		clustered+len(misc), clustered, len(clusters), len(misc))
	for _, edges := range []string{"cats", "cats+links", "cats+coown"} {
		fmt.Printf(" edges: %s\n", edges)
		for _, threshold := range []float64{0.05, 0.1, 0.15, 0.2, 0.3} {
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
					connected := cos(rows[i], rows[j]) > threshold
					if !connected && edges == "cats+links" {
						connected = linkCos(rows[i], rows[j]) > threshold
					}
					if !connected && edges == "cats+coown" {
						connected = containment(rows[i].title, rows[j].title) > threshold
					}
					if connected {
						parent[find(i)] = find(j)
					}
				}
			}
			size := make(map[int]int)
			sample := make(map[int]string)
			for i := range rows {
				r := find(i)
				size[r]++
				if strings.Count(sample[r], "/") < 2 {
					if sample[r] != "" {
						sample[r] += "/"
					}
					sample[r] += rows[i].title
				}
			}
			type comp struct {
				size  int
				label string
			}
			var comps []comp
			neff := 0.0
			for r, s := range size {
				p := float64(s) / float64(len(rows))
				neff += p * p
				comps = append(comps, comp{s, sample[r]})
			}
			neff = 1 / neff
			slices.SortFunc(comps, func(a, b comp) int { return cmp.Compare(b.size, a.size) })
			var labels []string
			for _, c := range comps[:min(6, len(comps))] {
				labels = append(labels, fmt.Sprintf("%d·%s", c.size, c.label))
			}
			fmt.Printf("  t=%.2f: %2d components | neff %.2f | %s\n",
				threshold, len(size), neff, strings.Join(labels, " "))
		}
	}
	return nil
}

// junkTitle flags cards no reader opens on purpose: bare dates, years and
// death indexes. Plain "List of …" pages are deliberately not counted —
// on topic they can be a fine read (a telescope-type list for an
// astronomy reader).
var junkTitle = regexp.MustCompile(`^\d{1,4}s?( BC)?$|^Deaths in |^(January|February|March|April|May|June|July|August|September|October|November|December)( \d+)?$`)

func main() {
	iterations := flag.Int("iterations", 60, "feed fetches per persona")
	limit := flag.Int("limit", 20, "cards per feed fetch")
	draws := flag.Int("draws", 10, "articles consumed per fetch")
	run := flag.String("run", "", "run only this persona")
	seed := flag.Uint64("seed", 1, "behavior random seed")
	seedsFlag := flag.String("seeds", "", "comma-separated behavior seeds for -bench: runs each persona across all of them and prints a variance band (overrides -seed)")
	verbose := flag.Bool("v", false, "print feed transcripts at checkpoints")
	flag.IntVar(&stopAfter, "stopafter", 0, "halt a simulation after this many fetches, leaving the reader's state in the database for -explain/-capinfo inspection (0 = run to completion)")
	explainReader := flag.String("explain", "", "dump a reader's scored pool by interest instead of simulating")
	walkReader := flag.String("walklab", "", "run the random-walk config matrix against a reader's state instead of simulating")
	bench := flag.Bool("bench", false, "run the standard benchmark suite: every persona, fixed lengths, one scorecard")
	capReader := flag.String("capinfo", "", "dump a reader's state-coherence structure (satiation-cap diagnostics) instead of simulating")
	engThresh := flag.Float64("engage-threshold", 0.20, "completion fraction for positive engagement")
	engTokens := flag.Int("engage-tokens", 200, "absolute token count for positive engagement")
	impDamp := flag.Float64("imp-damping", 1.0, "impression excess-damping multiplier")
	impWarmup := flag.Float64("imp-warmup", 200, "total impression count before discount activates")
	simPenalty := flag.Float64("sim-penalty", 0.6, "MMR similarity penalty strength")
	decayFloor := flag.Float64("decay-floor", 0.3, "fixed decay floor (overrides adaptive when set)")
	decayMin := flag.Float64("decay-min", 0.3, "adaptive decay floor minimum (new readers)")
	decayMax := flag.Float64("decay-max", 0.6, "adaptive decay floor maximum (established readers)")
	exploreBase := flag.Float64("explore-base", 0.20, "exploration share for established readers")
	exploreMax := flag.Float64("explore-max", 0.75, "exploration share for new readers")
	capBase := flag.Float64("fresh-cap", 0.5, "baseline satiation cap")
	clusterBoost := flag.Float64("cluster-boost", 2.0, "recency boost for newest cluster rotation turns (1=equal, 3=3x)")
	exploreRamp := flag.Int("explore-ramp", 30, "clustered rows where explore share reaches base")
	tokenSat := flag.Float64("token-sat", 500, "token saturation: tokens read where engagement weight maxes out")
	satiate := flag.Bool("satiate", false, "intra-session satiation: finish probability drops after 2+ same-interest articles per fetch")
	fast := flag.Bool("fast", false, "fast mode: 20 iterations (phased personas scaled down), 7 key personas, no impression recording")
	engOff := flag.Bool("engage-off", false, "disable completion-driven engagement (v3 baseline: finished=+1, skipped=-1)")
	dynSim := flag.Bool("dyn-sim", false, "dynamic similarity penalty: adapts per reader based on cluster count")
	exploreSimCap := flag.Float64("explore-sim-cap", 1.0, "max cosine between exploration picks (0=all diverse, 1=no filter)")
	clusterWindow := flag.Int("cluster-window", 50, "recent state rows that build rotation clusters (0=no window)")
	poolWindow := flag.Int("pool-window", 200, "recent state rows that generate the candidate pool and walk seeds (0=no window)")
	flag.Parse()
	feed.EngageOff = *engOff
	feed.DynamicSimPenalty = *dynSim
	feed.ExploreSimCap = *exploreSimCap
	feed.TokenSaturation = *tokenSat
	feed.EngageThreshold = *engThresh
	feed.EngageTokens = *engTokens
	feed.ImpressionDamping = *impDamp
	feed.ImpressionWarmup = *impWarmup
	feed.SimilarityPenalty = *simPenalty
	feed.StateDecayFloor = *decayFloor
	feed.DecayFloorMin = *decayMin
	feed.DecayFloorMax = *decayMax
	feed.ExploreShareBase = *exploreBase
	feed.ExploreShareMax = *exploreMax
	feed.FreshSimCap = *capBase
	feed.ClusterRecencyBoost = *clusterBoost
	feed.ExploreRampRows = *exploreRamp
	feed.ClusterWindow = *clusterWindow
	feed.PoolWindow = *poolWindow
	// Impressions are always recorded now — the graduated warmup ramp
	// makes the discount gentle enough for short runs, and skipping
	// impressions made the discount mechanism untestable in fast mode.
	useSatiation = *satiate

	dsn := config.FromEnv().DatabaseURL
	if dsn == "" {
		log.Fatal("feedsim: WIKIFYP_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *explainReader != "" {
		if err := explainPool(context.Background(), db, *explainReader); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *walkReader != "" {
		if err := walkLab(context.Background(), db, *walkReader, *seed); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *capReader != "" {
		if err := capInfo(context.Background(), db, *capReader); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *bench {
		// The standard suite: every persona at a fixed length, so two runs
		// weeks apart are the same experiment. Bump the version when a
		// persona, a metric or a length changes — numbers across versions
		// do not compare.
		var n int
		if err := db.QueryRow("SELECT count(*) FROM articles").Scan(&n); err != nil {
			log.Fatal(err)
		}
		// Pin the feed clock: jitter and exploration must not depend on the
		// day or hour the benchmark happened to run.
		defer feed.PinClock(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC))()
		seeds := []uint64{*seed}
		if *seedsFlag != "" {
			seeds = nil
			for _, part := range strings.Split(*seedsFlag, ",") {
				n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
				if err != nil {
					log.Fatalf("-seeds: %v", err)
				}
				seeds = append(seeds, n)
			}
		}
		fmt.Printf("feed benchmark v3 | %d articles | seeds %v | %d draws of %d per fetch | pinned clock\n",
			n, seeds, *draws, *limit)
		for _, p := range personas {
			// -run narrows the suite to one persona under the exact bench
			// conditions (pinned clock, fixed length) — the cheap iteration
			// loop while tuning a mechanism; the full suite still judges.
			if *run != "" && p.name != *run {
				continue
			}
			if *fast {
				switch p.name {
				case "monomaniac-football", "juggler-five", "gundam-dogs-fungi", "picky",
					"cold-start", "serial-rabbitholer", "scattered":
				default:
					continue
				}
			}
			iters := 60
			if *fast {
				iters = 20
			}
			var runs []runStats
			for _, sd := range seeds {
				stats, err := simulate(context.Background(), db, p, iters, *limit, *draws, sd, *verbose)
				if err != nil {
					log.Fatalf("%s: %v", p.name, err)
				}
				runs = append(runs, stats)
			}
			if len(runs) > 1 {
				fmt.Printf("%-30s %s\n", "", band(runs))
			}
		}
		return
	}

	for _, p := range personas {
		if *run != "" && p.name != *run {
			continue
		}
		if _, err := simulate(context.Background(), db, p, *iterations, *limit, *draws, *seed, *verbose); err != nil {
			log.Fatalf("%s: %v", p.name, err)
		}
	}
}

// explainPool classifies a reader's scored candidate pool by every known
// interest matcher: where the monoculture (or its cure) actually lives.
func explainPool(ctx context.Context, db *sql.DB, reader string) error {
	all := []interest{cars, computing, football, animals, astronomy, presidents, history}
	state, _, scored, err := feed.Explain(ctx, db, reader)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d state rows in window, %d scored candidates\n", reader, len(state), len(scored))
	type bucket struct {
		pool, eligible int
		top            float64
	}
	buckets := map[string]*bucket{}
	for _, name := range append([]string{"(none)"}, names(all)...) {
		buckets[name] = &bucket{}
	}
	for _, c := range scored {
		matched := matching(all, c.Categories)
		if len(matched) == 0 {
			matched = []string{"(none)"}
		}
		for _, m := range matched {
			b := buckets[m]
			b.pool++
			if c.Score >= feed.RelevanceFloor {
				b.eligible++
			}
			b.top = max(b.top, c.Score)
		}
	}
	for _, name := range append(names(all), "(none)") {
		b := buckets[name]
		if b.pool > 0 {
			fmt.Printf("  %-12s pool %5d | score>=floor %5d | best %.2f\n", name, b.pool, b.eligible, b.top)
		}
	}
	fmt.Println("  top 25 by score:")
	for i, c := range scored[:min(25, len(scored))] {
		fmt.Printf("   %2d %6.2f %-40s %v owners=%v\n", i, c.Score, c.Title, matching(all, c.Categories), c.Owners)
	}
	return nil
}

func names(is []interest) []string {
	out := make([]string, len(is))
	for i, in := range is {
		out[i] = in.name
	}
	return out
}

// action is one persona reaction to a served card.
type action struct {
	card       feed.Card
	state      string
	completion float64 // 0–1: simulated reading depth
}

// runStats accumulates what one persona was served and did.
type runStats struct {
	consumed, appealing, saved, junk, repeats, explore int
	perInterest                                        map[string]int
	churnEvents, boringStreak, maxBoring               int
	adoptedAt                                          int            // fetch the latent interest activated at, 0 = never
	firstServed                                        map[string]int // fetch that first served each interest
	uniqueMatching                                     map[int64]bool // distinct on-interest articles consumed: rabbit-hole depth
	starved                                            int            // fetches that served no active-interest card at all
	phases                                             []phaseStats
}

// phaseStats scores one stretch of a scripted reader's life.
type phaseStats struct {
	consumed, active, residue int
	firstActive               int // fetch within the phase the active interest first reached the reader, 0 = never
}

// quitThreshold is how many boring articles in a row end a session: a
// churn event. One boring card is a swipe; ten is closing the app.
const quitThreshold = 10

// stopAfter (the -stopafter flag) halts a simulation after that many
// fetches without reporting, leaving the reader's state in the database
// frozen at an interesting moment for -explain and -capinfo.
var stopAfter int
var skipImpressions bool
var useSatiation bool

// topBias weights which feed card the app shows next: position i is drawn
// with weight topBias^i, so the top of the ranking is likeliest but any
// card can surface — "random but weighted towards the top".
const topBias = 0.85

// draw picks one card index by weight (without replacement: pass the
// remaining indices).
func draw(rng *rand.Rand, indices []int) int {
	total := 0.0
	for _, i := range indices {
		total += math.Pow(topBias, float64(i))
	}
	r := rng.Float64() * total
	for n, i := range indices {
		r -= math.Pow(topBias, float64(i))
		if r <= 0 {
			return n
		}
	}
	return len(indices) - 1
}

// finishCompletion returns a realistic reading completion for a finished
// article, scaled by word count. Short articles (< 500 words) are read
// nearly to the end (85–100%); medium articles get the standard 60–100%
// range; long articles (> 3000 words) see lower completions (45–85%)
// because real readers tire or skip sections. The engagement model's
// token weighting means a long article at 50% still scores high in
// absolute terms.
func finishCompletion(rng *rand.Rand, words int) float64 {
	switch {
	case words < 500:
		return 0.85 + rng.Float64()*0.15 // 85–100%
	case words < 1500:
		return 0.60 + rng.Float64()*0.40 // 60–100%
	case words < 3000:
		return 0.50 + rng.Float64()*0.45 // 50–95%
	default:
		return 0.45 + rng.Float64()*0.40 // 45–85%
	}
}

func simulate(ctx context.Context, db *sql.DB, p persona, iterations, limit, draws int, behaviorSeed uint64, verbose bool) (runStats, error) {
	reader := "sim-" + p.name
	nameHash := fnv.New64a()
	nameHash.Write([]byte(p.name))
	rng := rand.New(rand.NewPCG(behaviorSeed, nameHash.Sum64()))
	if _, err := db.ExecContext(ctx, "DELETE FROM reader_article_state WHERE reader_id = $1", reader); err != nil {
		return runStats{}, err
	}
	finish := func(titles []string) error {
		for _, title := range titles {
			res, err := db.ExecContext(ctx, `
				INSERT INTO reader_article_state (reader_id, source, page_id, state, completion)
				SELECT $1, source, page_id, 'finished', 1.0 FROM latest_articles
				WHERE source = 'simplewiki' AND title = $2
				ON CONFLICT (reader_id, source, page_id)
				DO UPDATE SET state = excluded.state, completion = 1.0, updated_at = now()`, reader, title)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n != 1 {
				return fmt.Errorf("seed article %q not in the corpus", title)
			}
		}
		return nil
	}
	if err := finish(p.seeds); err != nil {
		return runStats{}, err
	}

	interests := append([]interest(nil), p.interests...)
	latent := append([]interest(nil), p.latent...)
	stats := runStats{perInterest: make(map[string]int), firstServed: make(map[string]int),
		uniqueMatching: make(map[int64]bool)}
	prev := make(map[string]bool)
	finishP, skipP := p.finish, p.skip
	if finishP == 0 {
		finishP = 0.75
	}
	if skipP == 0 {
		skipP = 0.15
	}
	// A scripted reader's phases override the run length — but if the
	// caller passed a smaller cap (fast mode), scale each phase down
	// proportionally so the total fits. Copy the slice first: the
	// backing array belongs to the global personas table and must
	// survive across seeds.
	if len(p.phases) > 0 {
		phases := make([]phase, len(p.phases))
		copy(phases, p.phases)
		p.phases = phases
		total := 0
		for _, ph := range p.phases {
			total += ph.fetches
		}
		if iterations > 0 && iterations < total {
			scale := float64(iterations) / float64(total)
			total = 0
			for i := range p.phases {
				p.phases[i].fetches = max(1, int(math.Round(float64(p.phases[i].fetches)*scale)))
				total += p.phases[i].fetches
			}
		}
		iterations = total
		stats.phases = make([]phaseStats, len(p.phases))
	}
	currentPhase, phaseStart := -1, 1
	var prevInterests []interest

	for it := 1; it <= iterations; it++ {
		if len(p.phases) > 0 {
			offset, idx := it, 0
			for idx < len(p.phases)-1 && offset > p.phases[idx].fetches {
				offset -= p.phases[idx].fetches
				idx++
			}
			if idx != currentPhase {
				if currentPhase >= 0 {
					prevInterests = append([]interest(nil), interests...)
				}
				currentPhase, phaseStart = idx, it
				interests = append([]interest(nil), p.phases[idx].interests...)
				if err := finish(p.phases[idx].seeds); err != nil {
					return stats, err
				}
			}
		}
		cards, err := feed.For(ctx, db, reader, limit)
		if err != nil {
			return stats, err
		}
		if len(cards) == 0 {
			return stats, fmt.Errorf("empty feed at fetch %d", it)
		}
		// Record impressions so the feed's impression-normalized interest
		// sees the same signal a real reader's feed serves would generate.
		imps := make([]struct {
			Source string
			PageID int64
		}, len(cards))
		for i, c := range cards {
			imps[i].Source = c.Source
			imps[i].PageID = c.PageID
		}
		if !skipImpressions {
			if err := event.RecordImpressions(ctx, db, reader, imps); err != nil {
				return stats, err
			}
		}
		cats, err := cardCategories(ctx, db, cards)
		if err != nil {
			return stats, err
		}
		// At a phase boundary the interesting question is what the feed
		// offered, not just what the reader happened to draw: print the
		// whole feed for the first two fetches of each phase.
		if verbose && len(p.phases) > 0 && it-phaseStart < 2 {
			fmt.Printf("--- %s, fetch %d (phase %d, full feed) ---\n", p.name, it, currentPhase+1)
			for i, c := range cards {
				mark := "  "
				if c.Exploration {
					mark = "E "
				}
				fmt.Printf("  %2d %s%-55s %v\n", i, mark, c.Title, matching(interests, cats[c.ArticleID]))
			}
		}

		// The app shows one article at a time, drawn from the ranked feed
		// weighted toward its top; the reader consumes draws articles,
		// then the client refetches with their new state applied.
		remaining := make([]int, len(cards))
		for i := range cards {
			remaining[i] = i
		}
		titles := make(map[string]bool, draws)
		var actions []action
		var transcript []string
		fetchHadActive := false
		// Intra-session satiation: track how many articles of each
		// interest the reader consumed in this fetch. After 2, the
		// finish probability drops — a reader who just read 4 cars
		// articles is less keen on the 5th, even if it appeals.
		sessionCounts := make(map[string]int)
		for range min(draws, len(cards)) {
			n := draw(rng, remaining)
			c := cards[remaining[n]]
			remaining = append(remaining[:n], remaining[n+1:]...)
			stats.consumed++
			titles[c.Title] = true
			if prev[c.Title] {
				stats.repeats++
			}
			if c.Exploration {
				stats.explore++
			}
			if junkTitle.MatchString(c.Title) {
				stats.junk++
			}
			matched := matching(interests, cats[c.ArticleID])
			appeals := len(matched) > 0
			if currentPhase >= 0 {
				ph := &stats.phases[currentPhase]
				ph.consumed++
				if appeals {
					ph.active++
					if ph.firstActive == 0 {
						ph.firstActive = it - phaseStart + 1
					}
				}
				if len(matching(prevInterests, cats[c.ArticleID])) > 0 {
					ph.residue++
				}
			}
			if appeals {
				stats.appealing++
				stats.boringStreak = 0
				fetchHadActive = true
				stats.uniqueMatching[c.ArticleID] = true
				for _, m := range matched {
					stats.perInterest[m]++
					sessionCounts[m]++
					if _, ok := stats.firstServed[m]; !ok {
						stats.firstServed[m] = it
					}
				}
			}

			// Act like a reader. A latent interest only surfaces when an
			// exploration card reveals it; once it does, it becomes real.
			// Most unappealing cards are swiped past (ignored), not
			// explicitly skipped — a skip is a deliberate "not this" and
			// real readers don't hand one to every card they scroll by.
			//
			// Intra-session satiation: after 2 articles of the same
			// interest in one fetch, finish probability drops. A reader
			// who just read 4 cars articles in a row is less likely to
			// read the 5th, even if it appeals.
			satiation := 1.0
			if useSatiation && appeals {
				maxSeen := 0
				for _, m := range matched {
					maxSeen = max(maxSeen, sessionCounts[m])
				}
				if maxSeen > 2 {
					satiation = max(0.3, 1-0.2*float64(maxSeen-2))
				}
			}
			st := ""
			comp := 0.0
			switch {
			case appeals && rng.Float64() < finishP*p.pickiness*satiation:
				st = feed.StateFinished
				comp = finishCompletion(rng, c.Words)
				// Some appealing articles are saved for later — about
				// 10% of the time. Saved = strong positive signal.
				if rng.Float64() < 0.10 {
					st = feed.StateSaved
					stats.saved++
				}
			case appeals && rng.Float64() < 0.05:
				st = feed.StateSkipped
				comp = 0.05 + rng.Float64()*0.25 // 5–30%: started, bailed
			// A reader with no active interests adopts from any matching
			// card, flag or not: with zero tastes everything is a door
			// (and a zero-state feed deliberately carries no exploration
			// marks). Once they have interests, discovery goes back to
			// exploration cards only.
			case !appeals && (c.Exploration || len(interests) == 0) && len(matching(latent, cats[c.ArticleID])) > 0 && rng.Float64() < 0.7:
				st = feed.StateFinished
				comp = finishCompletion(rng, c.Words)
				if stats.adoptedAt == 0 {
					stats.adoptedAt = it
				}
				// Adopt only the latent interests this card revealed;
				// the others stay dormant until their own reveal.
				var dormant []interest
				for _, li := range latent {
					if len(matching([]interest{li}, cats[c.ArticleID])) > 0 {
						interests = append(interests, li)
					} else {
						dormant = append(dormant, li)
					}
				}
				latent = dormant
			case !appeals && rng.Float64() < 0.04:
				st = feed.StateFinished
				comp = 0.40 + rng.Float64()*0.40 // 40–80%: accidental find, partial read
			case !appeals && rng.Float64() < skipP:
				st = feed.StateSkipped
				comp = 0.02 + rng.Float64()*0.18 // 2–20%: bounced early
			}
			if !appeals {
				stats.boringStreak++
				stats.maxBoring = max(stats.maxBoring, stats.boringStreak)
				if stats.boringStreak == quitThreshold {
					stats.churnEvents++
					stats.boringStreak = 0
				}
			}
			if st != "" {
				actions = append(actions, action{c, st, comp})
			}
			if verbose {
				mark := "  "
				if c.Exploration {
					mark = "E "
				}
				verdict := "-"
				if appeals {
					verdict = strings.Join(matched, ",")
				}
				transcript = append(transcript, fmt.Sprintf("  %s%-55s %s %s", mark, c.Title, verdict, st))
			}
		}
		if !fetchHadActive {
			stats.starved++
		}
		prev = titles

		if err := apply(ctx, db, reader, actions); err != nil {
			return stats, err
		}
		if verbose && (it == 1 || it == iterations/2 || it == iterations) {
			fmt.Printf("--- %s, fetch %d ---\n%s\n", p.name, it, strings.Join(transcript, "\n"))
		}
		if stopAfter > 0 && it == stopAfter {
			fmt.Printf("%s: stopped after fetch %d; state left in the database as %s\n", p.name, it, reader)
			return stats, nil
		}
	}

	report(p, iterations, stats)
	return stats, nil
}

func apply(ctx context.Context, db *sql.DB, reader string, actions []action) error {
	if len(actions) == 0 {
		return nil
	}
	ids := make([]int64, len(actions))
	states := make([]string, len(actions))
	completions := make([]float64, len(actions))
	for i, a := range actions {
		ids[i] = a.card.ArticleID
		states[i] = a.state
		completions[i] = a.completion
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO reader_article_state (reader_id, source, page_id, state, completion)
		SELECT $1, a.source, a.page_id, act.state, act.completion
		FROM unnest($2::bigint[], $3::text[], $4::real[]) AS act (id, state, completion)
		JOIN articles a ON a.id = act.id
		ON CONFLICT (reader_id, source, page_id)
		DO UPDATE SET state = excluded.state,
			completion = GREATEST(reader_article_state.completion, excluded.completion),
			updated_at = now()`, reader, ids, states, completions)
	return err
}

func cardCategories(ctx context.Context, db *sql.DB, cards []feed.Card) (map[int64][]string, error) {
	ids := make([]int64, len(cards))
	for i, c := range cards {
		ids[i] = c.ArticleID
	}
	rows, err := db.QueryContext(ctx,
		"SELECT article_id, category FROM article_categories WHERE article_id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]string)
	for rows.Next() {
		var id int64
		var c string
		if err := rows.Scan(&id, &c); err != nil {
			return nil, err
		}
		out[id] = append(out[id], c)
	}
	return out, rows.Err()
}

func matching(interests []interest, cats []string) []string {
	var hit []string
	for _, i := range interests {
		for _, c := range cats {
			if i.match.MatchString(c) {
				hit = append(hit, i.name)
				break
			}
		}
	}
	return hit
}

// band condenses one persona's runs across seeds into min-max ranges of
// the headline numbers, so a single line says whether a change is signal
// (band moved) or a trajectory roll (band width).
func band(runs []runStats) string {
	bounds := func(f func(runStats) float64) (lo, hi float64) {
		lo = f(runs[0])
		hi = lo
		for _, r := range runs[1:] {
			v := f(r)
			lo, hi = min(lo, v), max(hi, v)
		}
		return lo, hi
	}
	aLo, aHi := bounds(func(r runStats) float64 { return pct(r.appealing, r.consumed) })
	rLo, rHi := bounds(func(r runStats) float64 { return pct(r.repeats, r.consumed) })
	qLo, qHi := bounds(func(r runStats) float64 { return float64(r.churnEvents) })
	dLo, dHi := bounds(func(r runStats) float64 { return float64(len(r.uniqueMatching)) })
	sLo, sHi := bounds(func(r runStats) float64 { return float64(r.starved) })
	return fmt.Sprintf("band: appeal %.1f-%.1f | repeats %.1f-%.1f | quits %.0f-%.0f | depth %.0f-%.0f | starved %.0f-%.0f",
		aLo, aHi, rLo, rHi, qLo, qHi, dLo, dHi, sLo, sHi)
}

// pct is the shared percentage formula the per-run report and the band
// summary must agree on.
func pct(n, of int) float64 { return 100 * float64(n) / float64(max(of, 1)) }

func report(p persona, iterations int, s runStats) {
	fmt.Printf("%-30s consumed %4d | appeal %5.1f%% | saved %d | explore %5.1f%% | junk %4.1f%% | repeats %4.1f%% | quits %d | max-boring %d\n",
		p.name, s.consumed, pct(s.appealing, s.consumed),
		s.saved,
		pct(s.explore, s.consumed),
		pct(s.junk, s.consumed),
		pct(s.repeats, s.consumed),
		s.churnEvents, s.maxBoring)
	var parts []string
	for _, i := range p.interests {
		first := "never"
		if at, ok := s.firstServed[i.name]; ok {
			first = fmt.Sprintf("@%d", at)
		}
		parts = append(parts, fmt.Sprintf("%s %d (%s)", i.name, s.perInterest[i.name], first))
	}
	for _, i := range p.latent {
		parts = append(parts, fmt.Sprintf("latent %s %d", i.name, s.perInterest[i.name]))
	}
	if s.adoptedAt > 0 {
		parts = append(parts, fmt.Sprintf("adopted latent at feed %d", s.adoptedAt))
	}
	parts = append(parts, fmt.Sprintf("depth %d | starved %d", len(s.uniqueMatching), s.starved))
	fmt.Printf("%-30s interests: %s\n", "", strings.Join(parts, " | "))
	for i, ph := range s.phases {
		var names []string
		for _, in := range p.phases[i].interests {
			names = append(names, in.name)
		}
		first := "never"
		if ph.firstActive > 0 {
			first = fmt.Sprintf("@%d", ph.firstActive)
		}
		fmt.Printf("%-30s phase %d (%s): active %.0f%% (%s) | residue %.0f%%\n", "",
			i+1, strings.Join(names, ","),
			100*float64(ph.active)/float64(max(ph.consumed, 1)), first,
			100*float64(ph.residue)/float64(max(ph.consumed, 1)))
	}
}
