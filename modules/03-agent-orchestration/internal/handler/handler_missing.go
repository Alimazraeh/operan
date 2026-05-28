// Package handler provides HTTP request handlers for the orchestration engine.
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── EscalationHandler ───────────────────────────────────────────────────────

// EscalationHandler handles escalation-related HTTP endpoints.
type EscalationHandler struct {
	EscalationStore repository.EscalationStoreIface
	WorkflowStore   repository.WorkflowStoreIface
	Events          *events.Publisher
}

// NewEscalationHandler creates a new EscalationHandler.
func NewEscalationHandler(es repository.EscalationStoreIface, wf repository.WorkflowStoreIface) *EscalationHandler {
	return &EscalationHandler{
		EscalationStore: es,
		WorkflowStore:   wf,
	}
}

func (h *EscalationHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *EscalationHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// ListWorkflowEscalations handles GET /workflows/{id}/escalations.
func (h *EscalationHandler) ListWorkflowEscalations(w http.ResponseWriter, r *http.Request, workflowID string) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	wf, err := h.WorkflowStore.GetByID(workflowID)
	if err != nil || wf == nil {
		h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
		return
	}
	if wf.TenantID != tenantID {
		h.WriteError(w, http.StatusForbidden, 403, "tenant mismatch")
		return
	}

	escs := h.EscalationStore.ListByWorkflow(workflowID)
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"escalations": escs,
		"total":       len(escs),
		"has_more":    false,
	})
}

