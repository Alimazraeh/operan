package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// AuditHandler handles audit-related HTTP endpoints.
type AuditHandler struct {
	Audit *store.AuditStore
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(audit *store.AuditStore) *AuditHandler {
	return &AuditHandler{
		Audit: audit,
	}
}

// GetTrails handles GET /tenants/{id}/iam/audit/trails
func (h *AuditHandler) GetTrails(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	actorID := r.URL.Query().Get("actor_id")
	action := r.URL.Query().Get("action")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err == nil {
			from = &t
		}
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err == nil {
			to = &t
		}
	}

	events, total, err := h.Audit.List(tenantID, actorID, action, from, to, limit, offset)
	if err != nil {
		http.Error(w, `{"error":"failed to list audit trails"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"audit_trails": events,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// GetByID handles GET /tenants/{id}/iam/audit/trails/{trail_id}
func (h *AuditHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	trailID := extractTrailID(r.URL.Path)
	if trailID == "" {
		http.Error(w, `{"error":"trail_id is required"}`, http.StatusBadRequest)
		return
	}

	event, err := h.Audit.GetByID(trailID)
	if err != nil {
		http.Error(w, `{"error":"audit trail not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// extractTrailID extracts the trail_id from the URL path.
func extractTrailID(path string) string {
	parts := splitPath(path)
	if len(parts) >= 8 && parts[4] == "audit" && parts[6] == "trails" {
		return parts[7]
	}
	return ""
}

// RBACHandler handles RBAC/ABAC permission evaluation.
type RBACHandler struct {
	Users   *store.UserStore
	Roles   *store.RoleStore
	Audit   *store.AuditStore
}

// NewRBACHandler creates a new RBAC handler.
func NewRBACHandler(users *store.UserStore, roles *store.RoleStore, audit *store.AuditStore) *RBACHandler {
	return &RBACHandler{
		Users:   users,
		Roles:   roles,
		Audit:   audit,
	}
}

// Evaluate handles POST /tenants/{id}/iam/rbac/evaluate
func (h *RBACHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.PermissionCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Check if actor is a user
	user, err := h.Users.GetByID(req.ActorID)
	if err != nil {
		// If not a user, check if it's a service identity or agent identity
		result := models.PermissionCheckResult{
			Allowed:     false,
			Reason:      "actor not found",
			EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(result)
		return
	}

	// Check if user has the required permissions
	// For now, simple role-based check: if user is active, allow
	var allowed bool
	var reason string

	if !user.IsActive() {
		allowed = false
		reason = "user is not active"
	} else {
		// Check if any of the user's roles have the required permission
		allowed = h.checkPermissions(user.Roles, req.Action, req.Resource)
		if allowed {
			reason = "allowed by role policy"
		} else {
			reason = "no matching role policy"
		}
	}

	result := models.PermissionCheckResult{
		Allowed:     allowed,
		Reason:      reason,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "rbac_evaluate",
		ResourceType: "permission",
		ResourceID:   req.Resource,
		Result:       "success",
		Details: map[string]interface{}{
			"actor_id":   req.ActorID,
			"action":     req.Action,
			"resource":   req.Resource,
			"allowed":    allowed,
			"reason":     reason,
			"roles":      user.Roles,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
	}
	json.NewEncoder(w).Encode(result)
}

// checkPermissions checks if any role has the required permission.
func (h *RBACHandler) checkPermissions(roles []string, action, resource string) bool {
	// For now, simple check: if user has any role, allow all actions on all resources
	// In a real implementation, this would check specific role permissions
	if len(roles) > 0 {
		return true
	}
	return false
}
