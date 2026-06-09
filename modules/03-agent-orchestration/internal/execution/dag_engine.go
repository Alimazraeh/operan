// Package execution provides a LangGraph-style DAG execution engine for workflow orchestration.
package execution

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
	"github.com/google/uuid"
)

// NodeHandler is a function that executes a single workflow node.
// The implementation is provided by the handler at runtime.
type NodeHandler func(ctx context.Context, node store.WorkflowNode, workflowID string, variables map[string]interface{}) (map[string]interface{}, error)

// Engine executes workflows based on their DAG definitions.
type Engine struct {
	store       repository.WorkflowStoreIface
	eventPub    *events.Publisher
	nodeHandler NodeHandler
	stackType   events.StackType
	mu          sync.Mutex
	running     map[string]context.CancelFunc // workflowID -> cancel func
}

// NewEngine creates a new DAG execution engine.
func NewEngine(store repository.WorkflowStoreIface, eventPub *events.Publisher, nh NodeHandler, stack events.StackType) *Engine {
	return &Engine{
		store:       store,
		eventPub:    eventPub,
		nodeHandler: nh,
		stackType:   stack,
		running:     make(map[string]context.CancelFunc),
	}
}

// StartWorkflow initiates execution of a workflow by its ID.
func (e *Engine) StartWorkflow(workflowID string) error {
	e.mu.Lock()
	if _, isRunning := e.running[workflowID]; isRunning {
		e.mu.Unlock()
		return errors.New("workflow " + workflowID + " is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.running[workflowID] = cancel
	e.mu.Unlock()

	// Update status to running
	if err := e.store.UpdateStatus(workflowID, store.WorkflowStatusRunning); err != nil {
		e.mu.Lock()
		delete(e.running, workflowID)
		e.mu.Unlock()
		return errors.New("failed to update workflow status: " + err.Error())
	}

	// Record execution start event
	e.store.AddEvent(workflowID, store.ExecutionEvent{
		EventID:   uuid.New().String(),
		EventType: "workflow_start",
		Timestamp: time.Now().UTC(),
		Details:   map[string]interface{}{"workflow_id": workflowID},
	})

	// Publish event
	if e.eventPub != nil {
		e.eventPub.PublishWorkflowStarted(e.stackType, events.WorkflowStartedPayload{
			WorkflowID: workflowID,
			StartedAt:  time.Now().UTC(),
			InitialNodes: []string{"node-1"}, // Placeholder - would be computed from DAG
		})
	}

	// Execute in background goroutine
	go e.execute(ctx, workflowID)

	return nil
}

// StopWorkflow stops a running workflow.
func (e *Engine) StopWorkflow(workflowID string) error {
	e.mu.Lock()
	cancel, ok := e.running[workflowID]
	if ok {
		cancel()
		delete(e.running, workflowID)
	}
	e.mu.Unlock()

	if ok {
		if err := e.store.UpdateStatus(workflowID, store.WorkflowStatusCancelled); err != nil {
			log.Printf("[DAG] Failed to update workflow %s status: %v", workflowID, err)
		}
		e.store.AddEvent(workflowID, store.ExecutionEvent{
			EventID:   uuid.New().String(),
			EventType: "workflow_cancelled",
			Timestamp: time.Now().UTC(),
		})
		if e.eventPub != nil {
			e.eventPub.PublishWorkflowCancelled(e.stackType, events.WorkflowCancelledPayload{
				WorkflowID:         workflowID,
				CancelledBy:        "system",
				CancelledAt:        time.Now().UTC(),
				CancellationReason: "workflow stopped via API",
			})
		}
	}
	return nil
}

// IsRunning checks if a workflow is currently executing.
func (e *Engine) IsRunning(workflowID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.running[workflowID]
	return ok
}

// execute runs the DAG execution loop for a workflow.
func (e *Engine) execute(ctx context.Context, workflowID string) {
	// Get the workflow
	wf, err := e.store.GetByID(workflowID)
	if err != nil {
		log.Printf("[DAG] Failed to get workflow %s: %v", workflowID, err)
		e.store.UpdateStatus(workflowID, store.WorkflowStatusFailed)
		e.store.AddEvent(workflowID, store.ExecutionEvent{
			EventID:   uuid.New().String(),
			EventType: "workflow_error",
			Timestamp: time.Now().UTC(),
			Details:   map[string]interface{}{"error": err.Error()},
		})
		if e.eventPub != nil {
			e.eventPub.PublishWorkflowFailed(e.stackType, events.WorkflowFailedPayload{
				WorkflowID:   workflowID,
				FailedAt:     time.Now().UTC(),
				ErrorCode:    "workflow_not_found",
				ErrorMessage: err.Error(),
			})
		}
		return
	}

	graph := wf.Graph
	nodes := make(map[string]store.WorkflowNode)
	for _, n := range graph.Nodes {
		nodes[n.ID] = n
	}

	// Build adjacency list
	successors := make(map[string][]string)
	for _, edge := range graph.Edges {
		successors[edge.From] = append(successors[edge.From], edge.To)
	}

	// Compute in-degree for topological sort
	inDegree := make(map[string]int)
	for _, n := range graph.Nodes {
		if _, ok := inDegree[n.ID]; !ok {
			inDegree[n.ID] = 0
		}
	}
	for _, edge := range graph.Edges {
		inDegree[edge.To]++
	}

	// Find initial ready nodes (in-degree 0)
	ready := []string{}
	for _, n := range graph.Nodes {
		if inDegree[n.ID] == 0 {
			ready = append(ready, n.ID)
		}
	}

	// Track node states
	nodeStates := make(map[string]*store.NodeState)
	completed := make(map[string]bool)
	failed := make(map[string]bool)
	skipped := make(map[string]bool)

	// Initialize all node states
	for id, node := range nodes {
		nodeStates[id] = &store.NodeState{
			NodeID:     id,
			Status:     store.NodeStatusPending,
			RetryCount: 0,
		}
		_ = node
	}

	// Execute rounds
	for len(ready) > 0 {
		select {
		case <-ctx.Done():
			// Workflow was cancelled
			e.mu.Lock()
			delete(e.running, workflowID)
			e.mu.Unlock()
			e.finalizeWorkflow(workflowID, store.WorkflowStatusCancelled, nodeStates)
			return
		default:
		}

		currentBatch := ready
		ready = nil

		// Execute all ready nodes in parallel (bounded by concurrency).
		// batchMu guards the shared completed/failed maps and the ready slice,
		// which the per-node goroutines below write concurrently.
		var wg sync.WaitGroup
		var batchMu sync.Mutex
		batchErrors := make([]error, len(currentBatch))

		for i, nodeID := range currentBatch {
			node := nodes[nodeID]
			state := nodeStates[nodeID]

			// Skip if predecessor failed or skipped
			if e.hasFailedPredecessor(nodeID, graph.Edges, skipped, failed) {
				state.Status = store.NodeStatusSkipped
				skipped[nodeID] = true
				continue
			}

			// Check condition edge
			if !e.conditionsMet(nodeID, graph.Edges, completed, skipped, failed) {
				skipped[nodeID] = true
				state.Status = store.NodeStatusSkipped
				continue
			}

			wg.Add(1)
			go func(idx int, nid string, nd store.WorkflowNode, ns *store.NodeState) {
				defer wg.Done()
				result, err := e.executeNode(ctx, nid, nd, workflowID, wf.Variables, ns)
				batchErrors[idx] = err
				if err == nil {
					ns.Output = result
					ns.Status = store.NodeStatusCompleted
					batchMu.Lock()
					completed[nid] = true
					batchMu.Unlock()
				} else {
					ns.Error = err.Error()
					// Check if we should retry
					if nd.Retry != nil && ns.RetryCount < nd.Retry.MaxAttempts {
						ns.RetryCount++
						ns.Status = store.NodeStatusRunning
						batchMu.Lock()
						failed[nid] = true
						ready = append(ready, nid) // Re-queue for retry
						batchMu.Unlock()
					} else {
						ns.Status = store.NodeStatusFailed
						batchMu.Lock()
						failed[nid] = true
						batchMu.Unlock()
					}
				}
			}(i, nodeID, node, state)
		}

		wg.Wait()

		// Check for hard failures with abort strategy
		hardFailed := false
		for i, nodeID := range currentBatch {
			if batchErrors[i] != nil && (nodes[nodeID].Retry == nil || nodeStates[nodeID].RetryCount >= nodes[nodeID].Retry.MaxAttempts) {
				hardFailed = true
			}
		}

		if hardFailed && graph.ErrorStrategy == store.ErrorStrategyAbort {
			e.mu.Lock()
			delete(e.running, workflowID)
			e.mu.Unlock()
			e.finalizeWorkflow(workflowID, store.WorkflowStatusFailed, nodeStates)
			return
		}

		// Find next ready nodes
		for _, nodeID := range currentBatch {
			if !completed[nodeID] && !failed[nodeID] && !skipped[nodeID] {
				continue
			}
			// Check if any successors are now ready
			for _, succID := range successors[nodeID] {
				if e.allPredecessorsDone(succID, graph.Edges, completed, skipped, failed) {
					if _, alreadyReady := findInList(succID, ready); !alreadyReady {
						ready = append(ready, succID)
					}
				}
			}
		}

		// Deduplicate ready list
		ready = uniqueStrings(ready)
	}

	// Remove from running map
	e.mu.Lock()
	delete(e.running, workflowID)
	e.mu.Unlock()

	// All nodes executed
	e.finalizeWorkflow(workflowID, store.WorkflowStatusCompleted, nodeStates)
}

// executeNode runs a single node using the registered node handler.
func (e *Engine) executeNode(ctx context.Context, nodeID string, node store.WorkflowNode, workflowID string, variables map[string]interface{}, state *store.NodeState) (map[string]interface{}, error) {
	state.Status = store.NodeStatusRunning
	startedAt := time.Now().UTC()
	state.StartedAt = &startedAt

	// Record node start event
	e.store.AddEvent(workflowID, store.ExecutionEvent{
		EventID:   uuid.New().String(),
		NodeID:    nodeID,
		EventType: "node_start",
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"node_type": string(node.Type),
			"node_id":   nodeID,
		},
	})

	// Execute the node
	output, err := e.nodeHandler(ctx, node, workflowID, variables)

	if err != nil {
		completedAt := time.Now().UTC()
		state.CompletedAt = &completedAt
		state.Error = err.Error()
		state.Status = store.NodeStatusFailed

		e.store.AddEvent(workflowID, store.ExecutionEvent{
			EventID:   uuid.New().String(),
			NodeID:    nodeID,
			EventType: "node_failed",
			Timestamp: time.Now().UTC(),
			Details: map[string]interface{}{
				"node_id": nodeID,
				"error":   err.Error(),
				"retry":   state.RetryCount,
			},
		})

		return nil, err
	}

	completedAt := time.Now().UTC()
	state.CompletedAt = &completedAt
	state.Output = output
	state.Status = store.NodeStatusCompleted

	e.store.AddEvent(workflowID, store.ExecutionEvent{
		EventID:   uuid.New().String(),
		NodeID:    nodeID,
		EventType: "node_completed",
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"node_id": nodeID,
		},
	})

	return output, nil
}