// CreateEscalation handles POST /workflows/{id}/escalations.
func (h *EscalationHandler) CreateEscalation(w http.ResponseWriter, r *http.Request, workflowID string) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	wf, err := h.WorkflowStore.GetByID(workflowID)
	if err != nil || wf == nil {
		h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
		return
	}
	if wf.TenantID != tenantID {
		h.WriteError(w, http.StatusForbidden, 403, "tenant mismatch")
		return
	}

	var req struct {
		Severity       string `json:"severity"`
		Reason         string `json:"reason"`
		EscalatedTo    string `json:"escalated_to,omitempty"`
		NodeID         string `json:"node_id"`
		DepartmentID   string `json:"department_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	severity := store.EscalationMedium
	switch req.Severity {
	case string(store.EscalationLow):
		severity = store.EscalationLow
	case string(store.EscalationMedium):
		severity = store.EscalationMedium
	case string(store.EscalationHigh):
		severity = store.EscalationHigh
	case string(store.EscalationCritical):
		severity = store.EscalationCritical
	}

	esc := &store.Escalation{
		ID:           generateID(),
		WorkflowID:   workflowID,
		NodeID:       req.NodeID,
		TenantID:     tenantID,
		DepartmentID: req.DepartmentID,
		Severity:     severity,
		Reason:       req.Reason,
		EscalatedTo:  req.EscalatedTo,
		Status:       store.EscalationPending,
		CreatedAt:    time.Now().UTC(),
	}

	h.EscalationStore.Create(esc)

	// Publish escalation created event
	if h.Events != nil {
		h.Events.PublishEscalationCreated(events.StackLangGraph, events.EscalationCreatedPayload{
			EscalationID: esc.ID,
			WorkflowID:   esc.WorkflowID,
			NodeID:       esc.NodeID,
			Severity:     string(esc.Severity),
			Reason:       esc.Reason,
			CreatedAt:    esc.CreatedAt,
		})
	}

	h.WriteJSON(w, http.StatusCreated, esc)
}

// AcknowledgeEscalation handles PATCH /escalations/{id}/acknowledge.
func (h *EscalationHandler) AcknowledgeEscalation(w http.ResponseWriter, r *http.Request, escalationID string) {
	esc, ok := h.EscalationStore.GetByID(escalationID)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "escalation not found")
		return
	}

	if !h.EscalationStore.Acknowledge(escalationID) {
		h.WriteError(w, http.StatusConflict, 409, "escalation is not in pending status")
		return
	}

	esc, _ = h.EscalationStore.GetByID(escalationID)

	// Publish escalation acknowledged event
	if h.Events != nil {
		h.Events.PublishEscalationAcknowledged(events.StackLangGraph, events.EscalationAcknowledgedPayload{
			EscalationID:   esc.ID,
			AcknowledgedBy: middleware.UserIDFromContext(r.Context()),
			AcknowledgedAt: time.Now().UTC(),
		})
	}

	h.WriteJSON(w, http.StatusOK, esc)
}

// ResolveEscalation handles PATCH /escalations/{id}/resolve.
func (h *EscalationHandler) ResolveEscalation(w http.ResponseWriter, r *http.Request, escalationID string) {
	esc, ok := h.EscalationStore.GetByID(escalationID)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "escalation not found")
		return
	}

	if !h.EscalationStore.Resolve(escalationID) {
		h.WriteError(w, http.StatusConflict, 409, "escalation cannot be resolved")
		return
	}

	esc, _ = h.EscalationStore.GetByID(escalationID)

	// Publish escalation resolved event
	if h.Events != nil {
		h.Events.PublishEscalationResolved(events.StackLangGraph, events.EscalationResolvedPayload{
			EscalationID:    esc.ID,
			ResolvedBy:      middleware.UserIDFromContext(r.Context()),
			ResolvedAt:      time.Now().UTC(),
			ResolutionNotes: "auto-resolved by system",
		})
	}

	h.WriteJSON(w, http.StatusOK, esc)
}

// ─── RetryHandler ────────────────────────────────────────────────────────────

// RetryHandler handles retry-related HTTP endpoints.
type RetryHandler struct {
	RetryStore     repository.RetryRecordStoreIface
	WorkflowStore  repository.WorkflowStoreIface
	ExecutionStore repository.ExecutionStoreIface
	Events         *events.Publisher
}

// NewRetryHandler creates a new RetryHandler.
func NewRetryHandler(rs repository.RetryRecordStoreIface, wf repository.WorkflowStoreIface, ex repository.ExecutionStoreIface) *RetryHandler {
	return &RetryHandler{
		RetryStore:     rs,
		WorkflowStore:  wf,
		ExecutionStore: ex,
	}
}

// WithEvents sets the event publisher on the RetryHandler.
func (h *RetryHandler) WithEvents(e *events.Publisher) *RetryHandler {
	h.Events = e
	return h
}

func (h *RetryHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *RetryHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// ListWorkflowRetryRecords handles GET /workflows/{id}/retries.
func (h *RetryHandler) ListWorkflowRetryRecords(w http.ResponseWriter, r *http.Request, workflowID string) {
	records := h.RetryStore.ListByWorkflow(workflowID)
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"retry_records": records,
		"total":         len(records),
		"has_more":      false,
	})
}

// RetryNode handles POST /workflows/{id}/nodes/{nodeId}/retry.
func (h *RetryHandler) RetryNode(w http.ResponseWriter, r *http.Request, workflowID, nodeID string) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	wf, err := h.WorkflowStore.GetByID(workflowID)
	if err != nil || wf == nil {
		h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
		return
	}
	if wf.TenantID != tenantID {
		h.WriteError(w, http.StatusForbidden, 403, "tenant mismatch")
		return
	}

	records := h.RetryStore.ListByWorkflow(workflowID)
	maxAttempt := len(records) + 1

	retryRecord := &store.RetryRecord{
		ID:            generateID(),
		WorkflowID:    workflowID,
		NodeID:        nodeID,
		TenantID:      tenantID,
		AttemptNumber: maxAttempt,
		Status:        store.RetryInProgress,
		CreatedAt:     time.Now().UTC(),
	}

	h.RetryStore.Create(retryRecord)
	h.WriteJSON(w, http.StatusCreated, retryRecord)
}

// ─── NodesResultsHandler ─────────────────────────────────────────────────────

// NodesResultsHandler handles node listing and results endpoints.
type NodesResultsHandler struct {
	WorkflowStore repository.WorkflowStoreIface
}

// NewNodesResultsHandler creates a new NodesResultsHandler.
func NewNodesResultsHandler(wf repository.WorkflowStoreIface) *NodesResultsHandler {
	return &NodesResultsHandler{
		WorkflowStore: wf,
	}
}

func (h *NodesResultsHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *NodesResultsHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// ListWorkflowNodes handles GET /workflows/{id}/nodes.
func (h *NodesResultsHandler) ListWorkflowNodes(w http.ResponseWriter, r *http.Request, workflowID string) {
	wf, err := h.WorkflowStore.GetByID(workflowID)
	if err != nil || wf == nil {
		h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
		return
	}

	nodes := wf.Graph.Nodes
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"nodes":    nodes,
		"total":    len(nodes),
		"has_more": false,
	})
}

// ListWorkflowResults handles GET /workflows/{id}/results.
func (h *NodesResultsHandler) ListWorkflowResults(w http.ResponseWriter, r *http.Request, workflowID string) {
	checkpoints := h.WorkflowStore.GetCheckpoints(workflowID)
	if checkpoints == nil {
		checkpoints = []store.Checkpoint{}
	}

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"results":  checkpoints,
		"total":    len(checkpoints),
		"has_more": false,
	})
}

// ─── AgentWorkersHandler ─────────────────────────────────────────────────────

// AgentWorkersHandler handles agent worker discovery.
type AgentWorkersHandler struct {
	AgentStore repository.AgentStoreIface
}

// NewAgentWorkersHandler creates a new AgentWorkersHandler.
func NewAgentWorkersHandler(ag repository.AgentStoreIface) *AgentWorkersHandler {
	return &AgentWorkersHandler{
		AgentStore: ag,
	}
}

func (h *AgentWorkersHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *AgentWorkersHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// GetAgentWorkers handles GET /agents/{agentId}/workers.
func (h *AgentWorkersHandler) GetAgentWorkers(w http.ResponseWriter, r *http.Request) {
	id := extractAgentIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "agent id required")
		return
	}

	_, err := h.AgentStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "agent not found")
		return
	}

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"workers":  []interface{}{},
		"total":    0,
		"has_more": false,
	})
}

// ─── StackHealthHandler ──────────────────────────────────────────────────────

// StackHealthHandler handles stack health status endpoints.
type StackHealthHandler struct {
	HealthStore repository.StackHealthStoreIface
}

// NewStackHealthHandler creates a new StackHealthHandler.
func NewStackHealthHandler(hs repository.StackHealthStoreIface) *StackHealthHandler {
	return &StackHealthHandler{
		HealthStore: hs,
	}
}

func (h *StackHealthHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *StackHealthHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// GetStackHealth handles GET /stacks/health.
func (h *StackHealthHandler) GetStackHealth(w http.ResponseWriter, r *http.Request) {
	entry := h.HealthStore.GetLatest()
	if entry == nil {
		tenantID := middleware.TenantIDFromContext(r.Context())
		entry = &store.StackHealthEntry{
			TenantID: tenantID,
			Stacks: map[string]*store.StackHealthModule{
				"langgraph": {Status: store.StackUnknown},
				"temporal":  {Status: store.StackUnknown},
				"ray":       {Status: store.StackUnknown},
				"celery":    {Status: store.StackUnknown},
			},
			At: time.Now().UTC(),
		}
	}

	h.WriteJSON(w, http.StatusOK, entry)
}

// ─── Multi-stack: LangGraph ──────────────────────────────────────────────────

// ListLangGraphs handles GET /stacks/langgraph.
func (h *StackHealthHandler) ListLangGraphs(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	entries := h.HealthStore.ListByStack(tenantID, "langgraph")
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"stacks":   entries,
		"total":    len(entries),
		"has_more": false,
	})
}

// CreateLangGraph handles POST /stacks/langgraph.
func (h *StackHealthHandler) CreateLangGraph(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		Name      string                 `json:"name"`
		Config    map[string]interface{} `json:"config,omitempty"`
		GraphDef  map[string]interface{} `json:"graph_def"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Name == "" || req.GraphDef == nil {
		h.WriteError(w, http.StatusBadRequest, 400, "name and graph_def are required")
		return
	}

	entry := &store.StackHealthEntry{
		ID:         generateID(),
		TenantID:   tenantID,
		StackType:  "langgraph",
		StackName:  req.Name,
		Config:     req.Config,
		GraphDef:   req.GraphDef,
		Status:     store.StackHealthy,
		At:         time.Now().UTC(),
	}
	h.HealthStore.Create(entry)
	h.WriteJSON(w, http.StatusCreated, entry)
}

