package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

// TenantClient talks to Module 01 (tenant-control-plane). Base path
// confirmed against modules/01-tenant-control-plane/cmd/tenant-control-plane/main.go:68
// (root.Handle("/", chain)) and internal/handler/response_types.go's route
// table, which registers every tenant route as "METHOD /v1/tenants/...".
// It is /v1, not bare and not /api/v1 — a detail worth stating explicitly
// because it is easy to get wrong by analogy with the other modules.
type TenantClient struct {
	BaseURL string // e.g. http://tenant-control-plane.operan.svc.cluster.local:8080
	Doer    *Doer
}

// CreateTenantRequest mirrors the anonymous struct read by M01's
// CreateTenant handler (handler_tenants.go:30-39).
type CreateTenantRequest struct {
	Name           string                 `json:"name"`
	DisplayName    string                 `json:"display_name,omitempty"`
	Plan           string                 `json:"plan"`
	Region         string                 `json:"region"`
	IsolationLevel string                 `json:"isolation_level"`
	ContactEmail   string                 `json:"contact_email,omitempty"`
	CustomMetadata map[string]interface{} `json:"custom_metadata,omitempty"`
}

// Tenant mirrors M01's TenantResponse (handler/response_types.go:15-28) —
// only the fields this tool uses.
type Tenant struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	Plan           string `json:"plan"`
	Region         string `json:"region"`
	IsolationLevel string `json:"isolation_level"`
	Status         string `json:"status"`
}

type tenantListResponse struct {
	Items []*Tenant `json:"items"`
	Total int       `json:"total"`
}

// CreateTenant calls POST /v1/tenants. M01 enforces Name uniqueness and
// answers 409 on a repeat — callers wanting idempotent behavior should use
// FindOrCreateTenant instead of calling this directly.
func (c *TenantClient) CreateTenant(ctx context.Context, token string, req CreateTenantRequest) (*Tenant, error) {
	var out Tenant
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/v1/tenants", token, "", req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTenants calls GET /v1/tenants. M01's list endpoint supports only
// page/page_size/status filters (handler_tenants.go:111-143) — there is no
// server-side name filter, so finding a tenant by name means paging through
// this and matching client-side, which FindTenantByName below does.
func (c *TenantClient) ListTenants(ctx context.Context, token string, page, pageSize int) ([]*Tenant, int, error) {
	url := fmt.Sprintf("%s/v1/tenants?page=%d&page_size=%d", c.BaseURL, page, pageSize)
	var out tenantListResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, url, token, "", nil, &out)
	if err != nil {
		return nil, 0, err
	}
	return out.Items, out.Total, nil
}

// FindTenantByName pages through every tenant M01 knows about looking for
// an exact name match. maxPages guards against an unbounded loop if the
// list endpoint's has_more/total accounting is ever wrong; 200 tenants at
// page_size 50 is far beyond anything a demo fixture needs.
func (c *TenantClient) FindTenantByName(ctx context.Context, token, name string) (*Tenant, error) {
	const pageSize = 50
	const maxPages = 200
	for page := 1; page <= maxPages; page++ {
		items, total, err := c.ListTenants(ctx, token, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, t := range items {
			if t.Name == name {
				return t, nil
			}
		}
		if page*pageSize >= total || len(items) == 0 {
			break
		}
	}
	return nil, nil // not found, not an error
}