// finalizeWorkflow completes execution and updates status.
func (e *Engine) finalizeWorkflow(workflowID string, status store.WorkflowStatus, nodeStates map[string]*store.NodeState) {
	// Save workflow state
	e.store.UpdateStatus(workflowID, status)
	e.store.AddEvent(workflowID, store.ExecutionEvent{
		EventID:   uuid.New().String(),
		EventType: "workflow_" + string(status),
		Timestamp: time.Now().UTC(),
	})

	if e.eventPub != nil {
		switch status {
		case store.WorkflowStatusCompleted:
			e.eventPub.PublishWorkflowCompleted(e.stackType, events.WorkflowCompletedPayload{
				WorkflowID: workflowID,
				CompletedAt: time.Now().UTC(),
				FinalStatus: string(status),
			})
		case store.WorkflowStatusFailed:
			e.eventPub.PublishWorkflowFailed(e.stackType, events.WorkflowFailedPayload{
				WorkflowID:   workflowID,
				FailedAt:     time.Now().UTC(),
				ErrorCode:    "execution_failed",
				ErrorMessage: "workflow execution failed",
			})
		case store.WorkflowStatusCancelled:
			e.eventPub.PublishWorkflowCancelled(e.stackType, events.WorkflowCancelledPayload{
				WorkflowID:         workflowID,
				CancelledBy:        "system",
				CancelledAt:        time.Now().UTC(),
				CancellationReason: "workflow cancelled after completion",
			})
		}
	}
}

