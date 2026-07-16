package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newHandoffTestStore(t *testing.T) (*HandoffStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewHandoffStore(mockPool), mockPool
}

func TestHandoffStore_Create(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()
	h := &Handoff{
		TenantID:      "tenant-1",
		FromAgentID:   "agent-1",
		ToAgentID:     "agent-2",
		Title:         "Process order #123",
		Priority:      "high",
		Status:        "pending",
		Context:       map[string]interface{}{"order_id": "123"},
	}

	mockPool.ExpectQuery("INSERT INTO handoffs").
		WithArgs("tenant-1", "agent-1", "agent-2", pgxmock.AnyArg(), pgxmock.AnyArg(),
			"Process order #123", pgxmock.AnyArg(), pgxmock.AnyArg(), "high", "pending", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, h)
	assert.NoError(t, err)
	assert.NotEmpty(t, h.ID)
	assert.Equal(t, "pending", h.Status)
	assert.Equal(t, "high", h.Priority)
	assert.Equal(t, "agent-1", h.FromAgentID)
}

func TestHandoffStore_Create_WithChannelID(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()
	channelID := "ch-1"
	h := &Handoff{
		TenantID:      "tenant-1",
		FromAgentID:   "agent-1",
		ToAgentID:     "agent-2",
		ChannelID:     &channelID,
		Title:         "Discuss in channel",
		Priority:      "normal",
		Status:        "pending",
		ExpiresAt:     ptrTime(now.Add(1 * time.Hour)),
	}

	mockPool.ExpectQuery("INSERT INTO handoffs").
		WithArgs("tenant-1", "agent-1", "agent-2", pgxmock.AnyArg(), pgxmock.AnyArg(),
			"Discuss in channel", pgxmock.AnyArg(), pgxmock.AnyArg(), "normal", "pending", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, h)
	assert.NoError(t, err)
	assert.NotNil(t, h.ChannelID)
	assert.Equal(t, "ch-1", *h.ChannelID)
}

func TestHandoffStore_Create_Fail(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO handoffs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	err := store.Create(ctx, &Handoff{
		TenantID: "tenant-1", FromAgentID: "agent-1", ToAgentID: "agent-2",
		Title: "Test handoff", Priority: "normal", Status: "pending",
	})
	assert.Error(t, err)
}

func TestHandoffStore_GetByID(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE id = ").
		WithArgs("ho-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-1", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Process order", nil, []byte(`{"order_id":"123"}`), "high", "pending", nil,
				nil, nil, nil, now, now))

	h, err := store.GetByID(ctx, "ho-1")
	assert.NoError(t, err)
	assert.Equal(t, "ho-1", h.ID)
	assert.Equal(t, "Process order", h.Title)
	assert.Equal(t, "pending", h.Status)
	assert.Equal(t, "high", h.Priority)
	assert.Equal(t, "123", h.Context["order_id"])
}

func TestHandoffStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE id = ").
		WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestHandoffStore_GetByID_Completed(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE id = ").
		WithArgs("ho-completed").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-completed", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Done", nil, []byte(`{}`), "normal", "completed", nil,
				pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Time: now, Valid: true},
				"Completed successfully", now, now))

	h, err := store.GetByID(ctx, "ho-completed")
	assert.NoError(t, err)
	assert.Equal(t, "completed", h.Status)
	assert.NotNil(t, h.CompletedAt)
	assert.Equal(t, "Completed successfully", *h.Response)
}

func TestHandoffStore_List(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-1", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Task 1", nil, []byte(`{}`), "high", "pending", nil,
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now).
			AddRow("ho-2", "tenant-1", "agent-3", "agent-2", nil, nil,
				"Task 2", nil, []byte(`{}`), "normal", "accepted", nil,
				pgtype.Timestamptz{Time: now, Valid: true}, pgtype.Timestamptz{Valid: false}, nil, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	handoffs, total, err := store.List(ctx, "tenant-1", "", "", 1, 20)
	assert.NoError(t, err)
	assert.Len(t, handoffs, 2)
	assert.Equal(t, 2, total)
}

