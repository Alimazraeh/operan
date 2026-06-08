// Package cache provides an in-memory LRU cache for agent data with event-driven
// invalidation support. It is thread-safe and suitable for single-process caching
// of frequently-looked-up agent records.
package cache

import (
	"sync"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/store"
)

// timeNow is a variable for testability.
var timeNow = time.Now

// ─── Item ─────────────────────────────────────────────────────────────────────

// Item is a cached value with its insertion timestamp.
type Item struct {
	Value     *store.Agent
	InsertedAt time.Time
}

// ─── Cache ────────────────────────────────────────────────────────────────────

// Cache is a thread-safe in-memory LRU cache of agents keyed by agent ID.
type Cache struct {
	mu         sync.RWMutex
	items      map[string]*Item
	maxSize    int
	onEvicted  func(key string, value *store.Agent)
}

// Option is a functional option for configuring the Cache.
type Option func(*Cache)

// WithMaxSize sets the maximum number of items in the cache.
// Defaults to 1000 if not specified.
func WithMaxSize(maxSize int) Option {
	return func(c *Cache) {
		c.maxSize = maxSize
	}
}

// WithEvictionCallback sets a callback that is called when an item is evicted.
func WithEvictionCallback(cb func(key string, value *store.Agent)) Option {
	return func(c *Cache) {
		c.onEvicted = cb
	}
}

// New creates a new Cache with the given options.
func New(opts ...Option) *Cache {
	c := &Cache{
		items:   make(map[string]*Item),
		maxSize: 1000,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get retrieves an agent by ID. Returns nil if not found.
func (c *Cache) Get(agentID string) *store.Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[agentID]
	if !ok {
		return nil
	}
	return item.Value
}

// Set adds or updates an agent in the cache.
// If the cache is full, it evicts the least recently used item.
func (c *Cache) Set(agent *store.Agent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already exists, update in place (move to front)
	if item, ok := c.items[agent.ID]; ok {
		item.Value = agent
		item.InsertedAt = timeNow()
		return
	}

	// Evict if necessary
	if len(c.items) >= c.maxSize {
		c.evict()
	}

	c.items[agent.ID] = &Item{
		Value:      agent,
		InsertedAt: timeNow(),
	}
}

// Delete removes an agent from the cache.
func (c *Cache) Delete(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[agentID]; ok {
		if c.onEvicted != nil {
			c.onEvicted(agentID, item.Value)
		}
		delete(c.items, agentID)
	}
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, item := range c.items {
		if c.onEvicted != nil {
			c.onEvicted(id, item.Value)
		}
	}
	c.items = make(map[string]*Item)
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Keys returns all keys in the cache.
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// evict removes the least recently used item (oldest InsertedAt).
// Must be called with c.mu.Lock held.
func (c *Cache) evict() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, item := range c.items {
		if first || item.InsertedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.InsertedAt
			first = false
		}
	}

	if !first {
		if c.onEvicted != nil {
			c.onEvicted(oldestKey, c.items[oldestKey].Value)
		}
		delete(c.items, oldestKey)
	}
}
