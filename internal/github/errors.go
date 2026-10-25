// Package github fetches data from the GitHub GraphQL and REST APIs.
package github

import "strings"

// Error carries the upstream error semantics: a message, an error kind and an
// optional explicit secondary message.
type Error struct {
	Message   string
	Kind      string
	Secondary string
}

func (e *Error) Error() string { return e.Message }

// Error kinds, mirroring github-stats-extended common/error.ts.
const (
	KindMaxRetry     = "MAX_RETRY"
	KindNoTokens     = "NO_TOKENS"
	KindUserNotFound = "USER_NOT_FOUND"
	KindGraphQL      = "GRAPHQL_ERROR"
	KindRestAPI      = "GITHUB_REST_API_ERROR"
	KindMissingParam = "MISSING_PARAMS"
)

var secondaryMessages = map[string]string{
	KindMaxRetry:     "You can deploy own instance or wait until public will be no longer limited",
	KindNoTokens:     "Please set the GITHUB_TOKEN (or PAT_1 / TOKEN) environment variable on your instance",
	KindUserNotFound: "Make sure the provided username is not an organization",
	KindGraphQL:      "Please try again later",
	KindRestAPI:      "Please try again later",
	KindMissingParam: "",
}

// SecondaryMessage returns the secondary message to show on the error card.
func (e *Error) SecondaryMessage() string {
	if e.Secondary != "" {
		return e.Secondary
	}
	if msg, ok := secondaryMessages[e.Kind]; ok {
		return msg
	}
	if e.Kind == "" {
		return ""
	}
	return e.Kind
}

// missingParamError builds the upstream-style missing parameter error.
func missingParamError(params []string, secondary string) *Error {
	quoted := make([]string, 0, len(params))
	for _, p := range params {
		quoted = append(quoted, `"`+p+`"`)
	}
	return &Error{
		Message:   "Missing params " + strings.Join(quoted, ", ") + " make sure you pass the parameters in URL",
		Kind:      KindMissingParam,
		Secondary: secondary,
	}
}
