package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// UserHandler handles user-related HTTP endpoints.
type UserHandler struct {
	Auth      *authentik.Client
	Users     *store.UserStore // Kept for backward compat with test helpers
	Audit     *store.AuditStore
	Publisher *events.Publisher
}

// NewUserHandler creates a new user handler.
func NewUserHandler(auth *authentik.Client, users *store.UserStore, audit *store.AuditStore, publisher *events.Publisher) *UserHandler {
	return &UserHandler{
		Auth:      auth,
		Users:     users,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Create handles POST /api/v1/iam/users
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, `{"error":"email is required"}`, http.StatusBadRequest)
		return
	}
	if req.DisplayName == "" {
		http.Error(w, `{"error":"display_name is required"}`, http.StatusBadRequest)
		return
	}
	if len(req.RoleIDs) == 0 {
		http.Error(w, `{"error":"at least one role is required"}`, http.StatusBadRequest)
		return
	}

	// Determine auth method
	authMethod := "password"
	mfaEnabled := false
	if req.MFAEnabled != nil && *req.MFAEnabled {
		mfaEnabled = true
		authMethod = "mfa"
	}

	// Create user via Authentik or in-memory store
	var user *models.User
	if h.Auth != nil {
		created, err := h.Auth.UsersAPI.Create(r.Context(), authentik.CreateUserRequest{
			Username: req.Email,
			Email:    req.Email,
			Name:     req.DisplayName,
			IsActive: true,
			Tenant:   tenantID,
		})
		if err != nil {
			if isConflictError(err) {
				http.Error(w, `{"error":"failed to create user"}`, http.StatusConflict)
				return
			}
			http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
			return
		}

		// Set roles via group membership
		for _, role := range req.RoleIDs {
			if err := addToGroup(r.Context(), h.Auth, created.UUID, role); err != nil {
				_ = err
			}
		}

		user = h.mapAuthentikUser(created, tenantID)
	} else {
		// Fallback to in-memory store for tests
		user = &models.User{
			ID:                   uuid.New().String(),
			TenantID:             tenantID,
			Email:                req.Email,
			DisplayName:          req.DisplayName,
			Status:               "pending",
			RoleIDs:              req.RoleIDs,
			MFAEnabled:           mfaEnabled,
			AuthenticationMethod: authMethod,
			CreatedAt:            time.Now().UTC(),
			UpdatedAt:            time.Now().UTC(),
		}
		h.Users.Create(user)
	}

	// Log audit event
	h.Audit.CreateWithTenant(&models.AuditEvent{
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "create_user",
		ResourceType: "user",
		ResourceID:   user.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"email":       user.Email,
			"role_ids":    user.RoleIDs,
			"auth_method": authMethod,
		},
		Timestamp: time.Now().UTC(),
	}, tenantID)

	// Publish event
	h.Publisher.UserCreated(r.Context(), user.ID, tenantID, user.Email, "default", actorID, authMethod, "", time.Now().UTC().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// List handles GET /api/v1/iam/users
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
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

	var users []models.User
	var total int
	if h.Auth != nil {
		// Fetch all users from Authentik
		authUsers, err := h.Auth.UsersAPI.List(r.Context())
		if err != nil {
			http.Error(w, `{"error":"failed to list users"}`, http.StatusInternalServerError)
			return
		}

		total = len(authUsers)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}

		users = make([]models.User, 0, end-start)
		for _, au := range authUsers[start:end] {
			user := h.mapAuthentikUser(au, tenantID)
			users = append(users, *user)
		}
	} else {
		// Fallback to in-memory store for tests
		pageUsers, total2, err := h.Users.List(tenantID, page, pageSize)
		if err != nil {
			http.Error(w, `{"error":"failed to list users"}`, http.StatusInternalServerError)
			return
		}
		total = total2
		users = pageUsers
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":       users,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

// GetByID handles GET /api/v1/iam/users/{user_id}
func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r.URL.Path)
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var result models.User
	if h.Auth != nil {
		user, err := h.Auth.UsersAPI.GetByID(r.Context(), userID)
		if err != nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		u := h.mapAuthentikUser(user, middleware.GetTenantID(r.Context()))
		result = *u
	} else {
		// Fallback to in-memory store for tests
		user, err := h.Users.GetByID(userID)
		if err != nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		result = *user
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Update handles PATCH /api/v1/iam/users/{user_id}
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	userID := extractUserID(r.URL.Path)
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	var result models.User
	if h.Auth != nil {
		// Build Authentik update request
		active := func() *bool {
			if req.Status == nil {
				return nil
			}
			v := *req.Status == "active"
			return &v
		}()
		updated, err := h.Auth.UsersAPI.Update(r.Context(), userID, authentik.UpdateUserRequest{
			Name:     req.DisplayName,
			IsActive: active,
			Enabled:  active,
		})
		if err != nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}

		u := h.mapAuthentikUser(updated, tenantID)
		result = *u

		if req.MFAEnabled != nil && *req.MFAEnabled {
			result.MFAEnabled = true
			result.AuthenticationMethod = "mfa"
		}
		if req.Status != nil {
			result.Status = *req.Status
		}
	} else {
		// Fallback to in-memory store for tests
		u, err := h.Users.Update(userID, &req)
		if err != nil {
			http.Error(w, `{"error":"failed to update user"}`, http.StatusInternalServerError)
			return
		}
		result = *u
	}

	// Log audit event
	h.Audit.CreateWithTenant(&models.AuditEvent{
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "update_user",
		ResourceType: "user",
		ResourceID:   result.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"email":        result.Email,
			"display_name": result.DisplayName,
			"status":       result.Status,
		},
		Timestamp: time.Now().UTC(),
	}, tenantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Deactivate handles DELETE /api/v1/iam/users/{user_id}
