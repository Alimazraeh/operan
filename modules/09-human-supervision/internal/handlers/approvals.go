package handlers

import (
	"net/http"
	"time"

	"github.com/operan/modules/09-human-supervision/internal/events"
	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

type createApprovalRequest struct {
	RequestID         string                  `json:"request_id"`
	RequesterID       string                  `json:"requester_id"`
	Type              string                  `json:"type"`
	Title             string                  `json:"title"`
	Description       string                  `json:"description"`
	Metadata          map[string]interface{}  `json:"metadata"`
	RequiredApprovers []store.ApprovalTarget  `json:"required_approvers"`
	ExpiresAt         *time.Time              `json:"expires_at"`
	ConditionalConfig *store.ConditionalConfig `json:"conditional_config"`
	ThresholdConfig   *store.ThresholdConfig   `json:"threshold_config"`
}

// CreateApproval handles POST /approvals.
func (h *SupervisionHandlers) CreateApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req createApprovalRequest
	if !decode(w, r, &req) {
		return
	}
	if req.RequestID == "" || req.RequesterID == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "request_id and requester_id are required")
		return
	}
	if !store.ValidApprovalType(req.Type) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "type must be one of: sequential, parallel, conditional, threshold")
		return
	}

	a, err := h.Approvals.Create(&store.Approval{
		TenantID:          tenantID,
		RequestID:         req.RequestID,
		RequesterID:       req.RequesterID,
		Type:              req.Type,
		Title:             req.Title,
		Description:       req.Description,
		Metadata:          req.Metadata,
		RequiredApprovers: req.RequiredApprovers,
		ExpiresAt:         req.ExpiresAt,
		ConditionalConfig: req.ConditionalConfig,
		ThresholdConfig:   req.ThresholdConfig,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}

	var approver *string
	if len(req.RequiredApprovers) > 0 && req.RequiredApprovers[0].UserID != "" {
		approver = &req.RequiredApprovers[0].UserID
	}
	h.Publisher.PublishGateRaised(events.GateRaisedPayload{
		GateID:          a.ID,
		TenantID:        tenantID,
		WorkflowID:      a.RequestID,
		NodeID:          metaString(a.Metadata, "node_id"),
		GateType:        "human_approval",
		HumanApproverID: approver,
		RaisedBy:        a.RequesterID,
		RaisedAt:        a.CreatedAt.Format(time.RFC3339),
		Priority:        "medium",
	}, middleware.TraceIDFromContext(r.Context()))

	writeJSON(w, http.StatusCreated, a)
}

// GetApproval handles GET /approvals/{id}.
func (h *SupervisionHandlers) GetApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	a, justExpired, err := h.Approvals.Get(r.PathValue("id"), tenantID)
	if err != nil {
		storeError(w, r, err)
		return
	}
	if justExpired {
		h.publishTimeout(a, r)
	}
	writeJSON(w, http.StatusOK, a)
}

type updateApprovalRequest struct {
	Title             *string                `json:"title"`
	Description       *string                `json:"description"`
	Metadata          map[string]interface{} `json:"metadata"`
	ExpiresAt         *time.Time             `json:"expires_at"`
	RequiredApprovers []store.ApprovalTarget `json:"required_approvers"`
}

