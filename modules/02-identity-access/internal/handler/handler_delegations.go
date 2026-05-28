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

// DelegationHandler handles delegated admin roles HTTP endpoints.
type DelegationHandler struct {
	Roles     *store.DelegationRoleStore
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewDelegationHandler creates a new delegation handler.
func NewDelegationHandler(roles *store.DelegationRoleStore, audit *store.AuditStore, publisher *events.Publisher) *DelegationHandler {
	return &DelegationHandler{
		Roles:     roles,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Create handles POST /api/v1/iam/admin/delegations
func (h *DelegationHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.CreateDelegationRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	role := &models.DelegationRole{
		TenantID:       tenantID,
		Name:           req.Name,
		Description:    req.Description,
		Scope:          req.Scope,
		Permissions:    req.Permissions,
		MaxDelegationDepth: func() int {
			if req.MaxDelegationDepth != nil {
				return *req.MaxDelegationDepth
			}
			return 0
		}(),
		IsSystem: false,
	}

	if err := h.Roles.Create(role); err != nil {
		http.Error(w, `{"error":"failed to create delegation role: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "create_delegation_role",
		ResourceType: "delegation_role",
		ResourceID:   role.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"name":               role.Name,
			"scope":              role.Scope,
			"permissions":        role.Permissions,
			"max_delegation_depth": role.MaxDelegationDepth,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.Publish(r.Context(), "delegation_role.created", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"name": role.Name,
		"scope": role.Scope,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(role)
}

// List handles GET /api/v1/iam/admin/delegations
func (h *DelegationHandler) List(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, `{"error":"failed to list delegation roles"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delegation_roles": roles,
		"total":            total,
		"page":             page,
		"page_size":        pageSize,
		"total_pages":      (total + pageSize - 1) / pageSize,
	})
}

// GetByID handles GET /api/v1/iam/admin/delegations/{id}
func (h *DelegationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	role, err := h.Roles.GetByID(roleID)
	if err != nil {
		http.Error(w, `{"error":"delegation role not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(role)
}

// Update handles PATCH /api/v1/iam/admin/delegations/{id}
func (h *DelegationHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	role, err := h.Roles.GetByID(roleID)
	if err != nil {
		http.Error(w, `{"error":"delegation role not found"}`, http.StatusNotFound)
		return
	}

	var req models.UpdateDelegationRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Apply updates
	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = *req.Description
	}
	if req.Scope != nil {
		role.Scope = *req.Scope
	}
	if req.Permissions != nil {
		role.Permissions = req.Permissions
	}
	if req.MaxDelegationDepth != nil {
		role.MaxDelegationDepth = *req.MaxDelegationDepth
	}

	updated, err := h.Roles.Update(
		roleID,
		role.Name,
		role.Description,
		role.Scope,
		role.Permissions,
		role.MaxDelegationDepth,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to update delegation role: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "update_delegation_role",
		ResourceType: "delegation_role",
		ResourceID:   updated.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"name":    updated.Name,
			"scope":   updated.Scope,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// Delete handles DELETE /api/v1/iam/admin/delegations/{id}
func (h *DelegationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.Roles.Delete(roleID); err != nil {
		http.Error(w, `{"error":"delegation role not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "delete_delegation_role",
		ResourceType: "delegation_role",
		ResourceID:   roleID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.WriteHeader(http.StatusNoContent)
}

// Grant handles POST /api/v1/iam/admin/delegations/{id}/grant
func (h *DelegationHandler) Grant(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	var req models.DelegateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	grant := &models.DelegationGrant{
		DelegationRoleID: roleID,
		UserID:           req.UserID,
		Scope:             req.Scope,
	}

	if err := h.Roles.GrantDelegation(grant); err != nil {
		http.Error(w, `{"error":"failed to grant delegation: `+err.Error()+`"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "grant_delegation",
		ResourceType: "delegation_role",
		ResourceID:   roleID,
		Result:       "success",
		Details: map[string]interface{}{
			"user_id":       req.UserID,
			"scope":         req.Scope,
			"grant_id":      grant.ID,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.Publish(r.Context(), "delegation.granted", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"delegation_role_id": roleID,
		"user_id":            req.UserID,
		"scope":              req.Scope,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(grant)
}

// Revoke handles POST /api/v1/iam/admin/delegations/{id}/revoke
func (h *DelegationHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	// Parse optional body
	var req models.RevokeDelegationRequest
	if r.Body != nil && r.ContentLength > 0 {
		json.NewDecoder(r.Body).Decode(&req)
	}

	// Get userID from query param or actor context
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = actorID
	}

	if err := h.Roles.RevokeDelegation(roleID, userID); err != nil {
		http.Error(w, `{"error":"failed to revoke delegation: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "revoke_delegation",
		ResourceType: "delegation_role",
		ResourceID:   roleID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "delegation revoked",
	})
}

// ListDelegations handles GET /api/v1/iam/admin/delegations/{id}/delegations
func (h *DelegationHandler) ListDelegations(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	grants, err := h.Roles.ListDelegations(tenantID, roleID, "")
	if err != nil {
		http.Error(w, `{"error":"failed to list delegations"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delegations": grants,
		"total":       len(grants),
	})
}

// extractDelegationRoleID extracts the delegation_role_id from the URL path.
// Handles: /api/v1/iam/admin/delegations/{id}
func extractDelegationRoleID(path string) string {
	path = strings.TrimSuffix(path, "/")
	if path == "/api/v1/iam/admin/delegations" {
		return ""
	}
	prefix := "/api/v1/iam/admin/delegations/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return path[len(prefix):]
}
