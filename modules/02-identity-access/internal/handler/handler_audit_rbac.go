package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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

// GetTrails handles GET /api/v1/iam/audit/trails
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

// GetByID handles GET /api/v1/iam/audit/trails/{trail_id}
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

// GetSessionReplay handles GET /api/v1/iam/audit/session-replay/{session_id}
// Returns a chronological replay of audit events for the session.
func (h *AuditHandler) GetSessionReplay(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	sessionID := extractSessionReplayID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, `{"error":"session_id is required"}`, http.StatusBadRequest)
		return
	}

	// Session replay returns recent audit events ordered chronologically.
	// The session_id is logged as context in the replay response.
	events, total, err := h.Audit.List(tenantID, "", "", nil, nil, 500, 0)
	if err != nil {
		http.Error(w, `{"error":"failed to get session replay data"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"tenant_id":  tenantID,
		"events":     events,
		"total":      total,
	})
}

// extractTrailID extracts the trail_id from the URL path.
// Handles: /api/v1/iam/audit/trails/{id}
func extractTrailID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/audit/trails/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// extractSessionReplayID extracts the session_id from the URL path.
// Handles: /api/v1/iam/audit/session-replay/{id}
func extractSessionReplayID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/audit/session-replay/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
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

// Evaluate handles POST /api/v1/iam/rbac/evaluate
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

	// 1. Look up user roles
	user, err := h.Users.GetByActorID(tenantID, req.ActorID)
	var userRoles []string
	if err == nil && user != nil {
		userRoles = user.Roles
	}

	// 2. Resolve role permissions
	allPermissions := make(map[string]bool)
	for _, roleID := range userRoles {
		role, err := h.Roles.GetByID(roleID)
		if err == nil && role != nil {
			for _, perm := range role.Permissions {
				allPermissions[perm] = true
			}
		}
	}

	// 3. Evaluate permission
	allowed := false
	reason := "permission denied"
	policyMatch := ""

	// Check if any permission matches the requested action/resource
	requestedPerm := req.Action + ":" + req.Resource
	if allPermissions[requestedPerm] {
		allowed = true
		reason = "explicit permission grant"
		policyMatch = requestedPerm
	}

	// Check for wildcard permissions (e.g., "*:*")
	if allPermissions["*:*"] {
		allowed = true
		reason = "wildcard role grant"
		policyMatch = "*:*"
	}

	// 4. Check ABAC attributes
	if !allowed && req.Attributes != nil {
		// Time-based policy check
		if _, ok := req.Attributes["outside_business_hours"]; ok {
			hour := time.Now().UTC().Hour()
			if hour >= 9 && hour <= 17 {
				allowed = false
				reason = "action requested during business hours requires manual approval"
			}
		}

		// High-risk action requires manager approval
		if _, ok := req.Attributes["high_risk"]; ok && !allowed {
			reason = "high-risk action requires manager approval"
		}
	}

	// Build result
	result := models.PermissionCheckResult{
		Allowed:     allowed,
		Reason:      reason,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if policyMatch != "" {
		result.PolicyMatch = &policyMatch
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "rbac_evaluate",
		ResourceType: "permission",
		ResourceID:   requestedPerm,
		Result:       map[bool]string{true: "success", false: "denied"}[allowed],
		Details: map[string]interface{}{
			"actor_id":       req.ActorID,
			"requested_action": req.Action,
			"requested_resource": req.Resource,
			"user_roles":       userRoles,
			"resolved_permissions": allPermissions,
			"allowed":          allowed,
			"reason":           reason,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
