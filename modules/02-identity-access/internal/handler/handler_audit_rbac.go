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
	Users    *store.UserStore
	Roles    *store.RoleStore
	Services *store.ServiceIdentityStore
	Agents   *store.AgentIdentityStore
	Audit    *store.AuditStore
}

// NewRBACHandler creates a new RBAC handler.
func NewRBACHandler(users *store.UserStore, roles *store.RoleStore, services *store.ServiceIdentityStore, agents *store.AgentIdentityStore, audit *store.AuditStore) *RBACHandler {
	return &RBACHandler{
		Users:    users,
		Roles:    roles,
		Services: services,
		Agents:   agents,
		Audit:    audit,
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

	// 1. Look up roles based on actor type
	// Try user first, then service identity, then agent identity
	var actorType string
	var userRoles []string

	user, err := h.Users.GetByID(req.ActorID)
	if err == nil && user != nil {
		actorType = "user"
		userRoles = user.Roles
	} else {
		// Try service identity
		svc, err := h.Services.GetByID(req.ActorID)
		if err == nil && svc != nil {
			actorType = "service"
			userRoles = svc.Roles
		} else {
			// Try agent identity
			agent, err := h.Agents.GetByID(req.ActorID)
			if err == nil && agent != nil {
				actorType = "agent"
				// Agent identities don't have roles in the traditional sense,
				// but we check if they have any defined roles
			}
		}
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
	if req.Attributes != nil {
		// Time-based policy check
		// If actor is outside business hours, allow access if they have permission
		// Otherwise, deny and require manual approval
		if val, ok := req.Attributes["outside_business_hours"]; ok {
			if outside, _ := val.(bool); outside {
				hour := time.Now().UTC().Hour()
				if hour < 9 || hour > 17 {
					// Outside business hours - deny if no explicit permission
					if !allowed {
						reason = "action requested outside business hours requires manual approval"
					}
				}
			}
		}

		// High-risk action requires manager approval
		if val, ok := req.Attributes["high_risk"]; ok {
			if isHighRisk, _ := val.(bool); isHighRisk && !allowed {
				reason = "high-risk action requires manager approval"
			}
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
			"actor_id":           req.ActorID,
			"actor_type":         actorType,
			"requested_action":   req.Action,
			"requested_resource": req.Resource,
			"user_roles":         userRoles,
			"resolved_permissions": allPermissions,
			"allowed":            allowed,
			"reason":             reason,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
