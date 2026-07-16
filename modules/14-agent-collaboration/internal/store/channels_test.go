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

func ptrStr(s string) *string { return &s }
func ptrTime(t time.Time) *time.Time { return &t }

func newChannelTestStore(t *testing.T) (*ChannelStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewChannelStore(mockPool), mockPool
}

func TestChannelStore_Create(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()
	ch := &Channel{
		TenantID:    "tenant-1",
		Name:        "general",
		Description: ptrStr("General discussion"),
		ChannelType: "general",
		CreatorID:   "agent-1",
		MaxMembers:  100,
		IsPublic:    true,
		Metadata:    map[string]interface{}{"tag": "main"},
	}

	mockPool.ExpectQuery("INSERT INTO channels").
		WithArgs("tenant-1", "general", pgxmock.AnyArg(), "general", "agent-1", 100, true, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, ch)
	assert.NoError(t, err)
	assert.NotEmpty(t, ch.ID)
	assert.Equal(t, now, ch.CreatedAt)
}

func TestChannelStore_Create_WithNilDescription(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()
	ch := &Channel{
		TenantID:    "tenant-1",
		Name:        "tech-updates",
		Description: nil,
		ChannelType: "department",
		CreatorID:   "agent-2",
		IsPublic:    false,
		Metadata:    nil,
	}

	mockPool.ExpectQuery("INSERT INTO channels").
		WithArgs("tenant-1", "tech-updates", pgxmock.AnyArg(), "department", "agent-2", 0, false, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, ch)
	assert.NoError(t, err)
	assert.Nil(t, ch.Description)
}

func TestChannelStore_Create_Fail(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO channels").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	err := store.Create(ctx, &Channel{
		TenantID:    "tenant-1",
		Name:        "test",
		ChannelType: "general",
		CreatorID:   "agent-1",
	})
	assert.Error(t, err)
}

func TestChannelStore_GetByID(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE id = ").
		WithArgs("test-channel-id").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).
			AddRow("test-channel-id", "tenant-1", "general",
				pgtype.Text{String: "Main channel", Valid: true}, "general", "agent-1",
				100, true, []byte(`{"tag":"main"}`), now, now))

	ch, err := store.GetByID(ctx, "test-channel-id")
	assert.NoError(t, err)
	assert.Equal(t, "test-channel-id", ch.ID)
	assert.Equal(t, "tenant-1", ch.TenantID)
	assert.Equal(t, "Main channel", *ch.Description)
	assert.Equal(t, 100, ch.MaxMembers)
	assert.True(t, ch.IsPublic)
}

func TestChannelStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE id = ").
		WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestChannelStore_GetByName(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE name = ").
		WithArgs("general", "tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).
			AddRow("ch-1", "tenant-1", "general",
				pgtype.Text{String: "General", Valid: true}, "general", "agent-1",
				50, true, []byte(`{}`), now, now))

	ch, err := store.GetByName(ctx, "tenant-1", "general")
	assert.NoError(t, err)
	assert.Equal(t, "ch-1", ch.ID)
	assert.Equal(t, "general", ch.Name)
}

