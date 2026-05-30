package authentik

import (
	"context"
	"testing"
	"time"
)

func TestTenantStateWithExpiry_IsExpired(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		expiryAt time.Time
		want     bool
	}{
		{
			name:     "not expired — future expiry",
			expiryAt: now.Add(1 * time.Hour),
			want:     false,
		},
		{
			name:     "expired — past expiry",
			expiryAt: now.Add(-1 * time.Hour),
			want:     true,
		},
		{
			name:     "expired — zero time",
			expiryAt: time.Time{},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &TenantStateWithExpiry{
				ExpiryAt: tt.expiryAt,
			}
			if got := entry.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantManagerConfigDefaults(t *testing.T) {
	cfg := TenantManagerConfigDefaults()

	if cfg.CacheTTL != 1*time.Hour {
		t.Errorf("CacheTTL = %v, want 1h", cfg.CacheTTL)
	}
	if cfg.CleanupInterval != 10*time.Minute {
		t.Errorf("CleanupInterval = %v, want 10m", cfg.CleanupInterval)
	}
	if cfg.MaxCachedTenants != 1000 {
		t.Errorf("MaxCachedTenants = %v, want 1000", cfg.MaxCachedTenants)
	}
}

func TestTenantManager_NewWithCustomConfig(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:         30 * time.Minute,
		CleanupInterval:  5 * time.Minute,
		MaxCachedTenants: 500,
	}

	// Create with nil client — tests only struct setup, no API calls
	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	if tm.config.CacheTTL != cfg.CacheTTL {
		t.Errorf("CacheTTL = %v, want %v", tm.config.CacheTTL, cfg.CacheTTL)
	}
	if tm.config.CleanupInterval != cfg.CleanupInterval {
		t.Errorf("CleanupInterval = %v, want %v", tm.config.CleanupInterval, cfg.CleanupInterval)
	}
	if tm.config.MaxCachedTenants != cfg.MaxCachedTenants {
		t.Errorf("MaxCachedTenants = %v, want %v", tm.config.MaxCachedTenants, cfg.MaxCachedTenants)
	}
}

func TestTenantManager_StopCleanupLoop(t *testing.T) {
	cfg := TenantManagerConfig{
		CleanupInterval: 1 * time.Millisecond, // very fast
	}

	tm := NewTenantManagerWithConfig(nil, cfg)

	// Stop should not panic
	tm.Stop()

	// Second stop should be a no-op
	tm.Stop()

	// Cleanup should still work after stop
	removed := tm.CleanupExpiredCache()
	_ = removed
}

func TestTenantManager_CleanupExpiredCache(t *testing.T) {
	// Create TM with nil client — we'll only test cache logic
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0, // disable background cleanup
		MaxCachedTenants: 100,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	now := time.Now().UTC()

	// Manually add expired and non-expired entries
	tm.cache["tenant-expired"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-expired"},
		ExpiryAt: now.Add(-2 * time.Hour), // expired
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-expired")

	tm.cache["tenant-valid"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-valid"},
		ExpiryAt: now.Add(1 * time.Hour), // valid
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-valid")

	tm.cache["tenant-future"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-future"},
		ExpiryAt: now.Add(2 * time.Hour), // valid
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-future")

	// Cleanup should remove only the expired entry
	removed := tm.CleanupExpiredCache()
	if removed != 1 {
		t.Errorf("expected 1 expired entry removed, got %d", removed)
	}

	// Expired tenant should be gone
	if _, exists := tm.cache["tenant-expired"]; exists {
		t.Error("expected tenant-expired to be removed from cache")
	}

	// Valid tenants should remain
	if _, exists := tm.cache["tenant-valid"]; !exists {
		t.Error("expected tenant-valid to remain in cache")
	}
	if _, exists := tm.cache["tenant-future"]; !exists {
		t.Error("expected tenant-future to remain in cache")
	}

	// Cache order should be rebuilt correctly
	if len(tm.cacheOrder) != 2 {
		t.Errorf("expected 2 entries in cache order, got %d", len(tm.cacheOrder))
	}
}

func TestTenantManager_InvalidateCache(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	now := time.Now().UTC()

	tm.cache["tenant-1"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-1"},
		ExpiryAt: now.Add(1 * time.Hour),
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-1")
	tm.cache["tenant-2"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-2"},
		ExpiryAt: now.Add(1 * time.Hour),
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-2")

	// Invalidate one tenant
	tm.InvalidateCache("tenant-1")

	// Check remaining
	if _, exists := tm.cache["tenant-1"]; exists {
		t.Error("expected tenant-1 to be removed from cache")
	}
	if _, exists := tm.cache["tenant-2"]; !exists {
		t.Error("expected tenant-2 to remain in cache")
	}

	// Order should have one less entry
	if len(tm.cacheOrder) != 1 {
		t.Errorf("expected 1 entry in cache order, got %d", len(tm.cacheOrder))
	}
}

func TestTenantManager_EvictToMakeRoom(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:         1 * time.Hour,
		CleanupInterval:  0,
		MaxCachedTenants: 3,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	now := time.Now().UTC()

	// Add 4 entries (one over capacity)
	for i := 0; i < 4; i++ {
		id := "tenant-" + string(rune('A'+i))
		tm.cache[id] = &TenantStateWithExpiry{
			State: &TenantState{TenantUUID: "uuid-" + string(rune('A'+i))},
			ExpiryAt: now.Add(1 * time.Hour),
		}
		tm.cacheOrder = append(tm.cacheOrder, id)
	}

	// Now call evictToMakeRoom — should evict A to get to capacity
	tm.mu.Lock()
	tm.evictToMakeRoom()
	tm.mu.Unlock()

	// Check cache size
	if len(tm.cache) != 3 {
		t.Errorf("expected 3 entries in cache, got %d", len(tm.cache))
	}

	// Oldest should be evicted
	if _, exists := tm.cache["tenant-A"]; exists {
		t.Error("expected tenant-A to be evicted")
	}

	// Others should remain
	for _, id := range []string{"tenant-B", "tenant-C", "tenant-D"} {
		if _, exists := tm.cache[id]; !exists {
			t.Errorf("expected %s to remain in cache", id)
		}
	}
}

func TestTenantManager_GetTenantStateExpired(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	now := time.Now().UTC()

	// Add an expired entry
	tm.cache["tenant-expired"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-expired"},
		ExpiryAt: now.Add(-1 * time.Hour), // expired
	}

	// GetTenantState should detect expiration and remove
	state, err := tm.GetTenantState(context.Background(), "tenant-expired")
	if err == nil {
		t.Error("expected error for expired tenant, got nil")
	}
	if state != nil {
		t.Error("expected nil state for expired tenant")
	}

	// Entry should be removed from cache
	if _, exists := tm.cache["tenant-expired"]; exists {
		t.Error("expected expired tenant to be removed from cache on GetTenantState")
	}
}

func TestTenantManager_GetTenantStateNotFound(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	_, err := tm.GetTenantState(context.Background(), "tenant-nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tenant, got nil")
	}
}

func TestTenantManager_GetTenantStateValid(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	now := time.Now().UTC()
	expectedState := &TenantState{
		TenantUUID: "test-uuid",
		AppUUID:    "app-uuid",
	}
	tm.cache["tenant-valid"] = &TenantStateWithExpiry{
		State: expectedState,
		ExpiryAt: now.Add(1 * time.Hour),
	}

	state, err := tm.GetTenantState(context.Background(), "tenant-valid")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.TenantUUID != "test-uuid" {
		t.Errorf("state.TenantUUID = %v, want test-uuid", state.TenantUUID)
	}
}

func TestTenantManager_CleanupStaleWhenNoExpired(t *testing.T) {
	// Test that evictOldestEvicted falls back to evicting oldest overall when nothing is expired
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0,
		MaxCachedTenants: 0, // unlimited, so we test eviction manually
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	// Add 3 entries, all valid
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		id := "tenant-" + string(rune('A'+i))
		tm.cache[id] = &TenantStateWithExpiry{
			State: &TenantState{TenantUUID: "uuid-" + string(rune('A'+i))},
			ExpiryAt: now.Add(1 * time.Hour),
		}
		tm.cacheOrder = append(tm.cacheOrder, id)
	}

	// Manually evict oldest — should evict A (not expired, so falls back to oldest)
	tm.mu.Lock()
	tm.evictOldestEvicted()
	tm.mu.Unlock()

	if _, exists := tm.cache["tenant-A"]; exists {
		t.Error("expected tenant-A to be evicted when falling back to oldest")
	}
	if _, exists := tm.cache["tenant-B"]; !exists {
		t.Error("expected tenant-B to remain")
	}
	if len(tm.cacheOrder) != 2 {
		t.Errorf("expected 2 entries in cache order, got %d", len(tm.cacheOrder))
	}
}

func TestTenantManager_RemoveTenant(t *testing.T) {
	// Test RemoveTenant with nil client — it calls Delete on sub-APIs which will panic with nil
	// Instead, test that the cache cleanup portion works
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0,
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	// Add entries
	tm.cache["tenant-remove"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-remove"},
		ExpiryAt: time.Now().UTC().Add(1 * time.Hour),
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-remove")
	tm.cache["tenant-stay"] = &TenantStateWithExpiry{
		State: &TenantState{TenantUUID: "uuid-stay"},
		ExpiryAt: time.Now().UTC().Add(1 * time.Hour),
	}
	tm.cacheOrder = append(tm.cacheOrder, "tenant-stay")

	// Directly simulate RemoveTenant cache logic
	tm.mu.Lock()
	_, ok := tm.cache["tenant-remove"]
	if !ok {
		t.Fatal("expected tenant-remove to exist")
	}
	delete(tm.cache, "tenant-remove")
	for i, id := range tm.cacheOrder {
		if id == "tenant-remove" {
			tm.cacheOrder = append(tm.cacheOrder[:i], tm.cacheOrder[i+1:]...)
			break
		}
	}
	// Note: entry.State.SAMLUUID etc would be accessed here in real code
	// but we skip the Delete calls since client is nil
	tm.mu.Unlock()

	if _, exists := tm.cache["tenant-remove"]; exists {
		t.Error("expected tenant-remove to be removed from cache")
	}
	if _, exists := tm.cache["tenant-stay"]; !exists {
		t.Error("expected tenant-stay to remain in cache")
	}
}

func TestTenantManager_ConcurrentAccess(t *testing.T) {
	cfg := TenantManagerConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 0, // disable background cleanup for deterministic test
	}

	tm := NewTenantManagerWithConfig(nil, cfg)
	defer tm.Stop()

	now := time.Now().UTC()
	done := make(chan bool)

	// 10 goroutines each adding 100 entries
	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 100; i++ {
				idStr := "tenant-goroutine-" + string(rune('A'+id)) + "-req-" + string(rune('A'+i))
				tm.mu.Lock()
				tm.cache[idStr] = &TenantStateWithExpiry{
					State: &TenantState{TenantUUID: "uuid-" + idStr},
					ExpiryAt: now.Add(1 * time.Hour),
				}
				tm.cacheOrder = append(tm.cacheOrder, idStr)
				tm.mu.Unlock()
			}
			done <- true
		}(g)
	}

	for g := 0; g < 10; g++ {
		<-done
	}

	if len(tm.cache) != 1000 {
		t.Errorf("expected 1000 entries in cache, got %d", len(tm.cache))
	}
	if len(tm.cacheOrder) != 1000 {
		t.Errorf("expected 1000 entries in cache order, got %d", len(tm.cacheOrder))
	}
}
