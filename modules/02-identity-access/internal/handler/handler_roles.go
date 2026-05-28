package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// RoleHandler handles role-related HTTP endpoints, delegating to Authentik for RBAC.
type RoleHandler struct {
	RBAC      authentik.RBACOperations
	Publisher *events.Publisher
}

// NewRoleHandler creates a new role handler.
func NewRoleHandler(auth *authentik.Client, publisher *events.Publisher) *RoleHandler {
	return &RoleHandler{
		RBAC:      auth.RBACAPI,
		Publisher: publisher,
	}
}

// NewTestRoleHandler creates a new role handler for testing.
// Deprecated: local store fallback has been removed. Use NewRoleHandler instead.
func NewTestRoleHandler(auth authentik.RBACOperations, publisher *events.Publisher) *RoleHandler {
	return &RoleHandler{
		RBAC:      auth,
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

	now := time.Now().UTC()
	role := &models.Role{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsSystem:    req.IsSystem != nil && *req.IsSystem,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// If authentik is configured, delegate role creation to authentik
	if h.RBAC != nil {
		authRoleName := "operan-" + tenantID + "-" + req.Name
		authCtx := r.Context()

		authRole, err := h.RBAC.Create(authCtx, authentik.CreateRoleRequest{
			Name:        authRoleName,
			Permissions: req.Permissions,
		})
		if err != nil {
			http.Error(w, `{"error":"failed to create role: `+err.Error()+`"}`, http.StatusConflict)
			return
		}

		role.ID = authRole.UUID
		role.Permissions = authRole.Permissions
	} else {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	// Publish event
	if h.Publisher != nil {
		ctx := r.Context()
		h.Publisher.PermissionGranted(ctx, tenantID, role.ID, "role", "", "", actorID, "", now.Format(time.RFC3339))
	}

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

	var filtered []models.Role
	var total int
	if h.RBAC != nil {
		// Fetch all Authentik roles, then filter by tenant prefix
		allRoles, err := h.RBAC.List(r.Context())
		if err != nil {
			http.Error(w, `{"error":"failed to list roles"}`, http.StatusInternalServerError)
			return
		}

		prefix := "operan-" + tenantID + "-"
		for _, ar := range allRoles {
			if !strings.HasPrefix(ar.Name, prefix) {
				continue
			}
			// Extract the human-readable role name (strip prefix)
			roleName := strings.TrimPrefix(ar.Name, prefix)

			role := models.Role{
				ID:              ar.UUID,
				TenantID:        tenantID,
				Name:            roleName,
				Permissions:     ar.Permissions,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			}
			filtered = append(filtered, role)
		}
		total = len(filtered)

		// Apply pagination to the filtered list
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		filtered = filtered[start:end]
	} else {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles":       filtered,
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

	tenantID := middleware.GetTenantID(r.Context())
	prefix := "operan-" + tenantID + "-"

	if h.RBAC != nil {
		// Authentik uses UUIDs for role IDs. If the provided ID looks like a UUID,
		// try a direct GET by UUID first.
		_, uuidErr := uuid.Parse(roleID)
		if uuidErr == nil {
			authRole, err := h.RBAC.GetByID(r.Context(), roleID)
			if err == nil {
				role := models.Role{
					ID:              authRole.UUID,
					TenantID:        tenantID,
					Name:            strings.TrimPrefix(authRole.Name, prefix),
					Permissions:     authRole.Permissions,
					CreatedAt:       time.Now().UTC(),
					UpdatedAt:       time.Now().UTC(),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(role)
				return
			}
		}

		// Direct UUID lookup failed — list and find by full name match
		allRoles, err := h.RBAC.List(r.Context())
		if err != nil {
			http.Error(w, `{"error":"failed to get role"}`, http.StatusInternalServerError)
			return
		}

		// Try to match by full prefix+name first, then by UUID
		for _, ar := range allRoles {
			fullName := prefix + roleID
			if ar.Name == fullName || ar.UUID == roleID {
				role := models.Role{
					ID:              ar.UUID,
					TenantID:        tenantID,
					Name:            strings.TrimPrefix(ar.Name, prefix),
					Permissions:     ar.Permissions,
					CreatedAt:       time.Now().UTC(),
					UpdatedAt:       time.Now().UTC(),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(role)
				return
			}
		}
	} else {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	http.Error(w, `{"error":"role not found"}`, http.StatusNotFound)
}

// Delete handles DELETE /api/v1/iam/roles/{id}
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	roleID := extractRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"role_id is required"}`, http.StatusBadRequest)
		return
	}

	if h.RBAC != nil {
		// Try direct UUID delete first
		_, uuidErr := uuid.Parse(roleID)
		if uuidErr == nil {
			err := h.RBAC.Delete(r.Context(), roleID)
			if err == nil {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// If Authentik returns not found, fall through to name-based lookup
			if apiErr, ok := err.(*authentik.APIError); ok && apiErr.StatusCode == 404 {
				// fall through
			} else if err != nil {
				http.Error(w, `{"error":"failed to delete role: `+err.Error()+`"}`, http.StatusInternalServerError)
				return
			}
		}

		// Name-based resolution: list and find by prefix+name
		prefix := "operan-" + tenantID + "-"
		allRoles, err := h.RBAC.List(r.Context())
		if err != nil {
			http.Error(w, `{"error":"failed to find role"}`, http.StatusInternalServerError)
			return
		}

		var targetUUID string
		for _, ar := range allRoles {
			fullName := prefix + roleID
			if ar.Name == fullName {
				targetUUID = ar.UUID
				break
			}
		}

		if targetUUID == "" {
			http.Error(w, `{"error":"role not found"}`, http.StatusNotFound)
			return
		}

		err = h.RBAC.Delete(r.Context(), targetUUID)
		if err != nil {
			http.Error(w, `{"error":"failed to delete role: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	} else {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}
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

// ---------------------------------------------------------------------------
// Compile-time checks
// ---------------------------------------------------------------------------

// Ensure context import is used (for r.Context() calls above).
var _ = context.Background

// Ensure no unused package.
var _ = fmt.Errorf
