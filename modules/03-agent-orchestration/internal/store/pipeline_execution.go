// Package store adds pipeline, execution, and human-task stores for Module 03 contract compliance.
package store

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Pipeline types ──────────────────────────────────────────────────────────

// PipelineStatus represents a pipeline's lifecycle state.
type PipelineStatus string

const (
	PipelineStatusActive    PipelineStatus = "active"
	PipelineStatusInactive  PipelineStatus = "inactive"
	PipelineStatusArchived  PipelineStatus = "archived"
)

// PipelineTriggerType represents how a pipeline is triggered.
type PipelineTriggerType string

const (
	PipelineTriggerManual    PipelineTriggerType = "manual"
	PipelineTriggerEvent     PipelineTriggerType = "event"
	PipelineTriggerSchedule  PipelineTriggerType = "schedule"
	PipelineTriggerWebhook   PipelineTriggerType = "webhook"
	PipelineTriggerAPI       PipelineTriggerType = "api"
)

// PipelineStepType represents the type of a pipeline step.
type PipelineStepType string

const (
	PipelineStepAPI       PipelineStepType = "api_call"
	PipelineStepAgent     PipelineStepType = "agent_task"
	PipelineStepData      PipelineStepType = "data_transform"
	PipelineStepCondition PipelineStepType = "condition"
	PipelineStepDelay     PipelineStepType = "delay"
	PipelineStepHuman     PipelineStepType = "human_approval"
	PipelineStepParallel  PipelineStepType = "parallel"
	PipelineStepForeach   PipelineStepType = "foreach"
	PipelineStepWebhook   PipelineStepType = "webhook"
	PipelineStepCode      PipelineStepType = "code"
	PipelineStepNotify    PipelineStepType = "notification"
)

// PipelineOnErrorAction represents error handling strategy for a step.
type PipelineOnErrorAction string

const (
	PipelineOnErrorFail   PipelineOnErrorAction = "fail"
	PipelineOnErrorRetry  PipelineOnErrorAction = "retry"
	PipelineOnErrorSkip   PipelineOnErrorAction = "skip"
	PipelineOnErrorBranch PipelineOnErrorAction = "branch"
)

// PipelineStep represents a step in a pipeline.
type PipelineStep struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name"`
	Type             PipelineStepType           `json:"type"`
	Inputs           map[string]interface{}     `json:"inputs,omitempty"`
	Config           map[string]interface{}     `json:"config,omitempty"`
	Condition        string                     `json:"condition,omitempty"`
	TimeoutSeconds   int                        `json:"timeout_seconds,omitempty"`
	OnError          PipelineOnErrorAction      `json:"on_error,omitempty"`
	NextStepID       string                     `json:"next_step_id,omitempty"`
	ParallelBranches int                        `json:"parallel_branches,omitempty"`
}

// PipelineErrorHandlingConfig represents pipeline-level error handling.
type PipelineErrorHandlingConfig struct {
	DefaultStrategy   PipelineErrorStrategyValue `json:"default_strategy,omitempty"`
	RetryMaxAttempts  int                        `json:"retry_max_attempts,omitempty"`
	RetryBackoff      PipelineBackoffValue       `json:"retry_backoff,omitempty"`
	RetryBaseDelayMS  int                        `json:"retry_base_delay_ms,omitempty"`
	DeadLetterEnabled bool                       `json:"dead_letter_enabled,omitempty"`
	AlertOnFailure    bool                       `json:"alert_on_failure,omitempty"`
}

// PipelineErrorStrategyValue represents pipeline-level error strategy.
type PipelineErrorStrategyValue string

const (
	PipelineErrorStrategyValueFail   PipelineErrorStrategyValue = "fail"
	PipelineErrorStrategyValueRetry  PipelineErrorStrategyValue = "retry"
	PipelineErrorStrategyValueSkip   PipelineErrorStrategyValue = "skip"
	PipelineErrorStrategyValueAbort  PipelineErrorStrategyValue = "abort"
)

// PipelineBackoffValue represents retry backoff strategy for pipelines.
type PipelineBackoffValue string

