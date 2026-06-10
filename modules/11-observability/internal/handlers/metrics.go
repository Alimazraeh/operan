package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/11-observability/internal/events"
	"github.com/operan/modules/11-observability/internal/middleware"
	"github.com/operan/modules/11-observability/internal/store"
)

// metricRecord matches the OpenAPI MetricRecord schema.
type metricRecord struct {
	TenantID    string                 `json:"tenant_id"`
	MetricName  string                 `json:"metric_name"`
	MetricValue float64                `json:"metric_value"`
	MetricType  string                 `json:"metric_type"`
	Labels      map[string]interface{} `json:"labels"`
	SourceID    string                 `json:"source_id"`
}

// RecordMetric handles POST /metrics.
func (h *ObservabilityHandlers) RecordMetric(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req metricRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if !tenantOK(w, r, req.TenantID, tenantID) {
		return
	}
	if req.MetricName == "" {
		writeError(w, r, http.StatusBadRequest, "metric_name is required")
		return
	}
	if !store.ValidMetricType(req.MetricType) {
		writeError(w, r, http.StatusBadRequest, "invalid metric_type")
		return
	}

	m, err := h.Metrics.Record(&store.Metric{
		TenantID:    tenantID,
		MetricName:  req.MetricName,
		MetricValue: req.MetricValue,
		MetricType:  store.MetricType(req.MetricType),
		Labels:      req.Labels,
		SourceID:    req.SourceID,
	})
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	h.Publisher.PublishMetricRecorded(events.MetricRecordedPayload{
		MetricID:    m.ID,
		TenantID:    m.TenantID,
		MetricName:  m.MetricName,
		MetricValue: m.MetricValue,
		MetricType:  string(m.MetricType),
		Labels:      m.Labels,
		SourceID:    m.SourceID,
		RecordedAt:  m.RecordedAt.Format(time.RFC3339),
	}, middleware.TraceIDFromContext(r.Context()))

	writeJSON(w, http.StatusCreated, m)
}

// ListMetrics handles GET /metrics.
func (h *ObservabilityHandlers) ListMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if !tenantOK(w, r, r.URL.Query().Get("tenant_id"), tenantID) {
		return
	}
	page, pageSize := h.pagination(r)

	if v := queryPtr(r, "metric_type"); v != nil && !store.ValidMetricType(*v) {
		writeError(w, r, http.StatusBadRequest, "invalid metric_type filter")
		return
	}

	filter := store.MetricFilter{
		MetricType: queryPtr(r, "metric_type"),
		MetricName: queryPtr(r, "metric_name"),
		SourceID:   queryPtr(r, "source_id"),
	}
	if v := r.URL.Query().Get("start"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "start must be RFC 3339")
			return
		}
		filter.Start = &t
	}
	if v := r.URL.Query().Get("end"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "end must be RFC 3339")
			return
		}
		filter.End = &t
	}

	items, total, hasMore := h.Metrics.List(tenantID, page, pageSize, filter)
	if items == nil {
		items = []store.Metric{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  hasMore,
	})
}
