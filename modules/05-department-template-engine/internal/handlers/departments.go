package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// Department resource handlers: the living department instances materialized
// by deploys (or created directly), exposing the operating model — org chart,
// service portfolio, value chain, risk register, quality standards and
// compliance controls — as first-class REST resources.

// ListDepartments handles GET /departments.
func (h *TemplateHandlers) ListDepartments(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := parsePositiveInt(p); err == nil {
			page = n
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if n, err := parsePositiveInt(ps); err == nil {
			pageSize = n
		}
	}
	if pageSize > h.MaxPageSize {
		pageSize = h.MaxPageSize
	}
	category := r.URL.Query().Get("category")
	status := r.URL.Query().Get("status")

	departments, total, hasMore := h.DepartmentStore.List(tenantID, page, pageSize, &category, &status)

	data := make([]interface{}, 0, len(departments))
	for i := range departments {
		data = append(data, toDepartmentSummary(&departments[i]))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": data,
		"meta": map[string]interface{}{
			"total": total, "page": page, "page_size": pageSize, "has_more": hasMore,
		},
	})
}

// CreateDepartment handles POST /departments (direct, no-template path).
func (h *TemplateHandlers) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	tenantID := middleware.TenantIDFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Failed to read request body", r.URL.Path, reqID)
		return
	}
	defer r.Body.Close()

	var dept store.Department
	if err := json.Unmarshal(body, &dept); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid JSON body", r.URL.Path, reqID)
		return
	}
	if dept.Name == "" || dept.Category == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"name and category are required", r.URL.Path, reqID)
		return
	}

	dept.ID = ""
	dept.TenantID = tenantID
	dept.CreatedBy = middleware.UserIDFromContext(r.Context())
	if dept.Status == "" {
		dept.Status = "operational" // direct creation skips provisioning
	}

	created, err := h.DepartmentStore.Create(&dept)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"Failed to create department", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishDepartmentCreated(events.DepartmentLifecyclePayload{
		DepartmentID: created.ID, TenantID: tenantID, Name: created.Name,
		Category: created.Category, Status: created.Status, Timestamp: time.Now(),
	})
	writeJSON(w, http.StatusCreated, created)
}

// HandleDepartmentByID dispatches /departments/{id} and its nested resources.
func (h *TemplateHandlers) HandleDepartmentByID(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	tenantID := middleware.TenantIDFromContext(r.Context())

	rest := strings.TrimPrefix(r.URL.Path, "/departments/")
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid department ID", r.URL.Path, reqID)
		return
	}

	dept, err := h.DepartmentStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Department not found", r.URL.Path, reqID)
		return
	}

	switch {
	case sub == "" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, dept)

	case sub == "" && r.Method == http.MethodPatch:
		h.patchDepartment(w, r, reqID, dept)

	case sub == "" && r.Method == http.MethodDelete:
		h.archiveDepartment(w, r, reqID, dept)

	case sub == "org-chart" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, orgChartResponse(dept))

	case sub == "services" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": orEmptyServices(dept.Services),
			"meta": map[string]interface{}{"total": len(dept.Services)},
		})

	case sub == "value-chain" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, valueChainResponse(dept))

	case sub == "risks" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": orEmptyRisks(dept.Risks),
			"meta": map[string]interface{}{"total": len(dept.Risks)},
		})

	case sub == "quality" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": orEmptyQuality(dept.QualityStandards),
			"meta": map[string]interface{}{"total": len(dept.QualityStandards)},
		})

	case sub == "compliance" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, complianceResponse(dept))

	case sub == "kpi-measurements" && r.Method == http.MethodGet:
		h.kpiMeasurements(w, r, dept)

	case sub == "requests" && r.Method == http.MethodGet:
		h.listRequests(w, r, dept)

	case sub == "requests" && r.Method == http.MethodPost:
		h.createRequest(w, r, reqID, dept)

	default:
		writeError(w, http.StatusMethodNotAllowed, "about:blank", "Method Not Allowed",
			"Invalid operation", r.URL.Path, reqID)
	}
}

