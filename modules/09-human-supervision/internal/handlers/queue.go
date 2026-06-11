package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

// GetHumanQueue handles GET /queue — the merged review queue of pending
// approvals, open escalations, and active interventions.
func (h *SupervisionHandlers) GetHumanQueue(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	page, pageSize := h.pagination(r)
	typeFilter := r.URL.Query().Get("type")
	userFilter := r.URL.Query().Get("user_id")

	if typeFilter != "" && typeFilter != "approval" && typeFilter != "escalation" && typeFilter != "intervention" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "type must be one of: approval, escalation, intervention")
		return
	}

	var items []store.QueueItem
	if typeFilter == "" || typeFilter == "approval" {
		for _, a := range h.Approvals.Pending(tenantID) {
			assigned := firstApprover(a.RequiredApprovers)
			items = append(items, store.QueueItem{
				ItemID:     a.ID,
				ItemType:   "approval",
				Title:      a.Title,
				Priority:   "medium",
				Status:     a.Status,
				CreatedAt:  a.CreatedAt,
				DueAt:      a.ExpiresAt,
				AssignedTo: assigned,
			})
		}
	}
	if typeFilter == "" || typeFilter == "escalation" {
		for _, e := range h.Escalations.Open(tenantID) {
			items = append(items, store.QueueItem{
				ItemID:     e.ID,
				ItemType:   "escalation",
				Title:      e.Title,
				Priority:   escalationPriority(e.Severity),
				Status:     e.Status,
				CreatedAt:  e.CreatedAt,
				AssignedTo: e.AssignedTo,
			})
		}
	}
	if typeFilter == "" || typeFilter == "intervention" {
		for _, iv := range h.Interventions.Active(tenantID) {
			items = append(items, store.QueueItem{
				ItemID:     iv.ID,
				ItemType:   "intervention",
				Title:      iv.Action + " " + iv.TargetAgentID,
				Priority:   "high",
				Status:     iv.Status,
				CreatedAt:  iv.IssuedAt,
				DueAt:      iv.ExpiresAt,
				AssignedTo: iv.IssuedBy,
			})
		}
	}

	if userFilter != "" {
		filtered := items[:0]
		for _, it := range items {
			if it.AssignedTo == userFilter {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })

	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := items[start:end]
	if pageItems == nil {
		pageItems = []store.QueueItem{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     pageItems,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  end < total,
	})
}

func firstApprover(targets []store.ApprovalTarget) string {
	for _, t := range targets {
		if t.UserID != "" {
			return t.UserID
		}
	}
	return ""
}

func escalationPriority(severity string) string {
	switch severity {
	case "p0", "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

// GetRiskDashboard handles GET /risk-dashboard.
func (h *SupervisionHandlers) GetRiskDashboard(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if q := r.URL.Query().Get("tenant_id"); q != "" && q != tenantID {
		writeError(w, r, http.StatusForbidden, "tenant_id does not match authenticated tenant")
		return
	}

	pendingApprovals := h.Approvals.Pending(tenantID)
	openEscalations := h.Escalations.Open(tenantID)
	activeInterventions := h.Interventions.Active(tenantID)

	bySeverity := map[string]int{}
	byCategory := map[string]int{}
	risk := 0.0
	for _, e := range openEscalations {
		bySeverity[e.Severity]++
		byCategory[e.Category]++
		switch e.Severity {
		case "p0":
			risk += 30
		case "critical":
			risk += 20
		case "high":
			risk += 10
		case "medium":
			risk += 5
		default:
			risk += 2
		}
	}
	risk += float64(len(pendingApprovals)) * 3
	risk += float64(len(activeInterventions)) * 15
	if risk > 100 {
		risk = 100
	}

	recentEscalations := make([]map[string]interface{}, 0, 5)
	for i, e := range h.Escalations.All(tenantID) {
		if i >= 5 {
			break
		}
		recentEscalations = append(recentEscalations, map[string]interface{}{
			"id": e.ID, "severity": e.Severity, "category": e.Category, "status": e.Status,
		})
	}
	recentInterventions := make([]map[string]interface{}, 0, 5)
	for i, iv := range h.Interventions.All(tenantID) {
		if i >= 5 {
			break
		}
		recentInterventions = append(recentInterventions, map[string]interface{}{
			"id": iv.ID, "action": iv.Action, "status": iv.Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":                 tenantID,
		"active_approvals_count":    len(pendingApprovals),
		"pending_escalations_count": len(openEscalations),
		"active_interventions_count": len(activeInterventions),
		"overall_risk_score":        risk,
		"escalation_by_severity":    bySeverity,
		"escalation_by_category":    byCategory,
		"recent_escalations":        recentEscalations,
		"recent_interventions":      recentInterventions,
		"generated_at":              time.Now().UTC().Format(time.RFC3339),
	})
}

type hitlAnswerRequest struct {
	Answer      string                 `json:"answer"`
	Confidence  string                 `json:"confidence"`
	ActionTaken string                 `json:"action_taken"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SubmitHitlAnswer handles POST /hitl/{request_id}/answer. When the request
// has an approval gate and the action is approve/reject, the gate decision
// is applied too.
func (h *SupervisionHandlers) SubmitHitlAnswer(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	requestID := r.PathValue("request_id")

	var req hitlAnswerRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Answer == "" {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", "answer is required")
		return
	}
	if req.Confidence != "" && !store.ValidHitlConfidence(req.Confidence) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "confidence must be one of: low, medium, high")
		return
	}
	if req.ActionTaken != "" && !store.ValidHitlAction(req.ActionTaken) {
		writeError(w, r, http.StatusUnprocessableEntity, "Validation failed", "action_taken is not a valid enum value")
		return
	}

	answer, err := h.Hitl.Submit(&store.HitlAnswer{
		TenantID:    tenantID,
		RequestID:   requestID,
		Answer:      req.Answer,
		Confidence:  req.Confidence,
		ActionTaken: req.ActionTaken,
		Metadata:    req.Metadata,
	})
	if err != nil {
		storeError(w, r, err)
		return
	}

	// Apply the decision to the originating approval gate, if one exists.
	if req.ActionTaken == "approve" || req.ActionTaken == "reject" {
		if a, lookupErr := h.Approvals.GetByRequestID(requestID, tenantID); lookupErr == nil {
			actor := middleware.UserIDFromContext(r.Context())
			act := store.ApprovalAction{ActorID: actor, Comment: req.Answer}
			var decided *store.Approval
			var decideErr error
			if req.ActionTaken == "approve" {
				decided, decideErr = h.Approvals.Approve(a.ID, tenantID, act)
			} else {
				decided, decideErr = h.Approvals.Reject(a.ID, tenantID, act)
			}
			if decideErr == nil {
				h.publishResponse(decided, req.ActionTaken, actor, req.Answer, r)
			}
		}
	}

	writeJSON(w, http.StatusCreated, answer)
}
