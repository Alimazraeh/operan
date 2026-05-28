// Package store implements the in-memory data store for the orchestration engine.
// All operations are protected with sync.RWMutex for thread safety.
package store

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Workflow types ──────────────────────────────────────────────────────────

// WorkflowStatus represents the lifecycle state of a workflow.
type WorkflowStatus string

const (
	WorkflowStatusPending    WorkflowStatus = "pending"
	WorkflowStatusRunning    WorkflowStatus = "running"
	WorkflowStatusPaused     WorkflowStatus = "paused"
	WorkflowStatusCompleted  WorkflowStatus = "completed"
	WorkflowStatusFailed     WorkflowStatus = "failed"
	WorkflowStatusCancelled  WorkflowStatus = "cancelled"
)

// ErrorStrategy represents workflow-level error handling.
type ErrorStrategy string

const (
	ErrorStrategyAbort    ErrorStrategy = "abort"
	ErrorStrategyContinue ErrorStrategy = "continue"
	ErrorStrategyRetryAll ErrorStrategy = "retry_all"
)

// WorkflowNodeType represents the type of a workflow node.
type WorkflowNodeType string

const (
	WorkflowNodeAgent       WorkflowNodeType = "agent"
	WorkflowNodeAction      WorkflowNodeType = "action"
	WorkflowNodeHumanGate   WorkflowNodeType = "human_gate"
	WorkflowNodeCondition   WorkflowNodeType = "condition"
	WorkflowNodeParallelBox WorkflowNodeType = "parallel_branch"
	WorkflowNodeDelay       WorkflowNodeType = "delay"
)

// NodeStatus represents the state of an individual node execution.
type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
)

// BackoffStrategy represents retry backoff behavior.
type BackoffStrategy string

const (
	BackoffConstant  BackoffStrategy = "constant"
	BackoffLinear    BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
)

// RetryPolicy defines retry behavior for a node.
type RetryPolicy struct {
	MaxAttempts   int              `json:"max_attempts,omitempty"`
	Backoff       BackoffStrategy  `json:"backoff,omitempty"`
	InitialDelay  int              `json:"initial_delay_ms,omitempty"`
	MaxDelay      int              `json:"max_delay_ms,omitempty"`
}

