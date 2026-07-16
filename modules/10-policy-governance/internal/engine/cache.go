package engine

import (
	"sync"
	"time"
)

// Cache is a simple in-memory cache for policy evaluation results.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	result  *EvaluateResult
	expired time.Time
}

// NewCache creates a new policy evaluation cache with the given TTL.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		items: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// Get retrieves a cached result by key.
func (c *Cache) Get(key string) (*EvaluateResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expired) {
		return nil, false
	}
	return entry.result, true
}

// Set stores a result in the cache.
func (c *Cache) Set(key string, result *EvaluateResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheEntry{
		result:  result,
		expired: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a specific cache entry.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all cache entries.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]cacheEntry)
}

// Size returns the number of cached entries.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}