// hasFailedPredecessor checks if any predecessor of a node has failed.
func (e *Engine) hasFailedPredecessor(nodeID string, edges []store.WorkflowEdge, skipped, failed map[string]bool) bool {
	for _, edge := range edges {
		if edge.To == nodeID {
			if failed[edge.From] || skipped[edge.From] {
				return true
			}
		}
	}
	return false
}

// conditionsMet checks if conditional edges allow execution.
func (e *Engine) conditionsMet(nodeID string, edges []store.WorkflowEdge, completed, skipped, failed map[string]bool) bool {
	for _, edge := range edges {
		if edge.To == nodeID && edge.Condition != "" {
			// For now, always allow conditional edges (condition evaluation is a future enhancement)
			_ = completed[edge.From]
			_ = skipped[edge.From]
			_ = failed[edge.From]
		}
	}
	return true
}

// allPredecessorsDone checks if all predecessors of a node are done (completed, failed, or skipped).
func (e *Engine) allPredecessorsDone(nodeID string, edges []store.WorkflowEdge, completed, skipped, failed map[string]bool) bool {
	for _, edge := range edges {
		if edge.To == nodeID {
			d := completed[edge.From] || failed[edge.From] || skipped[edge.From]
			if !d {
				return false
			}
		}
	}
	return true
}

