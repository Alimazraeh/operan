// Package handlers provides HTTP handlers for the Agent Registry API.
// All handlers enforce tenant isolation via context and use RFC 7807 error format.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/04-agent-registry/internal/cache"
	"github.com/operan/modules/04-agent-registry/internal/config"
	"github.com/operan/modules/04-agent-registry/internal/events"
	mw "github.com/operan/modules/04-agent-registry/internal/middleware"
	"github.com/operan/modules/04-agent-registry/internal/store"
)

// ─── Handler ────────────────────────────────────────────────────────────────

type AgentRegistryHandlers struct {
	AgentStore      *store.AgentStore
	VersionStore    *store.VersionStore
	CapabilityStore *store.CapabilityStore
	DependencyStore *store.DependencyStore
	EventPublisher  *events.Publisher
	Cache           *cache.Cache
	JWTSecret       string
}

func NewAgentRegistryHandlers(
	agentStore *store.AgentStore,
	versionStore *store.VersionStore,
	capabilityStore *store.CapabilityStore,
	dependencyStore *store.DependencyStore,
	cfg config.Config,
) *AgentRegistryHandlers {
	return &AgentRegistryHandlers{
		AgentStore:      agentStore,
		VersionStore:    versionStore,
		CapabilityStore: capabilityStore,
		DependencyStore: dependencyStore,
		EventPublisher:  events.NewPublisher(),
		Cache:           cache.New(),
		JWTSecret:       cfg.JWTSecret,
	}
}

// ─── Response helpers ───────────────────────────────────────────────────────

func (h *AgentRegistryHandlers) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *AgentRegistryHandlers) writeError(w http.ResponseWriter, status int, typ, title, detail string) {
	mw.WriteJSON(w, status, mw.ErrorResponse{
		Type:     typ,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: "",
	})
}

func now() time.Time { return time.Now().UTC() }

// ─── Agent CRUD ─────────────────────────────────────────────────────────────

func (h *AgentRegistryHandlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	tenantID := mw.TenantIDFromContext(r.Context())
	if tenantID == "" {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", "X-Tenant-ID header is required", r.URL.Path)
		return
	}

	role := r.URL.Query().Get("role")
	status := r.URL.Query().Get("status")
	capability := r.URL.Query().Get("capability")

	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1 {
			page = n
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n >= 1 && n <= 100 {
			pageSize = n
		}
	}

	agents, total, err := h.AgentStore.List(r.Context(), role, status, capability, page, pageSize)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
		return
	}

	// Populate cache for returned agents
	for _, a := range agents {
		h.Cache.Set(a)
	}

	mw.WriteJSON(w, http.StatusOK, AgentListResponse{
		Items:   agents,
		Total:   total,
		Page:    page,
		Size:    pageSize,
		HasMore: page*pageSize < total,
	})
}

func (h *AgentRegistryHandlers) CreateAgent(w http.ResponseWriter, r *http.Request) {
	tenantID := mw.TenantIDFromContext(r.Context())
	if tenantID == "" {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", "X-Tenant-ID header is required", r.URL.Path)
		return
	}

	var req store.CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	if req.Name == "" || req.Role == "" || req.TenantID != tenantID {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "name, role, and tenant_id are required")
		return
	}

	t := now()
	agent := &store.Agent{
		ID:                 uuid.New().String(),
		Name:               req.Name,
		Role:               req.Role,
		Description:        req.Description,
		TenantID:           tenantID,
		DepartmentID:       req.DepartmentID,
		Status:             store.AgentStatusActive,
		Objectives:         req.Objectives,
		Capabilities:       req.Capabilities,
		Tools:              req.Tools,
		MemoryAccess:       req.MemoryAccess,
		EscalationRules:    req.EscalationRules,
		GovernancePolicies: req.GovernancePolicies,
		SupportedLanguages: req.SupportedLanguages,
		RuntimeConstraints: req.RuntimeConstraints,
		CostProfile:        req.CostProfile,
		ExecutionBudget:    req.ExecutionBudget,
		CreatedAt:          t,
		UpdatedAt:          t,
	}

	if err := h.AgentStore.Create(r.Context(), agent); err != nil {
		h.writeError(w, http.StatusConflict, "agent_exists", "Conflict", err.Error())
		return
	}

	// Cache the newly created agent
	h.Cache.Set(agent)

	h.EventPublisher.PublishAgentRegistered(
		agent.ID, tenantID, agent.Name, agent.Role, "1.0.0",
		string(agent.Status), mw.UserIDFromContext(r.Context()),
		agent.Objectives, agent.Capabilities, agent.Tools,
		agent.EscalationRules, agent.GovernancePolicies,
		agent.SupportedLanguages, agent.ExecutionBudget, req.DepartmentID,
		agent.MemoryAccess, t,
	)

	h.writeJSON(w, http.StatusCreated, agentAPI{agent})
}

