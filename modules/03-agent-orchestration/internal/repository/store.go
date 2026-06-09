// Package repository provides the unified data access layer for the
// orchestration engine, supporting both in-memory stores and PostgreSQL.
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/operan/modules/03-agent-orchestration/internal/config"
	"github.com/operan/modules/03-agent-orchestration/internal/database"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── Mode ─────────────────────────────────────────────────────────────────────

// Mode selects the backing store.
type Mode int

const (
	ModeInMemory Mode = iota
	ModePostgreSQL
)

// NewModeFromConfig returns a Mode based on DB config.
// If DB_HOST is the default and no explicit mode is set, defaults to InMemory.
// To force PostgreSQL, set DB_MODE=postgres or DB_HOST to a non-default value.
func NewModeFromConfig(cfg config.Config) Mode {
	if mode := getEnvOrDefault("DB_MODE", ""); mode == "postgres" {
		return ModePostgreSQL
	}
	// If any DB_ env var is non-default, try PostgreSQL
	if cfg.DBHost != database.DefaultConfig().Host || cfg.DBPort != database.DefaultConfig().Port {
		return ModePostgreSQL
	}
	return ModeInMemory
}

// Store is the unified data access layer. It wraps either in-memory stores
// or a PostgreSQL-backed PostgresRepo. All handlers interact with this type.
type Store struct {
	mode    Mode
	// In-memory fields
	wf      *store.WorkflowStore
	sc      *store.ScheduleStore
	ag      *store.AgentStore
	pl      *store.PipelineStore
	ex      *store.ExecutionStore
	ht      *store.HumanTaskStore
	esc     *store.EscalationStore
	retry   *store.RetryRecordStore
	health  *store.StackHealthStore
	delegate *store.DelegationStore

	// PostgreSQL fields
	repo Repository
}

// Repository is the PostgreSQL repository interface.
type Repository interface {
	Workflow() WorkflowRepo
	Schedule() ScheduleRepo
	Agent() AgentRepo
	Pipeline() PipelineRepo
	Execution() ExecutionRepo
	HumanTask() HumanTaskRepo
	Escalation() EscalationRepo
	RetryRecord() RetryRecordRepo
	StackHealth() StackHealthRepo
}

// ─── Store interfaces for passthrough wrappers ────────────────────────────────

// WorkflowStoreIface is the set of methods WorkflowStore must expose.
type WorkflowStoreIface interface {
	Create(*store.Workflow) (*store.Workflow, error)
	GetByID(string) (*store.Workflow, error)
	GetByIDAndTenant(string, string) (*store.Workflow, error)
	UpdateStatus(string, store.WorkflowStatus) error
	UpdateStatusAndTenant(string, string, store.WorkflowStatus) error
	UpdateCurrentNodes(string, []string) error
	List(string, int, int, *string) ([]*store.Workflow, int, bool)
	AddCheckpoint(store.Checkpoint)
	GetCheckpoints(string) []store.Checkpoint
	AddVariable(string, string, string, interface{}) error
	GetVariables(string) (*store.WorkflowVariables, error)
	SetVariables(string, string, map[string]interface{}) error
	AddEvent(string, store.ExecutionEvent)
	GetExecutionHistory(string) []store.ExecutionEvent
	Delete(string) error
}

// ScheduleStoreIface is the set of methods ScheduleStore must expose.
type ScheduleStoreIface interface {
	Create(*store.Schedule) (*store.Schedule, error)
	GetByID(string) (*store.Schedule, error)
	GetByIDAndTenant(string, string) (*store.Schedule, error)
	Patch(string, *string, *string, *string, *map[string]interface{}, *bool) (*store.Schedule, error)
	UpdateStatusAndTenant(string, string, bool) error
	Delete(string) error
	List(string, int, int, *bool) ([]*store.Schedule, int, bool)
}

// AgentStoreIface is the set of methods AgentStore must expose.
type AgentStoreIface interface {
	CreateAssignment(*store.AgentAssignment) (*store.AgentAssignment, error)
	GetByID(string) (*store.AgentAssignment, error)
	SetAgentAvailability(*store.AgentAvailability)
	GetAgentAvailability(string) (*store.AgentAvailability, error)
	ListByWorkflow(string) ([]*store.AgentAssignment, error)
	ListAgentAvailability() []*store.AgentAvailability
	ListByTenant(string) []*store.AgentAvailability
}

// PipelineStoreIface is the set of methods PipelineStore must expose.
type PipelineStoreIface interface {
	Create(*store.Pipeline) (*store.Pipeline, error)
	GetByID(string) (*store.Pipeline, error)
	GetByIDAndTenant(string, string) (*store.Pipeline, error)
	Update(string, *string, *string, *[]store.PipelineStep, *store.PipelineErrorHandlingConfig, *int, *int, *store.PipelineStatus, *map[string]interface{}, *[]string) (*store.Pipeline, error)
	UpdateStatus(string, store.PipelineStatus) error
	UpdateStatusAndTenant(string, string, store.PipelineStatus) error
	Delete(string) error
	List(string, int, int, *string) ([]*store.Pipeline, int, bool)
	IncrementExecutionCount(string, bool)
}

// ExecutionStoreIface is the set of methods ExecutionStore must expose.
type ExecutionStoreIface interface {
	Create(*store.PipelineExecution) (*store.PipelineExecution, error)
	GetByID(string) (*store.PipelineExecution, error)
	GetByIDAndTenant(string, string) (*store.PipelineExecution, error)
	UpdateStatus(string, store.PipelineExecutionStatus) error
	UpdateStatusAndTenant(string, string, store.PipelineExecutionStatus) error
	Delete(string) error
	ListByPipeline(string, int, int, *string, int) ([]*store.PipelineExecution, int, bool)
	ListByTenant(string, int, int) ([]*store.PipelineExecution, int, bool)
	AddStep(*store.PipelineExecutionStep)
	GetSteps(string) []*store.PipelineExecutionStep
	IncrementRetryCount(string) int
}

// HumanTaskStoreIface is the set of methods HumanTaskStore must expose.
type HumanTaskStoreIface interface {
	Create(*store.HumanTask) (*store.HumanTask, error)
	GetByID(string) (*store.HumanTask, error)
	GetByIDAndTenant(string, string) (*store.HumanTask, error)
	Respond(string, string, map[string]interface{}, string, string) (*store.HumanTask, error)
	UpdateStatusAndTenant(string, string, store.HumanTaskStatus) error
	List(string, *string) ([]*store.HumanTask, int)
}

// EscalationStoreIface is the set of methods EscalationStore must expose.
type EscalationStoreIface interface {
	Create(*store.Escalation)
	GetByID(string) (*store.Escalation, bool)
	ListByWorkflow(string) []*store.Escalation
	Acknowledge(string) bool
	Resolve(string) bool
}

