package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ServiceIdentityHandler handles service identity-related HTTP endpoints
// by delegating to Authentik's Applications API (service accounts with tokens).
type ServiceIdentityHandler struct {
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewServiceIdentityHandler creates a new service identity handler backed by Authentik.
func NewServiceIdentityHandler(auth *authentik.Client, publisher *events.Publisher) *ServiceIdentityHandler {
	return &ServiceIdentityHandler{
		Auth:      auth,
		Publisher: publisher,
	}
}

// Create handles POST /api/v1/iam/service-identities
// Creates an Authentik application + generates an API token.
func (h *ServiceIdentityHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	var req models.CreateServiceIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if err := req.Validate(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Ensure tenant_id from request matches middleware-validated tenant
	if req.TenantID != "" && req.TenantID != tenantID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "tenant_id mismatch"})
		return
	}

	// Build application name for Authentik: operan-service-<tenantID>-<name>
	appName := "operan-service-" + tenantID + "-" + req.Name

	// Step 1: Create application in Authentik
	app, err := h.Auth.ApplicationsAPI.Create(ctx, authentik.CreateApplicationRequest{
		Slug:             strings.ReplaceAll(appName, " ", "-"),
		Name:             appName,
		ProtocolPrefix:   "https",
		AuthenticationRank: 0,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.authenticErrorStatus(err))
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create application: " + err.Error()})
		return
	}

	// Step 2: Create a user for the application token
	user, err := h.Auth.UsersAPI.Create(ctx, authentik.CreateUserRequest{
		Username: appName,
		Name:     appName,
		IsActive: true,
		Tenant:   tenantID,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.authenticErrorStatus(err))
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create user: " + err.Error()})
		return
	}

	// Step 3: Generate an API token for the user
	token, err := h.Auth.TokensAPI.Create(ctx, authentik.CreateTokenRequest{
		User: user.UUID,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.authenticErrorStatus(err))
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to generate API key: " + err.Error()})
		return
	}

	// Build response model
	identity := &models.ServiceIdentity{
		ID:          app.UUID,
		TenantID:    tenantID,
		Name:        app.Name,
		Roles:       req.Roles,
		APIKeyID:    token.UUID,
		Metadata:    func() string { if req.Metadata != nil { return *req.Metadata }; return ""}(),
		CreatedAt:   time.Now().UTC(),
	}

	// Publish event
	h.Publisher.IdentityRotated(ctx, identity.ID, tenantID, "service", identity.APIKeyID, actorID, "", time.Now().UTC().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(identity)
}

// List handles GET /api/v1/iam/service-identities
// Lists Authentik applications whose names contain the tenant prefix.
func (h *ServiceIdentityHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	ctx := r.Context()

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

	// Fetch all applications from Authentik
	apps, err := h.Auth.ApplicationsAPI.List(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to list applications"})
		return
	}

	// Filter by tenant prefix: operan-service-<tenantID>-<name>
	prefix := "operan-service-" + tenantID + "-"
	var serviceIDentities []models.ServiceIdentity
	for _, app := range apps {
		if !strings.HasPrefix(app.Name, prefix) {
			continue
		}
		serviceIDentities = append(serviceIDentities, models.ServiceIdentity{
			ID:       app.UUID,
			TenantID: tenantID,
			Name:     app.Name,
		})
	}

	total := len(serviceIDentities)

	// Apply client-side pagination
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := serviceIDentities[start:end]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service_identities": pageItems,
		"total":              total,
		"page":               page,
		"page_size":          pageSize,
		"total_pages":        (total + pageSize - 1) / pageSize,
	})
}

// GetByID handles GET /api/v1/iam/service-identities/{id}
// Retrieves an Authentik application by UUID.
func (h *ServiceIdentityHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	appUUID := extractIdentityID(r.URL.Path)
	if appUUID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "identity_id is required"})
		return
	}

	app, err := h.Auth.ApplicationsAPI.GetByID(r.Context(), appUUID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "service identity not found"})
		return
	}

	// Build response model
	identity := &models.ServiceIdentity{
		ID:       app.UUID,
		Name:     app.Name,
		Roles:    nil,
		Metadata: "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(identity)
}

// authenticErrorStatus maps Authentik API errors to HTTP status codes.
func (h *ServiceIdentityHandler) authenticErrorStatus(err error) int {
	var apiErr *authentik.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return http.StatusInternalServerError
}

// ---------------------------------------------------------------------------
// AgentIdentityHandler
// ---------------------------------------------------------------------------

