package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newPresenceTestStore(t *testing.T) (*PresenceStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewPresenceStore(mockPool), mockPool
}

func TestPresenceStore_Upsert(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-1", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	p := &Presence{
		TenantID:      "tenant-1",
		AgentID:       "agent-1",
		Status:        "online",
		LastHeartbeat: now,
	}

	err := store.Upsert(ctx, p)
	assert.NoError(t, err)
	assert.Equal(t, "online", p.Status)
	assert.Equal(t, now, p.LastHeartbeat)
}

func TestPresenceStore_Upsert_WithMetadata(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-2", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	p := &Presence{
		TenantID:      "tenant-1",
		AgentID:       "agent-2",
		Status:        "online",
		LastHeartbeat: now,
		Metadata:      map[string]interface{}{"last_task": "classify"},
	}

	err := store.Upsert(ctx, p)
	assert.NoError(t, err)
	assert.Equal(t, "classify", p.Metadata["last_task"])
}

func TestPresenceStore_Upsert_Update(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-1", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	p := &Presence{
		TenantID:      "tenant-1",
		AgentID:       "agent-1",
		Status:        "online",
		LastHeartbeat: time.Now(),
	}

	err := store.Upsert(ctx, p)
	assert.NoError(t, err)
}

func TestPresenceStore_GetByAgentID(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM presence WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "status", "last_heartbeat", "metadata",
		}).
			AddRow(1, "tenant-1", "agent-1", "online", now, []byte(`{"last_task":"classify"}`)))

	p, err := store.GetByAgentID(ctx, "tenant-1", "agent-1")
	assert.NoError(t, err)
	assert.Equal(t, "agent-1", p.AgentID)
	assert.Equal(t, "tenant-1", p.TenantID)
	assert.Equal(t, "online", p.Status)
	assert.Equal(t, "classify", p.Metadata["last_task"])
}

func TestPresenceStore_GetByAgentID_NotFound(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT.*FROM presence WHERE tenant_id = ").
		WithArgs("tenant-1", "nonexistent").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByAgentID(ctx, "tenant-1", "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPresenceStore_List(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM presence WHERE tenant_id = ").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "status", "last_heartbeat", "metadata",
		}).
			AddRow(1, "tenant-1", "agent-1", "online", now, []byte(`{}`)).
			AddRow(2, "tenant-1", "agent-2", "away", now.Add(-5*time.Minute), []byte(`{}`)).
			AddRow(3, "tenant-1", "agent-3", "offline", now.Add(-1*time.Hour), []byte(`{}`)))

	presences, err := store.List(ctx, "tenant-1", "")
	assert.NoError(t, err)
	assert.Len(t, presences, 3)

	statuses := make(map[string]string)
	for _, p := range presences {
		statuses[p.AgentID] = p.Status
	}
	assert.Equal(t, "online", statuses["agent-1"])
	assert.Equal(t, "away", statuses["agent-2"])
	assert.Equal(t, "offline", statuses["agent-3"])
}

func TestPresenceStore_List_FilteredByAgent(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM presence WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-2").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "status", "last_heartbeat", "metadata",
		}).
			AddRow(2, "tenant-1", "agent-2", "away", now.Add(-5*time.Minute), []byte(`{}`)))

	presences, err := store.List(ctx, "tenant-1", "agent-2")
	assert.NoError(t, err)
	assert.Len(t, presences, 1)
	assert.Equal(t, "agent-2", presences[0].AgentID)
	assert.Equal(t, "away", presences[0].Status)
}

func TestPresenceStore_MarkOffline(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE presence SET status = 'offline'").
		WithArgs("tenant-1", "agent-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.MarkOffline(ctx, "tenant-1", "agent-1")
	assert.NoError(t, err)
}

func TestPresenceStore_MarkAway(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE presence SET status = 'away'").
		WithArgs("tenant-1", "agent-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.MarkAway(ctx, "tenant-1", "agent-1")
	assert.NoError(t, err)
}

func TestPresenceStore_MarkOnline(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()

	// MarkOnline calls Upsert internally
	mockPool.ExpectExec("INSERT INTO presence").
		WithArgs("tenant-1", "agent-1", "online", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.MarkOnline(ctx, "tenant-1", "agent-1")
	assert.NoError(t, err)
}

func TestPresenceStore_List_AllOffline(t *testing.T) {
	store, mockPool := newPresenceTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM presence WHERE tenant_id = ").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "status", "last_heartbeat", "metadata",
		}).
			AddRow(1, "tenant-1", "agent-1", "offline", now.Add(-2*time.Hour), []byte(`{}`)))

	presences, err := store.List(ctx, "tenant-1", "")
	assert.NoError(t, err)
	assert.Len(t, presences, 1)
	assert.Equal(t, "offline", presences[0].Status)
}