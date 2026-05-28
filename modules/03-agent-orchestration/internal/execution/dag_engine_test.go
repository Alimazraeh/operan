package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// mockNodeHandler simulates node execution for testing.
type mockNodeHandler struct {
	mu       sync.Mutex
	results  map[string]interface{}
	failures map[string]error
	delay    time.Duration
	calls    map[string]int
}

func newMockNodeHandler() *mockNodeHandler {
	return &mockNodeHandler{
		results:  make(map[string]interface{}),
		failures: make(map[string]error),
		calls:    make(map[string]int),
	}
}

func (m *mockNodeHandler) SetResult(nodeID string, result interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[nodeID] = result
}

func (m *mockNodeHandler) SetFailure(nodeID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[nodeID] = err
}

func (m *mockNodeHandler) CallCount(nodeID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[nodeID]
}

func (m *mockNodeHandler) Handler(ctx context.Context, node store.WorkflowNode, workflowID string, variables map[string]interface{}) (map[string]interface{}, error) {
	m.mu.Lock()
	m.calls[node.ID]++
	calls := m.calls[node.ID]
	m.mu.Unlock()

	// Check if this node should fail
	m.mu.Lock()
	if err, ok := m.failures[node.ID]; ok && calls == 1 {
		m.mu.Unlock()
		return nil, err
	}
	m.mu.Unlock()

	// Simulate delay
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	// Return result
	m.mu.Lock()
	result, ok := m.results[node.ID]
	m.mu.Unlock()

	if !ok {
		result = map[string]interface{}{"node_id": node.ID, "status": "executed"}
	}

	return map[string]interface{}{"node_id": node.ID, "result": result}, nil
}

func TestStartWorkflow_SuccessfulExecution(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	// Create a simple 2-node workflow
	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "node-1", Type: store.WorkflowNodeAction, Action: "step1"},
				{ID: "node-2", Type: store.WorkflowNodeAction, Action: "step2"},
			},
			Edges: []store.WorkflowEdge{
				{From: "node-1", To: "node-2"},
			},
			ErrorStrategy: store.ErrorStrategyContinue,
		},
		Variables: make(map[string]interface{}),
	}

	_, err := wfStore.Create(wf)
	if err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	mockHandler.SetResult("node-1", "result1")
	mockHandler.SetResult("node-2", "result2")

	// Start execution
	err = engine.StartWorkflow("wf-1")
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// Wait for completion (with timeout)
	done := make(chan bool)
	go func() {
		for engine.IsRunning("wf-1") {
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
		// Workflow completed
	case <-time.After(2 * time.Second):
		t.Fatal("Workflow did not complete within timeout")
	}

	// Verify final status
	finishedWf, err := wfStore.GetByID("wf-1")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	if finishedWf.Status != store.WorkflowStatusCompleted {
		t.Errorf("Expected status %s, got %s", store.WorkflowStatusCompleted, finishedWf.Status)
	}

	// Verify execution events
	history := wfStore.GetExecutionHistory("wf-1")
	if len(history) < 3 { // start, node1, node2, complete
		t.Errorf("Expected at least 3 events, got %d", len(history))
	}
}

func TestStartWorkflow_AlreadyRunning(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "node-1", Type: store.WorkflowNodeAction},
			},
			Edges:         []store.WorkflowEdge{},
			ErrorStrategy: store.ErrorStrategyContinue,
		},
		Variables: make(map[string]interface{}),
	}

	wfStore.Create(wf)
	mockHandler.SetResult("node-1", "result1")

	engine.StartWorkflow("wf-1")

	// Starting again should fail
	err := engine.StartWorkflow("wf-1")
	if err == nil {
		t.Error("Expected error when starting already running workflow")
	}
}

func TestStartWorkflow_NodeFailure(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "node-1", Type: store.WorkflowNodeAction},
			},
			Edges:         []store.WorkflowEdge{},
			ErrorStrategy: store.ErrorStrategyAbort, // With abort strategy, node failure = workflow failure
		},
		Variables: make(map[string]interface{}),
	}

	wfStore.Create(wf)
	mockHandler.SetFailure("node-1", errors.New("node execution failed"))

	engine.StartWorkflow("wf-1")

	// Wait for completion
	done := make(chan bool)
	go func() {
		for engine.IsRunning("wf-1") {
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Workflow did not complete within timeout")
	}

	wf, err := wfStore.GetByID("wf-1")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	if wf.Status != store.WorkflowStatusFailed {
		t.Errorf("Expected status %s, got %s", store.WorkflowStatusFailed, wf.Status)
	}
}

