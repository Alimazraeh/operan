package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newMessageTestStore(t *testing.T) (*MessageStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewMessageStore(mockPool), mockPool
}

func TestMessageStore_Create(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()
	m := &Message{
		TenantID:    "tenant-1",
		ChannelID:   "ch-1",
		SenderID:    "agent-1",
		MessageType: "text",
		Content:     "Hello, world!",
		ReplyCount:  0,
	}

	mockPool.ExpectQuery("INSERT INTO messages").
		WithArgs("tenant-1", "ch-1", pgxmock.AnyArg(), "agent-1", pgxmock.AnyArg(),
			"text", "Hello, world!", pgxmock.AnyArg(), 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, m)
	assert.NoError(t, err)
	assert.NotEmpty(t, m.ID)
	assert.Equal(t, now, m.CreatedAt)
}

func TestMessageStore_Create_WithParentID(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()
	parentID := "msg-parent-1"
	m := &Message{
		TenantID:    "tenant-1",
		ChannelID:   "ch-1",
		SenderID:    "agent-2",
		MessageType: "text",
		Content:     "Reply content",
		ParentID:    &parentID,
		ReplyCount:  0,
	}

	mockPool.ExpectQuery("INSERT INTO messages").
		WithArgs("tenant-1", "ch-1", pgxmock.AnyArg(), "agent-2", pgxmock.AnyArg(),
			"text", "Reply content", pgxmock.AnyArg(), 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, m)
	assert.NoError(t, err)
	assert.Equal(t, parentID, *m.ParentID)
}

func TestMessageStore_Create_WithAttachments(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()
	m := &Message{
		TenantID:    "tenant-1",
		ChannelID:   "ch-1",
		SenderID:    "agent-1",
		MessageType: "text",
		Content:     "See attached",
		Attachments: []map[string]interface{}{{"url": "http://example.com/file.pdf", "name": "file.pdf"}},
		ReplyCount:  0,
	}

	mockPool.ExpectQuery("INSERT INTO messages").
		WithArgs("tenant-1", "ch-1", pgxmock.AnyArg(), "agent-1", pgxmock.AnyArg(),
			"text", "See attached", pgxmock.AnyArg(), 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	err := store.Create(ctx, m)
	assert.NoError(t, err)
	assert.Len(t, m.Attachments, 1)
	assert.Equal(t, "http://example.com/file.pdf", m.Attachments[0]["url"])
}

func TestMessageStore_Create_Fail(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	err := store.Create(ctx, &Message{
		TenantID: "tenant-1", ChannelID: "ch-1", SenderID: "agent-1",
		Content: "test", MessageType: "text",
	})
	assert.Error(t, err)
}

func TestMessageStore_GetByID(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM messages WHERE id = ").
		WithArgs("msg-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "parent_id", "sender_id", "sender_name",
			"message_type", "content", "attachments", "reply_count", "created_at", "updated_at",
		}).
			AddRow("msg-1", "tenant-1", "ch-1", nil, "agent-1", "Bot", "text",
				"Hello!", []byte("[]"), 3, now, now))

	m, err := store.GetByID(ctx, "msg-1")
	assert.NoError(t, err)
	assert.Equal(t, "msg-1", m.ID)
	assert.Equal(t, "ch-1", m.ChannelID)
	assert.Equal(t, "Hello!", m.Content)
	assert.Equal(t, 3, m.ReplyCount)
	assert.Equal(t, "text", m.MessageType)
}

func TestMessageStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT.*FROM messages WHERE id = ").
		WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMessageStore_GetByID_WithParentID(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()

	parentID := "msg-parent"

	mockPool.ExpectQuery("SELECT.*FROM messages WHERE id = ").
		WithArgs("msg-reply").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "parent_id", "sender_id", "sender_name",
			"message_type", "content", "attachments", "reply_count", "created_at", "updated_at",
		}).
			AddRow("msg-reply", "tenant-1", "ch-1", parentID, "agent-2", "User", "text",
				"Replying to parent", []byte("[]"), 0, now, now))

	m, err := store.GetByID(ctx, "msg-reply")
	assert.NoError(t, err)
	assert.Equal(t, "msg-reply", m.ID)
	assert.Equal(t, parentID, *m.ParentID)
}

