package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func ptrTimeVal(t time.Time) *time.Time { return &t }

func ptrInt(n int) *int                  { return &n }

func TestCreateSyncHistory(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	id := uuid.New()
	now := time.Now()

	pool.ExpectQuery("INSERT INTO connector_sync_history").
		WithArgs("tenant-1", id, "full", "running", 0, 0, 0, pgxmock.AnyArg(), now).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(id))

	sh := &ConnectorSyncHistory{
		TenantID:    "tenant-1",
		ConnectorID: id,
		SyncType:    "full",
		Status:      "running",
		StartedAt:   now,
	}
	err = store.Create(context.Background(), sh)
	require.NoError(t, err)
	require.Equal(t, id, sh.ID)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestUpdateComplete(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	id := uuid.New()
	completedAt := time.Now()

	// 6 args: fetched, updated, failed, durationMs, completedAt, id
	pool.ExpectExec("UPDATE connector_sync_history SET status").
		WithArgs(10, 8, 2, 500, completedAt, id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = store.UpdateComplete(context.Background(), id, 10, 8, 2, 500, completedAt)
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestUpdateError(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	id := uuid.New()
	errMsg := "connection timeout"

	// 2 args: errorMessage, id
	pool.ExpectExec("UPDATE connector_sync_history SET status").
		WithArgs(errMsg, id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = store.UpdateError(context.Background(), id, errMsg)
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestListSyncHistory(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	id := uuid.New()
	now := time.Now()

	// LIST query runs first
	pool.ExpectQuery("SELECT id, tenant_id, connector_id").
		WithArgs("tenant-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "connector_id", "sync_type", "status",
			"objects_fetched", "objects_updated", "objects_failed", "error_message",
			"started_at", "completed_at", "duration_ms",
		}).AddRow(id, "tenant-1", id, "full", "completed",
			10, 8, 2, nil, now, nil, ptrInt(500)))

	// COUNT query runs second
	pool.ExpectQuery("SELECT COUNT.*FROM connector_sync_history WHERE tenant_id").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	history, total, err := store.List(context.Background(), "tenant-1", "", "", 1, 20)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, 1, total)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestListSyncHistoryWithStatusFilter(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	// LIST query runs first
	pool.ExpectQuery("SELECT id, tenant_id, connector_id").
		WithArgs("tenant-1", "completed", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "connector_id", "sync_type", "status",
			"objects_fetched", "objects_updated", "objects_failed", "error_message",
			"started_at", "completed_at", "duration_ms",
		}))

	// COUNT query runs second
	pool.ExpectQuery("SELECT COUNT.*AND status").
		WithArgs("tenant-1", "completed").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	history, total, err := store.List(context.Background(), "tenant-1", "", "completed", 1, 20)
	require.NoError(t, err)
	require.Len(t, history, 0)
	require.Equal(t, 0, total)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestGetSyncHistoryByID(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	id := uuid.New()
	now := time.Now()

	pool.ExpectQuery("SELECT id, tenant_id, connector_id").
		WithArgs(id, "tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "connector_id", "sync_type", "status",
			"objects_fetched", "objects_updated", "objects_failed", "error_message",
			"started_at", "completed_at", "duration_ms",
		}).AddRow(id, "tenant-1", id, "full", "completed",
			10, 8, 2, nil, now, nil, ptrInt(500)))

	sh, err := store.GetByID(context.Background(), id, "tenant-1")
	require.NoError(t, err)
	require.Equal(t, id, sh.ID)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestGetSyncHistoryByID_NotFound(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewSyncStore(pool)

	id := uuid.New()
	pool.ExpectQuery("SELECT id, tenant_id, connector_id").
		WithArgs(id, "tenant-1").
		WillReturnError(pgx.ErrNoRows)

	_, err = store.GetByID(context.Background(), id, "tenant-1")
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestConnectorSyncHistory_ModelDefaults(t *testing.T) {
	sh := ConnectorSyncHistory{
		TenantID:    "tenant-1",
		ConnectorID: uuid.New(),
		SyncType:    "full",
		Status:      "pending",
	}
	if sh.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", sh.Status)
	}
	if sh.ObjectsFetched != 0 {
		t.Errorf("expected 0 objects fetched")
	}
}

func TestConnectorSyncHistory_ModelWithResult(t *testing.T) {
	completedAt := time.Now()
	sh := ConnectorSyncHistory{
		TenantID:       "tenant-1",
		ConnectorID:    uuid.New(),
		SyncType:       "full",
		Status:         "completed",
		ObjectsFetched: 42,
		ObjectsUpdated: 40,
		ObjectsFailed:  2,
		DurationMs:     ptrInt(1234),
		CompletedAt:    &completedAt,
	}
	require.Equal(t, "completed", sh.Status)
	require.Equal(t, 42, sh.ObjectsFetched)
	require.Equal(t, 2, sh.ObjectsFailed)
	require.Equal(t, 1234, *sh.DurationMs)
}