// WorkflowNode represents a single node in the workflow DAG.
type WorkflowNode struct {
	ID          string            `json:"id"`
	Type        WorkflowNodeType  `json:"type"`
	AgentID     string            `json:"agent_id,omitempty"`
	Action      string            `json:"action,omitempty"`
	TimeoutMs   int               `json:"timeout_ms,omitempty"`
	Retry       *RetryPolicy      `json:"retry,omitempty"`
	OnSuccess   string            `json:"on_success,omitempty"`
	OnFailure   string            `json:"on_failure,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// WorkflowEdge represents a directed edge in the workflow DAG.
type WorkflowEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
}

// WorkflowGraph represents the complete DAG definition.
type WorkflowGraph struct {
	Nodes         []WorkflowNode  `json:"nodes"`
	Edges         []WorkflowEdge  `json:"edges,omitempty"`
	ErrorStrategy ErrorStrategy   `json:"error_strategy,omitempty"`
}

// WorkflowVariables stores workflow runtime variables.
type WorkflowVariables struct {
	TenantID  string                 `json:"tenant_id"`
	Variables map[string]interface{} `json:"variables"`
	Version   int                    `json:"version"`
}

// Workflow represents a workflow instance.
type Workflow struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	DepartmentID  string            `json:"department_id,omitempty"`
	Name          string            `json:"name"`
	Version       string            `json:"version,omitempty"`
	Status        WorkflowStatus    `json:"status"`
	CurrentNodes  []string          `json:"current_nodes,omitempty"`
	Graph         WorkflowGraph     `json:"graph"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	Priority      int               `json:"priority"`
	Description   string            `json:"description,omitempty"`
	CreatedBy     string            `json:"created_by,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
}

// NodeState represents the runtime state of a node.
type NodeState struct {
	NodeID          string                 `json:"node_id"`
	Status          NodeStatus             `json:"status"`
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	Output          map[string]interface{} `json:"output,omitempty"`
	Error           string                 `json:"error,omitempty"`
	RetryCount      int                    `json:"retry_count"`
	ConfidenceScore float64                `json:"confidence_score,omitempty"`
}

// Checkpoint represents a workflow state checkpoint.
type Checkpoint struct {
	ID              string                 `json:"id"`
	WorkflowID      string                 `json:"workflow_id,omitempty"`
	NodeID          string                 `json:"node_id"`
	Timestamp       time.Time              `json:"timestamp"`
	StateSnapshot   map[string]interface{} `json:"state_snapshot,omitempty"`
	Checksum        string                 `json:"checksum,omitempty"`
}

// ExecutionEvent represents a lifecycle event during execution.
type ExecutionEvent struct {
	EventID   string                 `json:"event_id"`
	NodeID    string                 `json:"node_id"`
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// WorkflowState represents the complete runtime state of a workflow.
type WorkflowState struct {
	WorkflowID       string              `json:"workflow_id"`
	Status           WorkflowStatus      `json:"status"`
	Variables        map[string]interface{} `json:"variables,omitempty"`
	Nodes            []NodeState         `json:"nodes"`
	Checkpoints      []Checkpoint        `json:"checkpoints"`
	ExecutionHistory []ExecutionEvent    `json:"execution_history,omitempty"`
	Errors           []string            `json:"errors,omitempty"`
}

// ─── Schedule types ──────────────────────────────────────────────────────────

// Schedule represents a recurring workflow schedule.
type Schedule struct {
	ID                string                 `json:"id"`
	TenantID          string                 `json:"tenant_id"`
	Name              string                 `json:"name"`
	Cron              string                 `json:"cron"`
	WorkflowTemplateID string                `json:"workflow_template_id"`
	Variables         map[string]interface{} `json:"variables,omitempty"`
	Enabled           bool                   `json:"enabled"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// ─── Agent types ─────────────────────────────────────────────────────────────

// AgentStatus represents an agent's availability.
type AgentStatus string

const (
	AgentStatusAvailable  AgentStatus = "available"
	AgentStatusBusy       AgentStatus = "busy"
	AgentStatusOffline    AgentStatus = "offline"
)

// AgentAssignment represents an agent assignment to a workflow node.
type AgentAssignment struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	WorkflowID  string                 `json:"workflow_id"`
	NodeID      string                 `json:"node_id"`
	AgentID     string                 `json:"agent_id"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	AssignedAt  time.Time              `json:"assigned_at"`
}

// AgentAvailability represents an agent's availability status.
type AgentAvailability struct {
	AgentID          string    `json:"agent_id"`
	Status           AgentStatus `json:"status"`
	CurrentWorkflows int       `json:"current_workflows"`
	MaxConcurrency   int       `json:"max_concurrency"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
}

// ─── WorkflowStore ───────────────────────────────────────────────────────────

// WorkflowStore provides CRUD operations on workflow data.
type WorkflowStore struct {
	mu       sync.RWMutex
	workflows map[string]*Workflow
	byTenant  map[string][]string // tenant_id -> workflow IDs
	checkpoints map[string][]Checkpoint // workflow_id -> checkpoints
	variables   map[string]*WorkflowVariables // workflow_id -> variables
	history     map[string][]ExecutionEvent // workflow_id -> events
}

// NewWorkflowStore creates a new WorkflowStore.
func NewWorkflowStore() *WorkflowStore {
	return &WorkflowStore{
		workflows:   make(map[string]*Workflow),
		byTenant:    make(map[string][]string),
		checkpoints: make(map[string][]Checkpoint),
		variables:   make(map[string]*WorkflowVariables),
		history:     make(map[string][]ExecutionEvent),
	}
}

// Create adds a new workflow. Returns the created workflow.
func (s *WorkflowStore) Create(w *Workflow) (*Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	if w.Status == "" {
		w.Status = WorkflowStatusPending
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = timeNow()
	}
	if w.Priority < 1 {
		w.Priority = 5
	}
	if w.Priority > 10 {
		w.Priority = 10
	}

	if _, exists := s.workflows[w.ID]; exists {
		return nil, fmt.Errorf("workflow %s already exists", w.ID)
	}

	s.workflows[w.ID] = w
	s.byTenant[w.TenantID] = append(s.byTenant[w.TenantID], w.ID)

	return w, nil
}

// GetByID retrieves a workflow by ID.
func (s *WorkflowStore) GetByID(id string) (*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, ok := s.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow %s not found", id)
	}
	cpy := *w
	if w.CurrentNodes != nil {
		cpy.CurrentNodes = make([]string, len(w.CurrentNodes))
		copy(cpy.CurrentNodes, w.CurrentNodes)
	}
	if w.Graph.Nodes != nil {
		cpy.Graph = *w.Graph.DeepCopy()
	}
	if w.Variables != nil {
		cpy.Variables = make(map[string]interface{})
		for k, v := range w.Variables {
			cpy.Variables[k] = v
		}
	}
	return &cpy, nil
}