func (h *AgentRegistryHandlers) GetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := extractIDFromPath(r.URL.Path, "/registry/agents/")
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	// Check cache first
	if agent := h.Cache.Get(agentID); agent != nil {
		h.writeJSON(w, http.StatusOK, agentAPI{agent})
		return
	}

	agent, err := h.AgentStore.GetByID(r.Context(), agentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}

	// Populate cache
	h.Cache.Set(agent)
	h.writeJSON(w, http.StatusOK, agentAPI{agent})
}

func (h *AgentRegistryHandlers) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	agentID := extractIDFromPath(r.URL.Path, "/registry/agents/")
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	var req store.UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	err := h.AgentStore.Patch(r.Context(), agentID, func(a *store.Agent) {
		if req.Name != nil {
			a.Name = *req.Name
		}
		if req.Role != nil {
			a.Role = *req.Role
		}
		if req.Description != nil {
			a.Description = *req.Description
		}
		if req.DepartmentID != nil {
			a.DepartmentID = req.DepartmentID
		}
		if req.Objectives != nil {
			a.Objectives = *req.Objectives
		}
		if req.Capabilities != nil {
			a.Capabilities = *req.Capabilities
		}
		if req.Tools != nil {
			a.Tools = *req.Tools
		}
		if req.MemoryAccess != nil {
			a.MemoryAccess = req.MemoryAccess
		}
		if req.EscalationRules != nil {
			a.EscalationRules = *req.EscalationRules
		}
		if req.GovernancePolicies != nil {
			a.GovernancePolicies = *req.GovernancePolicies
		}
		if req.SupportedLanguages != nil {
			a.SupportedLanguages = *req.SupportedLanguages
		}
		if req.Status != nil {
			a.Status = *req.Status
		}
		if req.RuntimeConstraints != nil {
			a.RuntimeConstraints = req.RuntimeConstraints
		}
		if req.CostProfile != nil {
			a.CostProfile = req.CostProfile
		}
		if req.ExecutionBudget != nil {
			a.ExecutionBudget = req.ExecutionBudget
		}
		if req.AccessControl != nil {
			a.AccessControl = req.AccessControl
		}
		a.UpdatedAt = now()
	})
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}

	// Invalidate and re-cache
	h.Cache.Delete(agentID)
	agent, _ := h.AgentStore.GetByID(r.Context(), agentID)
	if agent != nil {
		h.Cache.Set(agent)
	}
	h.writeJSON(w, http.StatusOK, agentAPI{agent})
}

func (h *AgentRegistryHandlers) DeprecateAgent(w http.ResponseWriter, r *http.Request) {
	agentID := extractIDFromPath(r.URL.Path, "/registry/agents/")
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	_, err := h.AgentStore.GetByID(r.Context(), agentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}

	err = h.AgentStore.Patch(r.Context(), agentID, func(a *store.Agent) {
		a.Status = store.AgentStatusDeprecated
		a.UpdatedAt = now()
	})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
		return
	}

	// Invalidate cache
	h.Cache.Delete(agentID)

	h.EventPublisher.PublishAgentDeprecated(agentID, mw.UserIDFromContext(r.Context()),
		"Soft-deleted via API", string(store.AgentStatusDeprecated), nil, nil, now())
	w.WriteHeader(http.StatusNoContent)
}

func (h *AgentRegistryHandlers) ArchiveAgent(w http.ResponseWriter, r *http.Request) {
	agentID := extractIDFromPath(r.URL.Path, "/registry/agents/")
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	_, err := h.AgentStore.GetByID(r.Context(), agentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}

	err = h.AgentStore.Patch(r.Context(), agentID, func(a *store.Agent) {
		a.Status = store.AgentStatusArchived
		a.UpdatedAt = now()
	})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
		return
	}

	// Invalidate cache
	h.Cache.Delete(agentID)

	h.EventPublisher.PublishAgentArchived(agentID, mw.UserIDFromContext(r.Context()),
		"Agent archived via API", now())
	w.WriteHeader(http.StatusNoContent)
}

// ─── Search ─────────────────────────────────────────────────────────────────

