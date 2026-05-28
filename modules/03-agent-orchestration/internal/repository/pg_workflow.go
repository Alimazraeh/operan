package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// WorkflowPostgres implements WorkflowRepo using PostgreSQL.
type WorkflowPostgres struct {
	db *sql.DB
}

func (r *WorkflowPostgres) Create(wf *store.Workflow) error {
	ctx := defaultCtx()
	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}
	wf.CreatedAt = time.Now().UTC()
	if wf.Status == "" {
		wf.Status = store.WorkflowStatusPending
	}
	wf.Version = "1"

	graphBytes, _ := json.Marshal(wf.Graph)
	currentNodesBytes, _ := json.Marshal(wf.CurrentNodes)

	query := `INSERT INTO workflows
		(id, tenant_id, department_id, name, version, status, current_nodes, graph,
		 priority, description, created_by, started_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	_, err := r.db.ExecContext(ctx, query,
		wf.ID, wf.TenantID, wf.DepartmentID, wf.Name, wf.Version,
		wf.Status, currentNodesBytes, graphBytes,
		wf.Priority, wf.Description, wf.CreatedBy,
		wf.StartedAt, wf.CompletedAt,
	)
	return err
}

func (r *WorkflowPostgres) GetByID(id string) (*store.Workflow, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,department_id,name,version,status,current_nodes,graph,
		priority,description,created_by,created_at,started_at,completed_at
		FROM workflows WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanWorkflow(row)
}

func (r *WorkflowPostgres) UpdateStatus(id string, status store.WorkflowStatus) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		"UPDATE workflows SET status=$1 WHERE id=$2", status, id)
	return err
}

func (r *WorkflowPostgres) UpdateCurrentNodes(id string, nodeIDs []string) error {
	ctx := defaultCtx()
	bytes, _ := json.Marshal(nodeIDs)
	_, err := r.db.ExecContext(ctx,
		"UPDATE workflows SET current_nodes=$1 WHERE id=$2", bytes, id)
	return err
}

func (r *WorkflowPostgres) List(tenantID string, page, pageSize int, status *string) ([]*store.Workflow, int, error) {
	ctx := defaultCtx()
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	countQuery := "SELECT COUNT(*) FROM workflows WHERE tenant_id=$1"
	params := []interface{}{tenantID}
	if status != nil && *status != "" {
		countQuery += " AND status=$2"
		params = append(params, *status)
	}
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, params...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT id,tenant_id,department_id,name,version,status,current_nodes,graph,
		priority,description,created_by,created_at,started_at,completed_at
		FROM workflows WHERE tenant_id=$1`
	if status != nil && *status != "" {
		query += " AND status=$2"
	}
	query += " ORDER BY created_at DESC LIMIT $3 OFFSET $4"
	params = append(params, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.Workflow
	for rows.Next() {
		wf, err := scanWorkflow(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, wf)
	}
	return results, total, nil
}

func (r *WorkflowPostgres) AddCheckpoint(cp *store.Checkpoint) error {
	ctx := defaultCtx()
	if cp.ID == "" {
		cp.ID = uuid.New().String()
	}
	cp.Timestamp = time.Now().UTC()
	stateBytes, _ := json.Marshal(cp.StateSnapshot)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO workflow_checkpoints (id, workflow_id, node_id, timestamp, state_snapshot, checksum)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		cp.ID, cp.WorkflowID, cp.NodeID, cp.Timestamp, stateBytes, cp.Checksum)
	return err
}

func (r *WorkflowPostgres) GetCheckpoints(workflowID string) ([]*store.Checkpoint, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,workflow_id,node_id,timestamp,state_snapshot,checksum
		 FROM workflow_checkpoints WHERE workflow_id=$1 ORDER BY timestamp`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.Checkpoint
	for rows.Next() {
		var cp store.Checkpoint
		var stateBytes []byte
		err := rows.Scan(&cp.ID, &cp.WorkflowID, &cp.NodeID, &cp.Timestamp, &stateBytes, &cp.Checksum)
		if err != nil {
			return nil, err
		}
		if len(stateBytes) > 0 {
			_ = json.Unmarshal(stateBytes, &cp.StateSnapshot)
		}
		results = append(results, &cp)
	}
	return results, nil
}

func (r *WorkflowPostgres) AddVariable(workflowID, tenantID string, key string, value json.RawMessage) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO workflow_variables (workflow_id, tenant_id, key, value)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (workflow_id, key) DO UPDATE SET value=EXCLUDED.value, version=workflow_variables.version+1`,
		workflowID, tenantID, key, value)
	return err
}