// UpdateStatus updates the status of a workflow.
func (s *WorkflowStore) UpdateStatus(id string, status WorkflowStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	valid := validWorkflowTransitions[w.Status]
	if !slices.Contains(valid, status) {
		return fmt.Errorf("invalid workflow status transition from %s to %s", w.Status, status)
	}

	w.Status = status
	if status == WorkflowStatusRunning && w.StartedAt == nil {
		t := timeNow()
		w.StartedAt = &t
	}
	if status == WorkflowStatusCompleted || status == WorkflowStatusFailed || status == WorkflowStatusCancelled {
		now := timeNow()
		w.CompletedAt = &now
	}

	return nil
}

// UpdateCurrentNodes updates the currently executing nodes.
func (s *WorkflowStore) UpdateCurrentNodes(id string, nodeIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	w.CurrentNodes = make([]string, len(nodeIDs))
	copy(w.CurrentNodes, nodeIDs)
	return nil
}

// List returns a paginated list of workflows for a tenant.
func (s *WorkflowStore) List(tenantID string, page, pageSize int, status *string) ([]*Workflow, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return []*Workflow{}, 0, false
	}

	all := make([]*Workflow, 0, len(ids))
	for _, id := range ids {
		w := s.workflows[id]
		if status != nil && string(w.Status) != *status {
			continue
		}
		cpy := *w
		if w.Graph.Nodes != nil {
			cpy.Graph = *w.Graph.DeepCopy()
		}
		all = append(all, &cpy)
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	slices.SortFunc(all, func(a, b *Workflow) int {
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

// AddCheckpoint adds a checkpoint for a workflow.
func (s *WorkflowStore) AddCheckpoint(cp Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wfID := cp.WorkflowID
	if wfID == "" {
		wfID = cp.NodeID
	}
	s.checkpoints[wfID] = append(s.checkpoints[wfID], cp)
}

// GetCheckpoints returns all checkpoints for a workflow.
func (s *WorkflowStore) GetCheckpoints(workflowID string) []Checkpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cps := s.checkpoints[workflowID]
	result := make([]Checkpoint, len(cps))
	for i, cp := range cps {
		result[i] = cp
	}
	return result
}

// AddVariable adds or updates a workflow variable.
func (s *WorkflowStore) AddVariable(workflowID, tenantID string, key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.variables[workflowID]
	if !ok {
		s.variables[workflowID] = &WorkflowVariables{
			TenantID:  tenantID,
			Variables: map[string]interface{}{key: value},
			Version:   1,
		}
		return nil
	}

	v.Variables[key] = value
	v.Version++
	return nil
}

// GetVariables returns workflow variables.
func (s *WorkflowStore) GetVariables(workflowID string) (*WorkflowVariables, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.variables[workflowID]
	if !ok {
		return nil, fmt.Errorf("variables for workflow %s not found", workflowID)
	}
	cpy := *v
	cpy.Variables = make(map[string]interface{})
	for k, val := range v.Variables {
		cpy.Variables[k] = val
	}
	return &cpy, nil
}

// SetVariables replaces all workflow variables.
func (s *WorkflowStore) SetVariables(workflowID, tenantID string, vars map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newVars := make(map[string]interface{})
	for k, val := range vars {
		newVars[k] = val
	}

	v, ok := s.variables[workflowID]
	if !ok {
		s.variables[workflowID] = &WorkflowVariables{
			TenantID:  tenantID,
			Variables: newVars,
			Version:   1,
		}
		return nil
	}
	v.Variables = newVars
	v.Version++
	return nil
}

// AddEvent adds an execution event to the history.
func (s *WorkflowStore) AddEvent(workflowID string, evt ExecutionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history[workflowID] = append(s.history[workflowID], evt)
}

// GetExecutionHistory returns events for a workflow.
func (s *WorkflowStore) GetExecutionHistory(workflowID string) []ExecutionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	evts := s.history[workflowID]
	result := make([]ExecutionEvent, len(evts))
	for i, evt := range evts {
		result[i] = evt
	}
	return result
}

// Delete removes a workflow.
func (s *WorkflowStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	delete(s.workflows, id)
	delete(s.checkpoints, id)
	delete(s.variables, id)
	delete(s.history, id)

	for i, tid := range s.byTenant[w.TenantID] {
		if tid == id {
			s.byTenant[w.TenantID] = append(s.byTenant[w.TenantID][:i], s.byTenant[w.TenantID][i+1:]...)
			break
		}
	}

	return nil
}

// DeepCopy creates a deep copy of WorkflowGraph.
func (g *WorkflowGraph) DeepCopy() *WorkflowGraph {
	cpy := WorkflowGraph{
		ErrorStrategy: g.ErrorStrategy,
		Nodes:         make([]WorkflowNode, len(g.Nodes)),
		Edges:         make([]WorkflowEdge, len(g.Edges)),
	}

	for i, n := range g.Nodes {
		cpy.Nodes[i] = n
		if n.Parameters != nil {
			cpy.Nodes[i].Parameters = make(map[string]interface{})
			for k, v := range n.Parameters {
				cpy.Nodes[i].Parameters[k] = v
			}
		}
	}
	for i, e := range g.Edges {
		cpy.Edges[i] = e
	}

	return &cpy
}

// ─── valid transitions ───────────────────────────────────────────────────────

var validWorkflowTransitions = map[WorkflowStatus][]WorkflowStatus{
	WorkflowStatusPending:    {WorkflowStatusRunning, WorkflowStatusCancelled},
	WorkflowStatusRunning:    {WorkflowStatusPaused, WorkflowStatusCompleted, WorkflowStatusFailed, WorkflowStatusCancelled},
	WorkflowStatusPaused:     {WorkflowStatusRunning, WorkflowStatusCancelled, WorkflowStatusFailed},
	WorkflowStatusCompleted:  {},
	WorkflowStatusFailed:     {WorkflowStatusRunning, WorkflowStatusCancelled},
	WorkflowStatusCancelled:  {},
}

// ─── ScheduleStore ───────────────────────────────────────────────────────────

// ScheduleStore provides CRUD operations on schedule data.
type ScheduleStore struct {
	mu       sync.RWMutex
	schedules map[string]*Schedule
	byTenant map[string][]string // tenant_id -> schedule IDs
}

// NewScheduleStore creates a new ScheduleStore.
func NewScheduleStore() *ScheduleStore {
	return &ScheduleStore{
		schedules: make(map[string]*Schedule),
		byTenant:  make(map[string][]string),
	}
}

// Create adds a new schedule.
func (s *ScheduleStore) Create(sc *Schedule) (*Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sc.ID == "" {
		sc.ID = uuid.New().String()
	}
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = timeNow()
	}
	sc.UpdatedAt = timeNow()

	if sc.Enabled {
		sc.Enabled = true
	}

	if _, exists := s.schedules[sc.ID]; exists {
		return nil, fmt.Errorf("schedule %s already exists", sc.ID)
	}

	s.schedules[sc.ID] = sc
	s.byTenant[sc.TenantID] = append(s.byTenant[sc.TenantID], sc.ID)

	return sc, nil
}

