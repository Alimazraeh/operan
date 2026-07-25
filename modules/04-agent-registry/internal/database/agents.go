package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRow is the durable form of a registered agent. The columns that are
// queried or displayed are columns; everything the API only ever reads and
// writes whole travels in Detail.
type AgentRow struct {
	ID               string
	TenantID         string
	Name             string
	Role             string
	Description      string
	DepartmentID     *string
	Status           string
	CurrentVersionID *string
	Detail           []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// VersionRow is the durable form of an agent version.
type VersionRow struct {
	ID                string
	AgentID           string
	TenantID          string
	Version           string
	Status            string
	Description       string
	ChangeSummary     string
	DiffFromPrevious  *string
	PromptTemplateRef *string
	CreatedBy         string
	ModelConfig       []byte
	PromotedTo        []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AgentStore reads and writes registry rows.
type AgentStore struct{ pool *pgxpool.Pool }

// NewAgentStore returns a store over pool.
func NewAgentStore(pool *pgxpool.Pool) *AgentStore { return &AgentStore{pool: pool} }

// UpsertAgent writes an agent. Create and update share one statement because
// the in-memory store is the arbiter of whether an id already exists — by the
// time a write reaches here the decision has been made, and a second opinion
// from the database could only disagree with it.
func (s *AgentStore) UpsertAgent(ctx context.Context, a AgentRow) error {
	detail := a.Detail
	if len(detail) == 0 {
		detail = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO registry_agents
			(id, tenant_id, name, role, description, department_id, status,
			 current_version_id, detail, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			name = EXCLUDED.name,
			role = EXCLUDED.role,
			description = EXCLUDED.description,
			department_id = EXCLUDED.department_id,
			status = EXCLUDED.status,
			current_version_id = EXCLUDED.current_version_id,
			detail = EXCLUDED.detail,
			updated_at = EXCLUDED.updated_at
	`, a.ID, a.TenantID, a.Name, a.Role, a.Description, a.DepartmentID, a.Status,
		a.CurrentVersionID, detail, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert agent %s: %w", a.ID, err)
	}
	return nil
}

// DeleteAgent removes an agent and its versions. The versions go too because
// they are meaningless without the agent and nothing else references them.
func (s *AgentStore) DeleteAgent(ctx context.Context, tenantID, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM registry_agent_versions WHERE agent_id = $1 AND tenant_id = $2`, id, tenantID); err != nil {
		return fmt.Errorf("delete versions of %s: %w", id, err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM registry_agents WHERE id = $1 AND tenant_id = $2`, id, tenantID); err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}
	return tx.Commit(ctx)
}

// LoadAgents returns every agent across all tenants, for rehydration at boot.
func (s *AgentStore) LoadAgents(ctx context.Context) ([]AgentRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, role, description, department_id, status,
		       current_version_id, detail, created_at, updated_at
		FROM registry_agents ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	defer rows.Close()

	var out []AgentRow
	for rows.Next() {
		var a AgentRow
		if err := rows.Scan(&a.ID, &a.TenantID, &a.Name, &a.Role, &a.Description,
			&a.DepartmentID, &a.Status, &a.CurrentVersionID, &a.Detail,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpsertVersion writes an agent version.
func (s *AgentStore) UpsertVersion(ctx context.Context, v VersionRow) error {
	modelCfg, promoted := v.ModelConfig, v.PromotedTo
	if len(modelCfg) == 0 {
		modelCfg = []byte("{}")
	}
	if len(promoted) == 0 {
		promoted = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO registry_agent_versions
			(id, agent_id, tenant_id, version, status, description, change_summary,
			 diff_from_previous, prompt_template_ref, created_by, model_config,
			 promoted_to, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			version = EXCLUDED.version,
			status = EXCLUDED.status,
			description = EXCLUDED.description,
			change_summary = EXCLUDED.change_summary,
			diff_from_previous = EXCLUDED.diff_from_previous,
			prompt_template_ref = EXCLUDED.prompt_template_ref,
			model_config = EXCLUDED.model_config,
			promoted_to = EXCLUDED.promoted_to,
			updated_at = EXCLUDED.updated_at
	`, v.ID, v.AgentID, v.TenantID, v.Version, v.Status, v.Description, v.ChangeSummary,
		v.DiffFromPrevious, v.PromptTemplateRef, v.CreatedBy, modelCfg, promoted,
		v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert version %s: %w", v.ID, err)
	}
	return nil
}

// LoadVersions returns every version across all tenants, for rehydration.
func (s *AgentStore) LoadVersions(ctx context.Context) ([]VersionRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, agent_id, tenant_id, version, status, description, change_summary,
		       diff_from_previous, prompt_template_ref, created_by, model_config,
		       promoted_to, created_at, updated_at
		FROM registry_agent_versions ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load versions: %w", err)
	}
	defer rows.Close()

	var out []VersionRow
	for rows.Next() {
		var v VersionRow
		if err := rows.Scan(&v.ID, &v.AgentID, &v.TenantID, &v.Version, &v.Status,
			&v.Description, &v.ChangeSummary, &v.DiffFromPrevious, &v.PromptTemplateRef,
			&v.CreatedBy, &v.ModelConfig, &v.PromotedTo, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// MarshalJSONB is a small helper so callers do not have to handle the nil case
// at every call site — a nil map must become '{}', not SQL NULL.
func MarshalJSONB(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || string(b) == "null" {
		return []byte("{}"), nil
	}
	return b, nil
}