// UpdateApproval handles PATCH /approvals/{id}.
func (h *SupervisionHandlers) UpdateApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req updateApprovalRequest
	if !decode(w, r, &req) {
		return
	}
	a, err := h.Approvals.Update(r.PathValue("id"), tenantID, func(a *store.Approval) {
		if req.Title != nil {
			a.Title = *req.Title
		}
		if req.Description != nil {
			a.Description = *req.Description
		}
		if req.Metadata != nil {
			a.Metadata = req.Metadata
		}
		if req.ExpiresAt != nil {
			a.ExpiresAt = req.ExpiresAt
		}
		if req.RequiredApprovers != nil {
			a.RequiredApprovers = req.RequiredApprovers
		}
	})
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// DeleteApproval handles DELETE /approvals/{id}.
func (h *SupervisionHandlers) DeleteApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if err := h.Approvals.Delete(r.PathValue("id"), tenantID); err != nil {
		storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type approveRequest struct {
	ApproverID string                 `json:"approver_id"`
	Comment    string                 `json:"comment"`
	Conditions []string               `json:"conditions"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ApproveApproval handles POST /approvals/{id}/approve.
func (h *SupervisionHandlers) ApproveApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req approveRequest
	if !decode(w, r, &req) {
		return
	}
	if req.ApproverID == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "approver_id is required")
		return
	}

	a, err := h.Approvals.Approve(r.PathValue("id"), tenantID, store.ApprovalAction{
		ActorID:    req.ApproverID,
		Comment:    req.Comment,
		Conditions: req.Conditions,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}
	h.publishResponse(a, "approve", req.ApproverID, req.Comment, r)
	writeJSON(w, http.StatusOK, a)
}

type rejectRequest struct {
	RejectorID string `json:"rejector_id"`
	Reason     string `json:"reason"`
	Comment    string `json:"comment"`
}

// RejectApproval handles POST /approvals/{id}/reject.
func (h *SupervisionHandlers) RejectApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req rejectRequest
	if !decode(w, r, &req) {
		return
	}
	if req.RejectorID == "" || req.Reason == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "rejector_id and reason are required")
		return
	}

	comment := req.Reason
	if req.Comment != "" {
		comment += ": " + req.Comment
	}
	a, err := h.Approvals.Reject(r.PathValue("id"), tenantID, store.ApprovalAction{
		ActorID: req.RejectorID,
		Comment: comment,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}
	h.publishResponse(a, "reject", req.RejectorID, comment, r)
	writeJSON(w, http.StatusOK, a)
}

type delegateRequest struct {
	DelegatorID   string `json:"delegator_id"`
	NewApproverID string `json:"new_approver_id"`
	Reason        string `json:"reason"`
}

// DelegateApproval handles POST /approvals/{id}/delegate.
func (h *SupervisionHandlers) DelegateApproval(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req delegateRequest
	if !decode(w, r, &req) {
		return
	}
	if req.DelegatorID == "" || req.NewApproverID == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "delegator_id and new_approver_id are required")
		return
	}

	a, err := h.Approvals.Delegate(r.PathValue("id"), tenantID, store.ApprovalDelegate{
		FromUserID: req.DelegatorID,
		ToUserID:   req.NewApproverID,
		Reason:     req.Reason,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}

	h.Publisher.PublishGateEscalated(events.GateEscalatedPayload{
		GateID:             a.ID,
		TenantID:           tenantID,
		PreviousApproverID: req.DelegatorID,
		NewApproverID:      req.NewApproverID,
		EscalationReason:   req.Reason,
		EscalatedAt:        time.Now().UTC().Format(time.RFC3339),
		EscalationLevel:    len(a.Delegates) - 1,
	}, middleware.TraceIDFromContext(r.Context()))

	writeJSON(w, http.StatusOK, a)
}

// publishResponse emits GateResponded after an approve/reject decision.
func (h *SupervisionHandlers) publishResponse(a *store.Approval, response, by, comment string, r *http.Request) {
	var comments *string
	if comment != "" {
		comments = &comment
	}
	h.Publisher.PublishGateResponded(events.GateRespondedPayload{
		ResponseID: a.ID + "-" + response,
		GateID:     a.ID,
		TenantID:   a.TenantID,
		Response:   response,
		ResponseBy: by,
		ResponseAt: time.Now().UTC().Format(time.RFC3339),
		Comments:   comments,
	}, middleware.TraceIDFromContext(r.Context()))
}

// publishTimeout emits GateTimeout after a lazily-detected expiry.
func (h *SupervisionHandlers) publishTimeout(a *store.Approval, r *http.Request) {
	deadline := ""
	if a.ExpiresAt != nil {
		deadline = a.ExpiresAt.Format(time.RFC3339)
	}
	h.Publisher.PublishGateTimeout(events.GateTimeoutPayload{
		GateID:           a.ID,
		TenantID:         a.TenantID,
		TimeoutAction:    "expired",
		TimedOutAt:       time.Now().UTC().Format(time.RFC3339),
		OriginalDeadline: deadline,
	}, middleware.TraceIDFromContext(r.Context()))
}

func metaString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
