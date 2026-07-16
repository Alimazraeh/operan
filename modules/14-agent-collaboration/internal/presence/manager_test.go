package presence

import (
	"testing"
	"time"

	"github.com/operan/agent-collaboration/internal/store"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newPresenceManager(t *testing.T) (*Manager, *store.PresenceStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	ps := store.NewPresenceStore(mockPool)
	mgr := NewManager(ps)
	return mgr, ps, mockPool
}

func TestNewManager_Defaults(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mockPool.Close()

	ps := store.NewPresenceStore(mockPool)
	mgr := NewManager(ps)

	assert.NotNil(t, mgr)
	assert.Equal(t, 30*time.Second, mgr.timeout)
	assert.Equal(t, 1*time.Minute, mgr.interval)
	assert.NotNil(t, mgr.store)
}

func TestManager_UpdateHeartbeat(t *testing.T) {
	mgr, ps, mockPool := newPresenceManager(t)
	_ = ps

	// Expect the Upsert call from UpdateHeartbeat
	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-1", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := mgr.UpdateHeartbeat("tenant-1", "agent-1", "online", map[string]interface{}{"last_task": "classify"})
	assert.NoError(t, err)
}

func TestManager_UpdateHeartbeat_WithMetadata(t *testing.T) {
	mgr, ps, mockPool := newPresenceManager(t)
	_ = ps

	// Upsert has 5 args: tenant_id, agent_id, status, last_heartbeat, metadata
	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-2", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := mgr.UpdateHeartbeat("tenant-1", "agent-2", "online", map[string]interface{}{
		"current_task": "translate",
		"priority":     "high",
	})
	assert.NoError(t, err)
}

func TestManager_MarkAgentAway(t *testing.T) {
	mgr, ps, mockPool := newPresenceManager(t)
	_ = ps

	// MarkAway args: tenantID, agentID
	mockPool.ExpectExec("UPDATE presence SET status = 'away'").
		WithArgs("tenant-1", "agent-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Should not panic or error
	mgr.MarkAgentAway("tenant-1", "agent-1")
}

func TestManager_MarkAgentOffline(t *testing.T) {
	mgr, ps, mockPool := newPresenceManager(t)
	_ = ps

	// MarkOffline args: tenantID, agentID
	mockPool.ExpectExec("UPDATE presence SET status = 'offline'").
		WithArgs("tenant-1", "agent-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mgr.MarkAgentOffline("tenant-1", "agent-1")
}

func TestManager_MarkAgentOnline(t *testing.T) {
	mgr, ps, mockPool := newPresenceManager(t)
	_ = ps

	// MarkOnline calls Upsert internally (INSERT ... ON CONFLICT)
	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-1", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mgr.MarkAgentOnline("tenant-1", "agent-1")
}

func TestManager_StartStop(t *testing.T) {
	mgr, _, _ := newPresenceManager(t)

	// Start should create a goroutine
	mgr.Start()
	assert.NotNil(t, mgr.stopCh)

	// Stop should close the channel (channel remains non-nil but is closed)
	mgr.Stop()

	// Start again is a no-op because sync.Once already fired,
	// but it should not panic
	mgr.Start()
	assert.NotNil(t, mgr.stopCh)
}

func TestManager_StartMultipleCalls(t *testing.T) {
	mgr, _, _ := newPresenceManager(t)

	// Multiple Start calls should only create one goroutine
	mgr.Start()
	mgr.Start()
	mgr.Start()

	// stopCh should be non-nil and only one goroutine was started
	assert.NotNil(t, mgr.stopCh)
}