// GetByID retrieves a schedule by ID.
func (s *ScheduleStore) GetByID(id string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sc, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule %s not found", id)
	}
	cpy := *sc
	if sc.Variables != nil {
		cpy.Variables = make(map[string]interface{})
		for k, v := range sc.Variables {
			cpy.Variables[k] = v
		}
	}
	return &cpy, nil
}

// Patch updates fields of a schedule.
func (s *ScheduleStore) Patch(id string, name *string, cron *string, workflowTemplateID *string, variables *map[string]interface{}, enabled *bool) (*Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sc, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule %s not found", id)
	}

	if name != nil {
		sc.Name = *name
	}
	if cron != nil {
		sc.Cron = *cron
	}
	if workflowTemplateID != nil {
		sc.WorkflowTemplateID = *workflowTemplateID
	}
	if variables != nil {
		sc.Variables = make(map[string]interface{})
		for k, v := range *variables {
			sc.Variables[k] = v
		}
	}
	if enabled != nil {
		sc.Enabled = *enabled
	}

	sc.UpdatedAt = timeNow()
	return sc, nil
}

// Delete removes a schedule.
func (s *ScheduleStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sc, ok := s.schedules[id]
	if !ok {
		return fmt.Errorf("schedule %s not found", id)
	}

	delete(s.schedules, id)
	for i, tid := range s.byTenant[sc.TenantID] {
		if tid == id {
			s.byTenant[sc.TenantID] = append(s.byTenant[sc.TenantID][:i], s.byTenant[sc.TenantID][i+1:]...)
			break
		}
	}
	return nil
}