func (r *WorkflowPostgres) GetVariables(workflowID string) (*store.WorkflowVariables, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, value, version FROM workflow_variables WHERE workflow_id=$1`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vs := &store.WorkflowVariables{Variables: make(map[string]interface{})}
	for rows.Next() {
		var key string
		var valBytes []byte
		var version int
		if err := rows.Scan(&key, &valBytes, &version); err != nil {
			return nil, err
		}
		var val interface{}
		if err := json.Unmarshal(valBytes, &val); err != nil {
			val = string(valBytes)
		}
		vs.Variables[key] = val
	}
	return vs, nil
}

func (r *WorkflowPostgres) SetVariables(workflowID, tenantID string, vars map[string]interface{}) error {
	ctx := defaultCtx()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, val := range vars {
		valBytes, _ := json.Marshal(val)
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_variables (workflow_id, tenant_id, key, value)
			 VALUES ($1,$2,$3,$4)
			 ON CONFLICT (workflow_id, key) DO UPDATE SET value=EXCLUDED.value, version=workflow_variables.version+1`,
			workflowID, tenantID, key, valBytes)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *WorkflowPostgres) AddEvent(evt *store.ExecutionEvent) error {
	ctx := defaultCtx()
	if evt.EventID == "" {
		evt.EventID = uuid.New().String()
	}
	evt.Timestamp = time.Now().UTC()
	detailsBytes, _ := json.Marshal(evt.Details)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO workflow_events (event_id, workflow_id, node_id, event_type, timestamp, details)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		evt.EventID, evt.NodeID, evt.NodeID, evt.EventType, evt.Timestamp, detailsBytes)
	return err
}