// uniqueStrings returns a deduplicated string slice.
func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// findInList checks if a string exists in a slice.
func findInList(s string, list []string) (int, bool) {
	for i, item := range list {
		if item == s {
			return i, true
		}
	}
	return -1, false
}

// TopologicalSort returns nodes in topological order.
func TopologicalSort(nodes []store.WorkflowNode, edges []store.WorkflowEdge) ([]string, error) {
	nodeSet := make(map[string]bool)
	for _, n := range nodes {
		nodeSet[n.ID] = true
	}

	inDegree := make(map[string]int)
	for id := range nodeSet {
		inDegree[id] = 0
	}
	for _, e := range edges {
		if !nodeSet[e.From] || !nodeSet[e.To] {
			return nil, errors.New("edge references unknown node: " + e.From + " -> " + e.To)
		}
		inDegree[e.To]++
	}

	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	sort.Strings(queue) // Deterministic ordering

	result := []string{}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, e := range edges {
			if e.From == node {
				inDegree[e.To]--
				if inDegree[e.To] == 0 {
					queue = append(queue, e.To)
				}
			}
		}
	}

	if len(result) != len(nodeSet) {
		return nil, errors.New("cycle detected in workflow graph")
	}

	return result, nil
}

// ValidateDAG checks if a workflow graph is a valid DAG.
func ValidateDAG(graph store.WorkflowGraph) error {
	_, err := TopologicalSort(graph.Nodes, graph.Edges)
	return err
}

// ExecutionStats holds statistics about a workflow execution.
type ExecutionStats struct {
	WorkflowID     string               `json:"workflow_id"`
	Status         store.WorkflowStatus `json:"status"`
	TotalNodes     int                  `json:"total_nodes"`
	CompletedNodes int                  `json:"completed_nodes"`
	FailedNodes    int                  `json:"failed_nodes"`
	SkippedNodes   int                  `json:"skipped_nodes"`
	StartTime      time.Time            `json:"start_time,omitempty"`
	EndTime        time.Time            `json:"end_time,omitempty"`
	DurationMs     int64                `json:"duration_ms,omitempty"`
}

// GetExecutionStats returns execution statistics for a workflow.
func GetExecutionStats(wf *store.Workflow, nodeStates []store.NodeState) *ExecutionStats {
	stats := &ExecutionStats{
		WorkflowID:   wf.ID,
		Status:       wf.Status,
		TotalNodes:   len(wf.Graph.Nodes),
	}

	if wf.StartedAt != nil {
		stats.StartTime = *wf.StartedAt
	}

	if wf.CompletedAt != nil {
		stats.EndTime = *wf.CompletedAt
		stats.DurationMs = stats.EndTime.Sub(stats.StartTime).Milliseconds()
	}

	for _, ns := range nodeStates {
		switch ns.Status {
		case store.NodeStatusCompleted:
			stats.CompletedNodes++
		case store.NodeStatusFailed:
			stats.FailedNodes++
		case store.NodeStatusSkipped:
			stats.SkippedNodes++
		}
	}

	return stats
}

// RetryableError wraps an error with retry metadata.
type RetryableError struct {
	Err       error
	ShouldRetry bool
	DelayMs   int
}

// NewRetryableError creates a new retryable error.
func NewRetryableError(err error, shouldRetry bool, delayMs int) *RetryableError {
	return &RetryableError{
		Err:         err,
		ShouldRetry: shouldRetry,
		DelayMs:     delayMs,
	}
}

// IsRetryable checks if an error is a retryable error.
func IsRetryable(err error) (bool, int) {
	re, ok := err.(*RetryableError)
	if !ok {
		return false, 0
	}
	return re.ShouldRetry, re.DelayMs
}

// Error implements the error interface for RetryableError.
func (re *RetryableError) Error() string {
	return re.Err.Error()
}

// MarshalJSON implements custom JSON marshaling for RetryableError.
func (re *RetryableError) MarshalJSON() ([]byte, error) {
	return []byte(`{"error":"` + re.Err.Error() + `","should_retry":` + stringBool(re.ShouldRetry) + `,"delay_ms":` + fmt.Sprintf("%d", re.DelayMs) + `}`), nil
}

// stringBool converts a bool to a JSON string without quotes.
func stringBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