// AgentIdentityHandler handles agent identity-related HTTP endpoints
// by delegating to Authentik's Users API (agents are users in tenant agent groups).
type AgentIdentityHandler struct {
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewAgentIdentityHandler creates a new agent identity handler backed by Authentik.
func NewAgentIdentityHandler(auth *authentik.Client, publisher *events.Publisher) *AgentIdentityHandler {
	return &AgentIdentityHandler{
		Auth:      auth,
		Publisher: publisher,
	}
}

// Register handles POST /api/v1/iam/agent-identities
// Creates an Authentik user + assigns to the tenant's agent group.
func (h *AgentIdentityHandler) Register(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	ctx := r.Context()

	var req models.RegisterAgentIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if err := req.Validate(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Ensure tenant_id from request matches middleware-validated tenant
	if req.TenantID != "" && req.TenantID != tenantID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "tenant_id mismatch"})
		return
	}

	// Build username for Authentik: agent-{tenantID}-{agentID}
	username := "agent-" + tenantID + "-" + req.AgentID
	displayName := "Agent " + req.AgentID

	// Step 1: Create user in Authentik
	user, err := h.Auth.UsersAPI.Create(ctx, authentik.CreateUserRequest{
		Username: username,
		Email:    username + "@operan.internal",
		Name:     displayName,
		IsActive: true,
		Tenant:   tenantID,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.authenticErrorStatus(err))
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create user: " + err.Error()})
		return
	}

	// Step 2: Look up or create the agent group for this tenant
	groupID, err := h.getAgentGroupID(ctx, tenantID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to resolve agent group: " + err.Error()})
		return
	}

	// Step 3: Add user to the agent group
	if err := h.Auth.GroupsAPI.AddUser(ctx, groupID, user.UUID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to add user to group: " + err.Error()})
		return
	}

	// Build response model
	identity := &models.AgentIdentity{
		ID:                user.UUID,
		TenantID:          tenantID,
		AgentID:           req.AgentID,
		Capabilities:      req.Capabilities,
		MemoryScope:       req.MemoryScope,
		AllowedTools:      req.AllowedTools,
		EscalationTargets: req.EscalationTargets,
		CreatedAt:         time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(identity)
}

// List handles GET /api/v1/iam/agent-identities
// Lists Authentik users whose username matches the agent pattern for this tenant.
func (h *AgentIdentityHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	ctx := r.Context()

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

	// Fetch all users from Authentik
	users, err := h.Auth.UsersAPI.List(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to list users"})
		return
	}

	// Filter by tenant prefix: agent-{tenantID}-{agentID}
	prefix := "agent-" + tenantID + "-"
	var agentList []models.AgentIdentity
	for _, u := range users {
		if !strings.HasPrefix(u.Username, prefix) {
			continue
		}
		// Extract agent_id from username: agent-{tenantID}-{agentID}
		agentID := strings.TrimPrefix(u.Username, prefix)
		agentList = append(agentList, models.AgentIdentity{
			ID:       u.UUID,
			TenantID: tenantID,
			AgentID:  agentID,
		})
	}

	total := len(agentList)

	// Apply client-side pagination
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := agentList[start:end]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_identities": pageItems,
		"total":            total,
		"page":             page,
		"page_size":        pageSize,
		"total_pages":      (total + pageSize - 1) / pageSize,
	})
}

// GetByAgent handles GET /api/v1/iam/agent-identities/agent/{agent_id}
// Looks up an Authentik user by agent_id extracted from the username.
func (h *AgentIdentityHandler) GetByAgent(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	agentID := extractAgentID(r.URL.Path)
	if agentID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "agent_id is required"})
		return
	}

	username := "agent-" + tenantID + "-" + agentID

	// Fetch user from Authentik (by username match on full list since no direct lookup)
	users, err := h.Auth.UsersAPI.List(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to look up user"})
		return
	}

	var user *authentik.User
	for _, u := range users {
		if u.Username == username {
			user = u
			break
		}
	}
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "agent identity not found"})
		return
	}

	// Verify the user belongs to this tenant's agent group
	groupID, err := h.getAgentGroupID(r.Context(), tenantID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to resolve agent group: " + err.Error()})
		return
	}

	group, err := h.Auth.GroupsAPI.GetByID(r.Context(), groupID)
	if err != nil || !containsUUID(group.Users, user.UUID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "agent identity not found"})
		return
	}

	// Build response model
	identity := &models.AgentIdentity{
		ID:                user.UUID,
		TenantID:          tenantID,
		AgentID:           agentID,
		Capabilities:      nil, // not stored in Authentik by default
		MemoryScope:       nil,
		AllowedTools:      nil,
		EscalationTargets: nil,
	}
	if user.DateJoined != nil {
		identity.CreatedAt = *user.DateJoined
	} else {
		identity.CreatedAt = time.Now().UTC()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(identity)
}

// getAgentGroupID returns the UUID of the agent group for a tenant.
// Creates the group in Authentik if it does not yet exist.
func (h *AgentIdentityHandler) getAgentGroupID(ctx context.Context, tenantID string) (string, error) {
	groupName := "operan-agents-" + tenantID

	// Check if group already exists in Authentik
	groups, err := h.Auth.GroupsAPI.List(ctx)
	if err != nil {
		return "", err
	}

	for _, g := range groups {
		if g.Name == groupName {
			return g.UUID, nil
		}
	}

	// Create the group
	group, err := h.Auth.GroupsAPI.Create(ctx, authentik.CreateGroupRequest{
		Name:   groupName,
		Tenant: tenantID,
	})
	if err != nil {
		return "", err
	}

	return group.UUID, nil
}

// authenticErrorStatus maps Authentik API errors to HTTP status codes.
func (h *AgentIdentityHandler) authenticErrorStatus(err error) int {
	var apiErr *authentik.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return http.StatusInternalServerError
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractIdentityID extracts the service identity ID from the URL path.
// Handles: /api/v1/iam/service-identities/{id}
func extractIdentityID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/service-identities/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// extractAgentID extracts the agent_id from the URL path.
// Handles: /api/v1/iam/agent-identities/agent/{agent_id}
func extractAgentID(path string) string {
	const prefix = "/api/v1/iam/agent-identities/agent/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	agentID := path[len(prefix):]
	agentID = strings.TrimSuffix(agentID, "/")
	if agentID == "" {
		return ""
	}
	return agentID
}

// containsUUID checks whether a slice of UUID strings contains the given UUID.
func containsUUID(ids []string, uuid string) bool {
	for _, id := range ids {
		if id == uuid {
			return true
		}
	}
	return false
}