// GetLangGraph handles GET /stacks/langgraph/{id}.
func (h *StackHealthHandler) GetLangGraph(w http.ResponseWriter, r *http.Request, id string) {
	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "langgraph not found")
		return
	}
	h.WriteJSON(w, http.StatusOK, entry)
}

// UpdateLangGraph handles PUT /stacks/langgraph/{id}.
func (h *StackHealthHandler) UpdateLangGraph(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Status     *store.StackHealthStatus `json:"status,omitempty"`
		Config     map[string]interface{}   `json:"config,omitempty"`
		Metadata   map[string]interface{}   `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "langgraph not found")
		return
	}
	if req.Status != nil {
		entry.Status = *req.Status
	}
	if req.Config != nil {
		entry.Config = req.Config
	}
	if req.Metadata != nil {
		if entry.Metadata == nil {
			entry.Metadata = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			entry.Metadata[k] = v
		}
	}
	entry.At = time.Now().UTC()
	h.HealthStore.Update(entry)
	h.WriteJSON(w, http.StatusOK, entry)
}

// DeleteLangGraph handles DELETE /stacks/langgraph/{id}.
func (h *StackHealthHandler) DeleteLangGraph(w http.ResponseWriter, r *http.Request, id string) {
	if !h.HealthStore.Delete(id) {
		h.WriteError(w, http.StatusNotFound, 404, "langgraph not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Multi-stack: Temporal ───────────────────────────────────────────────────

// ListTemporalWorkflows handles GET /stacks/temporal.
func (h *StackHealthHandler) ListTemporalWorkflows(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	entries := h.HealthStore.ListByStack(tenantID, "temporal")
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"stacks":   entries,
		"total":    len(entries),
		"has_more": false,
	})
}

// CreateTemporalWorkflow handles POST /stacks/temporal.
func (h *StackHealthHandler) CreateTemporalWorkflow(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		Name         string                 `json:"name"`
		WorkflowType string                 `json:"workflow_type"`
		Config       map[string]interface{} `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Name == "" || req.WorkflowType == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "name and workflow_type are required")
		return
	}

	entry := &store.StackHealthEntry{
		ID:          generateID(),
		TenantID:    tenantID,
		StackType:   "temporal",
		StackName:   req.Name,
		Config:      req.Config,
		Status:      store.StackHealthy,
		At:          time.Now().UTC(),
	}
	h.HealthStore.Create(entry)
	h.WriteJSON(w, http.StatusCreated, entry)
}

