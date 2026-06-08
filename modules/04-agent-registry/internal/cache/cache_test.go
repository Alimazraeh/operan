package cache

import (
	"testing"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/store"
)

func makeAgent(id string) *store.Agent {
	return &store.Agent{
		ID:         id,
		Name:       "test-agent-" + id,
		TenantID:   "tenant-1",
		Status:     store.AgentStatusActive,
		Capabilities: []string{"chat", "analyze"},
	}
}

func TestGetSet(t *testing.T) {
	c := New()
	agent := makeAgent("a1")

	c.Set(agent)
	got := c.Get("a1")
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.Name != "test-agent-a1" {
		t.Errorf("expected test-agent-a1, got %s", got.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	c := New()
	got := c.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestUpdateExisting(t *testing.T) {
	c := New()
	agent := makeAgent("a1")
	c.Set(agent)

	// Update
	agent2 := makeAgent("a1")
	agent2.Capabilities = []string{"chat", "execute"}
	c.Set(agent2)

	got := c.Get("a1")
	if got == nil {
		t.Fatal("expected updated agent, got nil")
	}
	if len(got.Capabilities) != 2 || got.Capabilities[1] != "execute" {
		t.Errorf("expected [chat execute], got %v", got.Capabilities)
	}
}

func TestDelete(t *testing.T) {
	c := New()
	c.Set(makeAgent("a1"))
	c.Delete("a1")

	got := c.Get("a1")
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestDeleteNonexistent(t *testing.T) {
	c := New()
	// Should not panic
	c.Delete("nonexistent")
}

func TestClear(t *testing.T) {
	c := New()
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))
	c.Clear()

	if got := c.Len(); got != 0 {
		t.Errorf("expected 0 after clear, got %d", got)
	}
	if c.Get("a1") != nil {
		t.Error("expected a1 to be nil after clear")
	}
	if c.Get("a2") != nil {
		t.Error("expected a2 to be nil after clear")
	}
}

func TestLen(t *testing.T) {
	c := New()
	if got := c.Len(); got != 0 {
		t.Errorf("expected 0 initially, got %d", got)
	}

	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))
	c.Set(makeAgent("a3"))

	if got := c.Len(); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestKeys(t *testing.T) {
	c := New()
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))

	keys := c.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// Check both keys are present
	HasKey := func(s string) bool {
		for _, k := range keys {
			if k == s {
				return true
			}
		}
		return false
	}

	if !HasKey("a1") {
		t.Error("expected key a1")
	}
	if !HasKey("a2") {
		t.Error("expected key a2")
	}
}

func TestEviction(t *testing.T) {
	c := New(WithMaxSize(3))
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))
	c.Set(makeAgent("a3"))

	// Adding a4 should evict a1 (oldest)
	c.Set(makeAgent("a4"))

	if c.Get("a1") != nil {
		t.Error("expected a1 to be evicted")
	}
	if c.Get("a4") == nil {
		t.Error("expected a4 to be present")
	}
	if c.Len() != 3 {
		t.Errorf("expected len 3, got %d", c.Len())
	}
}

func TestEvictionOrder(t *testing.T) {
	c := New(WithMaxSize(3))
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))
	c.Set(makeAgent("a3"))

	// Update a1 to make it newer
	c.Set(makeAgent("a1"))

	// Adding a4 should evict a2 (oldest)
	c.Set(makeAgent("a4"))

	if c.Get("a1") == nil {
		t.Error("expected a1 to still be present (updated)")
	}
	if c.Get("a2") != nil {
		t.Error("expected a2 to be evicted")
	}
}

func TestEvictionCallback(t *testing.T) {
	var evictedKeys []string
	var evictedAgents []*store.Agent

	cb := func(key string, agent *store.Agent) {
		evictedKeys = append(evictedKeys, key)
		evictedAgents = append(evictedAgents, agent)
	}

	c := New(WithMaxSize(2), WithEvictionCallback(cb))
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))

	// This should evict a1
	c.Set(makeAgent("a3"))

	if len(evictedKeys) != 1 {
		t.Fatalf("expected 1 evicted key, got %d", len(evictedKeys))
	}
	if evictedKeys[0] != "a1" {
		t.Errorf("expected evicted key a1, got %s", evictedKeys[0])
	}
	if evictedAgents[0].ID != "a1" {
		t.Errorf("expected evicted agent a1, got %s", evictedAgents[0].ID)
	}
}

func TestClearCallsEvictionCallback(t *testing.T) {
	var evictedKeys []string

	cb := func(key string, agent *store.Agent) {
		evictedKeys = append(evictedKeys, key)
	}

	c := New(WithMaxSize(10), WithEvictionCallback(cb))
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))
	c.Clear()

	if len(evictedKeys) != 2 {
		t.Errorf("expected 2 evicted keys on clear, got %d", len(evictedKeys))
	}
}

func TestCustomTimeNow(t *testing.T) {
	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	oldTimeNow := timeNow
	timeNow = func() time.Time { return fixedTime }
	defer func() { timeNow = oldTimeNow }()

	c := New()
	c.Set(makeAgent("a1"))

	item := c.items["a1"]
	if !item.InsertedAt.Equal(fixedTime) {
		t.Errorf("expected inserted time %v, got %v", fixedTime, item.InsertedAt)
	}
}

func TestGetNonExistentAfterEviction(t *testing.T) {
	c := New(WithMaxSize(2))
	c.Set(makeAgent("a1"))
	c.Set(makeAgent("a2"))

	// Force eviction of a1
	c.Set(makeAgent("a3"))

	// a1 should be gone
	if c.Get("a1") != nil {
		t.Error("expected a1 to be nil after eviction")
	}

	// a2 should still be there
	if c.Get("a2") == nil {
		t.Error("expected a2 to still be present")
	}
}

func TestSetLargeNumber(t *testing.T) {
	c := New(WithMaxSize(100))
	for i := 0; i < 200; i++ {
		c.Set(makeAgent(string(rune('a' + i%26))))
	}

	// Should not exceed max size
	if c.Len() > 100 {
		t.Errorf("expected len <= 100, got %d", c.Len())
	}
}