// SearchAgents — CRITICAL: reads tenant from context, never from request body.
func (h *AgentRegistryHandlers) SearchAgents(w http.ResponseWriter, r *http.Request) {
	tenantID := mw.TenantIDFromContext(r.Context())
	if tenantID == "" {
		mw.WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", "X-Tenant-ID header is required", r.URL.Path)
		return
	}

	var req store.AgentSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}
	if req.TenantID != "" && req.TenantID != tenantID {
		h.writeError(w, http.StatusForbidden, "tenant_mismatch", "Forbidden", "Request tenant does not match context tenant")
		return
	}

	agents, _, err := h.AgentStore.List(r.Context(), "", "", "", 1, 100)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
		return
	}

	results := applySearchFilters(agents, &req)
	mw.WriteJSON(w, http.StatusOK, store.AgentSearchResponse{Results: results, Total: len(results)})
}

func applySearchFilters(agents []*store.Agent, req *store.AgentSearchRequest) []*store.Agent {
	var results []*store.Agent
	for _, a := range agents {
		if len(req.Capabilities) > 0 && !containsAny(a.Capabilities, req.Capabilities) {
			continue
		}
		if len(req.Tools) > 0 && !containsAny(a.Tools, req.Tools) {
			continue
		}
		if req.Status != nil && a.Status != *req.Status {
			continue
		}
		if req.DepartmentID != nil && (a.DepartmentID == nil || *a.DepartmentID != *req.DepartmentID) {
			continue
		}
		if len(req.SupportedLanguages) > 0 && !containsAny(a.SupportedLanguages, req.SupportedLanguages) {
			continue
		}
		results = append(results, a)
	}
	return results
}

func containsAny(list []string, targets []string) bool {
	for _, t := range targets {
		for _, v := range list {
			if v == t {
				return true
			}
		}
	}
	return false
}

// ─── Version CRUD ───────────────────────────────────────────────────────────

func (h *AgentRegistryHandlers) ListAgentVersions(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	status := r.URL.Query().Get("status")
	versions, err := h.VersionStore.ListByAgentAndStatus(r.Context(), agentID, status)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}
	mw.WriteJSON(w, http.StatusOK, VersionList{AgentID: agentID, Versions: versions})
}

func (h *AgentRegistryHandlers) CreateAgentVersion(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	exists, _ := h.AgentStore.Exists(r.Context(), agentID)
	if !exists {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}

	var req store.CreateVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	tenantID := mw.TenantIDFromContext(r.Context())
	previousVersion := ""
	allVers, _ := h.VersionStore.ListByAgent(r.Context(), agentID)
	if len(allVers) > 0 {
		previousVersion = allVers[len(allVers)-1].Version
	}

	t := now()
	version := &store.AgentVersion{
		ID:                uuid.New().String(),
		AgentID:           agentID,
		TenantID:          tenantID,
		Version:           req.Version,
		Status:            store.VersionStatusBeta,
		ModelConfig:       req.ModelConfig,
		PromptTemplateRef: req.PromptTemplateRef,
		Description:       req.Description,
		ChangeSummary:     req.ChangeSummary,
		CreatedBy:         mw.UserIDFromContext(r.Context()),
		PromotedTo:        make(map[string]string),
		CreatedAt:         t,
		UpdatedAt:         t,
	}

	if err := h.VersionStore.Create(r.Context(), version); err != nil {
		h.writeError(w, http.StatusConflict, "version_exists", "Conflict", err.Error())
		return
	}

	h.EventPublisher.PublishAgentVersionCreated(
		agentID, version.Version, previousVersion, version.ChangeSummary,
		string(version.Status), version.CreatedBy, nil, t,
	)
	h.writeJSON(w, http.StatusCreated, versionAPI{version})
}

func (h *AgentRegistryHandlers) GetAgentVersion(w http.ResponseWriter, r *http.Request) {
	versionID := extractVersionIDFromPath(r.URL.Path)
	if versionID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "version_id is required")
		return
	}
	version, err := h.VersionStore.GetByID(r.Context(), versionID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "version_not_found", "Not Found", "Version not found")
		return
	}
	h.writeJSON(w, http.StatusOK, versionAPI{version})
}