// List returns a paginated list of schedules for a tenant.
func (s *ScheduleStore) List(tenantID string, page, pageSize int, enabled *bool) ([]*Schedule, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return []*Schedule{}, 0, false
	}

	all := make([]*Schedule, 0, len(ids))
	for _, id := range ids {
		sc := s.schedules[id]
		if enabled != nil && sc.Enabled != *enabled {
			continue
		}
		cpy := *sc
		if sc.Variables != nil {
			cpy.Variables = make(map[string]interface{})
			for k, v := range sc.Variables {
				cpy.Variables[k] = v
			}
		}
		all = append(all, &cpy)
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	slices.SortFunc(all, func(a, b *Schedule) int {
		return a.NameCompare(b)
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

// NameCompare is a helper to sort schedules alphabetically by name.
func (sc *Schedule) NameCompare(other *Schedule) int {
	return strings.Compare(sc.Name, other.Name)
}

// ─── AgentStore ──────────────────────────────────────────────────────────────

// AgentStore provides operations on agent assignments.
type AgentStore struct {
	mu              sync.RWMutex
	assignments     map[string]*AgentAssignment // id -> assignment
	byWorkflow      map[string][]string // workflow_id -> assignment IDs
	agentAvailability map[string]*AgentAvailability // agent_id -> availability
}

// NewAgentStore creates a new AgentStore.
func NewAgentStore() *AgentStore {
	return &AgentStore{
		assignments:     make(map[string]*AgentAssignment),
		byWorkflow:      make(map[string][]string),
		agentAvailability: make(map[string]*AgentAvailability),
	}
}

// CreateAssignment adds a new agent assignment.
func (s *AgentStore) CreateAssignment(a *AgentAssignment) (*AgentAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.AssignedAt.IsZero() {
		a.AssignedAt = timeNow()
	}

	if _, exists := s.assignments[a.ID]; exists {
		return nil, fmt.Errorf("assignment %s already exists", a.ID)
	}

	s.assignments[a.ID] = a
	s.byWorkflow[a.WorkflowID] = append(s.byWorkflow[a.WorkflowID], a.ID)

	return a, nil
}

// GetByID retrieves an assignment by ID.
func (s *AgentStore) GetByID(id string) (*AgentAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.assignments[id]
	if !ok {
		return nil, fmt.Errorf("assignment %s not found", id)
	}
	cpy := *a
	if a.Parameters != nil {
		cpy.Parameters = make(map[string]interface{})
		for k, v := range a.Parameters {
			cpy.Parameters[k] = v
		}
	}
	return &cpy, nil
}

// SetAgentAvailability sets or updates an agent's availability status.
func (s *AgentStore) SetAgentAvailability(availability *AgentAvailability) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agentAvailability[availability.AgentID] = availability
}

// GetAgentAvailability retrieves an agent's availability.
func (s *AgentStore) GetAgentAvailability(agentID string) (*AgentAvailability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	avail, ok := s.agentAvailability[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	cpy := *avail
	return &cpy, nil
}

// ListByWorkflow returns all assignments for a workflow.
func (s *AgentStore) ListByWorkflow(workflowID string) ([]*AgentAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byWorkflow[workflowID]
	if !ok {
		return []*AgentAssignment{}, nil
	}

	result := make([]*AgentAssignment, 0, len(ids))
	for _, id := range ids {
		a := s.assignments[id]
		cpy := *a
		if a.Parameters != nil {
			cpy.Parameters = make(map[string]interface{})
			for k, v := range a.Parameters {
				cpy.Parameters[k] = v
			}
		}
		result = append(result, &cpy)
	}
	return result, nil
}

// ListAgentAvailability returns all registered agent availability entries.
func (s *AgentStore) ListAgentAvailability() []*AgentAvailability {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AgentAvailability, 0, len(s.agentAvailability))
	for _, avail := range s.agentAvailability {
		cpy := *avail
		result = append(result, &cpy)
	}
	return result
}

// ListByTenant returns all agent availability entries.
// Note: AgentAvailability doesn't have a TenantID field, so this returns all records.
func (s *AgentStore) ListByTenant(tenantID string) []*AgentAvailability {
	_ = tenantID // unused - AgentAvailability has no tenant field
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AgentAvailability, 0, len(s.agentAvailability))
	for _, avail := range s.agentAvailability {
		cpy := *avail
		result = append(result, &cpy)
	}
	return result
}

// timeNow returns the current UTC time. Overridable for testing.
var timeNow = time.Now