// RetryRecordStoreIface is the set of methods RetryRecordStore must expose.
type RetryRecordStoreIface interface {
	Create(*store.RetryRecord)
	GetByID(string) (*store.RetryRecord, bool)
	ListByWorkflow(string) []*store.RetryRecord
	UpdateStatus(string, store.RetryStatus, string, string) bool
}

// StackHealthStoreIface is the set of methods StackHealthStore must expose.
type StackHealthStoreIface interface {
	Create(*store.StackHealthEntry)
	GetByID(string) (*store.StackHealthEntry, bool)
	GetLatest() *store.StackHealthEntry
	Update(*store.StackHealthEntry)
	Delete(string) bool
	ListByStack(string, string) []*store.StackHealthEntry
}

// NewStore creates a Store with the given mode and backing data.
func NewStore(mode Mode, cfg config.Config) (*Store, error) {
	s := &Store{mode: mode}

	switch mode {
	case ModeInMemory:
		s.wf = store.NewWorkflowStore()
		s.sc = store.NewScheduleStore()
		s.ag = store.NewAgentStore()
		s.pl = store.NewPipelineStore()
		s.ex = store.NewExecutionStore()
		s.ht = store.NewHumanTaskStore()
		s.esc = store.NewEscalationStore()
		s.retry = store.NewRetryRecordStore()
		s.health = store.NewStackHealthStore()
		s.delegate = store.NewDelegationStore()
		log.Printf("[store] using in-memory store")

	case ModePostgreSQL:
		dbCfg := database.Config{
			Host:     cfg.DBHost,
			Port:     cfg.DBPort,
			User:     cfg.DBUser,
			Password: cfg.DBPassword,
			DBName:   cfg.DBName,
			MaxOpen:  cfg.DBMaxOpen,
			MaxIdle:  cfg.DBMaxIdle,
		}
		db, err := database.OpenPool(context.Background(), dbCfg)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		log.Printf("[store] using PostgreSQL store (host=%s, db=%s)", dbCfg.Host, dbCfg.DBName)
		s.repo = NewPostgresRepo(db)
	}

	return s, nil
}

// ─── Workflow ─────────────────────────────────────────────────────────────────

