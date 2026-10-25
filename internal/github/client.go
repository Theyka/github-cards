package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	graphqlURL      = "https://api.github.com/graphql"
	restURL         = "https://api.github.com"
	maxResponseSize = 20 << 20
)

// Client talks to the GitHub GraphQL and REST APIs with a token pool.
type Client struct {
	http   *http.Client
	mu     sync.Mutex
	tokens []string
	rng    *rand.Rand
}

// NewClient creates a client from the environment tokens (GITHUB_TOKEN,
// PAT_1..PAT_9, TOKEN; each may be a comma-separated list).
func NewClient() *Client {
	seen := map[string]bool{}
	var tokens []string
	add := func(value string) {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				tokens = append(tokens, part)
			}
		}
	}
	add(os.Getenv("GITHUB_TOKEN"))
	add(os.Getenv("PAT_1"))
	add(os.Getenv("TOKEN"))
	for i := 2; i <= 9; i++ {
		add(os.Getenv(fmt.Sprintf("PAT_%d", i)))
		add(os.Getenv(fmt.Sprintf("TOKEN%d", i)))
	}

	return &Client{
		http:   &http.Client{Timeout: 20 * time.Second},
		tokens: tokens,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// HasTokens reports whether at least one token is configured.
func (c *Client) HasTokens() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tokens) > 0
}

func (c *Client) pickToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.tokens) == 0 {
		return "", &Error{Message: "No GitHub API token configured", Kind: KindNoTokens}
	}
	return c.tokens[c.rng.Intn(len(c.tokens))], nil
}

type graphqlEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"errors"`
}

// GraphQL executes a GraphQL query and returns its data payload. Tokens are
// tried in random order; rate-limited or rejected tokens are retried with the
// next configured token instead of being dropped from the pool.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	return c.graphql(ctx, query, variables, false)
}

// GraphQLPartial is like GraphQL but tolerates GraphQL errors when the
// response still carries data. GitHub returns partial data plus errors for
// queries that resolve one field and miss another (e.g. an organization that
// matches no user login), which upstream effectively ignores.
func (c *Client) GraphQLPartial(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	return c.graphql(ctx, query, variables, true)
}

func (c *Client) graphql(ctx context.Context, query string, variables map[string]any, allowPartial bool) (json.RawMessage, error) {
	c.mu.Lock()
	tokens := append([]string(nil), c.tokens...)
	c.rng.Shuffle(len(tokens), func(i, j int) { tokens[i], tokens[j] = tokens[j], tokens[i] })
	c.mu.Unlock()

	if len(tokens) == 0 {
		return nil, &Error{Message: "No GitHub API token configured", Kind: KindNoTokens}
	}
	maxAttempts := len(tokens)
	if maxAttempts > 3 {
		maxAttempts = 3
	}

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		data, retryable, err := c.graphqlOnce(ctx, query, variables, tokens[i], allowPartial)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) graphqlOnce(ctx context.Context, query string, variables map[string]any, token string, allowPartial bool) (json.RawMessage, bool, error) {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v4.idl")
	req.Header.Set("User-Agent", "github-cards")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, &Error{Message: "Failed to reach the GitHub API: " + err.Error(), Kind: KindGraphQL}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, false, &Error{Message: "Failed to read the GitHub API response.", Kind: KindGraphQL}
	}

	var envelope graphqlEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, &Error{Message: fmt.Sprintf("Could not parse the GitHub API response (HTTP %d).", resp.StatusCode), Kind: KindGraphQL}
	}

	if len(envelope.Errors) > 0 {
		first := envelope.Errors[0]
		lower := strings.ToLower(first.Message)

		if strings.Contains(lower, "rate limit") || strings.Contains(lower, "rate-limit") {
			return nil, true, &Error{
				Message: "We are being rate-limited by the GitHub API",
				Kind:    KindMaxRetry,
			}
		}

		if allowPartial {
			trimmed := bytes.TrimSpace(envelope.Data)
			if len(trimmed) > 0 && string(trimmed) != "null" {
				return envelope.Data, false, nil
			}
		}

		if first.Type == "NOT_FOUND" {
			msg := first.Message
			if msg == "" {
				msg = "Could not fetch the requested data from GitHub."
			}
			return nil, false, &Error{Message: msg, Kind: KindUserNotFound}
		}

		if first.Message != "" {
			return nil, false, &Error{Message: truncateMessage(first.Message), Kind: KindGraphQL}
		}
		return nil, false, &Error{Message: "Something went wrong while trying to retrieve the data using the GraphQL API.", Kind: KindGraphQL}
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, true, &Error{Message: "Bad credentials: the configured GitHub token is invalid."}
	}
	if resp.StatusCode >= 400 {
		return nil, true, &Error{Message: fmt.Sprintf("GitHub API error (HTTP %d).", resp.StatusCode), Kind: KindGraphQL}
	}

	return envelope.Data, false, nil
}

// truncateMessage shortens long GitHub messages like upstream's wrap.
func truncateMessage(msg string) string {
	const max = 110
	runes := []rune(msg)
	if len(runes) <= max {
		return msg
	}
	return string(runes[:max]) + "..."
}

// restTotalItems fetches a total_count from the GitHub REST search API.
func (c *Client) restTotalItems(ctx context.Context, searchType, qualifier string) (float64, error) {
	token, err := c.pickToken()
	if err != nil {
		return 0, err
	}

	q := url.QueryEscape(qualifier)
	q = strings.ReplaceAll(q, "+", "%20")
	endpoint := restURL + "/search/" + searchType + "?per_page=1&q=" + q

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github.cloak-preview")
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("User-Agent", "github-cards")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, &Error{Message: "Failed to reach the GitHub REST API: " + err.Error(), Kind: KindRestAPI}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return 0, &Error{Message: "Failed to read the GitHub REST API response.", Kind: KindRestAPI}
	}

	var payload struct {
		TotalCount *float64 `json:"total_count"`
		Message    string   `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.TotalCount == nil {
		return 0, &Error{Message: "Could not fetch data from GitHub REST API.", Kind: KindRestAPI}
	}
	return *payload.TotalCount, nil
}
