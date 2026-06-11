// Package handlers implements the HTTP handlers for Module 09 (Human Supervision).
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/operan/modules/09-human-supervision/internal/events"
	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

// SupervisionHandlers bundles the stores and publisher used by all endpoints.
type SupervisionHandlers struct {
	Approvals     *store.ApprovalStore
	Escalations   *store.EscalationStore
	Interventions *store.InterventionStore
	Hitl          *store.HitlStore
	Publisher     *events.Publisher
	MaxPageSize   int
}

// NewSupervisionHandlers constructs a SupervisionHandlers.
func NewSupervisionHandlers(a *store.ApprovalStore, e *store.EscalationStore, i *store.InterventionStore, hl *store.HitlStore, p *events.Publisher, maxPageSize int) *SupervisionHandlers {
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	return &SupervisionHandlers{Approvals: a, Escalations: e, Interventions: i, Hitl: hl, Publisher: p, MaxPageSize: maxPageSize}
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// errorCode maps HTTP statuses to the contract's string error codes.
func errorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "UNPROCESSABLE_ENTITY"
	default:
		return "INTERNAL_ERROR"
	}
}

// writeError emits the module 09 contract ErrorResponse:
// { error: { code: string, message: string, details: [...], request_id } }.
func writeError(w http.ResponseWriter, r *http.Request, status int, message string, details ...string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]interface{}{
		"code":       errorCode(status),
		"message":    message,
		"request_id": middleware.RequestIDFromContext(r.Context()),
	}
	if len(details) > 0 {
		body["details"] = details
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": body})
}

// storeError translates store sentinel errors into contract responses.
func storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "Resource not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, r, http.StatusConflict, "Resource is in a state that prevents this operation")
	case errors.Is(err, store.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed")
	default:
		writeError(w, r, http.StatusBadRequest, err.Error())
	}
}

// pagination parses page/page_size query params, clamped to [1, max].
func (h *SupervisionHandlers) pagination(r *http.Request) (int, int) {
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

func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", err.Error())
		return false
	}
	return true
}