func (s *Store) CreateWorkflow(wf *store.Workflow) (*store.Workflow, error) {
	switch s.mode {
	case ModeInMemory:
		return s.wf.Create(wf)
	case ModePostgreSQL:
		if err := s.repo.Workflow().Create(wf); err != nil {
			return nil, err
		}
		return s.repo.Workflow().GetByID(wf.ID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetWorkflow(id string) (*store.Workflow, error) {
	switch s.mode {
	case ModeInMemory:
		return s.wf.GetByID(id)
	case ModePostgreSQL:
		return s.repo.Workflow().GetByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetWorkflowAndTenant(id, tenantID string) (*store.Workflow, error) {
	switch s.mode {
	case ModeInMemory:
		return s.wf.GetByIDAndTenant(id, tenantID)
	case ModePostgreSQL:
		return s.repo.Workflow().GetByIDAndTenant(id, tenantID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateWorkflowStatusAndTenant(id, tenantID string, status store.WorkflowStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.wf.UpdateStatusAndTenant(id, tenantID, status)
	case ModePostgreSQL:
		return s.repo.Workflow().UpdateStatusAndTenant(id, tenantID, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateWorkflowStatus(id string, status store.WorkflowStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.wf.UpdateStatus(id, status)
	case ModePostgreSQL:
		return s.repo.Workflow().UpdateStatus(id, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateWorkflowCurrentNodes(id string, nodeIDs []string) error {
	switch s.mode {
	case ModeInMemory:
		return s.wf.UpdateCurrentNodes(id, nodeIDs)
	case ModePostgreSQL:
		return s.repo.Workflow().UpdateCurrentNodes(id, nodeIDs)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListWorkflows(tenantID string, page, pageSize int, status *string) ([]*store.Workflow, int, bool) {
	switch s.mode {
	case ModeInMemory:
		return s.wf.List(tenantID, page, pageSize, status)
	case ModePostgreSQL:
		wfs, total, err := s.repo.Workflow().List(tenantID, page, pageSize, status)
		if err != nil {
			return nil, 0, false
		}
		return wfs, total, false
	default:
		return nil, 0, false
	}
}

func (s *Store) DeleteWorkflow(id string) error {
	switch s.mode {
	case ModeInMemory:
		return s.wf.Delete(id)
	case ModePostgreSQL:
		return s.repo.Workflow().Delete(id)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) AddCheckpoint(cp *store.Checkpoint) {
	switch s.mode {
	case ModeInMemory:
		s.wf.AddCheckpoint(*cp)
	case ModePostgreSQL:
		_ = s.repo.Workflow().AddCheckpoint(cp)
	default:
	}
}

func (s *Store) GetCheckpoints(workflowID string) []store.Checkpoint {
	switch s.mode {
	case ModeInMemory:
		return s.wf.GetCheckpoints(workflowID)
	case ModePostgreSQL:
		cps, err := s.repo.Workflow().GetCheckpoints(workflowID)
		if err != nil {
			return nil
		}
		result := make([]store.Checkpoint, len(cps))
		for i, c := range cps {
			result[i] = *c
		}
		return result
	default:
		return nil
	}
}

func (s *Store) AddWorkflowVariable(workflowID, tenantID string, key string, value interface{}) error {
	switch s.mode {
	case ModeInMemory:
		return s.wf.AddVariable(workflowID, tenantID, key, value)
	case ModePostgreSQL:
		valBytes, _ := json.Marshal(value)
		return s.repo.Workflow().AddVariable(workflowID, tenantID, key, json.RawMessage(valBytes))
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetWorkflowVariables(workflowID string) (*store.WorkflowVariables, error) {
	switch s.mode {
	case ModeInMemory:
		return s.wf.GetVariables(workflowID)
	case ModePostgreSQL:
		return s.repo.Workflow().GetVariables(workflowID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) SetWorkflowVariables(workflowID, tenantID string, vars map[string]interface{}) error {
	switch s.mode {
	case ModeInMemory:
		return s.wf.SetVariables(workflowID, tenantID, vars)
	case ModePostgreSQL:
		return s.repo.Workflow().SetVariables(workflowID, tenantID, vars)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) AddExecutionEvent(workflowID string, evt store.ExecutionEvent) {
	switch s.mode {
	case ModeInMemory:
		s.wf.AddEvent(workflowID, evt)
	case ModePostgreSQL:
		_ = s.repo.Workflow().AddEvent(&store.ExecutionEvent{
			EventID:   evt.EventID,
			NodeID:    evt.NodeID,
			EventType: evt.EventType,
			Timestamp: evt.Timestamp,
			Details:   evt.Details,
		})
	default:
	}
}

func (s *Store) GetExecutionHistory(workflowID string) []store.ExecutionEvent {
	switch s.mode {
	case ModeInMemory:
		return s.wf.GetExecutionHistory(workflowID)
	case ModePostgreSQL:
		evts, err := s.repo.Workflow().GetExecutionHistory(workflowID)
		if err != nil {
			return nil
		}
		result := make([]store.ExecutionEvent, len(evts))
		for i, e := range evts {
			result[i] = *e
		}
		return result
	default:
		return nil
	}
}

// ─── Schedule ─────────────────────────────────────────────────────────────────

func (s *Store) CreateSchedule(sc *store.Schedule) (*store.Schedule, error) {
	switch s.mode {
	case ModeInMemory:
		return s.sc.Create(sc)
	case ModePostgreSQL:
		if err := s.repo.Schedule().Create(sc); err != nil {
			return nil, err
		}
		return s.repo.Schedule().GetByID(sc.ID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetSchedule(id string) (*store.Schedule, error) {
	switch s.mode {
	case ModeInMemory:
		return s.sc.GetByID(id)
	case ModePostgreSQL:
		return s.repo.Schedule().GetByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetScheduleAndTenant(id, tenantID string) (*store.Schedule, error) {
	switch s.mode {
	case ModeInMemory:
		return s.sc.GetByIDAndTenant(id, tenantID)
	case ModePostgreSQL:
		return s.repo.Schedule().GetByIDAndTenant(id, tenantID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateScheduleStatusAndTenant(id, tenantID string, enabled bool) error {
	switch s.mode {
	case ModeInMemory:
		return s.sc.UpdateStatusAndTenant(id, tenantID, enabled)
	case ModePostgreSQL:
		return s.repo.Schedule().UpdateStatusAndTenant(id, tenantID, enabled)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) PatchSchedule(id string, name, cron, workflowTemplateID *string, variables *map[string]interface{}, enabled *bool) (*store.Schedule, error) {
	switch s.mode {
	case ModeInMemory:
		return s.sc.Patch(id, name, cron, workflowTemplateID, variables, enabled)
	case ModePostgreSQL:
		return s.repo.Schedule().Patch(id, name, cron, workflowTemplateID, variables, enabled)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) DeleteSchedule(id string) error {
	switch s.mode {
	case ModeInMemory:
		return s.sc.Delete(id)
	case ModePostgreSQL:
		return s.repo.Schedule().Delete(id)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListSchedules(tenantID string, page, pageSize int, enabled *bool) ([]*store.Schedule, int, bool) {
	switch s.mode {
	case ModeInMemory:
		return s.sc.List(tenantID, page, pageSize, enabled)
	case ModePostgreSQL:
		scs, total, err := s.repo.Schedule().List(tenantID, page, pageSize, enabled)
		if err != nil {
			return nil, 0, false
		}
		return scs, total, false
	default:
		return nil, 0, false
	}
}

// ─── Agent ────────────────────────────────────────────────────────────────────

func (s *Store) CreateAgent(a *store.Agent) error {
	switch s.mode {
	case ModeInMemory:
		// Agent is tracked via assignments in the in-memory store
		return nil
	case ModePostgreSQL:
		return s.repo.Agent().CreateAgent(a)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetAgent(id string) (*store.Agent, error) {
	switch s.mode {
	case ModeInMemory:
		return nil, fmt.Errorf("agent not found")
	case ModePostgreSQL:
		return s.repo.Agent().GetAgentByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateAgent(id string, name, desc, status, agentType *string, capabilities *[]string) (*store.Agent, error) {
	switch s.mode {
	case ModeInMemory:
		return nil, fmt.Errorf("update agent not implemented in in-memory store")
	case ModePostgreSQL:
		return s.repo.Agent().UpdateAgent(id, name, desc, status, agentType, capabilities)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) DeleteAgent(id string) error {
	switch s.mode {
	case ModeInMemory:
		return nil
	case ModePostgreSQL:
		return s.repo.Agent().DeleteAgent(id)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListAgents(tenantID string, page, pageSize int, status *string) ([]*store.Agent, int, bool) {
	switch s.mode {
	case ModeInMemory:
		return nil, 0, false
	case ModePostgreSQL:
		ags, total, err := s.repo.Agent().ListAgents(tenantID, page, pageSize, status)
		if err != nil {
			return nil, 0, false
		}
		return ags, total, false
	default:
		return nil, 0, false
	}
}

func (s *Store) CreateAssignment(a *store.AgentAssignment) (*store.AgentAssignment, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ag.CreateAssignment(a)
	case ModePostgreSQL:
		if err := s.repo.Agent().CreateAssignment(a); err != nil {
			return nil, err
		}
		return s.repo.Agent().GetAssignmentByID(a.ID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetAssignment(id string) (*store.AgentAssignment, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ag.GetByID(id)
	case ModePostgreSQL:
		return s.repo.Agent().GetAssignmentByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListAssignmentsByWorkflow(workflowID string) ([]*store.AgentAssignment, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ag.ListByWorkflow(workflowID)
	case ModePostgreSQL:
		return s.repo.Agent().ListAssignmentsByWorkflow(workflowID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) SetAgentAvailability(a *store.AgentAvailability) {
	switch s.mode {
	case ModeInMemory:
		s.ag.SetAgentAvailability(a)
	case ModePostgreSQL:
		_ = s.repo.Agent().SetAgentAvailability(a)
	default:
	}
}

func (s *Store) GetAgentAvailability(agentID string) (*store.AgentAvailability, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ag.GetAgentAvailability(agentID)
	case ModePostgreSQL:
		return s.repo.Agent().GetAgentAvailability(agentID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListAgentAvailability() ([]*store.AgentAvailability, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ag.ListAgentAvailability(), nil
	case ModePostgreSQL:
		return s.repo.Agent().ListAgentAvailability()
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

// ─── Pipeline ─────────────────────────────────────────────────────────────────

func (s *Store) CreatePipeline(p *store.Pipeline) (*store.Pipeline, error) {
	switch s.mode {
	case ModeInMemory:
		return s.pl.Create(p)
	case ModePostgreSQL:
		if err := s.repo.Pipeline().Create(p); err != nil {
			return nil, err
		}
		return s.repo.Pipeline().GetByID(p.ID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetPipeline(id string) (*store.Pipeline, error) {
	switch s.mode {
	case ModeInMemory:
		return s.pl.GetByID(id)
	case ModePostgreSQL:
		return s.repo.Pipeline().GetByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetPipelineAndTenant(id, tenantID string) (*store.Pipeline, error) {
	switch s.mode {
	case ModeInMemory:
		return s.pl.GetByIDAndTenant(id, tenantID)
	case ModePostgreSQL:
		return s.repo.Pipeline().GetByIDAndTenant(id, tenantID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdatePipelineStatusAndTenant(id, tenantID string, status store.PipelineStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.pl.UpdateStatusAndTenant(id, tenantID, status)
	case ModePostgreSQL:
		return s.repo.Pipeline().UpdateStatusAndTenant(id, tenantID, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdatePipeline(id string, name, description *string, steps []store.PipelineStep, errorHandling *store.PipelineErrorHandlingConfig, timeoutMinutes, maxRetries *int, status *store.PipelineStatus, variables *map[string]interface{}, tags *[]string) (*store.Pipeline, error) {
	switch s.mode {
	case ModeInMemory:
		return s.pl.Update(id, name, description, &steps, errorHandling, timeoutMinutes, maxRetries, status, variables, tags)
	case ModePostgreSQL:
		return s.repo.Pipeline().Update(id, name, description, steps, errorHandling, timeoutMinutes, maxRetries, status, variables, tags)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdatePipelineStatus(id string, status store.PipelineStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.pl.UpdateStatus(id, status)
	case ModePostgreSQL:
		return s.repo.Pipeline().UpdateStatus(id, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) DeletePipeline(id string) error {
	switch s.mode {
	case ModeInMemory:
		return s.pl.Delete(id)
	case ModePostgreSQL:
		return s.repo.Pipeline().Delete(id)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListPipelines(tenantID string, page, pageSize int, status *string) ([]*store.Pipeline, int, bool) {
	switch s.mode {
	case ModeInMemory:
		return s.pl.List(tenantID, page, pageSize, status)
	case ModePostgreSQL:
		pipelines, total, err := s.repo.Pipeline().List(tenantID, page, pageSize, status)
		if err != nil {
			return nil, 0, false
		}
		return pipelines, total, false
	default:
		return nil, 0, false
	}
}

func (s *Store) IncrementPipelineExecutionCount(pipelineID string, success bool) {
	switch s.mode {
	case ModeInMemory:
		s.pl.IncrementExecutionCount(pipelineID, success)
	case ModePostgreSQL:
		_ = s.repo.Pipeline().IncrementExecutionCount(pipelineID, success)
	default:
	}
}

// ─── Execution ────────────────────────────────────────────────────────────────

func (s *Store) CreateExecution(e *store.PipelineExecution) (*store.PipelineExecution, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ex.Create(e)
	case ModePostgreSQL:
		if err := s.repo.Execution().Create(e); err != nil {
			return nil, err
		}
		return s.repo.Execution().GetByID(e.ID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetExecution(id string) (*store.PipelineExecution, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ex.GetByID(id)
	case ModePostgreSQL:
		return s.repo.Execution().GetByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetExecutionAndTenant(id, tenantID string) (*store.PipelineExecution, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ex.GetByIDAndTenant(id, tenantID)
	case ModePostgreSQL:
		return s.repo.Execution().GetByIDAndTenant(id, tenantID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateExecutionStatusAndTenant(id, tenantID string, status store.PipelineExecutionStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.ex.UpdateStatusAndTenant(id, tenantID, status)
	case ModePostgreSQL:
		return s.repo.Execution().UpdateStatusAndTenant(id, tenantID, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateExecutionStatus(id string, status store.PipelineExecutionStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.ex.UpdateStatus(id, status)
	case ModePostgreSQL:
		return s.repo.Execution().UpdateStatus(id, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) DeleteExecution(id string) error {
	switch s.mode {
	case ModeInMemory:
		return s.ex.Delete(id)
	case ModePostgreSQL:
		return s.repo.Execution().Delete(id)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListExecutionsByPipeline(pipelineID string, page, pageSize int, status *string) ([]*store.PipelineExecution, int, bool) {
	switch s.mode {
	case ModeInMemory:
		return s.ex.ListByPipeline(pipelineID, page, pageSize, status, 0)
	case ModePostgreSQL:
		execs, total, err := s.repo.Execution().ListByPipeline(pipelineID, page, pageSize, status)
		if err != nil {
			return nil, 0, false
		}
		return execs, total, false
	default:
		return nil, 0, false
	}
}

func (s *Store) ListExecutionsByTenant(tenantID string, page, pageSize int) ([]*store.PipelineExecution, int, bool) {
	switch s.mode {
	case ModeInMemory:
		return s.ex.ListByTenant(tenantID, page, pageSize)
	case ModePostgreSQL:
		execs, total, err := s.repo.Execution().ListByTenant(tenantID, page, pageSize)
		if err != nil {
			return nil, 0, false
		}
		return execs, total, false
	default:
		return nil, 0, false
	}
}

func (s *Store) AddExecutionStep(step *store.PipelineExecutionStep) {
	switch s.mode {
	case ModeInMemory:
		s.ex.AddStep(step)
	case ModePostgreSQL:
		_ = s.repo.Execution().AddStep(step)
	default:
	}
}

func (s *Store) GetExecutionSteps(executionID string) []*store.PipelineExecutionStep {
	switch s.mode {
	case ModeInMemory:
		return s.ex.GetSteps(executionID)
	case ModePostgreSQL:
		steps, err := s.repo.Execution().GetSteps(executionID)
		if err != nil {
			return nil
		}
		result := make([]*store.PipelineExecutionStep, len(steps))
		for i, s := range steps {
			cpy := *s
			if s.Inputs != nil {
				cpy.Inputs = make(map[string]interface{})
				for k, v := range s.Inputs {
					cpy.Inputs[k] = v
				}
			}
			if s.Outputs != nil {
				cpy.Outputs = make(map[string]interface{})
				for k, v := range s.Outputs {
					cpy.Outputs[k] = v
				}
			}
			result[i] = &cpy
		}
		return result
	default:
		return nil
	}
}

func (s *Store) IncrementExecutionRetryCount(id string) int {
	switch s.mode {
	case ModeInMemory:
		return s.ex.IncrementRetryCount(id)
	case ModePostgreSQL:
		count, _ := s.repo.Execution().IncrementRetryCount(id)
		return count
	default:
		return 0
	}
}

// ─── Human Task ───────────────────────────────────────────────────────────────

func (s *Store) CreateHumanTask(t *store.HumanTask) (*store.HumanTask, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ht.Create(t)
	case ModePostgreSQL:
		if err := s.repo.HumanTask().Create(t); err != nil {
			return nil, err
		}
		return s.repo.HumanTask().GetByID(t.ID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetHumanTask(id string) (*store.HumanTask, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ht.GetByID(id)
	case ModePostgreSQL:
		return s.repo.HumanTask().GetByID(id)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) GetHumanTaskAndTenant(id, tenantID string) (*store.HumanTask, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ht.GetByIDAndTenant(id, tenantID)
	case ModePostgreSQL:
		return s.repo.HumanTask().GetByIDAndTenant(id, tenantID)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) UpdateHumanTaskStatusAndTenant(id, tenantID string, status store.HumanTaskStatus) error {
	switch s.mode {
	case ModeInMemory:
		return s.ht.UpdateStatusAndTenant(id, tenantID, status)
	case ModePostgreSQL:
		return s.repo.HumanTask().UpdateStatusAndTenant(id, tenantID, status)
	default:
		return fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) RespondToHumanTask(id string, action string, response map[string]interface{}, respondedBy, comments string) (*store.HumanTask, error) {
	switch s.mode {
	case ModeInMemory:
		return s.ht.Respond(id, action, response, respondedBy, comments)
	case ModePostgreSQL:
		return s.repo.HumanTask().Respond(id, action, response, respondedBy, comments)
	default:
		return nil, fmt.Errorf("unknown store mode: %d", s.mode)
	}
}

func (s *Store) ListHumanTasks(tenantID string, status *string) ([]*store.HumanTask, int) {
	switch s.mode {
	case ModeInMemory:
		tasks, total := s.ht.List(tenantID, status)
		return tasks, total
	case ModePostgreSQL:
		tasks, total, err := s.repo.HumanTask().List(tenantID, status)
		if err != nil {
			return nil, 0
		}
		return tasks, total
	default:
		return nil, 0
	}
}

// ─── Escalation ───────────────────────────────────────────────────────────────

func (s *Store) CreateEscalation(e *store.Escalation) {
	switch s.mode {
	case ModeInMemory:
		s.esc.Create(e)
	case ModePostgreSQL:
		_ = s.repo.Escalation().Create(e)
	default:
	}
}

func (s *Store) GetEscalation(id string) (*store.Escalation, bool) {
	switch s.mode {
	case ModeInMemory:
		e, ok := s.esc.GetByID(id)
		return e, ok
	case ModePostgreSQL:
		e, err := s.repo.Escalation().GetByID(id)
		return e, err == nil
	default:
		return nil, false
	}
}

func (s *Store) ListEscalationsByWorkflow(workflowID string) []*store.Escalation {
	switch s.mode {
	case ModeInMemory:
		return s.esc.ListByWorkflow(workflowID)
	case ModePostgreSQL:
		escs, err := s.repo.Escalation().ListByWorkflow(workflowID)
		if err != nil {
			return nil
		}
		result := make([]*store.Escalation, len(escs))
		for i, e := range escs {
			cpy := *e
			result[i] = &cpy
		}
		return result
	default:
		return nil
	}
}

func (s *Store) AcknowledgeEscalation(id string) bool {
	switch s.mode {
	case ModeInMemory:
		return s.esc.Acknowledge(id)
	case ModePostgreSQL:
		err := s.repo.Escalation().Acknowledge(id)
		return err == nil
	default:
		return false
	}
}

func (s *Store) ResolveEscalation(id string) bool {
	switch s.mode {
	case ModeInMemory:
		return s.esc.Resolve(id)
	case ModePostgreSQL:
		err := s.repo.Escalation().Resolve(id)
		return err == nil
	default:
		return false
	}
}

// ─── RetryRecord ──────────────────────────────────────────────────────────────

func (s *Store) CreateRetryRecord(r *store.RetryRecord) {
	switch s.mode {
	case ModeInMemory:
		s.retry.Create(r)
	case ModePostgreSQL:
		_ = s.repo.RetryRecord().Create(r)
	default:
	}
}

func (s *Store) GetRetryRecord(id string) (*store.RetryRecord, bool) {
	switch s.mode {
	case ModeInMemory:
		r, ok := s.retry.GetByID(id)
		return r, ok
	case ModePostgreSQL:
		r, err := s.repo.RetryRecord().GetByID(id)
		return r, err == nil
	default:
		return nil, false
	}
}

func (s *Store) ListRetryRecordsByWorkflow(workflowID string) []*store.RetryRecord {
	switch s.mode {
	case ModeInMemory:
		return s.retry.ListByWorkflow(workflowID)
	case ModePostgreSQL:
		records, err := s.repo.RetryRecord().ListByWorkflow(workflowID)
		if err != nil {
			return nil
		}
		result := make([]*store.RetryRecord, len(records))
		for i, r := range records {
			cpy := *r
			result[i] = &cpy
		}
		return result
	default:
		return nil
	}
}

func (s *Store) UpdateRetryRecordStatus(id string, status store.RetryStatus, errorCode, errorMessage string) bool {
	switch s.mode {
	case ModeInMemory:
		return s.retry.UpdateStatus(id, status, errorCode, errorMessage)
	case ModePostgreSQL:
		s.repo.RetryRecord().UpdateStatus(id, status, errorCode, errorMessage)
		return true
	default:
		return false
	}
}

// ─── Stack Health ─────────────────────────────────────────────────────────────

func (s *Store) CreateStackHealth(e *store.StackHealthEntry) {
	switch s.mode {
	case ModeInMemory:
		s.health.Create(e)
	case ModePostgreSQL:
		_ = s.repo.StackHealth().Create(e)
	default:
	}
}

func (s *Store) GetStackHealth(id string) (*store.StackHealthEntry, bool) {
	switch s.mode {
	case ModeInMemory:
		e, ok := s.health.GetByID(id)
		return e, ok
	case ModePostgreSQL:
		e, err := s.repo.StackHealth().GetByID(id)
		return e, err == nil
	default:
		return nil, false
	}
}

func (s *Store) GetLatestStackHealth() *store.StackHealthEntry {
	switch s.mode {
	case ModeInMemory:
		return s.health.GetLatest()
	case ModePostgreSQL:
		e, _ := s.repo.StackHealth().GetLatest()
		return e
	default:
		return nil
	}
}

func (s *Store) UpdateStackHealth(e *store.StackHealthEntry) {
	switch s.mode {
	case ModeInMemory:
		s.health.Update(e)
	case ModePostgreSQL:
		_ = s.repo.StackHealth().Update(e)
	default:
	}
}

func (s *Store) DeleteStackHealth(id string) bool {
	switch s.mode {
	case ModeInMemory:
		return s.health.Delete(id)
	case ModePostgreSQL:
		err := s.repo.StackHealth().Delete(id)
		return err == nil
	default:
		return false
	}
}

func (s *Store) ListStackHealthByStack(tenantID, stackType string) []*store.StackHealthEntry {
	switch s.mode {
	case ModeInMemory:
		return s.health.ListByStack(tenantID, stackType)
	case ModePostgreSQL:
		entries, err := s.repo.StackHealth().ListByStack(tenantID, stackType)
		if err != nil {
			return nil
		}
		result := make([]*store.StackHealthEntry, len(entries))
		for i, e := range entries {
			cpy := *e
			result[i] = &cpy
		}
		return result
	default:
		return nil
	}
}

// ─── Getters for handler compatibility ────────────────────────────────────────

// WorkflowStore returns the underlying workflow store (either in-memory or PostgreSQL passthrough).
func (s *Store) WorkflowStore() WorkflowStoreIface {
	if s.mode == ModeInMemory {
		return s.wf
	}
	return &storePassthroughWorkflow{repo: s.repo.Workflow()}
}

// ScheduleStore returns the underlying schedule store (either in-memory or PostgreSQL passthrough).
func (s *Store) ScheduleStore() ScheduleStoreIface {
	if s.mode == ModeInMemory {
		return s.sc
	}
	return &storePassthroughSchedule{repo: s.repo.Schedule()}
}

// AgentStore returns the underlying agent store (either in-memory or PostgreSQL passthrough).
func (s *Store) AgentStore() AgentStoreIface {
	if s.mode == ModeInMemory {
		return s.ag
	}
	return &storePassthroughAgent{repo: s.repo.Agent()}
}

// PipelineStore returns the underlying pipeline store (either in-memory or PostgreSQL passthrough).
func (s *Store) PipelineStore() PipelineStoreIface {
	if s.mode == ModeInMemory {
		return s.pl
	}
	return &storePassthroughPipeline{repo: s.repo.Pipeline()}
}

// ExecutionStore returns the underlying execution store (either in-memory or PostgreSQL passthrough).
func (s *Store) ExecutionStore() ExecutionStoreIface {
	if s.mode == ModeInMemory {
		return s.ex
	}
	return &storePassthroughExecution{repo: s.repo.Execution()}
}

// HumanTaskStore returns the underlying human task store (either in-memory or PostgreSQL passthrough).
func (s *Store) HumanTaskStore() HumanTaskStoreIface {
	if s.mode == ModeInMemory {
		return s.ht
	}
	return &storePassthroughHumanTask{repo: s.repo.HumanTask()}
}

// EscalationStore returns the underlying escalation store (either in-memory or PostgreSQL passthrough).
func (s *Store) EscalationStore() EscalationStoreIface {
	if s.mode == ModeInMemory {
		return s.esc
	}
	return &storePassthroughEscalation{repo: s.repo.Escalation()}
}

// RetryRecordStore returns the underlying retry record store (either in-memory or PostgreSQL passthrough).
func (s *Store) RetryRecordStore() RetryRecordStoreIface {
	if s.mode == ModeInMemory {
		return s.retry
	}
	return &storePassthroughRetry{repo: s.repo.RetryRecord()}
}

// StackHealthStore returns the underlying stack health store (either in-memory or PostgreSQL passthrough).
func (s *Store) StackHealthStore() StackHealthStoreIface {
	if s.mode == ModeInMemory {
		return s.health
	}
	return &storePassthroughHealth{repo: s.repo.StackHealth()}
}

// DelegationStore returns the underlying delegation store (in-memory only for now).
func (s *Store) DelegationStore() *store.DelegationStore {
	return s.delegate
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ─── Passthrough wrappers ────────────────────────────────────────────────────
// These wrap repository interfaces and provide the same method signatures as
// the in-memory store types, delegating to PostgreSQL when mode is PostgreSQL.

// storePassthroughWorkflow wraps WorkflowRepo to look like *store.WorkflowStore.
type storePassthroughWorkflow struct {
	repo WorkflowRepo
}

func (p *storePassthroughWorkflow) Create(wf *store.Workflow) (*store.Workflow, error) {
	if err := p.repo.Create(wf); err != nil {
		return nil, err
	}
	return p.repo.GetByID(wf.ID)
}
func (p *storePassthroughWorkflow) GetByID(id string) (*store.Workflow, error) { return p.repo.GetByID(id) }
func (p *storePassthroughWorkflow) GetByIDAndTenant(id, tenantID string) (*store.Workflow, error) {
	return p.repo.GetByIDAndTenant(id, tenantID)
}
func (p *storePassthroughWorkflow) UpdateStatus(id string, status store.WorkflowStatus) error {
	return p.repo.UpdateStatus(id, status)
}
func (p *storePassthroughWorkflow) UpdateStatusAndTenant(id, tenantID string, status store.WorkflowStatus) error {
	return p.repo.UpdateStatusAndTenant(id, tenantID, status)
}
func (p *storePassthroughWorkflow) UpdateCurrentNodes(id string, nodeIDs []string) error {
	return p.repo.UpdateCurrentNodes(id, nodeIDs)
}
func (p *storePassthroughWorkflow) List(tenantID string, page, pageSize int, status *string) ([]*store.Workflow, int, bool) {
	wfs, total, err := p.repo.List(tenantID, page, pageSize, status)
	if err != nil {
		return nil, 0, false
	}
	return wfs, total, false
}
func (p *storePassthroughWorkflow) AddCheckpoint(cp store.Checkpoint) {
	_ = p.repo.AddCheckpoint(&store.Checkpoint{
		ID:            cp.ID,
		WorkflowID:    cp.WorkflowID,
		NodeID:        cp.NodeID,
		Timestamp:     cp.Timestamp,
		StateSnapshot: cp.StateSnapshot,
		Checksum:      cp.Checksum,
	})
}
func (p *storePassthroughWorkflow) GetCheckpoints(workflowID string) []store.Checkpoint {
	cps, err := p.repo.GetCheckpoints(workflowID)
	if err != nil {
		return nil
	}
	result := make([]store.Checkpoint, len(cps))
	for i, c := range cps {
		result[i] = *c
	}
	return result
}
func (p *storePassthroughWorkflow) AddVariable(workflowID, tenantID string, key string, value interface{}) error {
	valBytes, _ := json.Marshal(value)
	return p.repo.AddVariable(workflowID, tenantID, key, json.RawMessage(valBytes))
}
func (p *storePassthroughWorkflow) GetVariables(workflowID string) (*store.WorkflowVariables, error) {
	return p.repo.GetVariables(workflowID)
}
func (p *storePassthroughWorkflow) SetVariables(workflowID, tenantID string, vars map[string]interface{}) error {
	return p.repo.SetVariables(workflowID, tenantID, vars)
}
func (p *storePassthroughWorkflow) AddEvent(workflowID string, evt store.ExecutionEvent) {
	_ = p.repo.AddEvent(&store.ExecutionEvent{
		EventID:   evt.EventID,
		NodeID:    evt.NodeID,
		EventType: evt.EventType,
		Timestamp: evt.Timestamp,
		Details:   evt.Details,
	})
}
func (p *storePassthroughWorkflow) GetExecutionHistory(workflowID string) []store.ExecutionEvent {
	evts, err := p.repo.GetExecutionHistory(workflowID)
	if err != nil {
		return nil
	}
	result := make([]store.ExecutionEvent, len(evts))
	for i, e := range evts {
		result[i] = *e
	}
	return result
}
func (p *storePassthroughWorkflow) Delete(id string) error { return p.repo.Delete(id) }

// storePassthroughSchedule wraps ScheduleRepo.
type storePassthroughSchedule struct {
	repo ScheduleRepo
}
func (p *storePassthroughSchedule) Create(sc *store.Schedule) (*store.Schedule, error) {
	if err := p.repo.Create(sc); err != nil {
		return nil, err
	}
	return p.repo.GetByID(sc.ID)
}
func (p *storePassthroughSchedule) GetByID(id string) (*store.Schedule, error) { return p.repo.GetByID(id) }
func (p *storePassthroughSchedule) GetByIDAndTenant(id, tenantID string) (*store.Schedule, error) {
	return p.repo.GetByIDAndTenant(id, tenantID)
}
func (p *storePassthroughSchedule) Patch(id string, name, cron, workflowTemplateID *string, variables *map[string]interface{}, enabled *bool) (*store.Schedule, error) {
	return p.repo.Patch(id, name, cron, workflowTemplateID, variables, enabled)
}
func (p *storePassthroughSchedule) Delete(id string) error { return p.repo.Delete(id) }
func (p *storePassthroughSchedule) UpdateStatusAndTenant(id, tenantID string, enabled bool) error {
	return p.repo.UpdateStatusAndTenant(id, tenantID, enabled)
}
func (p *storePassthroughSchedule) List(tenantID string, page, pageSize int, enabled *bool) ([]*store.Schedule, int, bool) {
	scs, total, err := p.repo.List(tenantID, page, pageSize, enabled)
	if err != nil {
		return nil, 0, false
	}
	return scs, total, false
}

// storePassthroughAgent wraps AgentRepo.
type storePassthroughAgent struct {
	repo AgentRepo
}
func (p *storePassthroughAgent) CreateAssignment(a *store.AgentAssignment) (*store.AgentAssignment, error) {
	if err := p.repo.CreateAssignment(a); err != nil {
		return nil, err
	}
	return p.repo.GetAssignmentByID(a.ID)
}
func (p *storePassthroughAgent) GetByID(id string) (*store.AgentAssignment, error) { return p.repo.GetAssignmentByID(id) }
func (p *storePassthroughAgent) SetAgentAvailability(a *store.AgentAvailability)   { _ = p.repo.SetAgentAvailability(a) }
func (p *storePassthroughAgent) GetAgentAvailability(agentID string) (*store.AgentAvailability, error) {
	return p.repo.GetAgentAvailability(agentID)
}
func (p *storePassthroughAgent) ListByWorkflow(workflowID string) ([]*store.AgentAssignment, error) {
	return p.repo.ListAssignmentsByWorkflow(workflowID)
}
func (p *storePassthroughAgent) ListAgentAvailability() []*store.AgentAvailability {
	avail, err := p.repo.ListAgentAvailability()
	if err != nil {
		return nil
	}
	result := make([]*store.AgentAvailability, len(avail))
	for i, a := range avail {
		cpy := *a
		result[i] = &cpy
	}
	return result
}
func (p *storePassthroughAgent) ListByTenant(tenantID string) []*store.AgentAvailability {
	return p.ListAgentAvailability()
}

// storePassthroughPipeline wraps PipelineRepo.
type storePassthroughPipeline struct {
	repo PipelineRepo
}
func (p *storePassthroughPipeline) Create(pipeline *store.Pipeline) (*store.Pipeline, error) {
	if err := p.repo.Create(pipeline); err != nil {
		return nil, err
	}
	return p.repo.GetByID(pipeline.ID)
}
func (p *storePassthroughPipeline) GetByID(id string) (*store.Pipeline, error) { return p.repo.GetByID(id) }
func (p *storePassthroughPipeline) GetByIDAndTenant(id, tenantID string) (*store.Pipeline, error) {
	return p.repo.GetByIDAndTenant(id, tenantID)
}
func (p *storePassthroughPipeline) Update(id string, name, description *string, steps *[]store.PipelineStep, errorHandling *store.PipelineErrorHandlingConfig, timeoutMinutes, maxRetries *int, status *store.PipelineStatus, variables *map[string]interface{}, tags *[]string) (*store.Pipeline, error) {
	var s []store.PipelineStep
	if steps != nil {
		s = *steps
	}
	return p.repo.Update(id, name, description, s, errorHandling, timeoutMinutes, maxRetries, status, variables, tags)
}
func (p *storePassthroughPipeline) UpdateStatus(id string, status store.PipelineStatus) error {
	return p.repo.UpdateStatus(id, status)
}
func (p *storePassthroughPipeline) UpdateStatusAndTenant(id, tenantID string, status store.PipelineStatus) error {
	return p.repo.UpdateStatusAndTenant(id, tenantID, status)
}
func (p *storePassthroughPipeline) Delete(id string) error { return p.repo.Delete(id) }
func (p *storePassthroughPipeline) List(tenantID string, page, pageSize int, status *string) ([]*store.Pipeline, int, bool) {
	pipelines, total, err := p.repo.List(tenantID, page, pageSize, status)
	if err != nil {
		return nil, 0, false
	}
	return pipelines, total, false
}
func (p *storePassthroughPipeline) IncrementExecutionCount(pipelineID string, success bool) {
	_ = p.repo.IncrementExecutionCount(pipelineID, success)
}

// storePassthroughExecution wraps ExecutionRepo.
type storePassthroughExecution struct {
	repo ExecutionRepo
}
func (p *storePassthroughExecution) Create(e *store.PipelineExecution) (*store.PipelineExecution, error) {
	if err := p.repo.Create(e); err != nil {
		return nil, err
	}
	return p.repo.GetByID(e.ID)
}
func (p *storePassthroughExecution) GetByID(id string) (*store.PipelineExecution, error) { return p.repo.GetByID(id) }
func (p *storePassthroughExecution) GetByIDAndTenant(id, tenantID string) (*store.PipelineExecution, error) {
	return p.repo.GetByIDAndTenant(id, tenantID)
}
func (p *storePassthroughExecution) UpdateStatus(id string, status store.PipelineExecutionStatus) error {
	return p.repo.UpdateStatus(id, status)
}
func (p *storePassthroughExecution) UpdateStatusAndTenant(id, tenantID string, status store.PipelineExecutionStatus) error {
	return p.repo.UpdateStatusAndTenant(id, tenantID, status)
}
func (p *storePassthroughExecution) Delete(id string) error { return p.repo.Delete(id) }
func (p *storePassthroughExecution) ListByPipeline(pipelineID string, page, pageSize int, status *string, limit int) ([]*store.PipelineExecution, int, bool) {
	execs, total, err := p.repo.ListByPipeline(pipelineID, page, pageSize, status)
	if err != nil {
		return nil, 0, false
	}
	return execs, total, false
}
func (p *storePassthroughExecution) ListByTenant(tenantID string, page, pageSize int) ([]*store.PipelineExecution, int, bool) {
	execs, total, err := p.repo.ListByTenant(tenantID, page, pageSize)
	if err != nil {
		return nil, 0, false
	}
	return execs, total, false
}
func (p *storePassthroughExecution) AddStep(step *store.PipelineExecutionStep) { _ = p.repo.AddStep(step) }
func (p *storePassthroughExecution) GetSteps(executionID string) []*store.PipelineExecutionStep {
	steps, err := p.repo.GetSteps(executionID)
	if err != nil {
		return nil
	}
	result := make([]*store.PipelineExecutionStep, len(steps))
	for i, s := range steps {
		cpy := *s
		if s.Inputs != nil {
			cpy.Inputs = make(map[string]interface{})
			for k, v := range s.Inputs {
				cpy.Inputs[k] = v
			}
		}
		if s.Outputs != nil {
			cpy.Outputs = make(map[string]interface{})
			for k, v := range s.Outputs {
				cpy.Outputs[k] = v
			}
		}
		result[i] = &cpy
	}
	return result
}
func (p *storePassthroughExecution) IncrementRetryCount(id string) int {
	count, _ := p.repo.IncrementRetryCount(id)
	return count
}

// storePassthroughHumanTask wraps HumanTaskRepo.
type storePassthroughHumanTask struct {
	repo HumanTaskRepo
}
func (p *storePassthroughHumanTask) Create(t *store.HumanTask) (*store.HumanTask, error) {
	if err := p.repo.Create(t); err != nil {
		return nil, err
	}
	return p.repo.GetByID(t.ID)
}
func (p *storePassthroughHumanTask) GetByID(id string) (*store.HumanTask, error) { return p.repo.GetByID(id) }
func (p *storePassthroughHumanTask) GetByIDAndTenant(id, tenantID string) (*store.HumanTask, error) {
	return p.repo.GetByIDAndTenant(id, tenantID)
}
func (p *storePassthroughHumanTask) Respond(id string, action string, response map[string]interface{}, respondedBy string, comments string) (*store.HumanTask, error) {
	return p.repo.Respond(id, action, response, respondedBy, comments)
}
func (p *storePassthroughHumanTask) UpdateStatusAndTenant(id, tenantID string, status store.HumanTaskStatus) error {
	return p.repo.UpdateStatusAndTenant(id, tenantID, status)
}
func (p *storePassthroughHumanTask) List(tenantID string, status *string) ([]*store.HumanTask, int) {
	tasks, total, err := p.repo.List(tenantID, status)
	if err != nil {
		return nil, 0
	}
	return tasks, total
}

// storePassthroughEscalation wraps EscalationRepo.
type storePassthroughEscalation struct {
	repo EscalationRepo
}
func (p *storePassthroughEscalation) Create(e *store.Escalation) { _ = p.repo.Create(e) }
func (p *storePassthroughEscalation) GetByID(id string) (*store.Escalation, bool) {
	e, err := p.repo.GetByID(id)
	return e, err == nil
}
func (p *storePassthroughEscalation) ListByWorkflow(workflowID string) []*store.Escalation {
	escs, err := p.repo.ListByWorkflow(workflowID)
	if err != nil {
		return nil
	}
	result := make([]*store.Escalation, len(escs))
	for i, e := range escs {
		cpy := *e
		result[i] = &cpy
	}
	return result
}
func (p *storePassthroughEscalation) Acknowledge(id string) bool {
	return p.repo.Acknowledge(id) == nil
}
func (p *storePassthroughEscalation) Resolve(id string) bool {
	return p.repo.Resolve(id) == nil
}

// storePassthroughRetry wraps RetryRecordRepo.
type storePassthroughRetry struct {
	repo RetryRecordRepo
}
func (p *storePassthroughRetry) Create(r *store.RetryRecord) { _ = p.repo.Create(r) }
func (p *storePassthroughRetry) GetByID(id string) (*store.RetryRecord, bool) {
	r, err := p.repo.GetByID(id)
	return r, err == nil
}
func (p *storePassthroughRetry) ListByWorkflow(workflowID string) []*store.RetryRecord {
	records, err := p.repo.ListByWorkflow(workflowID)
	if err != nil {
		return nil
	}
	result := make([]*store.RetryRecord, len(records))
	for i, r := range records {
		cpy := *r
		result[i] = &cpy
	}
	return result
}
func (p *storePassthroughRetry) UpdateStatus(id string, status store.RetryStatus, errorCode, errorMessage string) bool {
	_ = p.repo.UpdateStatus(id, status, errorCode, errorMessage)
	return true
}

// storePassthroughHealth wraps StackHealthRepo.
type storePassthroughHealth struct {
	repo StackHealthRepo
}
func (p *storePassthroughHealth) Create(e *store.StackHealthEntry) { _ = p.repo.Create(e) }
func (p *storePassthroughHealth) GetByID(id string) (*store.StackHealthEntry, bool) {
	e, err := p.repo.GetByID(id)
	return e, err == nil
}
func (p *storePassthroughHealth) GetLatest() *store.StackHealthEntry {
	e, _ := p.repo.GetLatest()
	return e
}
func (p *storePassthroughHealth) Update(e *store.StackHealthEntry) { _ = p.repo.Update(e) }
func (p *storePassthroughHealth) Delete(id string) bool            { return p.repo.Delete(id) == nil }
func (p *storePassthroughHealth) ListByStack(tenantID, stackType string) []*store.StackHealthEntry {
	entries, err := p.repo.ListByStack(tenantID, stackType)
	if err != nil {
		return nil
	}
	result := make([]*store.StackHealthEntry, len(entries))
	for i, e := range entries {
		cpy := *e
		result[i] = &cpy
	}
	return result
}
