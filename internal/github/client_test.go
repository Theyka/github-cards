package github

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"
)

func testLookupClient() *Client {
	return &Client{http: &http.Client{Timeout: time.Second}, rng: rand.New(rand.NewSource(1))}
}

// A canceled lookup (the client disconnected or gave up) must stay detectable
// through the Error wrapper so the server can refuse to cache the failure.
func TestGraphQLLookupErrorExposesCancellation(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := testLookupClient().graphqlOnce(canceled, "query { viewer { login } }", nil, "token", false)
	if err == nil {
		t.Fatal("a canceled context must fail the lookup")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(err, context.Canceled) = false for %v", err)
	}
	if !strings.Contains(err.Error(), "Failed to reach the GitHub API") {
		t.Fatalf("unexpected message: %v", err)
	}
	var ghErr *Error
	if !errors.As(err, &ghErr) || ghErr.Err == nil {
		t.Fatalf("underlying transport error must be preserved: %#v", err)
	}
}

func TestGraphQLLookupErrorExposesDeadline(t *testing.T) {
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, _, err := testLookupClient().graphqlOnce(expired, "query { viewer { login } }", nil, "token", false)
	if err == nil {
		t.Fatal("an expired deadline must fail the lookup")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("errors.Is(err, context.DeadlineExceeded) = false for %v", err)
	}
}

// Deterministic API errors carry no transport error, so they keep the regular
// error TTL instead of looking like transient network trouble.
func TestGraphQLAPIErrorsCarryNoTransportError(t *testing.T) {
	err := &Error{Message: "Not Found", Kind: KindUserNotFound}
	var ghErr *Error
	if !errors.As(err, &ghErr) || ghErr.Err != nil {
		t.Fatal("API-level errors must not look transient")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("API-level errors must not look like cancellations")
	}
}
