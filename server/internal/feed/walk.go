package feed

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
)

// WalkConfig shapes a Monte Carlo random walk over the article graph: how
// many steps out from the reader's reads, along which edges, and whether
// steps prefer specific pages over hubs. Walks reach the middle distance
// the rest of the engine misses — one hop is the reader's own
// neighbourhood, stratified exploration is anywhere at all; two to four
// weighted steps land on "related but surprising".
type WalkConfig struct {
	Steps    int  // walk length; positions reached at step >= 2 are hits
	Links    bool // step along an article's outgoing links
	Cats     bool // step to another member of one of the article's categories
	Weighted bool // prefer rare categories (1/df) and low-degree link targets
	Walks    int  // Monte Carlo walkers launched from the reader's reads
	Top      int  // keep only the most-visited Top hits (0 keeps all); the
	// cut runs before graph data loads, which is most of the cost
}

// WalkHit is an article the walk visited, with how often and how far out.
type WalkHit struct {
	Candidate
	Visits int
	Depth  int // first depth the walk reached it at
}

// walker is one Monte Carlo walker's position.
type walker struct {
	at   int64
	dead bool
}

// Walk runs cfg.Walks seeded random walks from the reader's positive state
// articles and returns the distinct articles visited at depth two or more,
// most-visited first (visit count approximates personalized-PageRank mass).
// The reader's own pages never count as hits; suppression is the caller's
// concern (hits carry page keys). Results are deterministic for a seed.
func Walk(ctx context.Context, db *sql.DB, state []StateArticle, cfg WalkConfig, seed int64) ([]WalkHit, error) {
	if !cfg.Links && !cfg.Cats {
		return nil, fmt.Errorf("feed: walk: no edge types enabled")
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0x77616c6b)) // "walk"

	// Seed walkers on the positive state articles, resolved to ids —
	// per (source, title), because state can span wikis and the same
	// title names different pages on different wikis.
	var sources, titles []string
	for _, s := range state {
		if s.State != StateSkipped {
			sources = append(sources, s.Source)
			titles = append(titles, s.Title)
		}
	}
	if len(titles) == 0 {
		return nil, nil
	}
	seedIDs, err := idsByTitle(ctx, db, sources, titles)
	if err != nil {
		return nil, err
	}
	if len(seedIDs) == 0 {
		return nil, nil
	}
	self := make(map[int64]bool, len(seedIDs))
	for _, id := range seedIDs {
		self[id] = true
	}

	walkers := make([]walker, cfg.Walks)
	for i := range walkers {
		walkers[i].at = seedIDs[rng.IntN(len(seedIDs))]
	}

	visits := make(map[int64]int)
	firstDepth := make(map[int64]int)
	for step := 1; step <= cfg.Steps; step++ {
		// Split this step's movers by edge type (mixed configs flip a coin
		// per walker per step), then batch each kind in one or two queries.
		var linkMove, catMove []int
		for i, w := range walkers {
			if w.dead {
				continue
			}
			switch {
			case cfg.Links && cfg.Cats:
				if rng.Float64() < 0.5 {
					linkMove = append(linkMove, i)
				} else {
					catMove = append(catMove, i)
				}
			case cfg.Links:
				linkMove = append(linkMove, i)
			default:
				catMove = append(catMove, i)
			}
		}
		if len(linkMove) > 0 {
			if err := stepLinks(ctx, db, walkers, linkMove, cfg.Weighted, rng); err != nil {
				return nil, err
			}
		}
		if len(catMove) > 0 {
			if err := stepCats(ctx, db, walkers, catMove, cfg.Weighted, rng); err != nil {
				return nil, err
			}
		}
		if step >= 2 {
			for _, w := range walkers {
				if !w.dead && !self[w.at] {
					visits[w.at]++
					if _, ok := firstDepth[w.at]; !ok {
						firstDepth[w.at] = step
					}
				}
			}
		}
	}

	ids := make([]int64, 0, len(visits))
	for id := range visits {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b int64) int {
		if visits[a] != visits[b] {
			return visits[b] - visits[a]
		}
		return int(a - b)
	})
	if cfg.Top > 0 && len(ids) > cfg.Top {
		ids = ids[:cfg.Top]
	}
	cands, err := candidatesByID(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	hits := make([]WalkHit, 0, len(cands))
	for _, c := range cands {
		hits = append(hits, WalkHit{Candidate: c, Visits: visits[c.ArticleID], Depth: firstDepth[c.ArticleID]})
	}
	slices.SortFunc(hits, func(a, b WalkHit) int {
		if a.Visits != b.Visits {
			return b.Visits - a.Visits
		}
		return int(a.ArticleID - b.ArticleID)
	})
	return hits, nil
}

