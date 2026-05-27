package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// RoleHandler handles role-related HTTP endpoints.
type RoleHandler struct {
	Roles     *store.RoleStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewRoleHandler creates a new role handler.
func NewRoleHandler(roles *store.RoleStore, audit *store.AuditStore, publisher *events.Publisher) *RoleHandler {
	return &RoleHandler{
		Roles:     roles,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Create handles POST /tenants/{id}/iam/roles
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	role := &models.Role{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsSystem:    req.IsSystem != nil && *req.IsSystem,
	}

	if err := h.Roles.Create(role); err != nil {
		http.Error(w, `{"error":"failed to create role: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "create_role",
		ResourceType: "role",
		ResourceID:   role.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"name":        role.Name,
			"permissions": role.Permissions,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.PermissionGranted(r.Context(), tenantID, role.ID, "role", "", "", actorID, "", time.Now().UTC().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(role)
}

// List handles GET /tenants/{id}/iam/roles
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	roles, err := h.Roles.List(tenantID)
	if err != nil {
		http.Error(w, `{"error":"failed to list roles"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles": roles,
	})
}

// GetByID handles GET /tenants/{id}/iam/roles/{role_id}
func (h *RoleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	// Extract role_id from URL path
	roleID := extractRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"role_id is required"}`, http.StatusBadRequest)
		return
	}

	role, err := h.Roles.GetByID(roleID)
	if err != nil {
		http.Error(w, `{"error":"role not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(role)
}

// extractRoleID extracts the role_id from the URL path.
func extractRoleID(path string) string {
	parts := splitPath(path)
	if len(parts) >= 6 && parts[4] == "roles" {
		return parts[5]
	}
	return ""
}
