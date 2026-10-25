package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Theyka/github-cards/internal/github"
)

// Regression test: a canceled or timed-out lookup used to be cached for the
// full error TTL, so a single client disconnect pinned "context canceled" as
// the card for that URL until the entry expired.
func TestErrorTTLNeverCachesCancellations(t *testing.T) {
	canceled := &github.Error{Message: "Failed to reach the GitHub API: context canceled", Kind: github.KindGraphQL, Err: context.Canceled}
	if got := errorTTL(canceled, defaultStatsTTL); got != 0 {
		t.Fatalf("canceled lookup cached for %v, want 0", got)
	}
	expired := &github.Error{Message: "Failed to reach the GitHub API: deadline exceeded", Kind: github.KindGraphQL, Err: context.DeadlineExceeded}
	if got := errorTTL(expired, defaultStreakTTL); got != 0 {
		t.Fatalf("timed-out lookup cached for %v, want 0", got)
	}
}

func TestErrorTTLCachesTransportFailuresOnlyBriefly(t *testing.T) {
	refused := &github.Error{Message: "Failed to reach the GitHub API: dial tcp: connection refused", Kind: github.KindGraphQL, Err: errors.New("dial tcp: connection refused")}
	if got := errorTTL(refused, defaultStatsTTL); got != transientErrorTTL {
		t.Fatalf("transport failure cached for %v, want %v", got, transientErrorTTL)
	}
	if got := errorTTL(refused, 30*time.Second); got != 30*time.Second {
		t.Fatalf("transport failure must respect a shorter configured TTL, got %v", got)
	}
	if got := errorTTL(refused, 0); got != 0 {
		t.Fatalf("transport failure with caching disabled cached for %v, want 0", got)
	}
}

func TestErrorTTLKeepsDeterministicErrors(t *testing.T) {
	notFound := &github.Error{Message: "Could not resolve to a User with the login of 'nobody'.", Kind: github.KindUserNotFound}
	if got := errorTTL(notFound, defaultStatsTTL); got != maxErrorTTL {
		t.Fatalf("missing user cached for %v, want %v", got, maxErrorTTL)
	}
	rateLimited := &github.Error{Message: "We are being rate-limited by the GitHub API", Kind: github.KindMaxRetry}
	if got := errorTTL(rateLimited, defaultStreakTTL); got != maxErrorTTL {
		t.Fatalf("rate limit cached for %v, want %v", got, maxErrorTTL)
	}
	if got := errorTTL(notFound, 0); got != 0 {
		t.Fatalf("error TTL with caching disabled cached for %v, want 0", got)
	}
}

// The lookup must survive the incoming request being canceled; otherwise the
// disconnect itself becomes the cached card.
func TestFetchContextOutlivesClientDisconnect(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/pin?username=Theyka&repo=Turnstile-Solver", nil)
	clientContext, cancelClient := context.WithCancel(request.Context())
	cancelClient()

	ctx, cancel := fetchContext(request.WithContext(clientContext))
	defer cancel()

	if err := ctx.Err(); err != nil {
		t.Fatalf("upstream context inherited the client's cancellation: %v", err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("upstream context must carry a deadline")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > upstreamFetchTimeout {
		t.Fatalf("unexpected upstream budget: %v", remaining)
	}
}

func TestWriteSVGUncachedResultsAreNoStore(t *testing.T) {
	uncached := httptest.NewRecorder()
	writeSVG(uncached, "<svg/>", 0)
	if got := uncached.Header().Get("Cache-Control"); got != "no-cache, no-store, max-age=0" {
		t.Fatalf("uncached card Cache-Control = %q", got)
	}
	cached := httptest.NewRecorder()
	writeSVG(cached, "<svg/>", time.Hour)
	if got := cached.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=3600") {
		t.Fatalf("cached card Cache-Control = %q", got)
	}
}