// stepLinks moves each listed walker along one outgoing link of its current
// article, resolved to a stored latest-revision article. Weighted steps
// prefer low-degree targets (1/sqrt(1+outdegree)); a walker with no
// resolvable links dies.
func stepLinks(ctx context.Context, db *sql.DB, walkers []walker, movers []int, weighted bool, rng *rand.Rand) error {
	ids := make([]int64, 0, len(movers))
	for _, i := range movers {
		ids = append(ids, walkers[i].at)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT l.article_id, a.id,
		       (SELECT count(*) FROM article_links l2 WHERE l2.article_id = a.id)
		FROM article_links l
		JOIN articles src ON src.id = l.article_id
		JOIN latest_articles a ON a.source = src.source AND a.title = l.target
		WHERE l.article_id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("feed: walk links: %w", err)
	}
	defer rows.Close()
	type edge struct {
		to  int64
		deg int
	}
	out := make(map[int64][]edge)
	for rows.Next() {
		var from, to int64
		var deg int
		if err := rows.Scan(&from, &to, &deg); err != nil {
			return fmt.Errorf("feed: walk links: %w", err)
		}
		out[from] = append(out[from], edge{to, deg})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("feed: walk links: %w", err)
	}
	// The seeded rng picks by index, so the slice order must not be the
	// query's row order: on a large corpus these arrive via bitmap heap
	// scans in physical order, which varies — and "deterministic for a
	// seed" silently broke (the reader benchmark measured it).
	for _, edges := range out {
		slices.SortFunc(edges, func(a, b edge) int { return int(a.to - b.to) })
	}
	for _, i := range movers {
		edges := out[walkers[i].at]
		if len(edges) == 0 {
			walkers[i].dead = true
			continue
		}
		if !weighted {
			walkers[i].at = edges[rng.IntN(len(edges))].to
			continue
		}
		// Default to the last edge so floating-point residue can never
		// leave the walker un-moved (stepCats guards the same way).
		pick := edges[len(edges)-1].to
		total := 0.0
		for _, e := range edges {
			total += 1 / math.Sqrt(float64(1+e.deg))
		}
		r := rng.Float64() * total
		for _, e := range edges {
			r -= 1 / math.Sqrt(float64(1+e.deg))
			if r <= 0 {
				pick = e.to
				break
			}
		}
		walkers[i].at = pick
	}
	return nil
}

