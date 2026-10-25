package github

import (
	"math"

	"github.com/Theyka/github-cards/internal/cards"
)

func exponentialCDF(x float64) float64 {
	return 1 - math.Pow(2, -x)
}

func logNormalCDF(x float64) float64 {
	return x / (1 + x)
}

// CalculateRank mirrors github-readme-stats' calculateRank.
func CalculateRank(allCommits bool, commits, prs, issues, reviews, stars, followers float64) cards.Rank {
	commitsMedian := 250.0
	if allCommits {
		commitsMedian = 1000
	}
	const (
		commitsWeight   = 2
		prsMedian       = 50
		prsWeight       = 3
		issuesMedian    = 25
		issuesWeight    = 1
		reviewsMedian   = 2
		reviewsWeight   = 1
		starsMedian     = 50
		starsWeight     = 4
		followersMedian = 10
		followersWeight = 1
	)
	const totalWeight = commitsWeight + prsWeight + issuesWeight + reviewsWeight + starsWeight + followersWeight

	thresholds := []float64{1, 12.5, 25, 37.5, 50, 62.5, 75, 87.5, 100}
	levels := []string{"S", "A+", "A", "A-", "B+", "B", "B-", "C+", "C"}

	rank := 1 - (commitsWeight*exponentialCDF(commits/commitsMedian)+
		prsWeight*exponentialCDF(prs/prsMedian)+
		issuesWeight*exponentialCDF(issues/issuesMedian)+
		reviewsWeight*exponentialCDF(reviews/reviewsMedian)+
		starsWeight*logNormalCDF(stars/starsMedian)+
		followersWeight*logNormalCDF(followers/followersMedian))/totalWeight

	level := "C"
	for i, threshold := range thresholds {
		if rank*100 <= threshold {
			level = levels[i]
			break
		}
	}

	return cards.Rank{Level: level, Percentile: rank * 100}
}
