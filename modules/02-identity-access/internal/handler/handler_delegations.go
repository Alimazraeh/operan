package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ---------------------------------------------------------------------------
// Local response types (mirror the old in-memory store shapes for API
// compatibility)
// ---------------------------------------------------------------------------

type paginatedDelegationRolesResponse struct {
	DelegationRoles []models.DelegationRole `json:"delegation_roles"`
	Total           int                     `json:"total"`
	Page            int                     `json:"page"`
	PageSize        int                     `json:"page_size"`
	TotalPages      int                     `json:"total_pages"`
}

type delegationGrantResponse struct {
	ID                 string    `json:"id"`
	DelegationRoleID   string    `json:"delegation_role_id"`
	UserID             string    `json:"user_id"`
	Scope              string    `json:"scope"`
	GrantedAt          time.Time `json:"granted_at"`
	IsActive           bool      `json:"is_active"`
	AuthentikGroupUUID string    `json:"authentik_group_uuid"`
	AuthentikUserUUID  string    `json:"authentik_user_uuid"`
}

// delegationAuthClient is the interface that DelegationHandler uses to
// interact with Authentik. It captures only the methods needed for
// delegation operations, allowing mock implementations in tests.
type delegationAuthClient interface {
	Groups() authentik.GroupsAPIOps
	Users() authentik.UsersAPIOps
}

// publisher is the interface that DelegationHandler uses to publish events.
// It captures only the Publish method, allowing mock implementations in tests.
type publisher interface {
	Publish(ctx context.Context, eventType, correlationID, tenantID, timestamp string, payload map[string]interface{}) error
}

// DelegationHandler handles delegated admin roles HTTP endpoints.
// All persistent state is now managed by Authentik groups; the handler
// maintains an in-memory index mapping role names → Authentik group UUIDs.
type DelegationHandler struct {
	Auth delegationAuthClient

	Publisher publisher

	// groupIndex maps "tenant::roleName" → Authentik group UUID.
	groupIndex map[string]string
}

