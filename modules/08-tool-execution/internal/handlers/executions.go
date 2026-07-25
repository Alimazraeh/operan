package handlers

import (
	"net/http"

	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// ExecuteTool used to be the in-process "executor": it echoed its input back
// as output and stamped the record completed, which is how 40% of catalogue
// steps came to "execute" as a note string. It is gone, and it answers 410
// rather than 404 so a caller learns it was removed on purpose: capability
// execution now has exactly one door — POST /invoke — where input is
// validated, policy and authority are checked, and the outcome is recorded
// truthfully, refusals included.
func (h *ToolHandlers) ExecuteTool(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusGone,
		"the echo executor has been removed — capabilities are performed via POST /invoke, the governed path")
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

// RetryExecution retried through the echo executor, which no longer exists.
// Same 410 as ExecuteTool: the governed path does not "retry" a record — a new
// invocation through POST /invoke is a new, fully checked attempt.
func (h *ToolHandlers) RetryExecution(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusGone,
		"the echo executor has been removed — perform the capability again via POST /invoke")
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