func TestStartWorkflow_RetryPolicy(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{
					ID:    "node-1",
					Type:  store.WorkflowNodeAction,
					Retry: &store.RetryPolicy{MaxAttempts: 2, Backoff: store.BackoffConstant, InitialDelay: 10},
				},
			},
			Edges:         []store.WorkflowEdge{},
			ErrorStrategy: store.ErrorStrategyContinue,
		},
		Variables: make(map[string]interface{}),
	}

	wfStore.Create(wf)
	// Fail first attempt, succeed on second
	callCount := 0
	customHandler := func(ctx context.Context, node store.WorkflowNode, workflowID string, variables map[string]interface{}) (map[string]interface{}, error) {
		callCount++
		if callCount < 2 {
			return nil, errors.New("transient failure")
		}
		return map[string]interface{}{"node_id": node.ID, "result": "success"}, nil
	}
	engine.nodeHandler = customHandler

	engine.StartWorkflow("wf-1")

	// Wait for completion
	done := make(chan bool)
	go func() {
		for engine.IsRunning("wf-1") {
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Workflow did not complete within timeout")
	}

	wf, err := wfStore.GetByID("wf-1")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	if wf.Status != store.WorkflowStatusCompleted {
		t.Errorf("Expected status %s, got %s", store.WorkflowStatusCompleted, wf.Status)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls (1 fail + 1 retry), got %d", callCount)
	}
}

func TestStartWorkflow_ErrorStrategyAbort(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "node-1", Type: store.WorkflowNodeAction},
				{ID: "node-2", Type: store.WorkflowNodeAction},
			},
			Edges: []store.WorkflowEdge{
				{From: "node-1", To: "node-2"},
			},
			ErrorStrategy: store.ErrorStrategyAbort,
		},
		Variables: make(map[string]interface{}),
	}

	wfStore.Create(wf)
	mockHandler.SetResult("node-1", "result1")
	mockHandler.SetFailure("node-2", errors.New("node-2 failed"))

	engine.StartWorkflow("wf-1")

	// Wait for completion
	done := make(chan bool)
	go func() {
		for engine.IsRunning("wf-1") {
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Workflow did not complete within timeout")
	}

	wf, err := wfStore.GetByID("wf-1")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	// node-2 failed with abort strategy, so workflow should fail
	if wf.Status != store.WorkflowStatusFailed {
		t.Errorf("Expected status %s with abort strategy, got %s", store.WorkflowStatusFailed, wf.Status)
	}
}

func TestStopWorkflow(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	mockHandler.delay = 100 * time.Millisecond // Simulate slow execution
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "node-1", Type: store.WorkflowNodeAction},
			},
			Edges:         []store.WorkflowEdge{},
			ErrorStrategy: store.ErrorStrategyContinue,
		},
		Variables: make(map[string]interface{}),
	}

	wfStore.Create(wf)
	mockHandler.SetResult("node-1", "result1")

	engine.StartWorkflow("wf-1")

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop the workflow
	err := engine.StopWorkflow("wf-1")
	if err != nil {
		t.Fatalf("Failed to stop workflow: %v", err)
	}

	// Verify workflow is cancelled
	wf, err = wfStore.GetByID("wf-1")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	if wf.Status != store.WorkflowStatusCancelled {
		t.Errorf("Expected status %s, got %s", store.WorkflowStatusCancelled, wf.Status)
	}

	// Not running anymore
	if engine.IsRunning("wf-1") {
		t.Error("Workflow should not be running after stop")
	}
}

func TestTopologicalSort_Simple(t *testing.T) {
	nodes := []store.WorkflowNode{
		{ID: "node-1"},
		{ID: "node-2"},
		{ID: "node-3"},
	}
	edges := []store.WorkflowEdge{
		{From: "node-1", To: "node-2"},
		{From: "node-2", To: "node-3"},
	}

	order, err := TopologicalSort(nodes, edges)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if order[0] != "node-1" {
		t.Errorf("Expected node-1 first, got %s", order[0])
	}
	if order[1] != "node-2" {
		t.Errorf("Expected node-2 second, got %s", order[1])
	}
	if order[2] != "node-3" {
		t.Errorf("Expected node-3 third, got %s", order[2])
	}
}

func TestTopologicalSort_DagWithCycles(t *testing.T) {
	nodes := []store.WorkflowNode{
		{ID: "node-1"},
		{ID: "node-2"},
		{ID: "node-3"},
	}
	edges := []store.WorkflowEdge{
		{From: "node-1", To: "node-2"},
		{From: "node-2", To: "node-3"},
		{From: "node-3", To: "node-1"}, // Cycle
	}

	_, err := TopologicalSort(nodes, edges)
	if err == nil {
		t.Error("Expected error for cyclic graph")
	}
}

func TestValidateDAG(t *testing.T) {
	validGraph := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{{ID: "n1"}, {ID: "n2"}},
		Edges: []store.WorkflowEdge{{From: "n1", To: "n2"}},
	}

	if err := ValidateDAG(validGraph); err != nil {
		t.Errorf("Expected valid DAG, got error: %v", err)
	}

	cyclicGraph := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{{ID: "n1"}, {ID: "n2"}},
		Edges: []store.WorkflowEdge{
			{From: "n1", To: "n2"},
			{From: "n2", To: "n1"},
		},
	}

	if err := ValidateDAG(cyclicGraph); err == nil {
		t.Error("Expected error for cyclic DAG")
	}
}