func (h *AgentRegistryHandlers) UpdateAgentVersion(w http.ResponseWriter, r *http.Request) {
	versionID := extractVersionIDFromPath(r.URL.Path)
	if versionID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "version_id is required")
		return
	}

	var req store.UpdateVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	err := h.VersionStore.Patch(r.Context(), versionID, func(v *store.AgentVersion) {
		if req.Status != nil {
			v.Status = *req.Status
		}
		if req.Description != nil {
			v.Description = *req.Description
		}
		if req.ChangeSummary != nil {
			v.ChangeSummary = *req.ChangeSummary
		}
		v.UpdatedAt = now()
	})
	if err != nil {
		h.writeError(w, http.StatusNotFound, "version_not_found", "Not Found", "Version not found")
		return
	}

	version, _ := h.VersionStore.GetByID(r.Context(), versionID)
	h.writeJSON(w, http.StatusOK, versionAPI{version})
}

func (h *AgentRegistryHandlers) PromoteVersion(w http.ResponseWriter, r *http.Request) {
	versionID := extractVersionIDFromPath(r.URL.Path)
	if versionID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "version_id is required")
		return
	}

	var req store.PromoteVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	if req.Environment != string(store.EnvironmentDev) && req.Environment != string(store.EnvironmentStaging) && req.Environment != string(store.EnvironmentProduction) {
		h.writeError(w, http.StatusBadRequest, "invalid_environment", "Bad Request", "environment must be dev, staging, or production")
		return
	}

	version, err := h.VersionStore.GetByID(r.Context(), versionID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "version_not_found", "Not Found", "Version not found")
		return
	}

	// SetPromoted marks a version as promoted in the given environment.
	// The fourth argument (promotedVersionID) is the version ID being promoted,
	// which is the same as the versionID here since we're promoting this specific version.
	err = h.VersionStore.SetPromoted(r.Context(), versionID, req.Environment, versionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
		return
	}

	// Derive from_env from the version's promotion history.
	// The version was previously promoted to the last environment in the pipeline
	// before the current target environment.
	fromEnv := deriveFromEnv(version.PromotedTo, req.Environment)

	h.EventPublisher.PublishAgentPromoted(version.AgentID, version.Version, mw.UserIDFromContext(r.Context()), fromEnv, req.Environment, now())
	h.writeJSON(w, http.StatusOK, versionAPI{version})
}

// deriveFromEnv returns the environment the version is being promoted from,
// based on its promotion history and target environment.
func deriveFromEnv(promotedTo map[string]string, toEnv string) string {
	if len(promotedTo) == 0 {
		return "none"
	}
	// Check pipeline order: dev → staging → production
	switch toEnv {
	case string(store.EnvironmentDev):
		return "none"
	case string(store.EnvironmentStaging):
		if _, hasDev := promotedTo[string(store.EnvironmentDev)]; hasDev {
			return string(store.EnvironmentDev)
		}
		return "none"
	case string(store.EnvironmentProduction):
		if _, hasStaging := promotedTo[string(store.EnvironmentStaging)]; hasStaging {
			return string(store.EnvironmentStaging)
		}
		if _, hasDev := promotedTo[string(store.EnvironmentDev)]; hasDev {
			return string(store.EnvironmentDev)
		}
		return "none"
	default:
		return "none"
	}
}

// ─── Capability CRUD ────────────────────────────────────────────────────────

func (h *AgentRegistryHandlers) ListAgentCapabilities(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	entries, err := h.CapabilityStore.ListAll(r.Context(), agentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}
	mw.WriteJSON(w, http.StatusOK, store.CapabilityList{AgentID: agentID, Capabilities: entries})
}

func (h *AgentRegistryHandlers) UpdateAgentCapabilities(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	var req store.CapabilityUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	tenantID := mw.TenantIDFromContext(r.Context())
	if len(req.Capabilities) > 0 {
		// Capture previous capabilities BEFORE upsert.
		var prevCaps []string
		if old, _ := h.CapabilityStore.ListAll(r.Context(), agentID); old != nil {
			for _, c := range old {
				prevCaps = append(prevCaps, c.Capability)
			}
		}

		var newCaps []string
		for _, cap := range req.Capabilities {
			entry := &store.CapabilityEntry{
				ID:            uuid.New().String(),
				AgentID:       agentID,
				TenantID:      tenantID,
				Capability:    cap.Capability,
				Score:         cap.Score,
				LastEvaluated: now(),
				Tier:          cap.Tier,
			}
			if err := h.CapabilityStore.Upsert(r.Context(), entry); err != nil {
				h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
				return
			}
			newCaps = append(newCaps, entry.Capability)
		}
		h.EventPublisher.PublishAgentCapabilitiesUpdated(agentID, tenantID, mw.UserIDFromContext(r.Context()), prevCaps, newCaps, now())
	}

	entries, _ := h.CapabilityStore.ListAll(r.Context(), agentID)
	mw.WriteJSON(w, http.StatusOK, store.CapabilityList{AgentID: agentID, Capabilities: entries})
}

