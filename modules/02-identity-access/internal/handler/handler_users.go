package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// UserHandler handles user-related HTTP endpoints.
type UserHandler struct {
	Users   *store.UserStore
	Audit   *store.AuditStore
	Publisher *events.Publisher
}

// NewUserHandler creates a new user handler.
func NewUserHandler(users *store.UserStore, audit *store.AuditStore, publisher *events.Publisher) *UserHandler {
	return &UserHandler{
		Users:     users,
		Audit:     audit,
		Publisher: publisher,
	}
}

// Create handles POST /tenants/{id}/iam/users
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
	if len(req.Roles) == 0 {
		http.Error(w, `{"error":"at least one role is required"}`, http.StatusBadRequest)
		return
	}

	user := &models.User{
		TenantID:           tenantID,
		Email:              req.Email,
		DisplayName:        req.DisplayName,
		Roles:              req.Roles,
		MFAEnabled:         false,
		AuthenticationMethod: "password",
	}
	if req.MFAEnabled != nil && *req.MFAEnabled {
		user.MFAEnabled = true
		user.AuthenticationMethod = "mfa"
	}
	if req.LDAPDN != nil {
		user.LDAPDN = req.LDAPDN
	}

	if err := h.Users.Create(user); err != nil {
		http.Error(w, `{"error":"failed to create user"}`, http.StatusConflict)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "create_user",
		ResourceType: "user",
		ResourceID:   user.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"email": user.Email,
			"roles": user.Roles,
		},
		Timestamp: time.Now().UTC(),
	})

	// Publish event
	h.Publisher.UserCreated(r.Context(), user.ID, tenantID, user.Email, "default", actorID, "password", "", time.Now().UTC().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// List handles GET /tenants/{id}/iam/users
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

	users, total, err := h.Users.List(tenantID, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"failed to list users"}`, http.StatusInternalServerError)
		return
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

// GetByID handles GET /tenants/{id}/iam/users/{user_id}
func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	// Extract user_id from URL path
	userID := extractUserID(r.URL.Path)
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.Users.GetByID(userID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Update handles PATCH /tenants/{id}/iam/users/{user_id}
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

	user, err := h.Users.Update(userID, &req)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "update_user",
		ResourceType: "user",
		ResourceID:   user.ID,
		Result:       "success",
		Details: map[string]interface{}{
			"email":        user.Email,
			"display_name": user.DisplayName,
			"status":       user.Status,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Deactivate handles DELETE /tenants/{id}/iam/users/{user_id}
func (h *UserHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	userID := extractUserID(r.URL.Path)
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.Users.Deactivate(userID); err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "deactivate_user",
		ResourceType: "user",
		ResourceID:   userID,
		Result:       "success",
		Timestamp:    time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deactivated"})
}

// SetRoles handles POST /tenants/{id}/iam/users/{user_id}/roles
func (h *UserHandler) SetRoles(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	userID := extractUserID(r.URL.Path)
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var req models.SetRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := h.Users.SetRoles(userID, req.Roles); err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	// Log audit event
	h.Audit.Create(&models.AuditEvent{
		TenantID:     tenantID,
		ActorID:      actorID,
		ActorType:    "system",
		Action:       "set_user_roles",
		ResourceType: "user",
		ResourceID:   userID,
		Result:       "success",
		Details: map[string]interface{}{
			"roles": req.Roles,
		},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"roles":   req.Roles,
	})
}

// extractUserID extracts the user_id from the URL path.
func extractUserID(path string) string {
	// Expected path: /tenants/{id}/iam/users/{user_id}
	parts := splitPath(path)
	if len(parts) >= 6 && parts[4] == "users" {
		return parts[5]
	}
	return ""
}

// splitPath splits a URL path into segments.
func splitPath(path string) []string {
	if path == "/" {
		return []string{""}
	}
	if path[0] == '/' {
		path = path[1:]
	}
	return splitString(path, '/')
}

// splitString splits a string by a separator.
func splitString(s string, sep rune) []string {
	var result []string
	var current string
	for _, r := range s {
		if r == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	result = append(result, current)
	return result
}