func (h *TemplateHandlers) patchDepartment(w http.ResponseWriter, r *http.Request, reqID string, dept *store.Department) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Failed to read request body", r.URL.Path, reqID)
		return
	}
	defer r.Body.Close()

	var patch map[string]interface{}
	if err := json.Unmarshal(body, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"Invalid JSON body", r.URL.Path, reqID)
		return
	}

	updated, err := h.DepartmentStore.UpdateByTenant(dept.ID, dept.TenantID, patch)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Department not found", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishDepartmentUpdated(events.DepartmentLifecyclePayload{
		DepartmentID: updated.ID, TenantID: updated.TenantID, Name: updated.Name,
		Category: updated.Category, Status: updated.Status, Timestamp: time.Now(),
	})
	writeJSON(w, http.StatusOK, updated)
}

func (h *TemplateHandlers) archiveDepartment(w http.ResponseWriter, r *http.Request, reqID string, dept *store.Department) {
	archived, err := h.DepartmentStore.Archive(dept.ID, dept.TenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Department not found", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishDepartmentArchived(events.DepartmentLifecyclePayload{
		DepartmentID: archived.ID, TenantID: archived.TenantID, Name: archived.Name,
		Category: archived.Category, Status: "archived", Timestamp: time.Now(),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id": archived.ID, "status": "archived",
		"note": "agents remain registered in Module 04; deactivate via the registry if required",
	})
}

// ─── Response shapes ─────────────────────────────────────────────────────────

func toDepartmentSummary(d *store.Department) map[string]interface{} {
	return map[string]interface{}{
		"id":            d.ID,
		"name":          d.Name,
		"slug":          d.Slug,
		"category":      d.Category,
		"description":   d.Description,
		"status":        d.Status,
		"mission":       d.Mission,
		"template_id":   d.TemplateID,
		"deployment_id": d.DeploymentID,
		"environment":   d.Environment,
		"agents_count":  len(d.AgentIDs),
		"services_count": len(d.Services),
		"positions_count": len(d.OrgChart),
		"risks_count":   len(d.Risks),
		"created_at":    d.CreatedAt,
		"updated_at":    d.UpdatedAt,
	}
}

type orgEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // reports_to, escalates_to
}

func orgChartResponse(d *store.Department) map[string]interface{} {
	positions := d.OrgChart
	if positions == nil {
		positions = []store.Position{}
	}
	edges := []orgEdge{}
	root := ""
	for _, p := range positions {
		if p.ReportsTo == "" && root == "" {
			root = p.ID
		}
		if p.ReportsTo != "" {
			edges = append(edges, orgEdge{From: p.ID, To: p.ReportsTo, Type: "reports_to"})
		}
		if p.EscalatesTo != "" {
			edges = append(edges, orgEdge{From: p.ID, To: p.EscalatesTo, Type: "escalates_to"})
		}
	}
	return map[string]interface{}{
		"root_position_id": root,
		"positions":        positions,
		"edges":            edges,
	}
}

func valueChainResponse(d *store.Department) map[string]interface{} {
	streams := d.ValueStreams
	if streams == nil {
		streams = []store.ValueStream{}
	}
	kpiIndex := map[string]store.KPIDefinition{}
	for _, k := range d.KPIS {
		kpiIndex[k.ID] = k
	}
	return map[string]interface{}{
		"value_streams": streams,
		"kpi_index":     kpiIndex,
	}
}

func complianceResponse(d *store.Department) map[string]interface{} {
	controls := d.ComplianceControls
	if controls == nil {
		controls = []store.ComplianceControl{}
	}
	frameworks := map[string][]store.ComplianceControl{}
	for _, c := range controls {
		frameworks[c.Framework] = append(frameworks[c.Framework], c)
	}
	names := make([]string, 0, len(frameworks))
	for f := range frameworks {
		names = append(names, f)
	}
	return map[string]interface{}{
		"frameworks": names,
		"by_framework": frameworks,
		"controls":   controls,
		"governance_rules": orEmptyRules(d.GovernanceRules),
	}
}

func orEmptyServices(s []store.ServiceOffering) []store.ServiceOffering {
	if s == nil {
		return []store.ServiceOffering{}
	}
	return s
}

func orEmptyRisks(s []store.RiskItem) []store.RiskItem {
	if s == nil {
		return []store.RiskItem{}
	}
	return s
}

func orEmptyQuality(s []store.QualityStandard) []store.QualityStandard {
	if s == nil {
		return []store.QualityStandard{}
	}
	return s
}

func orEmptyRules(s []store.GovernanceRule) []store.GovernanceRule {
	if s == nil {
		return []store.GovernanceRule{}
	}
	return s
}
