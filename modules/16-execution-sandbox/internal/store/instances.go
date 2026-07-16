package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InstanceStore handles sandbox instance CRUD operations.
type InstanceStore struct {
	pool PgxPool
}

// NewInstanceStore creates a new InstanceStore.
func NewInstanceStore(pool PgxPool) *InstanceStore {
	return &InstanceStore{pool: pool}
}

// Create inserts a new sandbox instance record.
func (s *InstanceStore) Create(ctx context.Context, inst *SandboxInstance) error {
	query := `
		INSERT INTO sandbox_instances (tenant_id, agent_id, profile_id, tool_name,
			input_data, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, query,
		inst.TenantID, inst.AgentID, inst.ProfileID, inst.ToolName,
		inst.InputData, inst.Status,
	).Scan(&id, &inst.CreatedAt)
	if err != nil {
		return err
	}
	inst.ID = id
	return nil
}

// UpdateStatus updates the status and related fields of an instance.
func (s *InstanceStore) UpdateStatus(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setParts := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+1)
	i := 1
	for k, v := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}
	args = append(args, id)

	query := fmt.Sprintf("UPDATE sandbox_instances SET %s WHERE id = $%d",
		setParts[0], len(updates)+1)
	for j := 1; j < len(setParts); j++ {
		query += ", " + setParts[j]
		query += " WHERE id = $" + fmt.Sprintf("%d", len(setParts)+1)
	}
	// Simpler approach:
	query = "UPDATE sandbox_instances SET " + setParts[0]
	for j := 1; j < len(setParts); j++ {
		query += ", " + setParts[j]
	}
	query += " WHERE id = $" + fmt.Sprintf("%d", len(setParts)+1)

	_, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	return nil
}

// GetByID retrieves an instance by ID.
func (s *InstanceStore) GetByID(ctx context.Context, id uuid.UUID, tenantID string) (*SandboxInstance, error) {
	query := `
		SELECT id, tenant_id, agent_id, profile_id, tool_name, input_data,
			exit_code, stdout, stderr, status, cpu_time_ms, memory_peak_mb,
			error_message, started_at, completed_at, created_at
		FROM sandbox_instances
		WHERE id = $1 AND tenant_id = $2`

	inst := &SandboxInstance{}
	var exitCode, cpuMs, memPeak *int
	var stdoutB, stderrB, inputDataB, errMsgB, agentIDB *string

	err := s.pool.QueryRow(ctx, query, id, tenantID).Scan(
		&inst.ID, &inst.TenantID, &agentIDB, &inst.ProfileID, &inst.ToolName,
		&inputDataB, &exitCode, &stdoutB, &stderrB, &inst.Status,
		&cpuMs, &memPeak, &errMsgB, &inst.StartedAt, &inst.CompletedAt, &inst.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	inst.ExitCode = exitCode
	inst.CPUMs = cpuMs
	inst.MemoryPeakMB = memPeak
	inst.Stdout = stdoutB
	inst.Stderr = stderrB
	inst.InputData = inputDataB
	inst.ErrorMessage = errMsgB
	inst.AgentID = agentIDB
	return inst, nil
}

// List returns paginated instances for a tenant with optional filters.
func (s *InstanceStore) List(ctx context.Context, tenantID, agentID, status string, page, pageSize int) ([]SandboxInstance, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Build query dynamically based on filters
	query := `
		SELECT id, tenant_id, agent_id, profile_id, tool_name, input_data,
			exit_code, stdout, stderr, status, cpu_time_ms, memory_peak_mb,
			error_message, started_at, completed_at, created_at
		FROM sandbox_instances
		WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if agentID != "" {
		query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, agentID)
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

	var instances []SandboxInstance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, *inst)
	}

	// Count query
	countQuery := `SELECT COUNT(*) FROM sandbox_instances WHERE tenant_id = $1`
	countArgs := []interface{}{tenantID}
	if agentID != "" {
		countQuery += " AND agent_id = $2"
		countArgs = append(countArgs, agentID)
	}
	if status != "" {
		countQuery += " AND status = $" + fmt.Sprintf("%d", len(countArgs)+1)
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

	return instances, total, nil
}

func scanInstance(rows interface{ Scan(...interface{}) error }) (*SandboxInstance, error) {
	inst := &SandboxInstance{}
	var exitCode, cpuMs, memPeak *int
	var stdoutB, stderrB, inputDataB, errMsgB, agentIDB *string

	err := rows.Scan(
		&inst.ID, &inst.TenantID, &agentIDB, &inst.ProfileID, &inst.ToolName,
		&inputDataB, &exitCode, &stdoutB, &stderrB, &inst.Status,
		&cpuMs, &memPeak, &errMsgB, &inst.StartedAt, &inst.CompletedAt, &inst.CreatedAt)
	if err != nil {
		return nil, err
	}
	inst.ExitCode = exitCode
	inst.CPUMs = cpuMs
	inst.MemoryPeakMB = memPeak
	inst.Stdout = stdoutB
	inst.Stderr = stderrB
	inst.InputData = inputDataB
	inst.ErrorMessage = errMsgB
	inst.AgentID = agentIDB
	return inst, nil
}