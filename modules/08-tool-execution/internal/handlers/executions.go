package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/08-tool-execution/internal/events"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// toolExecuteRequest is the body for POST /tools/execute.
type toolExecuteRequest struct {
	TenantID    string                 `json:"tenant_id"`
	AgentID     string                 `json:"agent_id"`
	Tool        string                 `json:"tool"`
	ToolVersion string                 `json:"tool_version"`
	Input       map[string]interface{} `json:"input"`
	TimeoutMS   int                    `json:"timeout_ms"`
	Priority    int                    `json:"priority"`
}

// ExecuteTool handles POST /tools/execute. It records the execution, runs the
// tool synchronously (the in-process executor echoes input and applies the
// tool's configured cost), and returns the completed record.
func (h *ToolHandlers) ExecuteTool(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req toolExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TenantID != "" && req.TenantID != tenantID {
		writeError(w, r, http.StatusConflict, "tenant_id does not match authenticated tenant")
		return
	}
	if req.AgentID == "" || req.Tool == "" {
		writeError(w, r, http.StatusBadRequest, "agent_id and tool are required")
		return
	}

	// Resolve the tool by name within the tenant to validate it exists and is active.
	tool := h.findToolByName(tenantID, req.Tool)
	if tool == nil {
		writeError(w, r, http.StatusNotFound, "tool not found for tenant")
		return
	}
	if tool.Status == "disabled" {
		writeError(w, r, http.StatusConflict, "tool is disabled")
		return
	}

	exec, err := h.Executions.Create(&store.ToolExecution{
		TenantID:    tenantID,
		AgentID:     req.AgentID,
		Tool:        req.Tool,
		ToolVersion: orDefault(req.ToolVersion, tool.Version),
		Input:       req.Input,
		Status:      store.ExecQueued,
	})
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	h.publishExec(h.Publisher.PublishExecutionRequested, exec, tool)

	// Run synchronously (in-process executor).
	completed := h.runExecution(tenantID, exec.ID, tool)
	writeJSON(w, http.StatusCreated, completed)
}

// runExecution performs the in-process tool run, updating status and emitting
// started/completed events. Returns the final record.
func (h *ToolHandlers) runExecution(tenantID, execID string, tool *store.Tool) *store.ToolExecution {
	started := time.Now()

	running, _ := h.Executions.Update(execID, tenantID, func(e *store.ToolExecution) {
		e.Status = store.ExecRunning
	})
	if running != nil {
		h.publishExec(h.Publisher.PublishExecutionStarted, running, tool)
	}

	// The in-process executor echoes the input as output. A real deployment
	// would dispatch to an external runtime (Module 16 sandbox).
	final, _ := h.Executions.Update(execID, tenantID, func(e *store.ToolExecution) {
		e.Status = store.ExecCompleted
		e.Output = map[string]interface{}{"echo": e.Input, "tool": e.Tool}
		e.ExecutionTimeMS = int(time.Since(started).Milliseconds())
		if tool.CostPerCall != nil {
			c := *tool.CostPerCall
			e.Cost = &c
		}
	})
	if final != nil {
		h.publishExec(h.Publisher.PublishExecutionCompleted, final, tool)
	}
	return final
}

// ListExecutions handles GET /tools/executions.
func (h *ToolHandlers) ListExecutions(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	page, pageSize := h.pagination(r)
	items, total, hasMore := h.Executions.List(tenantID, page, pageSize, queryPtr(r, "tool"), queryPtr(r, "status"))
	if items == nil {
		items = []store.ToolExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": pageSize, "has_more": hasMore,
	})
}

// GetExecution handles GET /tools/executions/{id}.
func (h *ToolHandlers) GetExecution(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	exec, err := h.Executions.GetByIDAndTenant(r.PathValue("id"), tenantID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "execution not found")
		return
	}
	writeJSON(w, http.StatusOK, exec)
}

// RetryExecution handles POST /tools/executions/{id}/retry. Only failed
// executions may be retried.
func (h *ToolHandlers) RetryExecution(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	id := r.PathValue("id")

	exec, err := h.Executions.GetByIDAndTenant(id, tenantID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "execution not found")
		return
	}
	if exec.Status != store.ExecFailed {
		writeError(w, r, http.StatusConflict, "only failed executions can be retried")
		return
	}

	tool := h.findToolByName(tenantID, exec.Tool)
	if tool == nil {
		writeError(w, r, http.StatusNotFound, "tool not found for tenant")
		return
	}

	if _, err := h.Executions.Update(id, tenantID, func(e *store.ToolExecution) {
		e.RetryCount++
		e.Status = store.ExecQueued
		e.ErrorCode = ""
		e.ErrorMessage = ""
	}); err != nil {
		writeError(w, r, http.StatusNotFound, "execution not found")
		return
	}

	completed := h.runExecution(tenantID, id, tool)
	writeJSON(w, http.StatusOK, completed)
}

// GetToolCost handles GET /tools/cost. Optional ?tool= scopes to one tool.
func (h *ToolHandlers) GetToolCost(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	sum := h.Executions.AggregateCost(tenantID, queryPtr(r, "tool"))

	avg := 0.0
	if sum.TotalCalls > 0 {
		avg = sum.TotalCost / float64(sum.TotalCalls)
	}
	resp := map[string]interface{}{
		"total_calls":       sum.TotalCalls,
		"total_cost":        map[string]interface{}{"amount": sum.TotalCost, "currency": sum.Currency},
		"avg_cost_per_call": map[string]interface{}{"amount": avg, "currency": sum.Currency},
	}
	if sum.Tool != "" {
		resp["tool"] = sum.Tool
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// findToolByName returns the first active-or-deprecated tool with the given
// name for a tenant, or nil.
func (h *ToolHandlers) findToolByName(tenantID, name string) *store.Tool {
	page := 1
	for {
		items, total, hasMore := h.Tools.List(tenantID, page, h.MaxPageSize, nil, nil)
		for i := range items {
			if items[i].Name == name {
				t := items[i]
				return &t
			}
		}
		if !hasMore || len(items) == 0 || page*h.MaxPageSize >= total {
			return nil
		}
		page++
	}
}

func (h *ToolHandlers) publishExec(fn func(events.ExecutionPayload) error, e *store.ToolExecution, tool *store.Tool) {
	if h.Publisher == nil || e == nil {
		return
	}
	_ = fn(events.ExecutionPayload{
		ExecutionID: e.ID, ToolID: tool.ID, Tool: e.Tool, ToolVersion: e.ToolVersion,
		AgentID: e.AgentID, TenantID: e.TenantID, Status: string(e.Status),
		Input: e.Input, Output: e.Output, ExecutionTimeMS: e.ExecutionTimeMS,
		ErrorCode: e.ErrorCode, ErrorMessage: e.ErrorMessage, RetryCount: e.RetryCount,
		Timestamp: time.Now().UTC(),
	})
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
