package feed

import (
	"context"
	"slices"
	"testing"
)

// TestWalkDeterministicAcrossRowOrder pins the walk's determinism claim
// against the one thing a seeded rng cannot control: the physical row order
// of unordered queries. On the full corpus these land as bitmap heap scans,
// which return rows in heap order — so a pick made by indexing a slice
// built in arrival order changes between runs, and the benchmark's
// "deterministic for a seed" promise silently broke. The test forces
// sequential scans (heap order on any corpus size), runs the same seeded
// walk before and after physically rewriting the graph tables in reverse,
// and demands identical hits.
func TestWalkDeterministicAcrossRowOrder(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	// One connection so the planner settings hold for every walk query;
	// seqscans make row order follow the heap like production bitmap scans.
	db.SetMaxOpenConns(1)
	for _, s := range []string{"SET enable_indexscan = off", "SET enable_bitmapscan = off"} {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	// Seed state: real simplewiki articles that carry categories, so
	// category hops have somewhere to go.
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT a.title FROM latest_articles a
		JOIN article_categories c ON c.article_id = a.id
		WHERE a.source = 'simplewiki' ORDER BY a.title LIMIT 5`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var state []StateArticle
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatal(err)
		}
		state = append(state, StateArticle{Source: "simplewiki", Title: title, State: StateFinished})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(state) < 2 {
		t.Fatalf("corpus too small: %d seed articles", len(state))
	}

	cfg := WalkConfig{Steps: 3, Walks: 80, Links: true, Cats: true}
	run := func() []WalkHit {
		hits, err := Walk(ctx, db, state, cfg, 42)
		if err != nil {
			t.Fatal(err)
		}
		return hits
	}
	before := run()
	if len(before) == 0 {
		t.Fatal("walk produced no hits; the determinism check would be vacuous")
	}

	// Rewrite the graph tables so a heap scan returns their rows in the
	// opposite order. Same data, different arrival order.
	reverse := func(table, order string) {
		t.Helper()
		for _, s := range []string{
			"CREATE TABLE tmp_reorder AS SELECT * FROM " + table + " ORDER BY " + order,
			"TRUNCATE " + table,
			"INSERT INTO " + table + " SELECT * FROM tmp_reorder",
			"DROP TABLE tmp_reorder",
		} {
			if _, err := db.ExecContext(ctx, s); err != nil {
				t.Fatal(err)
			}
		}
	}
	reverse("article_categories", "article_id DESC, category DESC")
	reverse("article_links", "article_id DESC, target DESC")

	after := run()
	if !slices.EqualFunc(before, after, func(a, b WalkHit) bool {
		return a.ArticleID == b.ArticleID && a.Visits == b.Visits && a.Depth == b.Depth
	}) {
		t.Errorf("walk hits depend on physical row order:\nbefore: %v\nafter:  %v",
			hitSummary(before), hitSummary(after))
	}
}

func hitSummary(hits []WalkHit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits[:min(8, len(hits))] {
		out = append(out, h.Title)
	}
	return out
}