func TestMessageStore_List(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM messages WHERE channel_id = ").
		WithArgs("ch-1", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "parent_id", "sender_id", "sender_name",
			"message_type", "content", "attachments", "reply_count", "created_at", "updated_at",
		}).
			AddRow("msg-1", "tenant-1", "ch-1", nil, "agent-1", "Bot", "text",
				"Hello!", []byte("[]"), 0, now, now).
			AddRow("msg-2", "tenant-1", "ch-1", nil, "agent-2", "User", "text",
				"Hi there!", []byte("[]"), 0, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM messages WHERE channel_id = ").
		WithArgs("ch-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	messages, total, err := store.List(ctx, "ch-1", "", "", 1, 50)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, 2, total)
	// Should filter to only top-level messages (parent_id IS NULL)
}

func TestMessageStore_List_WithMessageTypeFilter(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM messages WHERE channel_id = ").
		WithArgs("ch-1", "system", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "parent_id", "sender_id", "sender_name",
			"message_type", "content", "attachments", "reply_count", "created_at", "updated_at",
		}).
			AddRow("msg-3", "tenant-1", "ch-1", nil, "system", "System", "system",
				"User joined", []byte("[]"), 0, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM messages WHERE channel_id = ").
		WithArgs("ch-1", "system").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	messages, total, err := store.List(ctx, "ch-1", "system", "", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, "system", messages[0].MessageType)
	assert.Equal(t, 1, total)
}

func TestMessageStore_List_WithReplyTo(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()

	parentID := "msg-1"

	mockPool.ExpectQuery("SELECT.*FROM messages WHERE channel_id = ").
		WithArgs("ch-1", parentID, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "parent_id", "sender_id", "sender_name",
			"message_type", "content", "attachments", "reply_count", "created_at", "updated_at",
		}).
			AddRow("msg-reply-1", "tenant-1", "ch-1", parentID, "agent-2", "User", "text",
				"First reply", []byte("[]"), 0, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM messages WHERE channel_id = ").
		WithArgs("ch-1", parentID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	messages, total, err := store.List(ctx, "ch-1", "", parentID, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, parentID, *messages[0].ParentID)
	assert.Equal(t, 1, total)
}

func TestMessageStore_List_DefaultPagination(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()
	now := time.Now()

	// page < 1 should default to 1, pageSize > 100 should default to 50
	mockPool.ExpectQuery("SELECT.*FROM messages WHERE channel_id = ").
		WithArgs("ch-1", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "parent_id", "sender_id", "sender_name",
			"message_type", "content", "attachments", "reply_count", "created_at", "updated_at",
		}).
			AddRow("msg-1", "tenant-1", "ch-1", nil, "agent-1", "Bot", "text",
				"Hello", []byte("[]"), 0, now, now))

	mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM messages WHERE channel_id = ").
		WithArgs("ch-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	messages, _, err := store.List(ctx, "ch-1", "", "", 0, 200)
	assert.NoError(t, err)
	assert.Len(t, messages, 1)
}

func TestMessageStore_IncrementReplyCount(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE messages SET reply_count = reply_count \\+ 1").
		WithArgs("msg-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.IncrementReplyCount(ctx, "msg-1")
	assert.NoError(t, err)
}

func TestMessageStore_IncrementReplyCount_Fail(t *testing.T) {
	store, mockPool := newMessageTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE messages SET reply_count = reply_count \\+ 1").
		WithArgs("nonexistent").
		WillReturnError(assert.AnError)

	err := store.IncrementReplyCount(ctx, "nonexistent")
	assert.Error(t, err)
}