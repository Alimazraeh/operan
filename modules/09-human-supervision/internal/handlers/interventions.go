package handlers

import (
	"net/http"

	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

type createInterventionRequest struct {
	Action           string                  `json:"action"`
	TargetAgentID    string                  `json:"target_agent_id"`
	TargetWorkflowID string                  `json:"target_workflow_id"`
	Reason           string                  `json:"reason"`
	Scope            *store.InterventionScope `json:"scope"`
	DurationMinutes  int                     `json:"duration_minutes"`
	IssuedBy         string                  `json:"issued_by"`
	Metadata         map[string]interface{}  `json:"metadata"`
}

// CreateIntervention handles POST /interventions.
func (h *SupervisionHandlers) CreateIntervention(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req createInterventionRequest
	if !decode(w, r, &req) {
		return
	}
	if req.TargetAgentID == "" || req.Reason == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "target_agent_id and reason are required")
		return
	}
	if !store.ValidInterventionAction(req.Action) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "action must be one of: pause, stop, restrict, override, redirect, suspend")
		return
	}
	if req.DurationMinutes < 0 || req.DurationMinutes > 10080 {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "duration_minutes must be between 1 and 10080")
		return
	}

	issuedBy := req.IssuedBy
	if issuedBy == "" {
		issuedBy = middleware.UserIDFromContext(r.Context())
	}
	iv, err := h.Interventions.Create(&store.Intervention{
		TenantID:         tenantID,
		Action:           req.Action,
		TargetAgentID:    req.TargetAgentID,
		TargetWorkflowID: req.TargetWorkflowID,
		Reason:           req.Reason,
		Scope:            req.Scope,
		DurationMinutes:  req.DurationMinutes,
		IssuedBy:         issuedBy,
		Metadata:         req.Metadata,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, iv)
}

// GetIntervention handles GET /interventions/{id}.
func (h *SupervisionHandlers) GetIntervention(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	iv, err := h.Interventions.Get(r.PathValue("id"), tenantID)
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, iv)
}

type updateInterventionRequest struct {
	Reason          *string                  `json:"reason"`
	DurationMinutes *int                     `json:"duration_minutes"`
	Scope           *store.InterventionScope `json:"scope"`
}

// UpdateIntervention handles PATCH /interventions/{id}.
func (h *SupervisionHandlers) UpdateIntervention(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req updateInterventionRequest
	if !decode(w, r, &req) {
		return
	}
	if req.DurationMinutes != nil && (*req.DurationMinutes < 1 || *req.DurationMinutes > 10080) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "duration_minutes must be between 1 and 10080")
		return
	}
	if req.Scope != nil && !store.ValidScopeType(req.Scope.Type) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "scope.type is not a valid enum value")
		return
	}

	iv, err := h.Interventions.Update(r.PathValue("id"), tenantID, func(iv *store.Intervention) {
		if req.Reason != nil {
			iv.Reason = *req.Reason
		}
		if req.DurationMinutes != nil {
			iv.DurationMinutes = *req.DurationMinutes
		}
		if req.Scope != nil {
			iv.Scope = req.Scope
		}
	})
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, iv)
}

// RevokeIntervention handles POST /interventions/{id}/revoke.
func (h *SupervisionHandlers) RevokeIntervention(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	iv, err := h.Interventions.Revoke(r.PathValue("id"), tenantID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, iv)
}

// DeleteIntervention handles DELETE /interventions/{id}.
func (h *SupervisionHandlers) DeleteIntervention(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if err := h.Interventions.Delete(r.PathValue("id"), tenantID); err != nil {
		storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