func TestGetExecutionStats(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-5 * time.Second)
	completedAt := now

	wf := &store.Workflow{
		ID:          "wf-1",
		Status:      store.WorkflowStatusCompleted,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "n1"}, {ID: "n2"}, {ID: "n3"}, {ID: "n4"},
			},
		},
	}

	nodeStates := []store.NodeState{
		{NodeID: "n1", Status: store.NodeStatusCompleted},
		{NodeID: "n2", Status: store.NodeStatusCompleted},
		{NodeID: "n3", Status: store.NodeStatusFailed},
		{NodeID: "n4", Status: store.NodeStatusSkipped},
	}

	stats := GetExecutionStats(wf, nodeStates)

	if stats.WorkflowID != "wf-1" {
		t.Errorf("Expected workflow_id wf-1, got %s", stats.WorkflowID)
	}
	if stats.Status != store.WorkflowStatusCompleted {
		t.Errorf("Expected status completed, got %s", stats.Status)
	}
	if stats.TotalNodes != 4 {
		t.Errorf("Expected 4 total nodes, got %d", stats.TotalNodes)
	}
	if stats.CompletedNodes != 2 {
		t.Errorf("Expected 2 completed nodes, got %d", stats.CompletedNodes)
	}
	if stats.FailedNodes != 1 {
		t.Errorf("Expected 1 failed node, got %d", stats.FailedNodes)
	}
	if stats.SkippedNodes != 1 {
		t.Errorf("Expected 1 skipped node, got %d", stats.SkippedNodes)
	}
	if stats.DurationMs <= 0 {
		t.Errorf("Expected positive duration, got %d", stats.DurationMs)
	}
}

func TestExecutionStats_EmptyWorkflow(t *testing.T) {
	wf := &store.Workflow{
		ID:    "wf-1",
		Status: store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{},
		},
	}

	stats := GetExecutionStats(wf, []store.NodeState{})

	if stats.TotalNodes != 0 {
		t.Errorf("Expected 0 total nodes, got %d", stats.TotalNodes)
	}
	if stats.DurationMs != 0 {
		t.Errorf("Expected 0 duration for pending workflow, got %d", stats.DurationMs)
	}
}

func TestRetryableError(t *testing.T) {
	err := NewRetryableError(errors.New("transient error"), true, 1000)

	retryable, delay := IsRetryable(err)
	if !retryable {
		t.Error("Expected error to be retryable")
	}
	if delay != 1000 {
		t.Errorf("Expected delay 1000ms, got %d", delay)
	}

	// Non-retryable error
	nonRetryable := errors.New("permanent error")
	retryable, _ = IsRetryable(nonRetryable)
	if retryable {
		t.Error("Expected non-retryable error")
	}

	// Marshal JSON
	retryableErr := NewRetryableError(errors.New("test error"), true, 500)
	data, marshalErr := retryableErr.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("Failed to marshal: %v", marshalErr)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func TestStartWorkflow_ParallelNodes(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	eventPub := events.NewPublisher()
	mockHandler := newMockNodeHandler()
	engine := NewEngine(wfStore, eventPub, mockHandler.Handler, events.StackLangGraph)

	// Create workflow with 2 parallel nodes (both start at in-degree 0)
	wf := &store.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Parallel Workflow",
		Status:   store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "node-1", Type: store.WorkflowNodeAction},
				{ID: "node-2", Type: store.WorkflowNodeAction},
				{ID: "node-3", Type: store.WorkflowNodeAction},
			},
			Edges: []store.WorkflowEdge{
				{From: "node-1", To: "node-3"},
				{From: "node-2", To: "node-3"},
			},
			ErrorStrategy: store.ErrorStrategyContinue,
		},
		Variables: make(map[string]interface{}),
	}

	wfStore.Create(wf)
	mockHandler.SetResult("node-1", "result1")
	mockHandler.SetResult("node-2", "result2")
	mockHandler.SetResult("node-3", "result3")

	engine.StartWorkflow("wf-1")

	// Wait for completion
	done := make(chan bool)
	go func() {
		for engine.IsRunning("wf-1") {
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Workflow did not complete within timeout")
	}

	wf, err := wfStore.GetByID("wf-1")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	if wf.Status != store.WorkflowStatusCompleted {
		t.Errorf("Expected status %s, got %s", store.WorkflowStatusCompleted, wf.Status)
	}

	// Verify all nodes executed
	history := wfStore.GetExecutionHistory("wf-1")
	if len(history) < 4 { // start, node-1, node-2 (parallel), node-3, complete
		t.Errorf("Expected at least 4 events, got %d", len(history))
	}
}

func TestStartWorkflow_PriorityOrdering(t *testing.T) {
	// Test that nodes with same in-degree are executed in deterministic order
	nodes := []store.WorkflowNode{
		{ID: "z-node"},
		{ID: "a-node"},
		{ID: "m-node"},
	}
	edges := []store.WorkflowEdge{}

	order, err := TopologicalSort(nodes, edges)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be alphabetically sorted
	if order[0] != "a-node" {
		t.Errorf("Expected a-node first, got %s", order[0])
	}
	if order[1] != "m-node" {
		t.Errorf("Expected m-node second, got %s", order[1])
	}
	if order[2] != "z-node" {
		t.Errorf("Expected z-node third, got %s", order[2])
	}
}
