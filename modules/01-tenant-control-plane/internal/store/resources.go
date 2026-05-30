package store

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Resource types ──────────────────────────────────────────────────────────

// ResourceType represents a tenant-managed resource.
type ResourceType string

const (
	ResourceTypeDatabase     ResourceType = "database"
	ResourceTypeStorage      ResourceType = "storage"
	ResourceTypeCompute      ResourceType = "compute"
	ResourceTypeNetwork      ResourceType = "network"
	ResourceTypeKubernetes   ResourceType = "kubernetes_cluster"
	ResourceTypeOther        ResourceType = "other"
)

// ResourceStatus represents resource lifecycle.
type ResourceStatus string

const (
	ResourceStatusProvisioning  ResourceStatus = "provisioning"
	ResourceStatusProvisioned   ResourceStatus = "provisioned"
	ResourceStatusUpdating      ResourceStatus = "updating"
	ResourceStatusError         ResourceStatus = "error"
	ResourceStatusDeprovisioned ResourceStatus = "deprovisioned"
)

// ResourceSpec defines the configuration for a resource.
type ResourceSpec struct {
	Engine       string                 `json:"engine,omitempty"`
	Size         string                 `json:"size,omitempty"`
	VCPUs        int                    `json:"vcpus,omitempty"`
	RAMGB        int                    `json:"ram_gb,omitempty"`
	StorageGB    int                    `json:"storage_gb,omitempty"`
	Replicas     int                    `json:"replicas,omitempty"`
	ExtraConfig  map[string]interface{} `json:"extra_config,omitempty"`
}

// Resource represents a cloud resource managed for a tenant.
type Resource struct {
	ID            string        `json:"id"`
	TenantID      string        `json:"tenant_id"`
	Name          string        `json:"name"`
	Type          ResourceType  `json:"type"`
	Region        Region        `json:"region"`
	Spec          ResourceSpec  `json:"spec"`
	Status        ResourceStatus `json:"status"`
	Endpoint      string        `json:"endpoint"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// ResourcePatchRequest for partial updates.
type ResourcePatchRequest struct {
	Name   string      `json:"name,omitempty"`
	Spec   ResourceSpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// ─── ResourceStore ───────────────────────────────────────────────────────────

// ResourceStore manages tenant cloud resources.
type ResourceStore struct {
	mu       sync.RWMutex
	resources map[string]*Resource
	byTenant map[string][]string // keyed by TenantID -> ResourceIDs
}

// NewResourceStore creates a new ResourceStore.
func NewResourceStore() *ResourceStore {
	return &ResourceStore{
		resources: make(map[string]*Resource),
		byTenant:  make(map[string][]string),
	}
}

// Create adds a new resource.
func (s *ResourceStore) Create(res *Resource) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if res.ID == "" {
		res.ID = uuid.New().String()
	}
	if res.Status == "" {
		res.Status = ResourceStatusProvisioning
	}
	if res.CreatedAt.IsZero() {
		res.CreatedAt = timeNow()
	}
	res.UpdatedAt = timeNow()

	s.resources[res.ID] = res
	s.byTenant[res.TenantID] = append(s.byTenant[res.TenantID], res.ID)

	return res, nil
}

// GetByID retrieves a resource by ID (no tenant check — for admin use only).
func (s *ResourceStore) GetByID(id string) (*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, ok := s.resources[id]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", id)
	}
	cpy := *res
	return &cpy, nil
}

// GetByIDAndTenant retrieves a resource by ID and verifies the TenantID matches.
func (s *ResourceStore) GetByIDAndTenant(id, tenantID string) (*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, ok := s.resources[id]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", id)
	}
	if res.TenantID != tenantID {
		return nil, fmt.Errorf("permission denied: resource %s does not belong to tenant %s", id, tenantID)
	}
	cpy := *res
	return &cpy, nil
}

// Patch updates a resource.
func (s *ResourceStore) Patch(id string, req ResourcePatchRequest) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.resources[id]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", id)
	}

	if req.Name != "" {
		res.Name = req.Name
	}
	if req.Status != "" {
		res.Status = req.Status
	}
	if req.Spec.Engine != "" {
		res.Spec.Engine = req.Spec.Engine
	}
	if req.Spec.Size != "" {
		res.Spec.Size = req.Spec.Size
	}
	if req.Spec.VCPUs > 0 {
		res.Spec.VCPUs = req.Spec.VCPUs
	}
	if req.Spec.RAMGB > 0 {
		res.Spec.RAMGB = req.Spec.RAMGB
	}
	if req.Spec.StorageGB > 0 {
		res.Spec.StorageGB = req.Spec.StorageGB
	}
	if req.Spec.Replicas > 0 {
		res.Spec.Replicas = req.Spec.Replicas
	}
	if req.Spec.ExtraConfig != nil {
		if res.Spec.ExtraConfig == nil {
			res.Spec.ExtraConfig = make(map[string]interface{})
		}
		for k, v := range req.Spec.ExtraConfig {
			res.Spec.ExtraConfig[k] = v
		}
	}

	res.UpdatedAt = timeNow()

	return res, nil
}

// Delete removes a resource.
func (s *ResourceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.resources[id]
	if !ok {
		return fmt.Errorf("resource %s not found", id)
	}

	// Remove from byTenant
	tenantIDs, ok := s.byTenant[res.TenantID]
	if ok {
		idx := slices.Index(tenantIDs, id)
		if idx >= 0 {
			s.byTenant[res.TenantID] = append(tenantIDs[:idx], tenantIDs[idx+1:]...)
		}
	}

	delete(s.resources, id)

	return nil
}

// ListByTenant returns all resources for a tenant.
func (s *ResourceStore) ListByTenant(tenantID string, page, pageSize int) ([]*Resource, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Resource, 0, len(ids))
	for _, id := range ids {
		res, ok := s.resources[id]
		if !ok {
			continue
		}
		cpy := *res
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Resource) int {
		return a.NameCompare(b.Name)
	})

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := items[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

func (r *Resource) NameCompare(otherName string) int {
	if r.Name < otherName {
		return -1
	}
	if r.Name > otherName {
		return 1
	}
	return 0
}

// CountTotal returns the total number of resources.
func (s *ResourceStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.resources)
}

// ─── Agent types ─────────────────────────────────────────────────────────────

// AgentStatus represents an agent's operational state.
type AgentStatus string

const (
	AgentStatusReady    AgentStatus = "ready"
	AgentStatusRunning  AgentStatus = "running"
	AgentStatusPaused   AgentStatus = "paused"
	AgentStatusStopped  AgentStatus = "stopped"
	AgentStatusError    AgentStatus = "error"
	AgentStatusDraining AgentStatus = "draining"
)

// Agent represents a tenant's AI agent configuration.
type Agent struct {
	ID               string                 `json:"id"`
	TenantID         string                 `json:"tenant_id"`
	Name             string                 `json:"name"`
	Model            string                 `json:"model"`
	Role             string                 `json:"role"`
	SystemPrompt     string                 `json:"system_prompt"`
	Status           AgentStatus            `json:"status"`
	CurrentWorkflow  *string                `json:"current_workflow"`
	CurrentTask      *string                `json:"current_task"`
	ToolAccessJSON   json.RawMessage        `json:"tool_access_json,omitempty"`
	LastRunAt        *time.Time             `json:"last_run_at,omitempty"`
	SuccessCount     int                    `json:"success_count"`
	FailureCount     int                    `json:"failure_count"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// AgentPatchRequest for partial updates.
type AgentPatchRequest struct {
	Model           string        `json:"model,omitempty"`
	Role            string        `json:"role,omitempty"`
	SystemPrompt    string        `json:"system_prompt,omitempty"`
	Status          AgentStatus   `json:"status,omitempty"`
	ToolAccessJSON  json.RawMessage `json:"tool_access_json,omitempty"`
}

// ─── AgentStore ──────────────────────────────────────────────────────────────

// AgentStore manages tenant agents.
type AgentStore struct {
	mu       sync.RWMutex
	agents   map[string]*Agent
	byTenant map[string][]string // keyed by TenantID -> AgentIDs
}

// NewAgentStore creates a new AgentStore.
func NewAgentStore() *AgentStore {
	return &AgentStore{
		agents:   make(map[string]*Agent),
		byTenant: make(map[string][]string),
	}
}

// Create adds a new agent.
func (s *AgentStore) Create(agent *Agent) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}
	if agent.Status == "" {
		agent.Status = AgentStatusReady
	}
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = timeNow()
	}
	agent.UpdatedAt = timeNow()

	s.agents[agent.ID] = agent
	s.byTenant[agent.TenantID] = append(s.byTenant[agent.TenantID], agent.ID)

	return agent, nil
}