// GetTemporalWorkflow handles GET /stacks/temporal/{id}.
func (h *StackHealthHandler) GetTemporalWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "temporal workflow not found")
		return
	}
	h.WriteJSON(w, http.StatusOK, entry)
}

// UpdateTemporalWorkflow handles PUT /stacks/temporal/{id}.
func (h *StackHealthHandler) UpdateTemporalWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Status *store.StackHealthStatus `json:"status,omitempty"`
		Config map[string]interface{}   `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "temporal workflow not found")
		return
	}
	if req.Status != nil {
		entry.Status = *req.Status
	}
	if req.Config != nil {
		entry.Config = req.Config
	}
	entry.At = time.Now().UTC()
	h.HealthStore.Update(entry)
	h.WriteJSON(w, http.StatusOK, entry)
}

// DeleteTemporalWorkflow handles DELETE /stacks/temporal/{id}.
func (h *StackHealthHandler) DeleteTemporalWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	if !h.HealthStore.Delete(id) {
		h.WriteError(w, http.StatusNotFound, 404, "temporal workflow not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTemporalCheckpoints handles GET /stacks/temporal/{id}/checkpoints.
func (h *StackHealthHandler) ListTemporalCheckpoints(w http.ResponseWriter, r *http.Request, id string) {
	entries := h.HealthStore.ListByStack(middleware.TenantIDFromContext(r.Context()), "temporal")
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"checkpoints": entries,
		"total":       len(entries),
		"has_more":    false,
	})
}

// ─── Multi-stack: Ray ────────────────────────────────────────────────────────

// ListRayPools handles GET /stacks/ray.
func (h *StackHealthHandler) ListRayPools(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	entries := h.HealthStore.ListByStack(tenantID, "ray")
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"stacks":   entries,
		"total":    len(entries),
		"has_more": false,
	})
}

// CreateRayPool handles POST /stacks/ray.
func (h *StackHealthHandler) CreateRayPool(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		Name     string                 `json:"name"`
		Capacity *int                   `json:"capacity,omitempty"`
		Config   map[string]interface{} `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Name == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "name is required")
		return
	}

	entry := &store.StackHealthEntry{
		ID:         generateID(),
		TenantID:   tenantID,
		StackType:  "ray",
		StackName:  req.Name,
		Config:     req.Config,
		Metadata:   map[string]interface{}{"capacity": req.Capacity},
		Status:     store.StackHealthy,
		At:         time.Now().UTC(),
	}
	h.HealthStore.Create(entry)
	h.WriteJSON(w, http.StatusCreated, entry)
}

// GetRayPool handles GET /stacks/ray/{id}.
func (h *StackHealthHandler) GetRayPool(w http.ResponseWriter, r *http.Request, id string) {
	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "ray pool not found")
		return
	}
	h.WriteJSON(w, http.StatusOK, entry)
}