func (h *AgentRegistryHandlers) IndexCapabilities(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}
	if err := h.CapabilityStore.Index(r.Context(), agentID); err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"indexing_started"}`))
}

// ─── Dependency CRUD ────────────────────────────────────────────────────────

func (h *AgentRegistryHandlers) ListDependencies(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	deps, err := h.DependencyStore.ListByAgent(r.Context(), agentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "agent_not_found", "Not Found", "Agent not found")
		return
	}
	mw.WriteJSON(w, http.StatusOK, store.DependencyList{Dependencies: deps})
}

func (h *AgentRegistryHandlers) AddDependency(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	var req store.AddDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "Invalid request body")
		return
	}

	switch req.DependencyType {
	case store.DependencyTypeHard, store.DependencyTypeSoft, store.DependencyTypeOptional:
		// valid
	default:
		h.writeError(w, http.StatusBadRequest, "invalid_dependency_type", "Bad Request", "dependency_type must be hard, soft, or optional")
		return
	}

	tenantID := mw.TenantIDFromContext(r.Context())
	dep := &store.AgentDependency{
		ID:                uuid.New().String(),
		AgentID:           agentID,
		TenantID:          tenantID,
		DependencyAgentID: req.DependencyID,
		DependencyType:    req.DependencyType,
		VersionConstraint: req.VersionConstraint,
		Description:       req.Description,
		Active:            true,
		CreatedAt:         now(),
	}

	if err := h.DependencyStore.Add(r.Context(), dep); err != nil {
		h.writeError(w, http.StatusConflict, "dependency_exists", "Conflict", err.Error())
		return
	}

	h.EventPublisher.PublishDependencyAdded(agentID, dep.DependencyAgentID, string(dep.DependencyType),
		func() string {
			if dep.VersionConstraint != nil {
				return *dep.VersionConstraint
			}
			return ""
		}(), mw.UserIDFromContext(r.Context()), now())

	h.writeJSON(w, http.StatusCreated, dependencyAPI{dep})
}

func (h *AgentRegistryHandlers) RemoveDependency(w http.ResponseWriter, r *http.Request) {
	agentID := extractAgentIDFromPath(r.URL.Path)
	if agentID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
		return
	}

	depID := r.URL.Query().Get("dependency_id")
	if depID == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Bad Request", "dependency_id query param is required")
		return
	}

	if err := h.DependencyStore.Remove(r.Context(), depID); err != nil {
		h.writeError(w, http.StatusNotFound, "dependency_not_found", "Not Found", "Dependency not found")
		return
	}

	h.EventPublisher.PublishDependencyRemoved(agentID, depID, mw.UserIDFromContext(r.Context()), "Dependency removed via API", now())
	w.WriteHeader(http.StatusNoContent)
}

// ─── Path helpers ───────────────────────────────────────────────────────────

func extractIDFromPath(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(s, "/", 2)
	return parts[0]
}

func extractAgentIDFromPath(path string) string {
	s := strings.TrimPrefix(path, "/registry/agents/")
	parts := strings.SplitN(s, "/", 2)
	return parts[0]
}

func extractVersionIDFromPath(path string) string {
	s := strings.TrimPrefix(path, "/registry/agents/")
	parts := strings.SplitN(s, "/", 3)
	if len(parts) < 3 || parts[1] != "versions" {
		return ""
	}
	// parts[2] could be "versionId" or "versionId/..." (e.g., "versionId/promote")
	return strings.SplitN(parts[2], "/", 2)[0]
}

// ─── DTOs (kept for backward compat) ───────────────────────────────────────

type AgentListResponse struct {
	Items   []*store.Agent `json:"items"`
	Total   int            `json:"total"`
	Page    int            `json:"page"`
	Size    int            `json:"page_size"`
	HasMore bool           `json:"has_more"`
}

type VersionList struct {
	AgentID  string                 `json:"agent_id"`
	Versions []*store.AgentVersion  `json:"versions"`
}

// ─── API response wrappers ─────────────────────────────────────────────────

type agentAPI struct {
	*store.Agent
}

type versionAPI struct {
	*store.AgentVersion
}

type dependencyAPI struct {
	*store.AgentDependency
}