func (h *UserHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	userID := extractUserID(r.URL.Path)
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	if h.Auth != nil {
		// Soft delete via Authentik (set is_active=false)
		if err := h.Auth.UsersAPI.Delete(r.Context(), userID); err != nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
	} else {
		// Fallback to in-memory store for tests
		req := models.UpdateUserRequest{Status: strPtr("inactive")}
		_, err := h.Users.Update(userID, &req)
		if err != nil {
			http.Error(w, `{"error":"failed to deactivate user"}`, http.StatusInternalServerError)
			return
		}
	}

	// Log audit event
	h.Audit.CreateWithTenant(&models.AuditEvent{
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "deactivate_user",
		ResourceType: "user",
		ResourceID:   userID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	}, tenantID)

	w.WriteHeader(http.StatusNoContent)
}

// SetRoles handles PUT /api/v1/iam/users/{user_id}/roles
func (h *UserHandler) SetRoles(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	userID, ok := extractUserRolesPath(r.URL.Path)
	if !ok || userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var req models.SetRoleIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if h.Auth != nil {
		// Authentik path: manage group memberships
		groups, err := h.Auth.GroupsAPI.List(r.Context())
		if err != nil {
			http.Error(w, `{"error":"failed to list groups"}`, http.StatusInternalServerError)
			return
		}

		// Build a map of group name -> group UUID
		groupMap := make(map[string]string)
		for _, g := range groups {
			groupMap[g.Name] = g.UUID
		}

		// Remove user from all known role groups first
		for name, uuid := range groupMap {
			if isSystemGroup(name) {
				continue
			}
			_ = h.Auth.GroupsAPI.RemoveUser(r.Context(), uuid, userID)
		}

		// Add user to requested role groups
		for _, role := range req.RoleIDs {
			if groupUUID, exists := groupMap[role]; exists {
				if err := h.Auth.GroupsAPI.AddUser(r.Context(), groupUUID, userID); err != nil {
					_ = err
				}
			}
		}
	} else {
		// Fallback: update roles in in-memory store
		_, err := h.Users.GetByID(userID)
		if err != nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		roleIDs := make([]string, len(req.RoleIDs))
		copy(roleIDs, req.RoleIDs)
		h.Users.SetRoles(userID, roleIDs)
	}

	// Log audit event
	h.Audit.CreateWithTenant(&models.AuditEvent{
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "set_user_roles",
		ResourceType: "user",
		ResourceID:   userID,
		Result:       "success",
		Details: map[string]interface{}{
			"role_ids": req.RoleIDs,
		},
		Timestamp: time.Now().UTC(),
	}, tenantID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"role_ids": req.RoleIDs,
	})
}

