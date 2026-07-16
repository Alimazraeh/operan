package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PresenceStore handles presence operations.
type PresenceStore struct {
	pool PgxPool
}

func NewPresenceStore(pool PgxPool) *PresenceStore {
	return &PresenceStore{pool: pool}
}

func (s *PresenceStore) Upsert(ctx context.Context, p *Presence) error {
	metadataBytes, _ := json.Marshal(p.Metadata)
	query := `
		INSERT INTO presence (tenant_id, agent_id, status, last_heartbeat, metadata)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, agent_id) DO UPDATE
			SET status = $3, last_heartbeat = $4, metadata = $5, updated_at = NOW()
		RETURNING id, created_at`
	// Note: presence table doesn't have created_at/updated_at in the schema, but we scan for compatibility
	_ = query
	_, err := s.pool.Exec(ctx,
		`INSERT INTO presence (tenant_id, agent_id, status, last_heartbeat, metadata)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id, agent_id) DO UPDATE
				SET status = $3, last_heartbeat = $4, metadata = $5`,
		p.TenantID, p.AgentID, p.Status, p.LastHeartbeat, metadataBytes)
	return err
}

func (s *PresenceStore) GetByAgentID(ctx context.Context, tenantID, agentID string) (*Presence, error) {
	return s.getFields(ctx, "SELECT id, tenant_id, agent_id, status, last_heartbeat, metadata FROM presence WHERE tenant_id = $1 AND agent_id = $2", []interface{}{tenantID, agentID})
}

func (s *PresenceStore) List(ctx context.Context, tenantID, agentIDFilter string) ([]Presence, error) {
	query := "SELECT id, tenant_id, agent_id, status, last_heartbeat, metadata FROM presence WHERE tenant_id = $1"
	var args []interface{} = []interface{}{tenantID}
	argIdx := 2
	if agentIDFilter != "" {
		query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, agentIDFilter)
	}
	query += " ORDER BY last_heartbeat DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var presences []Presence
	for rows.Next() {
		p, err := s.scanPresence(rows)
		if err != nil {
			return nil, err
		}
		presences = append(presences, *p)
	}
	return presences, nil
}

func (s *PresenceStore) MarkOffline(ctx context.Context, tenantID, agentID string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE presence SET status = 'offline', last_heartbeat = NOW() WHERE tenant_id = $1 AND agent_id = $2",
		tenantID, agentID)
	return err
}

func (s *PresenceStore) MarkAway(ctx context.Context, tenantID, agentID string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE presence SET status = 'away' WHERE tenant_id = $1 AND agent_id = $2",
		tenantID, agentID)
	return err
}

func (s *PresenceStore) MarkOnline(ctx context.Context, tenantID, agentID string) error {
	return s.Upsert(ctx, &Presence{TenantID: tenantID, AgentID: agentID, Status: "online"})
}

func (s *PresenceStore) getFields(ctx context.Context, query string, vals []interface{}) (*Presence, error) {
	p := &Presence{}
	var metadataBytes []byte
	err := s.pool.QueryRow(ctx, query, vals...).Scan(
		&p.ID, &p.TenantID, &p.AgentID, &p.Status, &p.LastHeartbeat, &metadataBytes)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Metadata = make(map[string]interface{})
	if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
		p.Metadata = make(map[string]interface{})
	}
	return p, nil
}

func (s *PresenceStore) scanPresence(rows interface{ Scan(...interface{}) error }) (*Presence, error) {
	p := &Presence{}
	var metadataBytes []byte
	err := rows.Scan(&p.ID, &p.TenantID, &p.AgentID, &p.Status, &p.LastHeartbeat, &metadataBytes)
	if err != nil {
		return nil, err
	}
	p.Metadata = make(map[string]interface{})
	if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
		p.Metadata = make(map[string]interface{})
	}
	return p, nil
}