const (
	PipelineBackoffFixed     PipelineBackoffValue = "fixed"
	PipelineBackoffExponential PipelineBackoffValue = "exponential"
	PipelineBackoffLinear    PipelineBackoffValue = "linear"
)

// Pipeline represents a workflow pipeline.
type Pipeline struct {
	ID              string                       `json:"id"`
	TenantID        string                       `json:"tenant_id"`
	Name            string                       `json:"name"`
	Description     string                       `json:"description,omitempty"`
	Steps           []PipelineStep               `json:"steps"`
	ErrorHandling   *PipelineErrorHandlingConfig `json:"error_handling,omitempty"`
	TimeoutMinutes  int                          `json:"timeout_minutes,omitempty"`
	MaxRetries      int                          `json:"max_retries,omitempty"`
	TriggerType     PipelineTriggerType          `json:"trigger_type,omitempty"`
	Variables       map[string]interface{}       `json:"variables,omitempty"`
	Status          PipelineStatus               `json:"status"`
	ExecutionCount  int                          `json:"execution_count,omitempty"`
	LastExecutionAt *time.Time                   `json:"last_execution_at,omitempty"`
	SuccessRate     float64                      `json:"success_rate,omitempty"`
	AvgDurationMS   float64                      `json:"avg_duration_ms,omitempty"`
	CreatedBy       string                       `json:"created_by,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
	Tags            []string                     `json:"tags,omitempty"`
}

// ─── Execution types ─────────────────────────────────────────────────────────

// PipelineExecutionStatus represents an execution's lifecycle state.
type PipelineExecutionStatus string

const (
	PipelineExecutionPending   PipelineExecutionStatus = "pending"
	PipelineExecutionRunning   PipelineExecutionStatus = "running"
	PipelineExecutionCompleted PipelineExecutionStatus = "completed"
	PipelineExecutionFailed    PipelineExecutionStatus = "failed"
	PipelineExecutionCancelled PipelineExecutionStatus = "cancelled"
	PipelineExecutionRetrying  PipelineExecutionStatus = "retrying"
)

// PipelineExecutionStepStatus represents a step's execution state.
type PipelineExecutionStepStatus string

const (
	PipelineStepPending   PipelineExecutionStepStatus = "pending"
	PipelineStepRunning   PipelineExecutionStepStatus = "running"
	PipelineStepCompleted PipelineExecutionStepStatus = "completed"
	PipelineStepFailed    PipelineExecutionStepStatus = "failed"
	PipelineStepSkipped   PipelineExecutionStepStatus = "skipped"
	PipelineStepCancelled PipelineExecutionStepStatus = "cancelled"
)

// PipelineExecution represents a pipeline execution.
type PipelineExecution struct {
	ID              string                      `json:"id"`
	PipelineID      string                      `json:"pipeline_id"`
	TenantID        string                      `json:"tenant_id"`
	Status          PipelineExecutionStatus     `json:"status"`
	Inputs          map[string]interface{}      `json:"inputs,omitempty"`
	Outputs         map[string]interface{}      `json:"outputs,omitempty"`
	CurrentStepID   string                      `json:"current_step_id,omitempty"`
	CurrentStepStatus string                     `json:"current_step_status,omitempty"`
	ErrorMessage    string                      `json:"error_message,omitempty"`
	RetryCount      int                         `json:"retry_count,omitempty"`
	DurationMS      float64                     `json:"duration_ms,omitempty"`
	StartedAt       *time.Time                  `json:"started_at,omitempty"`
	CompletedAt     *time.Time                  `json:"completed_at,omitempty"`
	CreatedAt       time.Time                   `json:"created_at"`
}

// PipelineExecutionStep represents a step within an execution.
type PipelineExecutionStep struct {
	ID           string                      `json:"id"`
	ExecutionID  string                      `json:"execution_id"`
	StepID       string                      `json:"step_id"`
	StepName     string                      `json:"step_name"`
	Status       PipelineExecutionStepStatus  `json:"status"`
	Inputs       map[string]interface{}      `json:"inputs,omitempty"`
	Outputs      map[string]interface{}      `json:"outputs,omitempty"`
	ErrorMessage string                      `json:"error_message,omitempty"`
	StartedAt    *time.Time                  `json:"started_at,omitempty"`
	CompletedAt  *time.Time                  `json:"completed_at,omitempty"`
	DurationMS   float64                     `json:"duration_ms,omitempty"`
}

// PipelineAnalytics represents aggregated pipeline analytics.
type PipelineAnalytics struct {
	TotalExecutions       int            `json:"total_executions"`
	CompletedExecutions   int            `json:"completed_executions"`
	FailedExecutions      int            `json:"failed_executions"`
	CancelledExecutions   int            `json:"cancelled_executions"`
	SuccessRate           float64        `json:"success_rate"`
	AvgDurationMS         float64        `json:"avg_duration_ms"`
	P50DurationMS         float64        `json:"p50_duration_ms,omitempty"`
	P95DurationMS         float64        `json:"p95_duration_ms,omitempty"`
	P99DurationMS         float64        `json:"p99_duration_ms,omitempty"`
	TotalRetries          int            `json:"total_retries"`
	PeakExecutionsPerHour int            `json:"peak_executions_per_hour,omitempty"`
	StepMetrics           []StepMetric   `json:"step_metrics,omitempty"`
}

// StepMetric represents per-step analytics.
type StepMetric struct {
	StepID        string  `json:"step_id"`
	StepName      string  `json:"step_name"`
	TotalRuns     int     `json:"total_runs"`
	SuccessCount  int     `json:"success_count"`
	FailureCount  int     `json:"failure_count"`
	AvgDurationMS float64 `json:"avg_duration_ms,omitempty"`
}

// ─── Human task types ────────────────────────────────────────────────────────

// HumanTaskAssigneeType represents who a task is assigned to.
type HumanTaskAssigneeType string

const (
	HumanTaskAssigneeUser  HumanTaskAssigneeType = "user"
	HumanTaskAssigneeRole  HumanTaskAssigneeType = "role"
	HumanTaskAssigneeGroup HumanTaskAssigneeType = "group"
	HumanTaskAssigneeAgent HumanTaskAssigneeType = "agent"
)

// HumanTaskType represents the type of human task.
type HumanTaskType string

const (
	HumanTaskApproval  HumanTaskType = "approval"
	HumanTaskReject    HumanTaskType = "rejection"
	HumanTaskInput     HumanTaskType = "input"
	HumanTaskReview    HumanTaskType = "review"
	HumanTaskConfirm   HumanTaskType = "confirmation"
)

// HumanTaskPriority represents task priority level.
type HumanTaskPriority string

const (
	HumanTaskPriorityLow    HumanTaskPriority = "low"
	HumanTaskPriorityNormal HumanTaskPriority = "normal"
	HumanTaskPriorityHigh   HumanTaskPriority = "high"
	HumanTaskPriorityUrgent HumanTaskPriority = "urgent"
)

// HumanTaskStatus represents a human task's lifecycle state.
type HumanTaskStatus string

const (
	HumanTaskStatusPending   HumanTaskStatus = "pending"
	HumanTaskStatusApproved  HumanTaskStatus = "approved"
	HumanTaskStatusRejected  HumanTaskStatus = "rejected"
	HumanTaskStatusTimeout   HumanTaskStatus = "timeout"
	HumanTaskStatusCancelled HumanTaskStatus = "cancelled"
)

// HumanTask represents a human-in-the-loop task.
type HumanTask struct {
	ID                  string                   `json:"id"`
	TenantID            string                   `json:"tenant_id"`
	PipelineExecutionID string                   `json:"pipeline_execution_id"`
	StepID              string                   `json:"step_id,omitempty"`
	AssigneeType        HumanTaskAssigneeType    `json:"assignee_type"`
	AssigneeID          string                   `json:"assignee_id"`
	TaskType            HumanTaskType            `json:"task_type"`
	Instructions        string                   `json:"instructions"`
	Context             map[string]interface{}   `json:"context,omitempty"`
	TimeoutMinutes      int                      `json:"timeout_minutes,omitempty"`
	Label               string                   `json:"label,omitempty"`
	Priority            HumanTaskPriority        `json:"priority,omitempty"`
	Status              HumanTaskStatus          `json:"status"`
	Response            map[string]interface{}   `json:"response,omitempty"`
	RespondedBy         string                   `json:"responded_by,omitempty"`
	RespondedAt         *time.Time               `json:"responded_at,omitempty"`
	CreatedAt           time.Time                `json:"created_at"`
}

// ─── PipelineStore ───────────────────────────────────────────────────────────

// PipelineStore provides CRUD operations on pipeline data.
type PipelineStore struct {
	mu       sync.RWMutex
	pipelines map[string]*Pipeline
	byTenant map[string][]string // tenant_id -> pipeline IDs
}

// NewPipelineStore creates a new PipelineStore.
func NewPipelineStore() *PipelineStore {
	return &PipelineStore{
		pipelines: make(map[string]*Pipeline),
		byTenant:  make(map[string][]string),
	}
}

// Create adds a new pipeline.
func (s *PipelineStore) Create(p *Pipeline) (*Pipeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Status == "" {
		p.Status = PipelineStatusActive
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = timeNow()
	}
	p.UpdatedAt = timeNow()

	if _, exists := s.pipelines[p.ID]; exists {
		return nil, fmt.Errorf("pipeline %s already exists", p.ID)
	}

	s.pipelines[p.ID] = p
	s.byTenant[p.TenantID] = append(s.byTenant[p.TenantID], p.ID)

	return p, nil
}

// GetByID retrieves a pipeline by ID.
func (s *PipelineStore) GetByID(id string) (*Pipeline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	return p, nil
}

// Update patches a pipeline's fields.
func (s *PipelineStore) Update(id string, name *string, description *string, steps *[]PipelineStep, errorHandling *PipelineErrorHandlingConfig, timeoutMinutes *int, maxRetries *int, status *PipelineStatus, variables *map[string]interface{}, tags *[]string) (*Pipeline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}

	if name != nil {
		p.Name = *name
	}
	if description != nil {
		p.Description = *description
	}
	if steps != nil {
		p.Steps = *steps
	}
	if errorHandling != nil {
		p.ErrorHandling = errorHandling
	}
	if timeoutMinutes != nil {
		p.TimeoutMinutes = *timeoutMinutes
	}
	if maxRetries != nil {
		p.MaxRetries = *maxRetries
	}
	if status != nil {
		p.Status = *status
	}
	if variables != nil {
		p.Variables = *variables
	}
	if tags != nil {
		p.Tags = *tags
	}

	p.UpdatedAt = timeNow()
	return p, nil
}

// UpdateStatus sets the status of a pipeline.
func (s *PipelineStore) UpdateStatus(id string, status PipelineStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pipelines[id]
	if !ok {
		return fmt.Errorf("pipeline %s not found", id)
	}
	p.Status = status
	p.UpdatedAt = timeNow()
	return nil
}

// Delete removes a pipeline.
func (s *PipelineStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pipelines[id]
	if !ok {
		return fmt.Errorf("pipeline %s not found", id)
	}

	delete(s.pipelines, id)
	for i, tid := range s.byTenant[p.TenantID] {
		if tid == id {
			s.byTenant[p.TenantID] = append(s.byTenant[p.TenantID][:i], s.byTenant[p.TenantID][i+1:]...)
			break
		}
	}
	return nil
}

// List returns a paginated list of pipelines for a tenant.
func (s *PipelineStore) List(tenantID string, page, pageSize int, status *string) ([]*Pipeline, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return []*Pipeline{}, 0, false
	}

	all := make([]*Pipeline, 0, len(ids))
	for _, id := range ids {
		p := s.pipelines[id]
		if status != nil && string(p.Status) != *status {
			continue
		}
		all = append(all, p)
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	slices.SortFunc(all, func(a, b *Pipeline) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := all[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// IncrementExecutionCount increments execution count and updates success rate.
func (s *PipelineStore) IncrementExecutionCount(pipelineID string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pipelines[pipelineID]
	if !ok {
		return
	}
	p.ExecutionCount++
	if p.ExecutionCount == 0 {
		p.ExecutionCount = 1
	}
	total := float64(p.ExecutionCount)
	if p.SuccessRate == 0 {
		p.SuccessRate = 100.0
	} else {
		currentSuccess := (p.SuccessRate / 100.0) * float64(p.ExecutionCount-1)
		if success {
			currentSuccess++
		}
		p.SuccessRate = (currentSuccess / total) * 100.0
	}
}

// ─── ExecutionStore ──────────────────────────────────────────────────────────

// ExecutionStore provides CRUD operations on execution data.
type ExecutionStore struct {
	mu       sync.RWMutex
	executions map[string]*PipelineExecution
	byPipeline map[string][]string // pipeline_id -> execution IDs
	byTenant   map[string][]string // tenant_id -> execution IDs
	stepMap  map[string][]*PipelineExecutionStep // execution_id -> steps
}

// NewExecutionStore creates a new ExecutionStore.
func NewExecutionStore() *ExecutionStore {
	return &ExecutionStore{
		executions: make(map[string]*PipelineExecution),
		byPipeline: make(map[string][]string),
		byTenant:   make(map[string][]string),
		stepMap:    make(map[string][]*PipelineExecutionStep),
	}
}

// Create adds a new execution.
func (s *ExecutionStore) Create(e *PipelineExecution) (*PipelineExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.Status == "" {
		e.Status = PipelineExecutionPending
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = timeNow()
	}

	if _, exists := s.executions[e.ID]; exists {
		return nil, fmt.Errorf("execution %s already exists", e.ID)
	}

	s.executions[e.ID] = e
	s.byPipeline[e.PipelineID] = append(s.byPipeline[e.PipelineID], e.ID)
	s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID], e.ID)

	return e, nil
}

// GetByID retrieves an execution by ID.
func (s *ExecutionStore) GetByID(id string) (*PipelineExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution %s not found", id)
	}
	return e, nil
}

// UpdateStatus updates execution status.
func (s *ExecutionStore) UpdateStatus(id string, status PipelineExecutionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.executions[id]
	if !ok {
		return fmt.Errorf("execution %s not found", id)
	}
	e.Status = status
	if status == PipelineExecutionRunning && e.StartedAt == nil {
		t := timeNow()
		e.StartedAt = &t
	}
	if status == PipelineExecutionCompleted || status == PipelineExecutionFailed || status == PipelineExecutionCancelled {
		now := timeNow()
		e.CompletedAt = &now
	}
	return nil
}

// Delete removes an execution and its steps.
func (s *ExecutionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.executions[id]
	if !ok {
		return fmt.Errorf("execution %s not found", id)
	}

	delete(s.executions, id)

	for i, pid := range s.byPipeline[e.PipelineID] {
		if pid == id {
			s.byPipeline[e.PipelineID] = append(s.byPipeline[e.PipelineID][:i], s.byPipeline[e.PipelineID][i+1:]...)
			break
		}
	}
	for i, tid := range s.byTenant[e.TenantID] {
		if tid == id {
			s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID][:i], s.byTenant[e.TenantID][i+1:]...)
			break
		}
	}

	delete(s.stepMap, id)
	return nil
}

// ListByPipeline returns executions for a pipeline, with optional status filter.
func (s *ExecutionStore) ListByPipeline(pipelineID string, page, pageSize int, status *string, limit int) ([]*PipelineExecution, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byPipeline[pipelineID]
	if !ok {
		return []*PipelineExecution{}, 0, false
	}

	all := make([]*PipelineExecution, 0, len(ids))
	for _, id := range ids {
		e := s.executions[id]
		if status != nil && string(e.Status) != *status {
			continue
		}
		all = append(all, e)
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := all[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// ListByTenant returns paginated executions for a tenant.
func (s *ExecutionStore) ListByTenant(tenantID string, page, pageSize int) ([]*PipelineExecution, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return []*PipelineExecution{}, 0, false
	}

	all := make([]*PipelineExecution, 0, len(ids))
	for _, id := range ids {
		all = append(all, s.executions[id])
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := all[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// AddStep adds an execution step.
func (s *ExecutionStore) AddStep(step *PipelineExecutionStep) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stepMap[step.ExecutionID] = append(s.stepMap[step.ExecutionID], step)
}

// GetSteps returns all steps for an execution.
func (s *ExecutionStore) GetSteps(executionID string) []*PipelineExecutionStep {
	s.mu.RLock()
	defer s.mu.RUnlock()

	steps := s.stepMap[executionID]
	result := make([]*PipelineExecutionStep, len(steps))
	for i, step := range steps {
		cpy := *step
		if step.Inputs != nil {
			cpy.Inputs = make(map[string]interface{})
			for k, v := range step.Inputs {
				cpy.Inputs[k] = v
			}
		}
		if step.Outputs != nil {
			cpy.Outputs = make(map[string]interface{})
			for k, v := range step.Outputs {
				cpy.Outputs[k] = v
			}
		}
		result[i] = &cpy
	}
	return result
}

// IncrementRetryCount increments retry count.
func (s *ExecutionStore) IncrementRetryCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.executions[id]
	if !ok {
		return 0
	}
	e.RetryCount++
	return e.RetryCount
}

// ─── HumanTaskStore ──────────────────────────────────────────────────────────

// HumanTaskStore provides CRUD operations on human task data.
type HumanTaskStore struct {
	mu          sync.RWMutex
	tasks       map[string]*HumanTask
	byTenant    map[string][]string // tenant_id -> task IDs
	byExecution map[string][]string // execution_id -> task IDs
}

// NewHumanTaskStore creates a new HumanTaskStore.
func NewHumanTaskStore() *HumanTaskStore {
	return &HumanTaskStore{
		tasks:       make(map[string]*HumanTask),
		byTenant:    make(map[string][]string),
		byExecution: make(map[string][]string),
	}
}

// Create adds a new human task.
func (s *HumanTaskStore) Create(t *HumanTask) (*HumanTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Status == "" {
		t.Status = HumanTaskStatusPending
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = timeNow()
	}

	if _, exists := s.tasks[t.ID]; exists {
		return nil, fmt.Errorf("human task %s already exists", t.ID)
	}

	s.tasks[t.ID] = t
	s.byTenant[t.TenantID] = append(s.byTenant[t.TenantID], t.ID)
	s.byExecution[t.PipelineExecutionID] = append(s.byExecution[t.PipelineExecutionID], t.ID)

	return t, nil
}

// GetByID retrieves a human task by ID.
func (s *HumanTaskStore) GetByID(id string) (*HumanTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("human task %s not found", id)
	}
	return t, nil
}

// Respond updates a task's response and status.
func (s *HumanTaskStore) Respond(id string, action string, response map[string]interface{}, respondedBy string, comments string) (*HumanTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("human task %s not found", id)
	}

	if t.Status != HumanTaskStatusPending {
		return nil, fmt.Errorf("human task %s is not pending (current: %s)", id, t.Status)
	}

	t.Status = HumanTaskStatusApproved
	if action == "reject" || action == "request_info" {
		t.Status = HumanTaskStatusRejected
	}
	if response != nil {
		t.Response = make(map[string]interface{})
		for k, v := range response {
			t.Response[k] = v
		}
	}
	t.RespondedBy = respondedBy
	now := timeNow()
	t.RespondedAt = &now

	return t, nil
}

// List returns paginated tasks for a tenant, with optional status filter.
func (s *HumanTaskStore) List(tenantID string, status *string) ([]*HumanTask, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return []*HumanTask{}, 0
	}

	all := make([]*HumanTask, 0, len(ids))
	for _, id := range ids {
		t := s.tasks[id]
		if status != nil && string(t.Status) != *status {
			continue
		}
		all = append(all, t)
	}

	return all, len(all)
}