func (r *WorkflowPostgres) GetExecutionHistory(workflowID string) ([]*store.ExecutionEvent, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT event_id, node_id, event_type, timestamp, details
		 FROM workflow_events WHERE workflow_id=$1 ORDER BY timestamp`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.ExecutionEvent
	for rows.Next() {
		var evt store.ExecutionEvent
		var detailsBytes []byte
		err := rows.Scan(&evt.EventID, &evt.NodeID, &evt.EventType, &evt.Timestamp, &detailsBytes)
		if err != nil {
			return nil, err
		}
		if len(detailsBytes) > 0 {
			_ = json.Unmarshal(detailsBytes, &evt.Details)
		}
		results = append(results, &evt)
	}
	return results, nil
}

func (r *WorkflowPostgres) Delete(id string) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM workflows WHERE id=$1", id)
	return err
}

func defaultCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	return &cancelCtx{ctx, cancel}
}

type cancelCtx struct {
	context.Context
	cancel func()
}

func (c *cancelCtx) Done() <-chan struct{}  { return c.Context.Done() }
func (c *cancelCtx) Err() error             { return c.Context.Err() }
func (c *cancelCtx) Value(key interface{}) interface{} { return c.Context.Value(key) }
func (c *cancelCtx) Deadline() (deadline time.Time, ok bool) { return time.Time{}, false }

// ─── Schedule implementation ─────────────────────────────────────────────────

type SchedulePostgres struct {
	db *sql.DB
}

func (r *SchedulePostgres) Create(sc *store.Schedule) error {
	ctx := defaultCtx()
	if sc.ID == "" {
		sc.ID = uuid.New().String()
	}
	sc.CreatedAt = time.Now().UTC()
	sc.UpdatedAt = sc.CreatedAt

	variablesBytes, _ := json.Marshal(sc.Variables)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO schedules (id, tenant_id, name, cron, workflow_template_id, variables, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sc.ID, sc.TenantID, sc.Name, sc.Cron, sc.WorkflowTemplateID, variablesBytes, sc.Enabled)
	return err
}

func (r *SchedulePostgres) GetByID(id string) (*store.Schedule, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,name,cron,workflow_template_id,variables,enabled,created_at,updated_at
		FROM schedules WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanSchedule(row)
}

func (r *SchedulePostgres) Patch(id string, name, cron, workflowTemplateID *string, variables *map[string]interface{}, enabled *bool) (*store.Schedule, error) {
	ctx := defaultCtx()
	query := "UPDATE schedules SET updated_at=now()"
	args := []interface{}{}
	argIdx := 1

	if name != nil {
		query += fmt.Sprintf(", name=$%d", argIdx)
		args = append(args, *name)
		argIdx++
	}
	if cron != nil {
		query += fmt.Sprintf(", cron=$%d", argIdx)
		args = append(args, *cron)
		argIdx++
	}
	if workflowTemplateID != nil {
		query += fmt.Sprintf(", workflow_template_id=$%d", argIdx)
		args = append(args, *workflowTemplateID)
		argIdx++
	}
	if variables != nil {
		bytes, _ := json.Marshal(variables)
		query += fmt.Sprintf(", variables=$%d", argIdx)
		args = append(args, bytes)
		argIdx++
	}
	if enabled != nil {
		query += fmt.Sprintf(", enabled=$%d", argIdx)
		args = append(args, *enabled)
		argIdx++
	}

	query += " WHERE id=$" + fmt.Sprint(argIdx)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *SchedulePostgres) Delete(id string) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx, "DELETE FROM schedules WHERE id=$1", id)
	return err
}

func (r *SchedulePostgres) List(tenantID string, page, pageSize int, enabled *bool) ([]*store.Schedule, int, error) {
	ctx := defaultCtx()
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := `SELECT COUNT(*) FROM schedules WHERE tenant_id=$1`
	params := []interface{}{tenantID}
	if enabled != nil {
		query += " AND enabled=$2"
		params = append(params, *enabled)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, query, params...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query = `SELECT id,tenant_id,name,cron,workflow_template_id,variables,enabled,created_at,updated_at
		FROM schedules WHERE tenant_id=$1`
	if enabled != nil {
		query += " AND enabled=$2"
	}
	query += " ORDER BY name ASC LIMIT $3 OFFSET $4"
	params = append(params, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, sc)
	}
	return results, total, nil
}

// ─── Agent implementation ────────────────────────────────────────────────────

type AgentPostgres struct {
	db *sql.DB
}

func (r *AgentPostgres) CreateAgent(a *store.Agent) error {
	ctx := defaultCtx()
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now().UTC()
	a.UpdatedAt = a.CreatedAt

	capsBytes, _ := json.Marshal(a.Capabilities)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO agents (id, tenant_id, name, type, capabilities, status)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		a.ID, a.TenantID, a.Name, a.Type, capsBytes, a.Status)
	return err
}

func (r *AgentPostgres) GetAgentByID(id string) (*store.Agent, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,name,type,capabilities,status,created_at,updated_at
		FROM agents WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)

	var a store.Agent
	var capsBytes []byte
	err := row.Scan(&a.ID, &a.TenantID, &a.Name, &a.Type, &capsBytes, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(capsBytes) > 0 {
		_ = json.Unmarshal(capsBytes, &a.Capabilities)
	}
	return &a, nil
}

func (r *AgentPostgres) UpdateAgent(id string, name, desc, status, agentType *string, capabilities *[]string) (*store.Agent, error) {
	ctx := defaultCtx()
	query := "UPDATE agents SET updated_at=now()"
	args := []interface{}{}
	argIdx := 1

	if name != nil {
		query += fmt.Sprintf(", name=$%d", argIdx)
		args = append(args, *name)
		argIdx++
	}
	if status != nil {
		query += fmt.Sprintf(", status=$%d", argIdx)
		args = append(args, *status)
		argIdx++
	}
	if agentType != nil {
		query += fmt.Sprintf(", type=$%d", argIdx)
		args = append(args, *agentType)
		argIdx++
	}
	if capabilities != nil {
		bytes, _ := json.Marshal(capabilities)
		query += fmt.Sprintf(", capabilities=$%d", argIdx)
		args = append(args, bytes)
		argIdx++
	}

	query += " WHERE id=$" + fmt.Sprint(argIdx)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return r.GetAgentByID(id)
}

func (r *AgentPostgres) DeleteAgent(id string) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx, "DELETE FROM agents WHERE id=$1", id)
	return err
}

func (r *AgentPostgres) ListAgents(tenantID string, page, pageSize int, status *string) ([]*store.Agent, int, error) {
	ctx := defaultCtx()
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	offset := (page - 1) * pageSize

	query := `SELECT COUNT(*) FROM agents WHERE tenant_id=$1`
	params := []interface{}{tenantID}
	if status != nil && *status != "" {
		query += " AND status=$2"
		params = append(params, *status)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, query, params...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query = `SELECT id,tenant_id,name,type,capabilities,status,created_at,updated_at
		FROM agents WHERE tenant_id=$1`
	if status != nil && *status != "" {
		query += " AND status=$2"
	}
	query += " ORDER BY created_at DESC LIMIT $3 OFFSET $4"
	params = append(params, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.Agent
	for rows.Next() {
		a, err := r.scanAgent(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, a)
	}
	return results, total, nil
}

func (r *AgentPostgres) scanAgent(row scanner) (*store.Agent, error) {
	var a store.Agent
	var capsBytes []byte
	err := row.Scan(&a.ID, &a.TenantID, &a.Name, &a.Type, &capsBytes, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(capsBytes) > 0 {
		_ = json.Unmarshal(capsBytes, &a.Capabilities)
	}
	return &a, nil
}

func (r *AgentPostgres) CreateAssignment(a *store.AgentAssignment) error {
	ctx := defaultCtx()
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.AssignedAt = time.Now().UTC()

	paramsBytes, _ := json.Marshal(a.Parameters)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO agent_assignments (id, tenant_id, workflow_id, node_id, agent_id, parameters, assigned_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		a.ID, a.TenantID, a.WorkflowID, a.NodeID, a.AgentID, paramsBytes, a.AssignedAt)
	return err
}

func (r *AgentPostgres) GetAssignmentByID(id string) (*store.AgentAssignment, error) {
	ctx := defaultCtx()
	query := `SELECT id,tenant_id,workflow_id,node_id,agent_id,parameters,assigned_at
		FROM agent_assignments WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)

	var a store.AgentAssignment
	var paramsBytes []byte
	err := row.Scan(&a.ID, &a.TenantID, &a.WorkflowID, &a.NodeID, &a.AgentID, &paramsBytes, &a.AssignedAt)
	if err != nil {
		return nil, err
	}
	if len(paramsBytes) > 0 {
		_ = json.Unmarshal(paramsBytes, &a.Parameters)
	}
	return &a, nil
}