// extractUserID extracts the user_id from the URL path.
// Supports: /api/v1/iam/users/{id} and /api/v1/iam/users/{id}/roles
func extractUserID(path string) string {
	// Strip base path prefix
	path = strings.TrimPrefix(path, "/api/v1/iam/users/")
	path = strings.TrimSuffix(path, "/")
	// If there's still a slash, it's /{id}/roles → take first segment
	if idx := strings.Index(path, "/"); idx >= 0 {
		path = path[:idx]
	}
	if path == "" {
		return ""
	}
	return path
}

// extractUserRolesPath checks if the path is /api/v1/iam/users/{id}/roles
// and returns (user_id, true).
func extractUserRolesPath(path string) (string, bool) {
	const prefix = "/api/v1/iam/users/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	remaining := path[len(prefix):]
	remaining = strings.TrimSuffix(remaining, "/")
	if !strings.HasSuffix(remaining, "/roles") {
		return "", false
	}
	id := strings.TrimSuffix(remaining, "/roles")
	if id == "" {
		return "", false
	}
	return id, true
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

// isConflictError checks if an Authentik API error indicates a conflict (e.g., duplicate username).
func isConflictError(err error) bool {
	if apiErr, ok := err.(*authentik.APIError); ok {
		return apiErr.StatusCode == 409 || apiErr.StatusCode == 400
	}
	return false
}

// mapAuthentikUser converts an Authentik user response to an Operan user model.
func (h *UserHandler) mapAuthentikUser(created *authentik.User, tenantID string) *models.User {
	user := &models.User{
		ID:                   created.UUID,
		TenantID:             tenantID,
		Email:                created.Email,
		DisplayName:          created.Name,
		Status:               "active",
		RoleIDs:              []string{},
		MFAEnabled:           false,
		AuthenticationMethod: "password",
	}
	if created.DateJoined != nil {
		user.CreatedAt = created.DateJoined.UTC()
	} else {
		user.CreatedAt = time.Now().UTC()
	}
	user.UpdatedAt = time.Now().UTC()
	if created.LastLogin != nil {
		lastLogin := created.LastLogin.UTC()
		user.LastLoginAt = &lastLogin
	}
	if created.Attributes != nil {
		if ldapDN, ok := created.Attributes["ldap_dn"].(string); ok {
			user.LDAPDN = &ldapDN
		}
	}
	return user
}

// addToGroup adds a user to a group identified by name (interpreted as a role name).
func addToGroup(ctx context.Context, auth *authentik.Client, userUUID, roleName string) error {
	groups, err := auth.GroupsAPI.List(ctx)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if g.Name == roleName {
			return auth.GroupsAPI.AddUser(ctx, g.UUID, userUUID)
		}
	}
	// Group not found — create it
	_, err = auth.GroupsAPI.Create(ctx, authentik.CreateGroupRequest{
		Name: roleName,
	})
	if err != nil {
		// Non-fatal: group creation failure does not block user creation
		_ = err
	}
	return nil
}

// isSystemGroup returns true if a group name looks like a system/infrastructure group
// that should not be treated as an Operan role mapping.
func isSystemGroup(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "staff") {
		return true
	}
	if strings.Contains(lower, "system") {
		return true
	}
	if strings.Contains(lower, "internal") {
		return true
	}
	return false
}
