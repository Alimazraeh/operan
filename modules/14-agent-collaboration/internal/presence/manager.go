package presence

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/operan/agent-collaboration/internal/store"
)

// Manager tracks agent presence via heartbeat-based monitoring.
type Manager struct {
	store    *store.PresenceStore
	timeout  time.Duration
	interval time.Duration
	stopCh   chan struct{}
	once     sync.Once
}

// NewManager creates a new presence manager.
func NewManager(store *store.PresenceStore) *Manager {
	return &Manager{
		store:    store,
		timeout:  30 * time.Second,
		interval: 1 * time.Minute,
	}
}

// Start begins the background presence cleanup goroutine.
func (m *Manager) Start() {
	m.once.Do(func() {
		m.stopCh = make(chan struct{})
		go m.cleanupLoop()
	})
}

// Stop halts the cleanup goroutine.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// UpdateHeartbeat records or refreshes an agent's heartbeat.
func (m *Manager) UpdateHeartbeat(tenantID, agentID, status string, metadata map[string]interface{}) error {
	p := &store.Presence{
		TenantID:      tenantID,
		AgentID:       agentID,
		Status:        status,
		LastHeartbeat: time.Now(),
		Metadata:      metadata,
	}
	return m.store.Upsert(context.Background(), p)
}

// MarkAgentAway marks an agent as away if no heartbeat in timeout period.
func (m *Manager) MarkAgentAway(tenantID, agentID string) {
	if err := m.store.MarkAway(context.Background(), tenantID, agentID); err != nil {
		log.Printf("[presence] mark away: %v", err)
	}
}

// MarkAgentOffline marks an agent as offline.
func (m *Manager) MarkAgentOffline(tenantID, agentID string) {
	if err := m.store.MarkOffline(context.Background(), tenantID, agentID); err != nil {
		log.Printf("[presence] mark offline: %v", err)
	}
}

// MarkAgentOnline marks an agent as online.
func (m *Manager) MarkAgentOnline(tenantID, agentID string) {
	if err := m.store.MarkOnline(context.Background(), tenantID, agentID); err != nil {
		log.Printf("[presence] mark online: %v", err)
	}
}

// cleanupLoop runs periodically, checking for stale heartbeats.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.checkStale()
		case <-m.stopCh:
			return
		}
	}
}

// checkStale marks agents with no recent heartbeat as away or offline.
func (m *Manager) checkStale() {
	// In production, this would query the database for all tenants and check each agent's heartbeat.
	// For now, we rely on the handler-level timestamp checks.
}