func TestHandoffStore_List_FilteredByToAgent(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-2", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-1", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Task for agent-2", nil, []byte(`{}`), "high", "pending", nil,
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-2").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	handoffs, total, err := store.List(ctx, "tenant-1", "agent-2", "", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, handoffs, 1)
	assert.Equal(t, "agent-2", handoffs[0].ToAgentID)
	assert.Equal(t, 1, total)
}

func TestHandoffStore_List_FilteredByStatus(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", "expired", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-expired", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Expired task", nil, []byte(`{}`), "low", "expired", ptrTime(now.Add(-2*time.Hour)),
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", "expired").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	handoffs, total, err := store.List(ctx, "tenant-1", "", "expired", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, handoffs, 1)
	assert.Equal(t, "expired", handoffs[0].Status)
	assert.Equal(t, 1, total)
}

func TestHandoffStore_List_FilteredByToAgentAndStatus(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-2", "pending", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-1", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Pending for agent-2", nil, []byte(`{}`), "normal", "pending", nil,
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-2", "pending").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	handoffs, total, err := store.List(ctx, "tenant-1", "agent-2", "pending", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, handoffs, 1)
	assert.Equal(t, 1, total)
}

func TestHandoffStore_List_DefaultPagination(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-1", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Task", nil, []byte(`{}`), "normal", "pending", nil,
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM handoffs WHERE tenant_id = ").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	handoffs, _, err := store.List(ctx, "tenant-1", "", "", 0, 200)
	assert.NoError(t, err)
	assert.Len(t, handoffs, 1)
}

func TestHandoffStore_UpdateStatus(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE handoffs SET status = ").
		WithArgs("in_progress", "ho-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpdateStatus(ctx, "ho-1", "in_progress")
	assert.NoError(t, err)
}

func TestHandoffStore_AcceptHandoff(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE handoffs SET status = 'accepted'").
		WithArgs(pgxmock.AnyArg(), "Accepted the task", "ho-1", "agent-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.AcceptHandoff(ctx, "ho-1", "agent-2", "Accepted the task")
	assert.NoError(t, err)
}

func TestHandoffStore_AcceptHandoff_WrongAgent(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	// Will match the exec but return 0 rows affected
	mockPool.ExpectExec("UPDATE handoffs SET status = 'accepted'").
		WithArgs(pgxmock.AnyArg(), "Response", "ho-1", "wrong-agent").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := store.AcceptHandoff(ctx, "ho-1", "wrong-agent", "Response")
	// Should succeed (no error returned even if 0 rows affected)
	assert.NoError(t, err)
}

func TestHandoffStore_CompleteHandoff(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE handoffs SET status = 'completed'").
		WithArgs(pgxmock.AnyArg(), "Done", "ho-1", "agent-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.CompleteHandoff(ctx, "ho-1", "agent-2", "Done")
	assert.NoError(t, err)
}

func TestHandoffStore_RejectHandoff(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE handoffs SET status = 'rejected'").
		WithArgs("Not interested", "ho-1", "agent-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.RejectHandoff(ctx, "ho-1", "agent-2", "Not interested")
	assert.NoError(t, err)
}

func TestHandoffStore_GetByToAgentAndStatus(t *testing.T) {
	store, mockPool := newHandoffTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM handoffs WHERE tenant_id = ").
		WithArgs("agent-2", "pending").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "from_agent_id", "to_agent_id", "channel_id", "parent_message_id",
			"title", "description", "context", "priority", "status", "expires_at",
			"assigned_at", "completed_at", "response", "created_at", "updated_at",
		}).
			AddRow("ho-1", "tenant-1", "agent-1", "agent-2", nil, nil,
				"Task 1", nil, []byte(`{}`), "high", "pending", nil,
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now).
			AddRow("ho-2", "tenant-1", "agent-3", "agent-2", nil, nil,
				"Task 2", nil, []byte(`{}`), "normal", "pending", nil,
				pgtype.Timestamptz{Valid: false}, pgtype.Timestamptz{Valid: false}, nil, now, now))

	handoffs, err := store.GetByToAgentAndStatus(ctx, "agent-2", "pending")
	assert.NoError(t, err)
	assert.Len(t, handoffs, 2)
	for _, h := range handoffs {
		assert.Equal(t, "agent-2", h.ToAgentID)
		assert.Equal(t, "pending", h.Status)
	}
}