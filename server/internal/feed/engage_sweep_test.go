package feed

import (
	"fmt"
	"math"
	"testing"
)

// TestEngagementSweep runs hundreds of parameter combinations against
// synthetic persona profiles in milliseconds, no Postgres needed.
// Each persona is modeled as a distribution of completion values for
// appealing and non-appealing articles, and the score measures how
// well the engagement function separates them.
func TestEngagementSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sweep in short mode")
	}

	type persona struct {
		name string
		// completion distributions: {mean, stddev} for each action type
		appealFinish    [2]float64 // e.g. {0.97, 0.02}: read to the end
		appealSkip      [2]float64 // e.g. {0.15, 0.05}: glanced, not today
		appealSkipRate  float64    // fraction of appealing articles skipped
		noappealFinish  [2]float64 // accidental finish
		noappealFinishR float64    // rate
		noappealSkip    [2]float64 // bounce
		noappealSkipR   float64    // rate
		appealRate      float64    // what fraction of served articles appeal
		words           int        // avg article length
	}

	personas := []persona{
		{name: "mono", appealFinish: [2]float64{0.97, 0.02}, appealSkip: [2]float64{0.15, 0.05},
			appealSkipRate: 0.05, noappealFinish: [2]float64{0.95, 0.03}, noappealFinishR: 0.04,
			noappealSkip: [2]float64{0.08, 0.04}, noappealSkipR: 0.30, appealRate: 0.77, words: 500},
		{name: "juggler", appealFinish: [2]float64{0.97, 0.02}, appealSkip: [2]float64{0.15, 0.05},
			appealSkipRate: 0.05, noappealFinish: [2]float64{0.95, 0.03}, noappealFinishR: 0.04,
			noappealSkip: [2]float64{0.08, 0.04}, noappealSkipR: 0.15, appealRate: 0.70, words: 400},
		{name: "gundam", appealFinish: [2]float64{0.97, 0.02}, appealSkip: [2]float64{0.15, 0.05},
			appealSkipRate: 0.05, noappealFinish: [2]float64{0.95, 0.03}, noappealFinishR: 0.04,
			noappealSkip: [2]float64{0.08, 0.04}, noappealSkipR: 0.15, appealRate: 0.55, words: 250},
		{name: "picky", appealFinish: [2]float64{0.97, 0.02}, appealSkip: [2]float64{0.15, 0.05},
			appealSkipRate: 0.65, noappealFinish: [2]float64{0.95, 0.03}, noappealFinishR: 0.04,
			noappealSkip: [2]float64{0.08, 0.04}, noappealSkipR: 0.15, appealRate: 0.55, words: 400},
	}

	// The score: how well does the engagement function separate appealing
	// from non-appealing articles? Higher = better separation.
	score := func(p persona, thresh, tokThresh float64) float64 {
		origThresh := EngageThreshold
		origTok := EngageTokens
		EngageThreshold = thresh
		EngageTokens = int(tokThresh)
		defer func() {
			EngageThreshold = origThresh
			EngageTokens = origTok
		}()

		var appealPos, appealNeg, noappealPos, noappealNeg float64
		n := 1000

		for i := range n {
			// Deterministic pseudo-random from index
			r := float64(i) / float64(n)

			// Appealing article
			var comp float64
			if r < p.appealSkipRate {
				comp = clamp(p.appealSkip[0]+p.appealSkip[1]*(r/p.appealSkipRate-0.5)*2, 0, 1)
			} else {
				comp = clamp(p.appealFinish[0]+p.appealFinish[1]*(r-0.5)*2, 0, 1)
			}
			w := engagementWeight(comp, p.words, StateFinished)
			if w > 0 {
				appealPos += w
			} else {
				appealNeg += -w
			}

			// Non-appealing article
			if r < p.noappealFinishR {
				comp = clamp(p.noappealFinish[0]+p.noappealFinish[1]*(r-0.5)*2, 0, 1)
			} else if r < p.noappealFinishR+p.noappealSkipR {
				comp = clamp(p.noappealSkip[0]+p.noappealSkip[1]*(r-0.5)*2, 0, 1)
			} else {
				continue // swiped past, no signal
			}
			w = engagementWeight(comp, p.words, StateSkipped)
			if w > 0 {
				noappealPos += w
			} else {
				noappealNeg += -w
			}
		}

		// Good separation: high appealPos, low appealNeg, low noappealPos, high noappealNeg
		if appealPos+noappealNeg == 0 {
			return 0
		}
		signal := appealPos + noappealNeg
		noise := appealNeg + noappealPos
		return signal / (signal + noise)
	}

	fmt.Println("\n=== Engagement Sweep (pure math, no Postgres) ===")
	fmt.Printf("%-10s", "config")
	for _, p := range personas {
		fmt.Printf("  %-8s", p.name)
	}
	fmt.Println("  avg")

	best := 0.0
	bestConfig := ""

	for _, thresh := range []float64{0.10, 0.15, 0.20, 0.25, 0.30, 0.33, 0.40, 0.50} {
		for _, tok := range []float64{50, 100, 200, 300} {
			label := fmt.Sprintf("t=%.2f k=%d", thresh, int(tok))
			fmt.Printf("%-10s", label)
			sum := 0.0
			for _, p := range personas {
				s := score(p, thresh, tok)
				fmt.Printf("  %-8.3f", s)
				sum += s
			}
			avg := sum / float64(len(personas))
			fmt.Printf("  %.3f", avg)
			if avg > best {
				best = avg
				bestConfig = label
			}
			fmt.Println()
		}
	}
	fmt.Printf("\nBest: %s (avg=%.3f)\n", bestConfig, best)
}

func clamp(x, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, x))
}