func (r *AgentPostgres) ListAssignmentsByWorkflow(workflowID string) ([]*store.AgentAssignment, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,tenant_id,workflow_id,node_id,agent_id,parameters,assigned_at
		 FROM agent_assignments WHERE workflow_id=$1 ORDER BY assigned_at`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.AgentAssignment
	for rows.Next() {
		a, err := r.scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, nil
}

func (r *AgentPostgres) scanAssignment(row scanner) (*store.AgentAssignment, error) {
	var a store.AgentAssignment
	var paramsBytes []byte
	err := row.Scan(&a.ID, &a.TenantID, &a.WorkflowID, &a.NodeID, &a.AgentID, &paramsBytes, &a.AssignedAt)
	if err != nil {
		return nil, err
	}
	if len(paramsBytes) > 0 {
		_ = json.Unmarshal(paramsBytes, &a.Parameters)
	}
	return &a, nil
}

func (r *AgentPostgres) SetAgentAvailability(a *store.AgentAvailability) error {
	ctx := defaultCtx()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO agent_availability (agent_id, status, current_workflows, max_concurrency, last_seen_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (agent_id) DO UPDATE SET status=EXCLUDED.status, current_workflows=EXCLUDED.current_workflows,
		 max_concurrency=EXCLUDED.max_concurrency, last_seen_at=EXCLUDED.last_seen_at`,
		a.AgentID, a.Status, a.CurrentWorkflows, a.MaxConcurrency, a.LastSeenAt)
	return err
}

func (r *AgentPostgres) GetAgentAvailability(agentID string) (*store.AgentAvailability, error) {
	ctx := defaultCtx()
	query := `SELECT agent_id, status, current_workflows, max_concurrency, last_seen_at
		FROM agent_availability WHERE agent_id=$1`
	row := r.db.QueryRowContext(ctx, query, agentID)

	var a store.AgentAvailability
	err := row.Scan(&a.AgentID, &a.Status, &a.CurrentWorkflows, &a.MaxConcurrency, &a.LastSeenAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AgentPostgres) ListAgentAvailability() ([]*store.AgentAvailability, error) {
	ctx := defaultCtx()
	rows, err := r.db.QueryContext(ctx,
		`SELECT agent_id, status, current_workflows, max_concurrency, last_seen_at
		 FROM agent_availability`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.AgentAvailability
	for rows.Next() {
		var a store.AgentAvailability
		if err := rows.Scan(&a.AgentID, &a.Status, &a.CurrentWorkflows, &a.MaxConcurrency, &a.LastSeenAt); err != nil {
			return nil, err
		}
		results = append(results, &a)
	}
	return results, nil
}
