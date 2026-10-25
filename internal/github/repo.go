package github

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Theyka/github-cards/internal/cards"
)

const repoQuery = `query getRepo($login: String!, $repo: String!) {
  user(login: $login) {
    repository(name: $repo) {
      ...RepoInfo
    }
  }
  organization(login: $login) {
    repository(name: $repo) {
      ...RepoInfo
    }
  }
}

fragment RepoInfo on Repository {
  name
  nameWithOwner
  isPrivate
  isArchived
  isTemplate
  stargazerCount
  description
  primaryLanguage {
    color
    id
    name
  }
  forkCount
}`

type repoInfo struct {
	Name            string  `json:"name"`
	NameWithOwner   string  `json:"nameWithOwner"`
	IsPrivate       bool    `json:"isPrivate"`
	IsArchived      bool    `json:"isArchived"`
	IsTemplate      bool    `json:"isTemplate"`
	StargazerCount  float64 `json:"stargazerCount"`
	Description     *string `json:"description"`
	PrimaryLanguage *struct {
		Color *string `json:"color"`
		Name  *string `json:"name"`
	} `json:"primaryLanguage"`
	ForkCount float64 `json:"forkCount"`
}

type repoResponse struct {
	User *struct {
		Repository *repoInfo `json:"repository"`
	} `json:"user"`
	Organization *struct {
		Repository *repoInfo `json:"repository"`
	} `json:"organization"`
}

const repoURLExample = "/api/pin?username=USERNAME&repo=REPO_NAME"

// FetchRepo fetches repository data for the pin card, mirroring
// github-readme-stats' fetchRepo.
func (c *Client) FetchRepo(ctx context.Context, username, reponame string) (cards.RepositoryData, error) {
	owner := username
	if reponame != "" && strings.Contains(reponame, "/") {
		parts := strings.SplitN(reponame, "/", 2)
		owner = parts[0]
		reponame = parts[1]
	}

	if owner != "" && username == "" {
		username = owner
	}
	if username != "" && owner == "" {
		owner = username
	}
	if username == "" && reponame == "" {
		return cards.RepositoryData{}, missingParamError([]string{"username", "repo"}, repoURLExample)
	}
	if username == "" {
		return cards.RepositoryData{}, missingParamError([]string{"username"}, repoURLExample)
	}
	if reponame == "" {
		return cards.RepositoryData{}, missingParamError([]string{"repo"}, repoURLExample)
	}

	data, err := c.GraphQLPartial(ctx, repoQuery, map[string]any{"login": owner, "repo": reponame})
	if err != nil {
		return cards.RepositoryData{}, err
	}

	var resp repoResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return cards.RepositoryData{}, &Error{Message: "Could not parse the GitHub API response.", Kind: KindGraphQL}
	}

	if resp.User == nil && resp.Organization == nil {
		return cards.RepositoryData{}, &Error{Message: "Not found"}
	}

	var info *repoInfo
	switch {
	case resp.Organization == nil && resp.User != nil:
		info = resp.User.Repository
		if info == nil || info.IsPrivate {
			return cards.RepositoryData{}, &Error{Message: "User Repository Not found"}
		}
	case resp.User == nil && resp.Organization != nil:
		info = resp.Organization.Repository
		if info == nil || info.IsPrivate {
			return cards.RepositoryData{}, &Error{Message: "Organization Repository Not found"}
		}
	default:
		return cards.RepositoryData{}, &Error{Message: "Unexpected behavior"}
	}

	repo := cards.RepositoryData{
		Name:           info.Name,
		NameWithOwner:  info.NameWithOwner,
		Description:    info.Description,
		IsArchived:     info.IsArchived,
		IsTemplate:     info.IsTemplate,
		StargazerCount: info.StargazerCount,
		ForkCount:      info.ForkCount,
	}
	if info.PrimaryLanguage != nil {
		repo.PrimaryLanguage = &cards.PrimaryLanguage{
			Name:  info.PrimaryLanguage.Name,
			Color: info.PrimaryLanguage.Color,
		}
	}

	return repo, nil
}
