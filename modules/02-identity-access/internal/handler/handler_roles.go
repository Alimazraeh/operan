package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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

// Create handles POST /api/v1/iam/roles
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

// List handles GET /api/v1/iam/roles
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	page := 1
	pageSize := 50
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	roles, total, err := h.Roles.List(tenantID, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"failed to list roles"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles":       roles,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// GetByID handles GET /api/v1/iam/roles/{id}
func (h *RoleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

// Delete handles DELETE /api/v1/iam/roles/{id}
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	roleID := extractRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"role_id is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.Roles.Delete(roleID); err != nil {
		http.Error(w, `{"error":"role not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "delete_role",
		ResourceType: "role",
		ResourceID:   roleID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.WriteHeader(http.StatusNoContent)
}

// extractRoleID extracts the role_id from the URL path.
// Handles: /api/v1/iam/roles/{id}
func extractRoleID(path string) string {
	// Remove trailing slash if present
	path = strings.TrimSuffix(path, "/")
	// Exact match means no role ID
	if path == "/api/v1/iam/roles" {
		return ""
	}
	// Trim the prefix
	return strings.TrimPrefix(path, "/api/v1/iam/roles/")
}
