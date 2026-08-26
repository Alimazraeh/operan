package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

// DepartmentsClient talks to Module 05 (department-template-engine). Base
// path is bare — no /api/v1, no /v1 — confirmed literal
// (internal/handlers/router.go registers "/templates", "/departments",
// "/requests/", "/me/assignments" directly, mounted at root in main.go:150).
// Every route here requires Authorization: Bearer plus X-Tenant-ID; deploy
// additionally requires an "admin" role.
type DepartmentsClient struct {
	BaseURL string // e.g. http://department-templates.operan.svc.cluster.local:8005
	Doer    *Doer
}

// DeployRequest mirrors store.DeployRequest (store/models.go:448-453).
// There is no template_id field here — the template comes from the URL —
// and no id field: M05 has no caller-supplied-id affordance for
// departments, so the resulting department's id is only known from the
// response, never chosen in advance.
type DeployRequest struct {
	Environment    string `json:"environment"`
	DepartmentName string `json:"department_name,omitempty"`
}

// DeploymentResponse mirrors toDeploymentResponse (handlers/helpers.go:143-177)
// — only the fields this tool reads. DepartmentID is populated
// synchronously: MaterializeDepartment + DepartmentStore.Create both run
// before the handler responds (nested.go:150-164); only the *provisioning
// stages* continue in the background afterward, which is why restore still
// has to poll GetDepartment for Status to leave "provisioning".
type DeploymentResponse struct {
	ID           string `json:"id"`
	TemplateID   string `json:"template_id"`
	Status       string `json:"status"`
	Environment  string `json:"environment"`
	DepartmentID string `json:"department_id"`
}

// DeployTemplate calls POST /templates/{id}/deploy. Not idempotent: M05
// mints a new department (and a new deployment record) on every call
// (deploy/orchestrator.go — Department.ID is never set from the request, so
// DepartmentStore.Create always assigns a fresh UUID). Callers wanting
// idempotent restore behavior must find an existing department first (see
// FindDepartment) and only call this when none is found.
func (c *DepartmentsClient) DeployTemplate(ctx context.Context, token, tenantID, templateID string, req DeployRequest) (*DeploymentResponse, error) {
	var out DeploymentResponse
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/templates/"+templateID+"/deploy", token, tenantID, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DepartmentSummary mirrors toDepartmentSummary (handlers/departments.go:237-256)
// — the shape GET /departments (list) returns. Notably this does NOT
// include org_chart or services; fetching those requires GetDepartment.
type DepartmentSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Status       string `json:"status"`
	TemplateID   string `json:"template_id"`
	DeploymentID string `json:"deployment_id"`
}

type departmentListResponse struct {
	Data []*DepartmentSummary `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// ListDepartments calls GET /departments?page=&page_size=.
func (c *DepartmentsClient) ListDepartments(ctx context.Context, token, tenantID string, page, pageSize int) ([]*DepartmentSummary, int, error) {
	url := fmt.Sprintf("%s/departments?page=%d&page_size=%d", c.BaseURL, page, pageSize)
	var out departmentListResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, url, token, tenantID, nil, &out)
	if err != nil {
		return nil, 0, err
	}
	return out.Data, out.Meta.Total, nil
}

// FindDepartment pages through every department in the tenant looking for
// one deployed from templateID. When wantName is non-empty, it must also
// match the department's Name exactly — used to disambiguate a template
// deployed more than once under different names. When wantName is empty,
// any department deployed from templateID matches: an empty
// DeployRequest.department_name does NOT produce an empty Department.Name
// (M05 defaults it to the template's own name — deploy/orchestrator.go's
// MaterializeDepartment), so treating "" as "name must be empty" would
// never match anything for the common case of a demo tenant with exactly
// one deployment of a given template.
func (c *DepartmentsClient) FindDepartment(ctx context.Context, token, tenantID, templateID, wantName string) (*DepartmentSummary, error) {
	const pageSize = 50
	const maxPages = 200
	for page := 1; page <= maxPages; page++ {
		items, total, err := c.ListDepartments(ctx, token, tenantID, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, d := range items {
			if d.TemplateID != templateID {
				continue
			}
			if wantName == "" || d.Name == wantName {
				return d, nil
			}
		}
		if page*pageSize >= total || len(items) == 0 {
			break
		}
	}
	return nil, nil
}

// Position mirrors store.Position (store/models.go:101-115) — every field,
// since restore needs to enumerate positions to bind seats and export needs
// to read holder state back out.
type Position struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	RoleType         string   `json:"role_type"`
	HolderType       string   `json:"holder_type"`
	AgentID          string   `json:"agent_id,omitempty"`
	HumanRef         string   `json:"human_ref,omitempty"`
	ReportsTo        string   `json:"reports_to,omitempty"`
	AutonomyTier     string   `json:"autonomy_tier,omitempty"`
	ApprovalGateRefs []string `json:"approval_gate_refs,omitempty"`
}

// ServiceOffering mirrors store.ServiceOffering (store/models.go:125-137) —
// only the fields this tool reads.
type ServiceOffering struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	DeliveryWorkflowID string `json:"delivery_workflow_id,omitempty"`
	Status             string `json:"status,omitempty"`
}

// Department mirrors store.Department (store/models.go) — only the fields
// this tool reads. This is the response shape of GET /departments/{id},
// distinct from (and richer than) DepartmentSummary.
type Department struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Status      string            `json:"status"`
	TemplateID  string            `json:"template_id"`
	Environment string            `json:"environment"`
	OrgChart    []Position        `json:"org_chart"`
	Services    []ServiceOffering `json:"services"`
}

// GetDepartment calls GET /departments/{id}.
func (c *DepartmentsClient) GetDepartment(ctx context.Context, token, tenantID, id string) (*Department, error) {
	var out Department
	_, err := c.Doer.Call(ctx, http.MethodGet, c.BaseURL+"/departments/"+id, token, tenantID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SetHolderRequest mirrors setHolderBody (handlers/assignments.go:22-26).
type SetHolderRequest struct {
	HolderType string `json:"holder_type"` // human | ai_agent | vacant
	HumanRef   string `json:"human_ref,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
}

