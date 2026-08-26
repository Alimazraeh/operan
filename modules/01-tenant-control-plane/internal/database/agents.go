package database

import (
	"context"
	"fmt"
	"time"
)

// AgentRow is the durable form of a tenant agent configuration.
type AgentRow struct {
	ID              string
	TenantID        string
	Name            string
	Model           string
	Role            string
	SystemPrompt    string
	Status          string
	CurrentWorkflow *string
	CurrentTask     *string
	ToolAccess      []byte
	LastRunAt       *time.Time
	SuccessCount    int
	FailureCount    int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpsertAgent writes an agent configuration.
func (s *ControlPlaneStore) UpsertAgent(ctx context.Context, a AgentRow) error {
	toolAccess := a.ToolAccess
	if len(toolAccess) == 0 {
		toolAccess = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_agents
			(id, tenant_id, name, model, role, system_prompt, status,
			 current_workflow, current_task, tool_access, last_run_at,
			 success_count, failure_count, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			model = EXCLUDED.model,
			role = EXCLUDED.role,
			system_prompt = EXCLUDED.system_prompt,
			status = EXCLUDED.status,
			current_workflow = EXCLUDED.current_workflow,
			current_task = EXCLUDED.current_task,
			tool_access = EXCLUDED.tool_access,
			last_run_at = EXCLUDED.last_run_at,
			success_count = EXCLUDED.success_count,
			failure_count = EXCLUDED.failure_count,
			updated_at = EXCLUDED.updated_at
	`, a.ID, a.TenantID, a.Name, a.Model, a.Role, a.SystemPrompt, a.Status,
		a.CurrentWorkflow, a.CurrentTask, toolAccess, a.LastRunAt,
		a.SuccessCount, a.FailureCount, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert agent %s: %w", a.ID, err)
	}
	return nil
}

// DeleteAgent removes an agent configuration.
func (s *ControlPlaneStore) DeleteAgent(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_agents WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}
	return nil
}

// LoadAgents returns every agent configuration, for rehydration at boot.
func (s *ControlPlaneStore) LoadAgents(ctx context.Context) ([]AgentRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, model, role, system_prompt, status,
		       current_workflow, current_task, tool_access, last_run_at,
		       success_count, failure_count, created_at, updated_at
		FROM tctl_agents ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	defer rows.Close()

	var out []AgentRow
	for rows.Next() {
		var a AgentRow
		if err := rows.Scan(&a.ID, &a.TenantID, &a.Name, &a.Model, &a.Role,
			&a.SystemPrompt, &a.Status, &a.CurrentWorkflow, &a.CurrentTask,
			&a.ToolAccess, &a.LastRunAt, &a.SuccessCount, &a.FailureCount,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
