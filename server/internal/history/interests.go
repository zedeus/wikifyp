package history

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/zedeus/wikifyp/server/internal/feed"
)

// InterestCluster is a group of related categories the reader engages with.
type InterestCluster struct {
	Categories   []string         `json:"categories"`
	Weight       float64          `json:"weight"`
	Articles     []ClusterArticle `json:"articles"`
	ArticleCount int              `json:"article_count"`
}

// ClusterArticle is one article in an interest cluster.
type ClusterArticle struct {
	Title      string  `json:"title"`
	Completion float64 `json:"completion"`
	State      string  `json:"state,omitempty"`
}

// Interests is the reader's inferred interest profile.
type Interests struct {
	Clusters      []InterestCluster  `json:"clusters"`
	TopCategories []WeightedCategory `json:"top_categories"`
	TotalArticles int                `json:"total_articles"`
	ExploreShare  float64            `json:"explore_share"`
}

// WeightedCategory is a category with its discriminative weight.
type WeightedCategory struct {
	Category string  `json:"category"`
	Weight   float64 `json:"weight"`
}

// ReaderInterests computes the reader's interest profile by reusing the
// feed's existing clustering and weighting infrastructure. This is the
// same model the feed uses to pick articles, so the interests view shows
// exactly what drives recommendations.
func ReaderInterests(ctx context.Context, db *sql.DB, readerID string) (*Interests, error) {
	state, weights, _, err := feed.Explain(ctx, db, readerID)
	if err != nil {
		return nil, fmt.Errorf("interests: explain: %w", err)
	}

	clusters, misc := feed.StateClusters(state)
	result := &Interests{
		TotalArticles: len(state),
	}

	// Build interest clusters with their top articles.
	for _, cl := range clusters {
		ic := InterestCluster{
			ArticleCount: len(cl),
		}
		// Collect categories from cluster members.
		catSet := make(map[string]bool)
		for _, title := range cl {
			for _, s := range state {
				if s.Title == title {
					for _, c := range s.Categories {
						catSet[c] = true
					}
					ic.Articles = append(ic.Articles, ClusterArticle{
						Title:      s.Title,
						Completion: s.Completion,
						State:      s.State,
					})
					break
				}
			}
		}
		// Keep only the most weighted categories.
		type wc struct {
			cat    string
			weight float64
		}
		var weighted []wc
		for c := range catSet {
			if w, ok := weights[c]; ok {
				weighted = append(weighted, wc{c, w})
			}
		}
		slices.SortFunc(weighted, func(a, b wc) int {
			if a.weight > b.weight {
				return -1
			}
			if a.weight < b.weight {
				return 1
			}
			return 0
		})
		for i, w := range weighted {
			if i >= 5 {
				break
			}
			ic.Categories = append(ic.Categories, w.cat)
			ic.Weight += w.weight
		}
		// Sort articles by completion descending.
		slices.SortFunc(ic.Articles, func(a, b ClusterArticle) int {
			if a.Completion > b.Completion {
				return -1
			}
			if a.Completion < b.Completion {
				return 1
			}
			return 0
		})
		// Cap articles shown per cluster.
		if len(ic.Articles) > 5 {
			ic.Articles = ic.Articles[:5]
		}
		result.Clusters = append(result.Clusters, ic)
	}

	// Misc (unclustered) articles.
	if len(misc) > 0 {
		ic := InterestCluster{
			Categories:   []string{"Miscellaneous"},
			ArticleCount: len(misc),
		}
		for _, title := range misc {
			for _, s := range state {
				if s.Title == title && len(ic.Articles) < 5 {
					ic.Articles = append(ic.Articles, ClusterArticle{
						Title:      s.Title,
						Completion: s.Completion,
						State:      s.State,
					})
					break
				}
			}
		}
		result.Clusters = append(result.Clusters, ic)
	}

	// Sort clusters by weight descending.
	slices.SortFunc(result.Clusters, func(a, b InterestCluster) int {
		if a.Weight > b.Weight {
			return -1
		}
		if a.Weight < b.Weight {
			return 1
		}
		return 0
	})

	// Top weighted categories overall.
	type wc struct {
		cat    string
		weight float64
	}
	var allWeighted []wc
	for c, w := range weights {
		allWeighted = append(allWeighted, wc{c, w})
	}
	slices.SortFunc(allWeighted, func(a, b wc) int {
		if a.weight > b.weight {
			return -1
		}
		if a.weight < b.weight {
			return 1
		}
		return 0
	})
	for i, w := range allWeighted {
		if i >= 15 {
			break
		}
		result.TopCategories = append(result.TopCategories, WeightedCategory{
			Category: w.cat,
			Weight:   w.weight,
		})
	}

	// Explore share: what fraction of the feed is exploration.
	clusteredRows := 0
	for _, cl := range clusters {
		clusteredRows += len(cl)
	}
	result.ExploreShare = feed.ExploreShareBase +
		(feed.ExploreShareMax-feed.ExploreShareBase)*max(0, 1-float64(clusteredRows)/float64(feed.ExploreRampRows))

	return result, nil
}