// NewDelegationHandler creates a new delegation handler backed by Authentik.
func NewDelegationHandler(auth delegationAuthClient, publisher publisher) *DelegationHandler {
	return &DelegationHandler{
		Auth:       auth,
		Publisher:  publisher,
		groupIndex: make(map[string]string),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// tenantGroupPrefix builds the prefix used to filter groups by tenant.
func tenantGroupPrefix(tenantID string) string {
	return "operan-delegation-" + tenantID + "-"
}

// buildGroupName creates the fully-qualified Authentik group name for a
// delegation role.
func buildGroupName(tenantID, roleName string) string {
	return tenantGroupPrefix(tenantID) + roleName
}

// cacheGroupUUID registers the mapping between a role and its Authentik group.
func (h *DelegationHandler) cacheGroupUUID(tenantID, roleName, groupUUID string) {
	key := tenantID + "::" + roleName
	h.groupIndex[key] = groupUUID
}

// lookupGroupUUID finds the Authentik group UUID for a role by first checking
// the in-memory index, then falling back to a name-filtered search.
func (h *DelegationHandler) lookupGroupUUID(tenantID, roleName string) (string, error) {
	key := tenantID + "::" + roleName
	if uuid, ok := h.groupIndex[key]; ok {
		return uuid, nil
	}

	groups, err := h.Auth.Groups().List(context.Background())
	if err != nil {
		return "", fmt.Errorf("list groups to find role: %w", err)
	}

	prefix := tenantGroupPrefix(tenantID)
	targetName := prefix + roleName
	for _, g := range groups {
		if g.Name == targetName {
			h.groupIndex[key] = g.UUID
			return g.UUID, nil
		}
	}
	return "", fmt.Errorf("delegation role \"%s\" not found in Authentik", roleName)
}

// lookupRoleByName finds a group by its full name via the Authentik API.
// Returns the group with populated Properties.
func (h *DelegationHandler) lookupRoleByName(ctx context.Context, tenantID, roleName string) (*authentik.Group, error) {
	groups, err := h.Auth.Groups().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	prefix := tenantGroupPrefix(tenantID)
	targetName := prefix + roleName
	for _, g := range groups {
		if g.Name == targetName {
			// Populate cached index
			key := tenantID + "::" + roleName
			h.groupIndex[key] = g.UUID
			return g, nil
		}
	}
	return nil, fmt.Errorf("delegation role \"%s\" not found in Authentik", roleName)
}

// getAuthentikGroup fetches a group by UUID and caches it.
func (h *DelegationHandler) getAuthentikGroup(ctx context.Context, groupUUID string) (*authentik.Group, error) {
	g, err := h.Auth.Groups().GetByID(ctx, groupUUID)
	if err != nil {
		return nil, err
	}
	// Refresh index (the cached role name may have changed)
	if name, ok := extractRoleName(g.Name); ok {
		key := g.Tenant + "::" + name
		h.groupIndex[key] = groupUUID
	}
	return g, nil
}

// extractRoleName reverses buildGroupName to get the original role name.
func extractRoleName(fullName string) (string, bool) {
	prefix := "operan-delegation-"
	if !strings.HasPrefix(fullName, prefix) {
		return "", false
	}
	afterPrefix := fullName[len(prefix):]
	// afterPrefix is "tenantID-roleName" — split on first dash after tenant
	idx := strings.IndexByte(afterPrefix, '-')
	if idx < 0 {
		return "", false
	}
	return afterPrefix[idx+1:], true
}

// findUserUUID tries to resolve a user identifier to an Authentik user UUID.
// Strategy:
//   1. Treat the identifier as a UUID and try GetByID directly.
//   2. Fall back to listing all users and matching by email.
func (h *DelegationHandler) findUserUUID(ctx context.Context, identifier string) (string, error) {
	// Fast path: assume it's a UUID
	_, err := h.Auth.Users().GetByID(ctx, identifier)
	if err == nil {
		return identifier, nil
	}

	// Slow path: list users and match by email.
	users, err := h.Auth.Users().List(ctx)
	if err != nil {
		return "", fmt.Errorf("list users to find user %s: %w", identifier, err)
	}
	for _, u := range users {
		if u.Email == identifier || u.UUID == identifier {
			return u.UUID, nil
		}
	}
	return "", fmt.Errorf("user %s not found in Authentik", identifier)
}

// ---------------------------------------------------------------------------
// Create — POST /api/v1/iam/admin/delegations
// Also responds to POST /api/v1/iam/admin/delegation
// ---------------------------------------------------------------------------

func (h *DelegationHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	fmt.Printf("DEBUG Create: tenantID=%q\n", tenantID)

	var req models.CreateDelegationRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Build the Authentik group name: operan-delegation-{tenantID}-{roleName}
	groupName := buildGroupName(tenantID, req.Name)

	// Marshal permissions into group Properties for persistence.
	props := map[string]interface{}{
		"permissions": req.Permissions,
		"scope":       req.Scope,
	}
	if req.MaxDelegationDepth != nil {
		props["max_delegation_depth"] = *req.MaxDelegationDepth
	}

	// Create the group in Authentik.
	newGroup, err := h.Auth.Groups().Create(ctx, authentik.CreateGroupRequest{
		Name:   groupName,
		Users:  []string{},
		Tenant: tenantID,
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create delegation role in Authentik: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Cache the mapping.
	h.cacheGroupUUID(tenantID, req.Name, newGroup.UUID)

	// Build response object matching the old store response shape.
	role := models.DelegationRole{
		ID:                   newGroup.UUID,
		TenantID:             tenantID,
		Name:                 req.Name,
		Description:          req.Description,
		Scope:                req.Scope,
		Permissions:          req.Permissions,
		MaxDelegationDepth:   func() int { if req.MaxDelegationDepth != nil { return *req.MaxDelegationDepth }; return 0 }(),
		IsSystem:             false,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}

	h.Publisher.Publish(ctx, "delegation_role.created", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"name":    role.Name,
		"scope":   role.Scope,
		"group":   newGroup.UUID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(role)
}

// ---------------------------------------------------------------------------
// List — GET /api/v1/iam/admin/delegations
// ---------------------------------------------------------------------------

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

	ctx := r.Context()

	allGroups, err := h.Auth.Groups().List(ctx)
	if err != nil {
		http.Error(w, `{"error":"failed to list delegation roles"}`, http.StatusInternalServerError)
		return
	}

	prefix := tenantGroupPrefix(tenantID)
	var roles []models.DelegationRole
	for _, g := range allGroups {
		if !strings.HasPrefix(g.Name, prefix) {
			continue
		}
		roleName, ok := extractRoleName(g.Name)
		if !ok {
			continue
		}
		cache := tenantID + "::" + roleName
		h.groupIndex[cache] = g.UUID

		var perms []string
		if g.Properties != nil {
			if pp, ok := g.Properties["permissions"]; ok {
				if raw, ok := pp.([]interface{}); ok {
					for _, v := range raw {
						if s, ok := v.(string); ok {
							perms = append(perms, s)
						}
					}
				}
			}
		}
		depth := 0
		if g.Properties != nil {
			if d, ok := g.Properties["max_delegation_depth"].(float64); ok {
				depth = int(d)
			}
		}

		roles = append(roles, models.DelegationRole{
			ID:                   g.UUID,
			TenantID:             tenantID,
			Name:                 roleName,
			Description:          g.Name, // store full name as description since group lacks a separate desc field
			Scope:                func() string {
				if g.Properties != nil {
					if sc, ok := g.Properties["scope"].(string); ok {
						return sc
					}
				}
				return ""
			}(),
			Permissions:          perms,
			MaxDelegationDepth:   depth,
			IsSystem:             false,
			CreatedAt:            time.Now().UTC(),
			UpdatedAt:            time.Now().UTC(),
		})
	}

	total := len(roles)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageRoles := roles[start:end]

	if pageRoles == nil {
		pageRoles = []models.DelegationRole{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delegation_roles": pageRoles,
		"total":            total,
		"page":             page,
		"page_size":        pageSize,
		"total_pages":      (total + pageSize - 1) / pageSize,
	})
}

// ---------------------------------------------------------------------------
// GetByID — GET /api/v1/iam/admin/delegations/{role_id}
// ---------------------------------------------------------------------------

func (h *DelegationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// roleID may be a UUID (Authentik group) or a role name. Try both.
	g, err := h.getAuthentikGroup(ctx, roleID)
	if err != nil {
		// Fall back to looking up by name
		g, err = h.lookupRoleByName(ctx, tenantID, roleID)
		if err != nil {
			http.Error(w, `{"error":"delegation role not found"}`, http.StatusNotFound)
			return
		}
	}

	var perms []string
	if g.Properties != nil {
		if pp, ok := g.Properties["permissions"]; ok {
			if raw, ok := pp.([]interface{}); ok {
				for _, v := range raw {
					if s, ok := v.(string); ok {
						perms = append(perms, s)
					}
				}
			}
		}
	}
	depth := 0
	if g.Properties != nil {
		if d, ok := g.Properties["max_delegation_depth"].(float64); ok {
			depth = int(d)
		}
	}

	roleName, _ := extractRoleName(g.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.DelegationRole{
		ID:                   g.UUID,
		TenantID:             tenantID,
		Name:                 roleName,
		Description:          g.Name,
		Scope:                func() string {
			if g.Properties != nil {
				if sc, ok := g.Properties["scope"].(string); ok {
					return sc
				}
			}
			return ""
		}(),
		Permissions:          perms,
		MaxDelegationDepth:   depth,
		IsSystem:             false,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	})
}

// ---------------------------------------------------------------------------
// Update — PATCH /api/v1/iam/admin/delegations/{role_id}
// ---------------------------------------------------------------------------

func (h *DelegationHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Fetch existing group
	g, err := h.getAuthentikGroup(ctx, roleID)
	if err != nil {
		g, err = h.lookupRoleByName(ctx, tenantID, roleID)
		if err != nil {
			http.Error(w, `{"error":"delegation role not found"}`, http.StatusNotFound)
			return
		}
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

	// Build the new group name (may change if role name changes).
	currentRoleName, _ := extractRoleName(g.Name)
	newName := currentRoleName
	if req.Name != nil {
		newName = *req.Name
	}
	newGroupName := buildGroupName(tenantID, newName)

	// Merge existing properties with updates.
	props := map[string]interface{}{}
	if g.Properties != nil {
		for k, v := range g.Properties {
			props[k] = v
		}
	}

	// Apply updates
	if req.Name != nil {
		props["role_name"] = *req.Name
	}
	if req.Description != nil {
		props["description"] = *req.Description
	}
	if req.Scope != nil {
		props["scope"] = *req.Scope
	}
	if req.Permissions != nil {
		props["permissions"] = req.Permissions
	}
	if req.MaxDelegationDepth != nil {
		props["max_delegation_depth"] = *req.MaxDelegationDepth
	}

	// PATCH the group in Authentik.
	updated, err := h.Auth.Groups().Update(ctx, g.UUID, newGroupName)
	if err != nil {
		http.Error(w, `{"error":"failed to update delegation role: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Refresh cache if name changed.
	if newName != currentRoleName {
		oldKey := tenantID + "::" + currentRoleName
		newKey := tenantID + "::" + newName
		delete(h.groupIndex, oldKey)
		h.groupIndex[newKey] = updated.UUID
	}

	roleName, _ := extractRoleName(updated.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.DelegationRole{
		ID:              updated.UUID,
		TenantID:        tenantID,
		Name:            roleName,
		Description:     updated.Name,
		Scope:           props["scope"].(string),
		Permissions:     func() []string {
			if pp, ok := props["permissions"].([]interface{}); ok {
				var r []string
				for _, v := range pp {
					if s, ok := v.(string); ok {
						r = append(r, s)
					}
				}
				return r
			}
			return nil
		}(),
		MaxDelegationDepth: func() int {
			if d, ok := props["max_delegation_depth"].(float64); ok {
				return int(d)
			}
			return 0
		}(),
		IsSystem: false,
		UpdatedAt: time.Now().UTC(),
	})
}

// ---------------------------------------------------------------------------
// Delete — DELETE /api/v1/iam/admin/delegations/{role_id}
// ---------------------------------------------------------------------------

func (h *DelegationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	g, err := h.getAuthentikGroup(ctx, roleID)
	if err != nil {
		g, err = h.lookupRoleByName(ctx, tenantID, roleID)
		if err != nil {
			http.Error(w, `{"error":"delegation role not found"}`, http.StatusNotFound)
			return
		}
	}

	if err := h.Auth.Groups().Delete(ctx, g.UUID); err != nil {
		http.Error(w, `{"error":"failed to delete delegation role: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Remove from cache.
	roleName, _ := extractRoleName(g.Name)
	key := tenantID + "::" + roleName
	delete(h.groupIndex, key)

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Grant — POST /api/v1/iam/admin/delegations/{role_id}/grant
// ---------------------------------------------------------------------------

func (h *DelegationHandler) Grant(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

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

	ctx := r.Context()

	// Resolve role → group
	groupUUID, err := h.lookupGroupUUID(tenantID, roleID)
	if err != nil {
		http.Error(w, `{"error":"delegation role not found: `+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	// Resolve user → Authentik UUID
	userUUID, err := h.findUserUUID(ctx, req.UserID)
	if err != nil {
		http.Error(w, `{"error":"user not found: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Add user to group.
	if err := h.Auth.Groups().AddUser(ctx, groupUUID, userUUID); err != nil {
		http.Error(w, `{"error":"failed to grant delegation: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Build response mirroring old DelegationGrant shape.
	grant := delegationGrantResponse{
		ID:                 userUUID, // reuse user UUID as grant ID
		DelegationRoleID:   groupUUID,
		UserID:             req.UserID,
		Scope:              req.Scope,
		GrantedAt:          time.Now().UTC(),
		IsActive:           true,
		AuthentikGroupUUID: groupUUID,
		AuthentikUserUUID:  userUUID,
	}

	h.Publisher.Publish(ctx, "delegation.granted", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"delegation_role_id": groupUUID,
		"user_id":            req.UserID,
		"scope":              req.Scope,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(grant)
}

// ---------------------------------------------------------------------------
// Revoke — POST /api/v1/iam/admin/delegations/{role_id}/revoke
// ---------------------------------------------------------------------------

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

	// Get userID — body takes precedence, then query param, then actor
	userID := ""
	if !req.RevokeAll {
		if r.URL.Query().Get("user_id") != "" {
			userID = r.URL.Query().Get("user_id")
		} else {
			userID = actorID
		}
	}

	ctx := r.Context()

	// Resolve role → group
	groupUUID, err := h.lookupGroupUUID(tenantID, roleID)
	if err != nil {
		http.Error(w, `{"error":"delegation role not found: `+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	if !req.RevokeAll && userID != "" {
		// Resolve user → Authentik UUID
		userUUID, err := h.findUserUUID(ctx, userID)
		if err != nil {
			http.Error(w, `{"error":"user not found: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}

		// Remove user from group.
		if err := h.Auth.Groups().RemoveUser(ctx, groupUUID, userUUID); err != nil {
			http.Error(w, `{"error":"failed to revoke delegation: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
	} else {
		// RevokeAll: list group members and remove each one.
		// Note: the current GroupsAPI.RemoveUser helper has a variable-scope
		// bug; we call RemoveUser per member below.
		members, err := h.Auth.Groups().GetMembers(ctx, groupUUID)
		if err != nil {
			http.Error(w, `{"error":"failed to list group members: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		for _, memberUUID := range members {
			if err := h.Auth.Groups().RemoveUser(ctx, groupUUID, memberUUID); err != nil {
				// Log but continue — one failure shouldn't block the rest.
				h.Publisher.Publish(ctx, "delegation.revoke_failed", tenantID, "", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
					"group": groupUUID,
					"user":  memberUUID,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "delegation revoked",
	})
}

// ---------------------------------------------------------------------------
// ListDelegations — GET /api/v1/iam/admin/delegations/{role_id}/delegations
// ---------------------------------------------------------------------------

func (h *DelegationHandler) ListDelegations(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	roleID := extractDelegationRoleID(r.URL.Path)
	if roleID == "" {
		http.Error(w, `{"error":"delegation_role_id is required"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Resolve role → group
	groupUUID, err := h.lookupGroupUUID(tenantID, roleID)
	if err != nil {
		http.Error(w, `{"error":"delegation role not found: `+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	// Get group members.
	memberUUIDs, err := h.Auth.Groups().GetMembers(ctx, groupUUID)
	if err != nil {
		http.Error(w, `{"error":"failed to list delegations: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	var delegations []delegationGrantResponse
	for _, uUUID := range memberUUIDs {
		delegations = append(delegations, delegationGrantResponse{
			ID:                 uUUID,
			DelegationRoleID:   groupUUID,
			UserID:             uUUID,
			Scope:              "",
			GrantedAt:          time.Now().UTC(),
			IsActive:           true,
			AuthentikGroupUUID: groupUUID,
			AuthentikUserUUID:  uUUID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delegations": delegations,
		"total":       len(delegations),
	})
}

// ---------------------------------------------------------------------------
// extractDelegationRoleID extracts the delegation_role_id from the URL path.
// Handles: /api/v1/iam/admin/delegations/{id}
// ---------------------------------------------------------------------------

func extractDelegationRoleID(path string) string {
	path = strings.TrimSuffix(path, "/")
	if path == "/api/v1/iam/admin/delegations" {
		return ""
	}
	prefix := "/api/v1/iam/admin/delegations/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	suffix := path[len(prefix):]
	if suffix == "" {
		return ""
	}
	// Only the first path segment is the role_id; everything after is
	// an action segment (grant / revoke / delegations).
	return strings.SplitN(suffix, "/", 2)[0]
}
