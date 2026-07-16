package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MessageStore handles message operations.
type MessageStore struct {
	pool PgxPool
}

func NewMessageStore(pool PgxPool) *MessageStore {
	return &MessageStore{pool: pool}
}

func (s *MessageStore) Create(ctx context.Context, m *Message) error {
	attachmentsBytes, _ := json.Marshal(m.Attachments)
	query := `
		INSERT INTO messages (tenant_id, channel_id, parent_id, sender_id, sender_name,
			message_type, content, attachments, reply_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`

	var id string
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		m.TenantID, m.ChannelID, m.ParentID, m.SenderID, m.SenderName,
		m.MessageType, m.Content, attachmentsBytes, m.ReplyCount,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	m.ID = id
	m.CreatedAt = createdAt
	m.UpdatedAt = updatedAt
	return nil
}

func (s *MessageStore) GetByID(ctx context.Context, id string) (*Message, error) {
	return s.getFields(ctx, "SELECT id, tenant_id, channel_id, parent_id, sender_id, sender_name, message_type, content, attachments, reply_count, created_at, updated_at FROM messages WHERE id = $1", []interface{}{id})
}

func (s *MessageStore) getFields(ctx context.Context, query string, vals []interface{}) (*Message, error) {
	m := &Message{}
	var parentID, senderName pgtype.Text
	var attachmentsBytes []byte
	err := s.pool.QueryRow(ctx, query, vals...).Scan(
		&m.ID, &m.TenantID, &m.ChannelID, &parentID,
		&m.SenderID, &senderName,
		&m.MessageType, &m.Content, &attachmentsBytes, &m.ReplyCount, &m.CreatedAt, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		m.ParentID = &parentID.String
	}
	if senderName.Valid {
		m.SenderName = &senderName.String
	}
	m.Attachments = []map[string]interface{}{}
	if err := json.Unmarshal(attachmentsBytes, &m.Attachments); err != nil {
		m.Attachments = []map[string]interface{}{}
	}
	return m, nil
}

func (s *MessageStore) List(ctx context.Context, channelID, messageType, replyTo string, page, pageSize int) ([]Message, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, channel_id, parent_id, sender_id, sender_name, message_type, content, attachments, reply_count, created_at, updated_at
		FROM messages WHERE channel_id = $1`
	args := []interface{}{channelID}
	argIdx := 2

	if messageType != "" {
		query += fmt.Sprintf(" AND message_type = $%d", argIdx)
		args = append(args, messageType)
		argIdx++
	}
	if replyTo != "" {
		query += fmt.Sprintf(" AND parent_id = $%d", argIdx)
		args = append(args, replyTo)
		argIdx++
	} else {
		query += " AND parent_id IS NULL"
	}
	query += " ORDER BY created_at ASC LIMIT $" + fmt.Sprintf("%d", argIdx) + " OFFSET $" + fmt.Sprintf("%d", argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		m, err := s.scanMessage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, *m)
	}

	countQuery := "SELECT COUNT(*) FROM messages WHERE channel_id = $1"
	countArgs := []interface{}{channelID}
	cIdx := 2
	if messageType != "" {
		countQuery += fmt.Sprintf(" AND message_type = $%d", cIdx)
		countArgs = append(countArgs, messageType)
		cIdx++
	}
	if replyTo != "" {
		countQuery += fmt.Sprintf(" AND parent_id = $%d", cIdx)
		countArgs = append(countArgs, replyTo)
	} else {
		countQuery += " AND parent_id IS NULL"
	}
	countRows, err := s.pool.Query(ctx, countQuery, countArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer countRows.Close()
	var total int
	if countRows.Next() {
		countRows.Scan(&total)
	}

	return messages, total, nil
}

func (s *MessageStore) IncrementReplyCount(ctx context.Context, messageID string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE messages SET reply_count = reply_count + 1, updated_at = NOW() WHERE id = $1", messageID)
	return err
}

func (s *MessageStore) scanMessage(rows interface{ Scan(...interface{}) error }) (*Message, error) {
	m := &Message{}
	var parentID, senderName pgtype.Text
	var attachmentsBytes []byte
	err := rows.Scan(&m.ID, &m.TenantID, &m.ChannelID, &parentID,
		&m.SenderID, &senderName,
		&m.MessageType, &m.Content, &attachmentsBytes, &m.ReplyCount, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		m.ParentID = &parentID.String
	}
	if senderName.Valid {
		m.SenderName = &senderName.String
	}
	m.Attachments = []map[string]interface{}{}
	if err := json.Unmarshal(attachmentsBytes, &m.Attachments); err != nil {
		m.Attachments = []map[string]interface{}{}
	}
	return m, nil
}