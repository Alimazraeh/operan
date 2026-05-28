// Package repository defines interfaces for data access and provides a
// PostgreSQL-backed implementation for each.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── Interfaces ──────────────────────────────────────────────────────────────

// WorkflowRepo provides data access for workflows, variables, checkpoints, and events.
type WorkflowRepo interface {
	Create(wf *store.Workflow) error
	GetByID(id string) (*store.Workflow, error)
	UpdateStatus(id string, status store.WorkflowStatus) error
	UpdateCurrentNodes(id string, nodeIDs []string) error
	List(tenantID string, page, pageSize int, status *string) ([]*store.Workflow, int, error)
	AddCheckpoint(cp *store.Checkpoint) error
	GetCheckpoints(workflowID string) ([]*store.Checkpoint, error)
	AddVariable(workflowID, tenantID string, key string, value json.RawMessage) error
	GetVariables(workflowID string) (*store.WorkflowVariables, error)
	SetVariables(workflowID, tenantID string, vars map[string]interface{}) error
	AddEvent(evt *store.ExecutionEvent) error
	GetExecutionHistory(workflowID string) ([]*store.ExecutionEvent, error)
	Delete(id string) error
}

// ScheduleRepo provides data access for schedules.
type ScheduleRepo interface {
	Create(sc *store.Schedule) error
	GetByID(id string) (*store.Schedule, error)
	Patch(id string, name, cron, workflowTemplateID *string, variables *map[string]interface{}, enabled *bool) (*store.Schedule, error)
	Delete(id string) error
	List(tenantID string, page, pageSize int, enabled *bool) ([]*store.Schedule, int, error)
}

// AgentRepo provides data access for agents, assignments, and availability.
type AgentRepo interface {
	CreateAgent(a *store.Agent) error
	GetAgentByID(id string) (*store.Agent, error)
	UpdateAgent(id string, name, desc, status, agentType *string, capabilities *[]string) (*store.Agent, error)
	DeleteAgent(id string) error
	ListAgents(tenantID string, page, pageSize int, status *string) ([]*store.Agent, int, error)
	CreateAssignment(a *store.AgentAssignment) error
	GetAssignmentByID(id string) (*store.AgentAssignment, error)
	ListAssignmentsByWorkflow(workflowID string) ([]*store.AgentAssignment, error)
	SetAgentAvailability(a *store.AgentAvailability) error
	GetAgentAvailability(agentID string) (*store.AgentAvailability, error)
	ListAgentAvailability() ([]*store.AgentAvailability, error)
}

// PipelineRepo provides data access for pipelines.
type PipelineRepo interface {
	Create(p *store.Pipeline) error
	GetByID(id string) (*store.Pipeline, error)
	Update(id string, name, description *string, steps []store.PipelineStep, errorHandling *store.PipelineErrorHandlingConfig, timeoutMinutes, maxRetries *int, status *store.PipelineStatus, variables *map[string]interface{}, tags *[]string) (*store.Pipeline, error)
	UpdateStatus(id string, status store.PipelineStatus) error
	Delete(id string) error
	List(tenantID string, page, pageSize int, status *string) ([]*store.Pipeline, int, error)
	IncrementExecutionCount(pipelineID string, success bool) error
}

// ExecutionRepo provides data access for executions and their steps.
type ExecutionRepo interface {
	Create(e *store.PipelineExecution) error
	GetByID(id string) (*store.PipelineExecution, error)
	UpdateStatus(id string, status store.PipelineExecutionStatus) error
	Delete(id string) error
	ListByPipeline(pipelineID string, page, pageSize int, status *string) ([]*store.PipelineExecution, int, error)
	ListByTenant(tenantID string, page, pageSize int) ([]*store.PipelineExecution, int, error)
	AddStep(step *store.PipelineExecutionStep) error
	GetSteps(executionID string) ([]*store.PipelineExecutionStep, error)
	IncrementRetryCount(id string) (int, error)
}

// HumanTaskRepo provides data access for human tasks.
type HumanTaskRepo interface {
	Create(t *store.HumanTask) error
	GetByID(id string) (*store.HumanTask, error)
	Respond(id string, action string, response map[string]interface{}, respondedBy, comments string) (*store.HumanTask, error)
	List(tenantID string, status *string) ([]*store.HumanTask, int, error)
}

// EscalationRepo provides data access for escalations.
type EscalationRepo interface {
	Create(e *store.Escalation) error
	GetByID(id string) (*store.Escalation, error)
	ListByWorkflow(workflowID string) ([]*store.Escalation, error)
	Acknowledge(id string) error
	Resolve(id string) error
}

