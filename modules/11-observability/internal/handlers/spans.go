package handlers

import (
	"net/http"

	"github.com/operan/modules/11-observability/internal/middleware"
	"github.com/operan/modules/11-observability/internal/store"
)

// ListSpans handles GET /spans.
func (h *ObservabilityHandlers) ListSpans(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if !tenantOK(w, r, r.URL.Query().Get("tenant_id"), tenantID) {
		return
	}
	page, pageSize := h.pagination(r)

	if v := queryPtr(r, "span_type"); v != nil && !store.ValidSpanType(*v) {
		writeError(w, r, http.StatusBadRequest, "invalid span_type filter")
		return
	}
	if v := queryPtr(r, "status"); v != nil && !store.ValidSpanStatus(*v) {
		writeError(w, r, http.StatusBadRequest, "invalid status filter")
		return
	}

	items, total, hasMore := h.Spans.List(tenantID, page, pageSize, store.SpanFilter{
		TraceID:    queryPtr(r, "trace_id"),
		SpanType:   queryPtr(r, "span_type"),
		WorkflowID: queryPtr(r, "workflow_id"),
		AgentID:    queryPtr(r, "agent_id"),
		Status:     queryPtr(r, "status"),
	})
	if items == nil {
		items = []store.TraceSpan{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  hasMore,
	})
}

// GetTrace handles GET /traces/{id}.
func (h *ObservabilityHandlers) GetTrace(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	traceID := r.PathValue("id")

	spans, totalDuration, err := h.Spans.Trace(traceID, tenantID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "trace not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"trace_id":          traceID,
		"tenant_id":         tenantID,
		"spans":             spans,
		"total_duration_ms": totalDuration,
	})
}