// OrgChartResponse mirrors orgChartResponse (handlers/departments.go:264-287).
type OrgChartResponse struct {
	RootPositionID string     `json:"root_position_id"`
	Positions      []Position `json:"positions"`
}

// SetPositionHolder calls PUT /departments/{id}/org-chart/{positionId}/holder.
// This is naturally idempotent — it sets a seat's holder rather than
// appending anything — so restore can call it unconditionally on every run.
func (c *DepartmentsClient) SetPositionHolder(ctx context.Context, token, tenantID, deptID, positionID string, req SetHolderRequest) (*OrgChartResponse, error) {
	var out OrgChartResponse
	url := c.BaseURL + "/departments/" + deptID + "/org-chart/" + positionID + "/holder"
	_, err := c.Doer.Call(ctx, http.MethodPut, url, token, tenantID, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SyncWorkflowsResponse mirrors the response of syncServiceWorkflows
// (handlers/departments.go:367-420).
type SyncWorkflowsResponse struct {
	TemplateVersion string   `json:"template_version"`
	Changed         int      `json:"changed"`
	Skipped         []string `json:"skipped"`
}

// SyncWorkflows calls POST /departments/{id}/services/sync-workflows with no
// body. Documented as idempotent and as refusing undefined SOPs (see
// handoff notes verified live on 2026-07-24); restore relies on that.
func (c *DepartmentsClient) SyncWorkflows(ctx context.Context, token, tenantID, deptID string) (*SyncWorkflowsResponse, error) {
	var out SyncWorkflowsResponse
	url := c.BaseURL + "/departments/" + deptID + "/services/sync-workflows"
	_, err := c.Doer.Call(ctx, http.MethodPost, url, token, tenantID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateRequestBody mirrors createRequestBody (handlers/requests.go:22-27).
type CreateRequestBody struct {
	ServiceID string `json:"service_id"`
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	Priority  string `json:"priority,omitempty"`
}

// RequestEvent mirrors store.RequestEvent (store/requests.go:55-60).
type RequestEvent struct {
	At     string `json:"at"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
	Node   string `json:"node,omitempty"`
}

// ServiceRequest mirrors store.ServiceRequest (store/requests.go:16-46) —
// only the fields this tool reads.
type ServiceRequest struct {
	ID           string         `json:"id"`
	DepartmentID string         `json:"department_id"`
	ServiceID    string         `json:"service_id"`
	Title        string         `json:"title"`
	Priority     string         `json:"priority"`
	Status       string         `json:"status"`
	Timeline     []RequestEvent `json:"timeline"`
}

// CreateRequest calls POST /departments/{id}/requests. Not idempotent — M05
// mints a new request on every call — which is intentional here: the
// replay step is meant to raise a fresh demonstration request each time
// it runs, not to reproduce a specific historical one (see
// fixture.HistoricalRequest's doc comment for why history is read-only).
// Requires the department to be "operational" or "degraded"
// (requests.go:34-38).
func (c *DepartmentsClient) CreateRequest(ctx context.Context, token, tenantID, deptID string, req CreateRequestBody) (*ServiceRequest, error) {
	var out ServiceRequest
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/departments/"+deptID+"/requests", token, tenantID, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRequest calls GET /requests/{id}.
func (c *DepartmentsClient) GetRequest(ctx context.Context, token, tenantID, id string) (*ServiceRequest, error) {
	var out ServiceRequest
	_, err := c.Doer.Call(ctx, http.MethodGet, c.BaseURL+"/requests/"+id, token, tenantID, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type requestListResponse struct {
	Data []*ServiceRequest `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// ListRequests calls GET /departments/{id}/requests?page=&page_size=. Used
// by export to capture request history; restore does not call this.
func (c *DepartmentsClient) ListRequests(ctx context.Context, token, tenantID, deptID string, page, pageSize int) ([]*ServiceRequest, int, error) {
	url := fmt.Sprintf("%s/departments/%s/requests?page=%d&page_size=%d", c.BaseURL, deptID, page, pageSize)
	var out requestListResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, url, token, tenantID, nil, &out)
	if err != nil {
		return nil, 0, err
	}
	return out.Data, out.Meta.Total, nil
}
