package server

import (
	"testing"
	"time"
)

func TestGetOrComputeReportsTTL(t *testing.T) {
	cache := NewCache()
	calls := 0

	data, ttl := cache.GetOrCompute("card", func() ([]byte, time.Duration) {
		calls++
		return []byte("real card"), 90 * time.Second
	})
	if string(data) != "real card" || ttl != 90*time.Second || calls != 1 {
		t.Fatalf("first compute: data=%q ttl=%v calls=%d", data, ttl, calls)
	}

	// A cached card must be served with its remaining lifetime, not recomputed.
	data, ttl = cache.GetOrCompute("card", func() ([]byte, time.Duration) {
		calls++
		return []byte("should not be used"), time.Minute
	})
	if calls != 1 || string(data) != "real card" {
		t.Fatalf("cached card was recomputed: data=%q calls=%d", data, calls)
	}
	if ttl <= 0 || ttl > 90*time.Second {
		t.Fatalf("cached card reported TTL %v", ttl)
	}
}

func TestGetOrComputeDoesNotStoreZeroTTL(t *testing.T) {
	cache := NewCache()
	calls := 0
	compute := func() ([]byte, time.Duration) {
		calls++
		return []byte("error card"), 0
	}
	if _, ttl := cache.GetOrCompute("error", compute); ttl != 0 {
		t.Fatalf("uncached result reported TTL %v", ttl)
	}
	if _, ttl := cache.GetOrCompute("error", compute); ttl != 0 || calls != 2 {
		t.Fatalf("uncached result was stored: ttl=%v calls=%d", ttl, calls)
	}
}
