package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/09-human-supervision/internal/events"
	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

type createEscalationRequest struct {
	Severity          string                 `json:"severity"`
	Category          string                 `json:"category"`
	Title             string                 `json:"title"`
	Description       string                 `json:"description"`
	RelatedApprovalID string                 `json:"related_approval_id"`
	SourceAgentID     string                 `json:"source_agent_id"`
	ImpactScope       string                 `json:"impact_scope"`
	RequestedAction   string                 `json:"requested_action"`
	AssignedTo        string                 `json:"assigned_to"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// CreateEscalation handles POST /escalations.
func (h *SupervisionHandlers) CreateEscalation(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req createEscalationRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Title == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "title is required")
		return
	}
	if !store.ValidEscalationSeverity(req.Severity) || !store.ValidEscalationCategory(req.Category) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "severity or category is not a valid enum value")
		return
	}

	e, err := h.Escalations.Create(&store.Escalation{
		TenantID:          tenantID,
		Severity:          req.Severity,
		Category:          req.Category,
		Title:             req.Title,
		Description:       req.Description,
		RelatedApprovalID: req.RelatedApprovalID,
		SourceAgentID:     req.SourceAgentID,
		ImpactScope:       req.ImpactScope,
		RequestedAction:   req.RequestedAction,
		AssignedTo:        req.AssignedTo,
		Metadata:          req.Metadata,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}

	// Security/compliance escalations from an identified agent are surfaced
	// to the platform as policy violations.
	if (e.Category == "security" || e.Category == "compliance") && e.SourceAgentID != "" {
		h.Publisher.PublishPolicyViolationDetected(events.PolicyViolationDetectedPayload{
			ViolationID:   uuid.New().String(),
			TenantID:      tenantID,
			AgentID:       e.SourceAgentID,
			WorkflowID:    metaString(e.Metadata, "workflow_id"),
			PolicyID:      metaString(e.Metadata, "policy_id"),
			ViolationType: e.Category,
			Severity:      e.Severity,
			Details:       map[string]interface{}{"escalation_id": e.ID, "title": e.Title},
			DetectedAt:    e.CreatedAt.Format(time.RFC3339),
		}, middleware.TraceIDFromContext(r.Context()))
	}

	writeJSON(w, http.StatusCreated, e)
}

// GetEscalation handles GET /escalations/{id}.
func (h *SupervisionHandlers) GetEscalation(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	e, err := h.Escalations.Get(r.PathValue("id"), tenantID)
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

type updateEscalationRequest struct {
	Severity    *string                `json:"severity"`
	Category    *string                `json:"category"`
	Title       *string                `json:"title"`
	Description *string                `json:"description"`
	AssignedTo  *string                `json:"assigned_to"`
	Status      *string                `json:"status"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UpdateEscalation handles PATCH /escalations/{id}.
func (h *SupervisionHandlers) UpdateEscalation(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req updateEscalationRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Severity != nil && !store.ValidEscalationSeverity(*req.Severity) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "severity must be one of: low, medium, high, critical, p0")
		return
	}
	if req.Category != nil && !store.ValidEscalationCategory(*req.Category) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "category is not a valid enum value")
		return
	}
	if req.Status != nil && !store.ValidEscalationStatus(*req.Status) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "status is not a valid enum value")
		return
	}

	e, err := h.Escalations.Update(r.PathValue("id"), tenantID, func(e *store.Escalation) {
		if req.Severity != nil {
			e.Severity = *req.Severity
		}
		if req.Category != nil {
			e.Category = *req.Category
		}
		if req.Title != nil {
			e.Title = *req.Title
		}
		if req.Description != nil {
			e.Description = *req.Description
		}
		if req.AssignedTo != nil {
			e.AssignedTo = *req.AssignedTo
		}
		if req.Status != nil {
			e.Status = *req.Status
		}
		if req.Metadata != nil {
			e.Metadata = req.Metadata
		}
	})
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

type resolveEscalationRequest struct {
	ResolverID      string                 `json:"resolver_id"`
	ResolutionNotes string                 `json:"resolution_notes"`
	ResolutionType  string                 `json:"resolution_type"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// ResolveEscalation handles POST /escalations/{id}/resolve.
func (h *SupervisionHandlers) ResolveEscalation(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req resolveEscalationRequest
	if !decode(w, r, &req) {
		return
	}
	if req.ResolverID == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "resolver_id is required")
		return
	}
	if req.ResolutionType != "" && !store.ValidResolutionType(req.ResolutionType) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "resolution_type is not a valid enum value")
		return
	}

	e, err := h.Escalations.Resolve(r.PathValue("id"), tenantID, req.ResolverID, req.ResolutionNotes, req.ResolutionType)
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// DeleteEscalation handles DELETE /escalations/{id}.
func (h *SupervisionHandlers) DeleteEscalation(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if err := h.Escalations.Delete(r.PathValue("id"), tenantID); err != nil {
		storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
