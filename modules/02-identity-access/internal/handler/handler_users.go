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

// UserHandler handles user-related HTTP endpoints.
type UserHandler struct {
	Users     *store.UserStore
	Audit     *store.AuditStore
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
	if len(req.Roles) == 0 {
		http.Error(w, `{"error":"at least one role is required"}`, http.StatusBadRequest)
		return
	}

	user := &models.User{
		TenantID:             tenantID,
		Email:                req.Email,
		DisplayName:          req.DisplayName,
		Roles:                req.Roles,
		MFAEnabled:           false,
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

// GetByID handles GET /api/v1/iam/users/{user_id}
func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

// Deactivate handles DELETE /api/v1/iam/users/{user_id}
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

// SetRoles handles PUT /api/v1/iam/users/{user_id}/roles
func (h *UserHandler) SetRoles(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	userID, ok := extractUserRolesPath(r.URL.Path)
	if !ok || userID == "" {
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
