package engine

import (
	"sync"
	"time"
)

// ThrottleManager manages throttle state overrides.
type ThrottleManager struct {
	mu      sync.RWMutex
	states  map[string]string // tenantID -> "none"|"soft"|"hard"
	times   map[string]time.Time
}

// NewThrottleManager creates a new ThrottleManager.
func NewThrottleManager() *ThrottleManager {
	return &ThrottleManager{
		states: make(map[string]string),
		times:  make(map[string]time.Time),
	}
}

// SetState sets the throttle state for a tenant.
func (m *ThrottleManager) SetState(tenantID, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[tenantID] = state
	m.times[tenantID] = time.Now()
}

// GetState returns the throttle state for a tenant.
func (m *ThrottleManager) GetState(tenantID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[tenantID]
}

// GetThrottleInfo returns both state and timestamp for a tenant.
func (m *ThrottleManager) GetThrottleInfo(tenantID string) (state string, updatedAt time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[tenantID], m.times[tenantID]
}