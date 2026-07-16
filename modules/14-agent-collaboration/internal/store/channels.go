package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type PgxPool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Close()
}

var ErrNotFound = errors.New("record not found")

// ChannelStore handles channel CRUD operations.
type ChannelStore struct {
	pool PgxPool
}

func NewChannelStore(pool PgxPool) *ChannelStore {
	return &ChannelStore{pool: pool}
}

func (s *ChannelStore) Create(ctx context.Context, ch *Channel) error {
	metadataBytes, _ := json.Marshal(ch.Metadata)
	query := `
		INSERT INTO channels (tenant_id, name, description, channel_type, creator_id,
			max_members, is_public, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`

	var id string
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		ch.TenantID, ch.Name, ch.Description, ch.ChannelType, ch.CreatorID,
		ch.MaxMembers, ch.IsPublic, metadataBytes,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	ch.ID = id
	ch.CreatedAt = createdAt
	ch.UpdatedAt = updatedAt
	return nil
}

func (s *ChannelStore) GetByID(ctx context.Context, id string) (*Channel, error) {
	return s.getFields(ctx, "id", []interface{}{id})
}

func (s *ChannelStore) GetByName(ctx context.Context, tenantID, name string) (*Channel, error) {
	return s.getFields(ctx, "name", []interface{}{name, tenantID})
}

func (s *ChannelStore) getFields(ctx context.Context, col string, vals []interface{}) (*Channel, error) {
	var query string
	if col == "name" {
		query = `SELECT id, tenant_id, name, description, channel_type, creator_id,
			max_members, is_public, metadata, created_at, updated_at
			FROM channels WHERE name = $1 AND tenant_id = $2`
	} else {
		query = `SELECT id, tenant_id, name, description, channel_type, creator_id,
			max_members, is_public, metadata, created_at, updated_at
			FROM channels WHERE id = $1`
	}

	ch := &Channel{}
	var desc pgtype.Text
	var metadataBytes []byte
	err := s.pool.QueryRow(ctx, query, vals...).Scan(
		&ch.ID, &ch.TenantID, &ch.Name, &desc, &ch.ChannelType, &ch.CreatorID,
		&ch.MaxMembers, &ch.IsPublic, &metadataBytes, &ch.CreatedAt, &ch.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		ch.Description = &desc.String
	}
	if err := json.Unmarshal(metadataBytes, &ch.Metadata); err != nil {
		ch.Metadata = make(map[string]interface{})
	}
	return ch, nil
}

func (s *ChannelStore) Update(ctx context.Context, ch *Channel) error {
	metadataBytes, _ := json.Marshal(ch.Metadata)
	query := `
		UPDATE channels SET name = $1, description = $2, channel_type = $3,
			max_members = $4, is_public = $5, metadata = $6, updated_at = NOW()
		WHERE id = $7 AND tenant_id = $8
		RETURNING updated_at`

	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		ch.Name, ch.Description, ch.ChannelType, ch.MaxMembers,
		ch.IsPublic, metadataBytes, ch.ID, ch.TenantID,
	).Scan(&updatedAt)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	ch.UpdatedAt = updatedAt
	return nil
}

func (s *ChannelStore) Delete(ctx context.Context, id, tenantID string) error {
	result, err := s.pool.Exec(ctx,
		"DELETE FROM channels WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ChannelStore) List(ctx context.Context, tenantID, channelType string, page, pageSize int) ([]Channel, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, name, description, channel_type, creator_id,
			max_members, is_public, metadata, created_at, updated_at
		FROM channels WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if channelType != "" {
		query += fmt.Sprintf(" AND channel_type = $%d", argIdx)
		args = append(args, channelType)
		argIdx++
	}
	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", argIdx) + " OFFSET $" + fmt.Sprintf("%d", argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, *ch)
	}

	countQuery := "SELECT COUNT(*) FROM channels WHERE tenant_id = $1"
	countArgs := []interface{}{tenantID}
	if channelType != "" {
		countQuery += " AND channel_type = $2"
		countArgs = append(countArgs, channelType)
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

	return channels, total, nil
}

func (s *ChannelStore) AddMember(ctx context.Context, channelID, agentID, role string) error {
	_, err := s.pool.Exec(ctx,
		"INSERT INTO channel_members (channel_id, agent_id, role) VALUES ($1, $2, $3)",
		channelID, agentID, role)
	return err
}

func (s *ChannelStore) RemoveMember(ctx context.Context, channelID, agentID string) error {
	result, err := s.pool.Exec(ctx,
		"DELETE FROM channel_members WHERE channel_id = $1 AND agent_id = $2", channelID, agentID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ChannelStore) IsMember(ctx context.Context, channelID, agentID string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM channel_members WHERE channel_id = $1 AND agent_id = $2",
		channelID, agentID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *ChannelStore) MemberCount(ctx context.Context, channelID string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM channel_members WHERE channel_id = $1", channelID).Scan(&count)
	return count, err
}

func scanChannel(rows interface{ Scan(...interface{}) error }) (*Channel, error) {
	ch := &Channel{}
	var desc pgtype.Text
	var metadataBytes []byte
	err := rows.Scan(&ch.ID, &ch.TenantID, &ch.Name, &desc, &ch.ChannelType, &ch.CreatorID,
		&ch.MaxMembers, &ch.IsPublic, &metadataBytes, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		ch.Description = &desc.String
	}
	ch.Metadata = make(map[string]interface{})
	if err := json.Unmarshal(metadataBytes, &ch.Metadata); err != nil {
		ch.Metadata = make(map[string]interface{})
	}
	return ch, nil
}