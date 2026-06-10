// Package handlers implements the HTTP handlers for Module 11 (Observability).
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/operan/modules/11-observability/internal/events"
	"github.com/operan/modules/11-observability/internal/middleware"
	"github.com/operan/modules/11-observability/internal/store"
)

// ObservabilityHandlers bundles the stores and publisher used by all endpoints.
type ObservabilityHandlers struct {
	Metrics     *store.MetricStore
	Spans       *store.SpanStore
	Alerts      *store.AlertStore
	Health      *store.HealthStore
	Publisher   *events.Publisher
	MaxPageSize int
}

// NewObservabilityHandlers constructs an ObservabilityHandlers.
func NewObservabilityHandlers(m *store.MetricStore, s *store.SpanStore, a *store.AlertStore, h *store.HealthStore, p *events.Publisher, maxPageSize int) *ObservabilityHandlers {
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	return &ObservabilityHandlers{Metrics: m, Spans: s, Alerts: a, Health: h, Publisher: p, MaxPageSize: maxPageSize}
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError emits the contract Error schema:
// { code: int, message: string, details: string, request_id: uuid }.
func writeError(w http.ResponseWriter, r *http.Request, status int, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":       status,
		"message":    http.StatusText(status),
		"details":    details,
		"request_id": middleware.RequestIDFromContext(r.Context()),
	})
}

// pagination parses page/page_size query params, clamped to [1, max].
func (h *ObservabilityHandlers) pagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > h.MaxPageSize {
		pageSize = h.MaxPageSize
	}
	return page, pageSize
}

func queryPtr(r *http.Request, key string) *string {
	if v := r.URL.Query().Get(key); v != "" {
		return &v
	}
	return nil
}

// tenantOK verifies an optional tenant_id query/body value matches the
// authenticated tenant. Empty values pass (context tenant is authoritative).
func tenantOK(w http.ResponseWriter, r *http.Request, claimed, actual string) bool {
	if claimed != "" && claimed != actual {
		writeError(w, r, http.StatusForbidden, "tenant_id does not match authenticated tenant")
		return false
	}
	return true
}