// stepCats moves each listed walker to another member of one of its current
// article's categories. Weighted steps pick the category with probability
// proportional to 1/df (a rare category is a tight semantic hop, a huge one
// a teleport); the member is a seeded random probe either way. A walker on
// an uncategorized article dies.
func stepCats(ctx context.Context, db *sql.DB, walkers []walker, movers []int, weighted bool, rng *rand.Rand) error {
	ids := make([]int64, 0, len(movers))
	for _, i := range movers {
		ids = append(ids, walkers[i].at)
	}
	// The per-category df count only matters for the weighted pick; the
	// unweighted production path skips it (counting a huge category's
	// members on every step is the expensive part).
	query := `SELECT c.article_id, c.category, 0
		FROM article_categories c WHERE c.article_id = ANY($1)`
	if weighted {
		query = `SELECT c.article_id, c.category,
		       (SELECT count(*) FROM article_categories c2 WHERE c2.category = c.category)
		FROM article_categories c WHERE c.article_id = ANY($1)`
	}
	rows, err := db.QueryContext(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("feed: walk cats: %w", err)
	}
	defer rows.Close()
	type catDF struct {
		cat string
		df  int
	}
	cats := make(map[int64][]catDF)
	for rows.Next() {
		var id int64
		var c catDF
		if err := rows.Scan(&id, &c.cat, &c.df); err != nil {
			return fmt.Errorf("feed: walk cats: %w", err)
		}
		cats[id] = append(cats[id], c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("feed: walk cats: %w", err)
	}
	// Row order is not deterministic (see the stepLinks sort); the seeded
	// category pick indexes the slice, so sort it first.
	for _, cs := range cats {
		slices.SortFunc(cs, func(a, b catDF) int { return cmp.Compare(a.cat, b.cat) })
	}

	// Pick each mover's category, then resolve one random member per pick
	// in a single probe query: a seeded random point in the id range, the
	// next member at or past it on the (category, article_id) index,
	// wrapping to the category's first member on overshoot.
	var lo, hi sql.NullInt64
	if err := db.QueryRowContext(ctx,
		"SELECT min(article_id), max(article_id) FROM article_categories").Scan(&lo, &hi); err != nil {
		return fmt.Errorf("feed: walk cats: %w", err)
	}
	if !hi.Valid {
		for _, i := range movers {
			walkers[i].dead = true
		}
		return nil
	}
	chosen := make([]string, 0, len(movers))
	from := make([]int64, 0, len(movers))
	probes := make([]int64, 0, len(movers))
	var alive []int
	for _, i := range movers {
		cs := cats[walkers[i].at]
		if len(cs) == 0 {
			walkers[i].dead = true
			continue
		}
		pick := cs[rng.IntN(len(cs))]
		if weighted {
			total := 0.0
			for _, c := range cs {
				total += 1 / float64(max(c.df, 1))
			}
			r := rng.Float64() * total
			for _, c := range cs {
				r -= 1 / float64(max(c.df, 1))
				if r <= 0 {
					pick = c
					break
				}
			}
		}
		alive = append(alive, i)
		chosen = append(chosen, pick.cat)
		from = append(from, walkers[i].at)
		probes = append(probes, lo.Int64+rng.Int64N(hi.Int64-lo.Int64+1))
	}
	if len(alive) == 0 {
		return nil
	}
	member, err := db.QueryContext(ctx, `
		SELECT o.ord, coalesce(
			(SELECT c.article_id FROM article_categories c
			 WHERE c.category = o.category AND c.article_id >= o.probe AND c.article_id <> o.self
			 ORDER BY c.article_id LIMIT 1),
			(SELECT c.article_id FROM article_categories c
			 WHERE c.category = o.category AND c.article_id <> o.self
			 ORDER BY c.article_id LIMIT 1))
		FROM unnest($1::text[], $2::bigint[], $3::bigint[]) WITH ORDINALITY AS o (category, self, probe, ord)`,
		chosen, from, probes)
	if err != nil {
		return fmt.Errorf("feed: walk cat members: %w", err)
	}
	defer member.Close()
	landed := make(map[int]int64)
	for member.Next() {
		var ord int
		var id sql.NullInt64
		if err := member.Scan(&ord, &id); err != nil {
			return fmt.Errorf("feed: walk cat members: %w", err)
		}
		if id.Valid {
			landed[ord-1] = id.Int64
		}
	}
	if err := member.Err(); err != nil {
		return fmt.Errorf("feed: walk cat members: %w", err)
	}
	for n, i := range alive {
		if id, ok := landed[n]; ok {
			walkers[i].at = id
		} else {
			walkers[i].dead = true
		}
	}
	return nil
}

// idsByTitle resolves latest-revision article ids for titles of one source.
func idsByTitle(ctx context.Context, db *sql.DB, sources, titles []string) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id FROM latest_articles a
		JOIN unnest($1::text[], $2::text[]) AS f (source, title)
		  ON a.source = f.source AND a.title = f.title`, sources, titles)
	if err != nil {
		return nil, fmt.Errorf("feed: walk seeds: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("feed: walk seeds: %w", err)
		}
		ids = append(ids, id)
	}
	// Walker seeding indexes this slice with the seeded rng; sort it so
	// the assignment survives arbitrary row order (see the stepLinks sort).
	slices.Sort(ids)
	return ids, rows.Err()
}

// candidatesByID loads full candidates (with graph data) for article ids.
func candidatesByID(ctx context.Context, db *sql.DB, ids []int64) ([]Candidate, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.page_id, a.source, a.title FROM latest_articles a WHERE a.id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("feed: walk candidates: %w", err)
	}
	defer rows.Close()
	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ArticleID, &c.PageID, &c.Source, &c.Title); err != nil {
			return nil, fmt.Errorf("feed: walk candidates: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed: walk candidates: %w", err)
	}
	links, cats, err := graphData(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Links = links[out[i].ArticleID]
		out[i].Categories = cats[out[i].ArticleID]
	}
	return out, nil
}
