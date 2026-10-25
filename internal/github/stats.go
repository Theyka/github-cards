package github

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/Theyka/github-cards/internal/cards"
)

const statsQuery = `query userInfo(
  $login: String!
  $startTime: DateTime = null
  $ownerAffiliations: [RepositoryAffiliation]
  $includeUserRepositories: Boolean!
) {
  user(login: $login) {
    name
    login
    commits: contributionsCollection(from: $startTime) {
      totalCommitContributions
    }
    reviews: contributionsCollection {
      totalPullRequestReviewContributions
    }
    repositoriesContributedTo(
      first: 1
      contributionTypes: [COMMIT, ISSUE, PULL_REQUEST, REPOSITORY]
      includeUserRepositories: $includeUserRepositories
    ) {
      totalCount
    }
    pullRequests(first: 1) {
      totalCount
    }
    openIssues: issues(states: OPEN) {
      totalCount
    }
    closedIssues: issues(states: CLOSED) {
      totalCount
    }
    followers {
      totalCount
    }
    contributionsCollection {
      contributionYears
    }
    repositories(first: 100, ownerAffiliations: $ownerAffiliations, orderBy: { direction: DESC, field: STARGAZERS }) {
      totalCount
      nodes {
        name
        stargazerCount
      }
    }
  }
}`

type statsResponse struct {
	User *struct {
		Name    string `json:"name"`
		Login   string `json:"login"`
		Commits struct {
			TotalCommitContributions float64 `json:"totalCommitContributions"`
		} `json:"commits"`
		Reviews struct {
			TotalPullRequestReviewContributions float64 `json:"totalPullRequestReviewContributions"`
		} `json:"reviews"`
		RepositoriesContributedTo struct {
			TotalCount float64 `json:"totalCount"`
		} `json:"repositoriesContributedTo"`
		PullRequests struct {
			TotalCount float64 `json:"totalCount"`
		} `json:"pullRequests"`
		OpenIssues struct {
			TotalCount float64 `json:"totalCount"`
		} `json:"openIssues"`
		ClosedIssues struct {
			TotalCount float64 `json:"totalCount"`
		} `json:"closedIssues"`
		Followers struct {
			TotalCount float64 `json:"totalCount"`
		} `json:"followers"`
		ContributionsCollection struct {
			ContributionYears []int `json:"contributionYears"`
		} `json:"contributionsCollection"`
		Repositories struct {
			TotalCount float64 `json:"totalCount"`
			Nodes      []struct {
				Name           string  `json:"name"`
				StargazerCount float64 `json:"stargazerCount"`
			} `json:"nodes"`
		} `json:"repositories"`
	} `json:"user"`
}

// StatsParams are the inputs of FetchStats.
type StatsParams struct {
	Username                string
	IncludeAllCommits       bool
	ExcludeRepos            []string
	CommitsYear             *int
	OwnerAffiliations       []string
	ContribsIncludeOwnRepos bool
}

var validAffiliations = map[string]bool{
	"OWNER":               true,
	"ORGANIZATION_MEMBER": true,
	"COLLABORATOR":        true,
}

// FetchStats fetches a user's stats, mirroring github-readme-stats' fetchStats.
func (c *Client) FetchStats(ctx context.Context, p StatsParams) (cards.StatsData, error) {
	if p.Username == "" {
		return cards.StatsData{}, missingParamError([]string{"username"}, "")
	}

	affiliations := p.OwnerAffiliations
	if len(affiliations) == 0 {
		affiliations = []string{"OWNER"}
	}
	for _, a := range affiliations {
		if !validAffiliations[strings.ToUpper(a)] {
			return cards.StatsData{}, &Error{Message: "Invalid owner affiliation", Kind: "INVALID_AFFILIATION"}
		}
	}

	var startTime any
	if p.CommitsYear != nil {
		startTime = fmt.Sprintf("%d-01-01T00:00:00Z", *p.CommitsYear)
	}

	vars := map[string]any{
		"login":                   p.Username,
		"startTime":               startTime,
		"ownerAffiliations":       affiliations,
		"includeUserRepositories": p.ContribsIncludeOwnRepos,
	}

	data, err := c.GraphQL(ctx, statsQuery, vars)
	if err != nil {
		return cards.StatsData{}, err
	}

	var resp statsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return cards.StatsData{}, &Error{Message: "Could not parse the GitHub API response.", Kind: KindGraphQL}
	}
	if resp.User == nil {
		return cards.StatsData{}, &Error{Message: "Could not fetch user.", Kind: KindUserNotFound}
	}
	user := resp.User

	stats := cards.StatsData{Name: user.Name}
	if stats.Name == "" {
		stats.Name = user.Login
	}

	if p.IncludeAllCommits {
		count, err := c.restTotalItems(ctx, "commits", "author:"+p.Username)
		if err != nil {
			return cards.StatsData{}, err
		}
		stats.TotalCommits = count
	} else {
		stats.TotalCommits = user.Commits.TotalCommitContributions
	}

	stats.TotalPRs = user.PullRequests.TotalCount
	stats.TotalReviews = user.Reviews.TotalPullRequestReviewContributions
	stats.TotalIssues = user.OpenIssues.TotalCount + user.ClosedIssues.TotalCount
	stats.ContributedTo = user.RepositoriesContributedTo.TotalCount

	excluded := make(map[string]bool, len(p.ExcludeRepos))
	for _, r := range p.ExcludeRepos {
		excluded[r] = true
	}
	stars := 0.0
	for _, node := range user.Repositories.Nodes {
		if !excluded[node.Name] {
			stars += node.StargazerCount
		}
	}
	stats.TotalStars = math.Max(0, stars)

	stats.Rank = CalculateRank(
		p.IncludeAllCommits,
		stats.TotalCommits,
		stats.TotalPRs,
		stats.TotalIssues,
		stats.TotalReviews,
		stats.TotalStars,
		user.Followers.TotalCount,
	)

	return stats, nil
}
