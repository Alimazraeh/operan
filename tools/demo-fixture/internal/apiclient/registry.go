package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

// RegistryClient talks to Module 04 (agent-registry). Base path confirmed
// literal "/registry/agents" (internal/handlers/router.go:29) — not
// /api/v1/agents. Every route requires Authorization: Bearer plus
// X-Tenant-ID; CreateAgent additionally requires the JWT to carry role
// "admin" or "registry_admin" (authWithRole wrapper on the route).
type RegistryClient struct {
	BaseURL string // e.g. http://agent-registry.operan.svc.cluster.local:8083
	Doer    *Doer
}

// CreateAgentRequest mirrors store.CreateAgentRequest
// (internal/store/models.go:145-171) — only the fields this tool sets. ID
// is the caller-supplied-id affordance this whole fixture format leans on:
// when set, it must be a UUID (agent_registry.go:137-143); a repeat
// registration under the same ID conflicts (409) rather than creating a
// duplicate. There is no status/active field on create — the server always
// sets status to "active" (agent_registry.go:153).
type CreateAgentRequest struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Description  string   `json:"description,omitempty"`
	TenantID     string   `json:"tenant_id"`
	Capabilities []string `json:"capabilities,omitempty"`
	Tools        []string `json:"tools,omitempty"`
}

// Agent mirrors store.Agent (internal/store/models.go:120-142) — only the
// fields this tool reads.
type Agent struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Description  string   `json:"description"`
	TenantID     string   `json:"tenant_id"`
	Status       string   `json:"status"`
	Capabilities []string `json:"capabilities"`
	Tools        []string `json:"tools"`
}

// CreateAgent calls POST /registry/agents. req.TenantID must equal the
// X-Tenant-ID header or the handler 400s (agent_registry.go:129-132).
func (c *RegistryClient) CreateAgent(ctx context.Context, token, tenantID string, req CreateAgentRequest) (*Agent, error) {
	req.TenantID = tenantID
	var out Agent
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/registry/agents", token, tenantID, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgent calls GET /registry/agents/{id}.
func (c *RegistryClient) GetAgent(ctx context.Context, token, tenantID, id string) (*Agent, error) {
	var out Agent
	_, err := c.Doer.Call(ctx, http.MethodGet, c.BaseURL+"/registry/agents/"+id, token, tenantID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type agentListResponse struct {
	Items []*Agent `json:"items"`
	Total int      `json:"total"`
}

// ListAgents calls GET /registry/agents?page=&page_size=.
func (c *RegistryClient) ListAgents(ctx context.Context, token, tenantID string, page, pageSize int) ([]*Agent, int, error) {
	url := fmt.Sprintf("%s/registry/agents?page=%d&page_size=%d", c.BaseURL, page, pageSize)
	var out agentListResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, url, token, tenantID, nil, &out)
	if err != nil {
		return nil, 0, err
	}
	return out.Items, out.Total, nil
}