func TestChannelStore_Update(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("UPDATE channels SET").
		WithArgs("Updated Name", pgxmock.AnyArg(), "general", 200, true, pgxmock.AnyArg(), "test-channel-id", "tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}).AddRow(now))

	ch := &Channel{
		ID:          "test-channel-id",
		TenantID:    "tenant-1",
		Name:        "Updated Name",
		ChannelType: "general",
		MaxMembers:  200,
		IsPublic:    true,
	}

	err := store.Update(ctx, ch)
	assert.NoError(t, err)
	assert.Equal(t, now, ch.UpdatedAt)
}

func TestChannelStore_Update_NotFound(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("UPDATE channels SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	err := store.Update(ctx, &Channel{ID: "nonexistent", TenantID: "tenant-1"})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestChannelStore_Delete(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM channels").
		WithArgs("test-channel-id", "tenant-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := store.Delete(ctx, "test-channel-id", "tenant-1")
	assert.NoError(t, err)
}

func TestChannelStore_Delete_NotFound(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM channels").
		WithArgs("nonexistent", "tenant-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err := store.Delete(ctx, "nonexistent", "tenant-1")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestChannelStore_List(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE tenant_id = ").
		WithArgs("tenant-1", "general", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).
			AddRow("ch-1", "tenant-1", "general", "General", "general", "agent-1",
				100, true, []byte(`{}`), now, now).
			AddRow("ch-2", "tenant-1", "tech", "Tech channel", "department", "agent-2",
				50, false, []byte(`{}`), now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM channels WHERE tenant_id = ").
		WithArgs("tenant-1", "general").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	channels, total, err := store.List(ctx, "tenant-1", "general", 1, 20)
	assert.NoError(t, err)
	assert.Len(t, channels, 2)
	assert.Equal(t, 2, total)
}

func TestChannelStore_List_Filtered(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE tenant_id = ").
		WithArgs("tenant-1", "private", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).
			AddRow("ch-3", "tenant-1", "secret", "Secret", "private", "agent-3",
				10, false, []byte(`{}`), now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM channels WHERE tenant_id = ").
		WithArgs("tenant-1", "private").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	channels, total, err := store.List(ctx, "tenant-1", "private", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, channels, 1)
	assert.Equal(t, "secret", channels[0].Name)
	assert.Equal(t, 1, total)
}

func TestChannelStore_List_DefaultPagination(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE tenant_id = ").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).
			AddRow("ch-1", "tenant-1", "general", "", "general", "agent-1",
				0, false, []byte(`{}`), now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM channels WHERE tenant_id = ").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	channels, _, err := store.List(ctx, "tenant-1", "", 0, 200)
	assert.NoError(t, err)
	assert.Len(t, channels, 1)
}

func TestChannelStore_AddMember(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("INSERT INTO channel_members").
		WithArgs("ch-1", "agent-1", "member").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.AddMember(ctx, "ch-1", "agent-1", "member")
	assert.NoError(t, err)
}

func TestChannelStore_RemoveMember(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM channel_members").
		WithArgs("ch-1", "agent-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := store.RemoveMember(ctx, "ch-1", "agent-1")
	assert.NoError(t, err)
}

func TestChannelStore_RemoveMember_NotFound(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM channel_members").
		WithArgs("ch-1", "nonexistent").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err := store.RemoveMember(ctx, "ch-1", "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestChannelStore_IsMember_True(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM channel_members").
		WithArgs("ch-1", "agent-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	isMember, err := store.IsMember(ctx, "ch-1", "agent-1")
	assert.NoError(t, err)
	assert.True(t, isMember)
}

func TestChannelStore_IsMember_False(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM channel_members").
		WithArgs("ch-1", "agent-2").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	isMember, err := store.IsMember(ctx, "ch-1", "agent-2")
	assert.NoError(t, err)
	assert.False(t, isMember)
}

func TestChannelStore_MemberCount(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM channel_members").
		WithArgs("ch-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(5))

	count, err := store.MemberCount(ctx, "ch-1")
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
}

// ─── Test scanChannel ───

func TestScanChannel_JSONBadMetadata(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	// Use GetByID which calls scanChannel internally
	mockPool.ExpectQuery("SELECT.*FROM channels WHERE id = ").
		WithArgs("ch-bad-json").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).AddRow("ch-bad-json", "tenant-1", "test", nil, "general", "agent-1",
			0, false, []byte("not-json"), now, now))

	ch, err := store.GetByID(ctx, "ch-bad-json")
	assert.NoError(t, err)
	assert.Equal(t, "ch-bad-json", ch.ID)
	assert.Equal(t, "test", ch.Name)
	assert.Equal(t, 0, len(ch.Metadata))
}

func TestScanChannel_NilDescription(t *testing.T) {
	store, mockPool := newChannelTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM channels WHERE id = ").
		WithArgs("ch-no-desc").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "channel_type", "creator_id",
			"max_members", "is_public", "metadata", "created_at", "updated_at",
		}).AddRow("ch-no-desc", "tenant-1", "no-desc", nil, "general", "agent-1",
			10, false, []byte(`{}`), now, now))

	ch, err := store.GetByID(ctx, "ch-no-desc")
	assert.NoError(t, err)
	assert.Equal(t, "ch-no-desc", ch.ID)
	assert.Nil(t, ch.Description)
}