// DeleteRayPool handles DELETE /stacks/ray/{id}.
func (h *StackHealthHandler) DeleteRayPool(w http.ResponseWriter, r *http.Request, id string) {
	if !h.HealthStore.Delete(id) {
		h.WriteError(w, http.StatusNotFound, 404, "ray pool not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ScaleRayPool handles POST /stacks/ray/{id}/scale.
func (h *StackHealthHandler) ScaleRayPool(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Capacity int `json:"capacity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Capacity <= 0 {
		h.WriteError(w, http.StatusBadRequest, 400, "valid capacity is required")
		return
	}

	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "ray pool not found")
		return
	}
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]interface{})
	}
	entry.Metadata["capacity"] = req.Capacity
	entry.At = time.Now().UTC()
	h.HealthStore.Update(entry)
	h.WriteJSON(w, http.StatusOK, entry)
}

// ─── Multi-stack: Celery ─────────────────────────────────────────────────────

// ListCeleryQueues handles GET /stacks/celery.
func (h *StackHealthHandler) ListCeleryQueues(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	entries := h.HealthStore.ListByStack(tenantID, "celery")
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"stacks":   entries,
		"total":    len(entries),
		"has_more": false,
	})
}

// CreateCeleryQueue handles POST /stacks/celery.
func (h *StackHealthHandler) CreateCeleryQueue(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		Name     string                 `json:"name"`
		Backend  string                 `json:"backend"`
		Config   map[string]interface{} `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Name == "" || req.Backend == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "name and backend are required")
		return
	}

	entry := &store.StackHealthEntry{
		ID:         generateID(),
		TenantID:   tenantID,
		StackType:  "celery",
		StackName:  req.Name,
		Config:     req.Config,
		Status:     store.StackHealthy,
		At:         time.Now().UTC(),
	}
	h.HealthStore.Create(entry)
	h.WriteJSON(w, http.StatusCreated, entry)
}

// GetCeleryQueue handles GET /stacks/celery/{id}.
func (h *StackHealthHandler) GetCeleryQueue(w http.ResponseWriter, r *http.Request, id string) {
	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "celery queue not found")
		return
	}
	h.WriteJSON(w, http.StatusOK, entry)
}

// UpdateCeleryQueue handles PUT /stacks/celery/{id}.
func (h *StackHealthHandler) UpdateCeleryQueue(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Status *store.StackHealthStatus `json:"status,omitempty"`
		Config map[string]interface{}   `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	entry, ok := h.HealthStore.GetByID(id)
	if !ok {
		h.WriteError(w, http.StatusNotFound, 404, "celery queue not found")
		return
	}
	if req.Status != nil {
		entry.Status = *req.Status
	}
	if req.Config != nil {
		entry.Config = req.Config
	}
	entry.At = time.Now().UTC()
	h.HealthStore.Update(entry)
	h.WriteJSON(w, http.StatusOK, entry)
}

// DeleteCeleryQueue handles DELETE /stacks/celery/{id}.
func (h *StackHealthHandler) DeleteCeleryQueue(w http.ResponseWriter, r *http.Request, id string) {
	if !h.HealthStore.Delete(id) {
		h.WriteError(w, http.StatusNotFound, 404, "celery queue not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListCeleryConsumers handles GET /stacks/celery/{id}/consumers.
func (h *StackHealthHandler) ListCeleryConsumers(w http.ResponseWriter, r *http.Request, queueID string) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	entries := h.HealthStore.ListByStack(tenantID, "celery-consumer")
	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"consumers": entries,
		"total":     len(entries),
		"has_more":  false,
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// ListAgents is a standalone handler for GET /agents.
func ListAgents(ag repository.AgentStoreIface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.TenantIDFromContext(r.Context())
		availabilities := ag.ListByTenant(tenantID)
		handlers := make([]interface{}, 0, len(availabilities))
		for _, a := range availabilities {
			handlers = append(handlers, a)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"agents":   handlers,
			"total":    len(handlers),
			"has_more": false,
		})
	}
}

// writeJSON is a helper to write JSON responses.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func extractNodeIDFromPath(path string) string {
	const prefix = "/api/v1/orchestration/workflows/"
	s := strings.TrimPrefix(path, prefix)
	// s = {workflowId}/nodes/{nodeId}/retry
	idx1 := strings.Index(s, "/")
	if idx1 == -1 {
		return ""
	}
	s = s[idx1+1:] // {nodeId}/retry
	idx2 := strings.Index(s, "/")
	if idx2 == -1 {
		return s
	}
	return s[:idx2]
}

func extractAgentIDFromPath(path string) string {
	const prefix = "/api/v1/orchestration/agents/"
	s := strings.TrimPrefix(path, prefix)
	idx := strings.Index(s, "/")
	if idx == -1 {
		return s
	}
	return s[:idx]
}
