package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache(5 * time.Minute)
	result := &EvaluateResult{Allowed: true, Action: "allow"}
	c.Set("key1", result)
	got, ok := c.Get("key1")
	assert.True(t, ok)
	assert.True(t, got.Allowed)
}

func TestCache_GetMiss(t *testing.T) {
	c := NewCache(5 * time.Minute)
	_, ok := c.Get("nonexistent")
	assert.False(t, ok)
}

func TestCache_Invalidate(t *testing.T) {
	c := NewCache(5 * time.Minute)
	c.Set("key1", &EvaluateResult{Allowed: true})
	c.Invalidate("key1")
	_, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(5 * time.Minute)
	c.Set("key1", &EvaluateResult{Allowed: true})
	c.Set("key2", &EvaluateResult{Allowed: false})
	c.Clear()
	_, ok1 := c.Get("key1")
	_, ok2 := c.Get("key2")
	assert.False(t, ok1)
	assert.False(t, ok2)
}

func TestCache_Size(t *testing.T) {
	c := NewCache(5 * time.Minute)
	assert.Equal(t, 0, c.Size())
	c.Set("key1", &EvaluateResult{Allowed: true})
	assert.Equal(t, 1, c.Size())
	c.Set("key2", &EvaluateResult{Allowed: false})
	assert.Equal(t, 2, c.Size())
}

func TestCache_TTL_Expiry(t *testing.T) {
	c := NewCache(1 * time.Millisecond)
	c.Set("key1", &EvaluateResult{Allowed: true})
	time.Sleep(10 * time.Millisecond) // Wait for expiry
	_, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestCache_Overwrite(t *testing.T) {
	c := NewCache(5 * time.Minute)
	c.Set("key1", &EvaluateResult{Allowed: true, Action: "allow"})
	c.Set("key1", &EvaluateResult{Allowed: false, Action: "deny"})
	got, _ := c.Get("key1")
	assert.False(t, got.Allowed)
	assert.Equal(t, "deny", got.Action)
}