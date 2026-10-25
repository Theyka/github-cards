package server

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data    []byte
	expires time.Time
}

// Cache is a small in-memory TTL cache with singleflight-style
// deduplication of concurrent computations for the same key.
type Cache struct {
	mu       sync.Mutex
	entries  map[string]cacheEntry
	inflight map[string]chan struct{}
}

// NewCache creates an empty cache.
func NewCache() *Cache {
	return &Cache{
		entries:  map[string]cacheEntry{},
		inflight: map[string]chan struct{}{},
	}
}

// GetOrCompute returns the cached value for key, or computes it with fn.
// fn returns the data plus the TTL it should be cached with (0 = no caching).
func (c *Cache) GetOrCompute(key string, fn func() ([]byte, time.Duration)) []byte {
	for {
		c.mu.Lock()
		if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expires) {
			c.mu.Unlock()
			return entry.data
		}
		if ch, ok := c.inflight[key]; ok {
			c.mu.Unlock()
			<-ch
			continue
		}
		ch := make(chan struct{})
		c.inflight[key] = ch
		c.mu.Unlock()

		data, ttl := fn()

		c.mu.Lock()
		if ttl > 0 {
			// Crude guard against unbounded growth from unique query strings.
			if len(c.entries) > 10000 {
				c.entries = map[string]cacheEntry{}
			}
			c.entries[key] = cacheEntry{data: data, expires: time.Now().Add(ttl)}
		} else {
			delete(c.entries, key)
		}
		delete(c.inflight, key)
		close(ch)
		c.mu.Unlock()
		return data
	}
}