// RetryRecordRepo provides data access for retry records.
type RetryRecordRepo interface {
	Create(r *store.RetryRecord) error
	GetByID(id string) (*store.RetryRecord, error)
	ListByWorkflow(workflowID string) ([]*store.RetryRecord, error)
	UpdateStatus(id string, status store.RetryStatus, errorCode, errorMessage string) error
}

// StackHealthRepo provides data access for stack health entries.
type StackHealthRepo interface {
	Create(e *store.StackHealthEntry) error
	GetByID(id string) (*store.StackHealthEntry, error)
	GetLatest() (*store.StackHealthEntry, error)
	Update(e *store.StackHealthEntry) error
	Delete(id string) error
	ListByStack(tenantID, stackType string) ([]*store.StackHealthEntry, error)
}

// ─── PostgreSQL implementation ───────────────────────────────────────────────

// PostgresRepo holds the database connection and provides all repository instances.
type PostgresRepo struct {
	DB      *sql.DB
	_wf     *WorkflowPostgres
	_sc     *SchedulePostgres
	_ag     *AgentPostgres
	_pl     *PipelinePostgres
	_ex     *ExecutionPostgres
	_ht     *HumanTaskPostgres
	_esc    *EscalationPostgres
	_rr     *RetryRecordPostgres
	_sh     *StackHealthPostgres
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{
		DB:  db,
		_wf: &WorkflowPostgres{db: db},
		_sc: &SchedulePostgres{db: db},
		_ag: &AgentPostgres{db: db},
		_pl: &PipelinePostgres{db: db},
		_ex: &ExecutionPostgres{db: db},
		_ht: &HumanTaskPostgres{db: db},
		_esc: &EscalationPostgres{db: db},
		_rr: &RetryRecordPostgres{db: db},
		_sh: &StackHealthPostgres{db: db},
	}
}

func (r *PostgresRepo) Workflow() WorkflowRepo              { return r._wf }
func (r *PostgresRepo) Schedule() ScheduleRepo              { return r._sc }
func (r *PostgresRepo) Agent() AgentRepo                    { return r._ag }
func (r *PostgresRepo) Pipeline() PipelineRepo              { return r._pl }
func (r *PostgresRepo) Execution() ExecutionRepo            { return r._ex }
func (r *PostgresRepo) HumanTask() HumanTaskRepo            { return r._ht }
func (r *PostgresRepo) Escalation() EscalationRepo          { return r._esc }
func (r *PostgresRepo) RetryRecord() RetryRecordRepo        { return r._rr }
func (r *PostgresRepo) StackHealth() StackHealthRepo        { return r._sh }

// ─── Helper functions ────────────────────────────────────────────────────────

// scanWorkflow scans a single row into a Workflow struct.
func scanWorkflow(row scanner) (*store.Workflow, error) {
	var wf store.Workflow
	var graphBytes, currentNodesBytes []byte

	err := row.Scan(
		&wf.ID, &wf.TenantID, &wf.DepartmentID, &wf.Name, &wf.Version,
		&wf.Status, &currentNodesBytes, &graphBytes,
		&wf.Priority, &wf.Description, &wf.CreatedBy, &wf.CreatedAt,
		&wf.StartedAt, &wf.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(graphBytes) > 0 {
		_ = json.Unmarshal(graphBytes, &wf.Graph)
	}
	if len(currentNodesBytes) > 0 {
		_ = json.Unmarshal(currentNodesBytes, &wf.CurrentNodes)
	}

	return &wf, nil
}

// scanSchedule scans a single row into a Schedule struct.
func scanSchedule(row scanner) (*store.Schedule, error) {
	var sc store.Schedule
	var variablesBytes []byte

	err := row.Scan(&sc.ID, &sc.TenantID, &sc.Name, &sc.Cron, &sc.WorkflowTemplateID,
		&variablesBytes, &sc.Enabled, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if len(variablesBytes) > 0 {
		_ = json.Unmarshal(variablesBytes, &sc.Variables)
	}

	return &sc, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

// ensureTenant ensures a tenant exists (or is valid) by checking the tenants table.
// This is a no-op for now since tenant validation is handled at the API layer.
func ensureTenant(ctx context.Context, db *sql.DB, tenantID string) error {
	// Tenant validation is handled by middleware before reaching the repository.
	// This is a hook for future multi-tenant enforcement.
	_ = tenantID
	_ = ctx
	return nil
}

// encodeJSONB serializes a value to JSON bytes, returning []byte or nil.
func encodeJSONB(v interface{}) []byte {
	if v == nil {
		return []byte("null")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}