// GetByID retrieves an agent by ID (no tenant check — for admin use only).
func (s *AgentStore) GetByID(id string) (*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, ok := s.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", id)
	}
	cpy := *agent
	return &cpy, nil
}

// GetByIDAndTenant retrieves an agent by ID and verifies the TenantID matches.
func (s *AgentStore) GetByIDAndTenant(id, tenantID string) (*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, ok := s.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", id)
	}
	if agent.TenantID != tenantID {
		return nil, fmt.Errorf("permission denied: agent %s does not belong to tenant %s", id, tenantID)
	}
	cpy := *agent
	return &cpy, nil
}

// Patch updates an agent.
func (s *AgentStore) Patch(id string, req AgentPatchRequest) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", id)
	}

	if req.Model != "" {
		agent.Model = req.Model
	}
	if req.Role != "" {
		agent.Role = req.Role
	}
	if req.SystemPrompt != "" {
		agent.SystemPrompt = req.SystemPrompt
	}
	if req.Status != "" {
		agent.Status = req.Status
	}
	if req.ToolAccessJSON != nil {
		agent.ToolAccessJSON = req.ToolAccessJSON
	}

	agent.UpdatedAt = timeNow()

	return agent, nil
}

// Delete removes an agent.
func (s *AgentStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[id]
	if !ok {
		return fmt.Errorf("agent %s not found", id)
	}

	// Remove from byTenant
	tenantIDs, ok := s.byTenant[agent.TenantID]
	if ok {
		idx := slices.Index(tenantIDs, id)
		if idx >= 0 {
			s.byTenant[agent.TenantID] = append(tenantIDs[:idx], tenantIDs[idx+1:]...)
		}
	}

	delete(s.agents, id)

	return nil
}

// ListByTenant returns all agents for a tenant.
func (s *AgentStore) ListByTenant(tenantID string, page, pageSize int) ([]*Agent, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Agent, 0, len(ids))
	for _, id := range ids {
		agent, ok := s.agents[id]
		if !ok {
			continue
		}
		cpy := *agent
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Agent) int {
		return a.NameCompare(b.Name)
	})

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := items[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

func (a *Agent) NameCompare(otherName string) int {
	if a.Name < otherName {
		return -1
	}
	if a.Name > otherName {
		return 1
	}
	return 0
}

// CountTotal returns the total number of agents.
func (s *AgentStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}
