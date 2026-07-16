package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// HandoffStore handles handoff CRUD operations.
type HandoffStore struct {
	pool PgxPool
}

func NewHandoffStore(pool PgxPool) *HandoffStore {
	return &HandoffStore{pool: pool}
}

func (s *HandoffStore) Create(ctx context.Context, h *Handoff) error {
	contextBytes, _ := json.Marshal(h.Context)
	var channelID, parentMsgID *string
	if h.ChannelID != nil {
		channelID = h.ChannelID
	}
	if h.ParentMessageID != nil {
		parentMsgID = h.ParentMessageID
	}
	query := `
		INSERT INTO handoffs (tenant_id, from_agent_id, to_agent_id, channel_id,
			parent_message_id, title, description, context, priority, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`

	var id string
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		h.TenantID, h.FromAgentID, h.ToAgentID, channelID, parentMsgID,
		h.Title, h.Description, contextBytes, h.Priority, h.Status, h.ExpiresAt,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	h.ID = id
	h.CreatedAt = createdAt
	h.UpdatedAt = updatedAt
	return nil
}

func (s *HandoffStore) GetByID(ctx context.Context, id string) (*Handoff, error) {
	return s.getFields(ctx, "SELECT id, tenant_id, from_agent_id, to_agent_id, channel_id, parent_message_id, title, description, context, priority, status, expires_at, assigned_at, completed_at, response, created_at, updated_at FROM handoffs WHERE id = $1", []interface{}{id})
}

func (s *HandoffStore) getFields(ctx context.Context, query string, vals []interface{}) (*Handoff, error) {
	h := &Handoff{}
	var channelID, parentMessageID, description, response pgtype.Text
	var assignedAt, completedAt pgtype.Timestamptz
	var contextBytes []byte
	err := s.pool.QueryRow(ctx, query, vals...).Scan(
		&h.ID, &h.TenantID, &h.FromAgentID, &h.ToAgentID,
		&channelID, &parentMessageID,
		&h.Title, &description, &contextBytes, &h.Priority, &h.Status, &h.ExpiresAt,
		&assignedAt, &completedAt, &response, &h.CreatedAt, &h.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if channelID.Valid {
		h.ChannelID = &channelID.String
	}
	if parentMessageID.Valid {
		h.ParentMessageID = &parentMessageID.String
	}
	if description.Valid {
		h.Description = &description.String
	}
	h.Context = make(map[string]interface{})
	if err := json.Unmarshal(contextBytes, &h.Context); err != nil {
		h.Context = make(map[string]interface{})
	}
	if response.Valid {
		h.Response = &response.String
	}
	if assignedAt.Valid {
		h.AssignedAt = &assignedAt.Time
	}
	if completedAt.Valid {
		h.CompletedAt = &completedAt.Time
	}
	return h, nil
}

func (s *HandoffStore) List(ctx context.Context, tenantID, toAgentID, status string, page, pageSize int) ([]Handoff, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, from_agent_id, to_agent_id, channel_id, parent_message_id,
			title, description, context, priority, status, expires_at, assigned_at, completed_at,
			response, created_at, updated_at
		FROM handoffs WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if toAgentID != "" {
		query += fmt.Sprintf(" AND to_agent_id = $%d", argIdx)
		args = append(args, toAgentID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", argIdx) + " OFFSET $" + fmt.Sprintf("%d", argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var handoffs []Handoff
	for rows.Next() {
		h, err := s.scanHandoff(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan handoff: %w", err)
		}
		handoffs = append(handoffs, *h)
	}

	countQuery := "SELECT COUNT(*) FROM handoffs WHERE tenant_id = $1"
	countArgs := []interface{}{tenantID}
	cIdx := 2
	if toAgentID != "" {
		countQuery += fmt.Sprintf(" AND to_agent_id = $%d", cIdx)
		countArgs = append(countArgs, toAgentID)
		cIdx++
	}
	if status != "" {
		countQuery += fmt.Sprintf(" AND status = $%d", cIdx)
		countArgs = append(countArgs, status)
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

	return handoffs, total, nil
}

func (s *HandoffStore) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE handoffs SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	return err
}

func (s *HandoffStore) AcceptHandoff(ctx context.Context, id, agentID, response string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE handoffs SET status = 'accepted', assigned_at = $1, response = $2, updated_at = NOW() WHERE id = $3 AND to_agent_id = $4 AND status = 'pending'`,
		&now, response, id, agentID)
	return err
}

func (s *HandoffStore) CompleteHandoff(ctx context.Context, id, agentID, response string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE handoffs SET status = 'completed', completed_at = $1, response = $2, updated_at = NOW() WHERE id = $3 AND to_agent_id = $4 AND status IN ('accepted', 'in_progress')`,
		&now, response, id, agentID)
	return err
}

func (s *HandoffStore) RejectHandoff(ctx context.Context, id, agentID, response string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE handoffs SET status = 'rejected', response = $1, updated_at = NOW() WHERE id = $2 AND to_agent_id = $3 AND status = 'pending'`,
		response, id, agentID)
	return err
}

func (s *HandoffStore) GetByToAgentAndStatus(ctx context.Context, toAgentID, status string) ([]Handoff, error) {
	query := `
		SELECT id, tenant_id, from_agent_id, to_agent_id, channel_id, parent_message_id,
			title, description, context, priority, status, expires_at, assigned_at, completed_at,
			response, created_at, updated_at
		FROM handoffs WHERE tenant_id = $1 AND to_agent_id = $2 AND status = $3`

	rows, err := s.pool.Query(ctx, query, toAgentID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var handoffs []Handoff
	for rows.Next() {
		h, err := s.scanHandoff(rows)
		if err != nil {
			return nil, err
		}
		handoffs = append(handoffs, *h)
	}
	return handoffs, nil
}

func (s *HandoffStore) scanHandoff(rows interface{ Scan(...interface{}) error }) (*Handoff, error) {
	h := &Handoff{}
	var channelID, parentMessageID, description, response pgtype.Text
	var assignedAt, completedAt pgtype.Timestamptz
	var contextBytes []byte
	err := rows.Scan(&h.ID, &h.TenantID, &h.FromAgentID, &h.ToAgentID,
		&channelID, &parentMessageID,
		&h.Title, &description, &contextBytes, &h.Priority, &h.Status, &h.ExpiresAt,
		&assignedAt, &completedAt, &response, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if channelID.Valid {
		h.ChannelID = &channelID.String
	}
	if parentMessageID.Valid {
		h.ParentMessageID = &parentMessageID.String
	}
	if description.Valid {
		h.Description = &description.String
	}
	h.Context = make(map[string]interface{})
	if err := json.Unmarshal(contextBytes, &h.Context); err != nil {
		h.Context = make(map[string]interface{})
	}
	if response.Valid {
		h.Response = &response.String
	}
	if assignedAt.Valid {
		h.AssignedAt = &assignedAt.Time
	}
	if completedAt.Valid {
		h.CompletedAt = &completedAt.Time
	}
	return h, nil
}