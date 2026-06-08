// Package handlers implements the HTTP handlers for Module 08 (Tool Execution).
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/operan/modules/08-tool-execution/internal/events"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// ToolHandlers bundles the stores and publisher used by all endpoints.
type ToolHandlers struct {
	Tools       *store.ToolStore
	Versions    *store.VersionStore
	Executions  *store.ExecutionStore
	Publisher   *events.Publisher
	MaxPageSize int
}

// NewToolHandlers constructs a ToolHandlers.
func NewToolHandlers(t *store.ToolStore, v *store.VersionStore, e *store.ExecutionStore, p *events.Publisher, maxPageSize int) *ToolHandlers {
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	return &ToolHandlers{Tools: t, Versions: v, Executions: e, Publisher: p, MaxPageSize: maxPageSize}
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":       "about:blank",
		"title":      http.StatusText(status),
		"status":     status,
		"detail":     detail,
		"instance":   r.URL.Path,
		"request_id": middleware.RequestIDFromContext(r.Context()),
	})
}

// pagination parses page/page_size query params, clamped to [1, max].
func (h *ToolHandlers) pagination(r *http.Request) (int, int) {
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
