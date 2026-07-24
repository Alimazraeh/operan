package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/sla"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// RequestDispatcher starts the execution run for a freshly created request.
// Implemented by internal/workloop (the work-loop runtime); nil disables
// dispatch (the request stays open with an honest timeline note).
type RequestDispatcher interface {
	Dispatch(auth, tenantID string, req *store.ServiceRequest, dept *store.Department)
}

type createRequestBody struct {
	ServiceID string `json:"service_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Priority  string `json:"priority"`
}

// createRequest handles POST /departments/{id}/requests (dispatched from
// HandleDepartmentByID with the department already tenant-verified).
func (h *TemplateHandlers) createRequest(w http.ResponseWriter, r *http.Request, reqID string, dept *store.Department) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	if dept.Status != "operational" && dept.Status != "degraded" {
		writeError(w, http.StatusConflict, "about:blank", "Conflict",
			"department is "+dept.Status+" — requests need an operational department", r.URL.Path, reqID)
		return
	}

	var body createRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"invalid JSON body", r.URL.Path, reqID)
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"title is required", r.URL.Path, reqID)
		return
	}

	// The request must target one of the department's services.
	var svc *store.ServiceOffering
	for i := range dept.Services {
		if dept.Services[i].ID == body.ServiceID {
			svc = &dept.Services[i]
			break
		}
	}
	if svc == nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"service_id does not belong to this department", r.URL.Path, reqID)
		return
	}

	req := &store.ServiceRequest{
		TenantID:     tenantID,
		DepartmentID: dept.ID,
		ServiceID:    svc.ID,
		ServiceName:  svc.Name,
		Title:        strings.TrimSpace(body.Title),
		Body:         body.Body,
		Priority:     body.Priority,
		Requester: store.Requester{
			UserID: middleware.UserIDFromContext(r.Context()),
		},
	}

	// SLA clocks from the service's declared levels.
	now := time.Now().UTC()
	if svc.SLA != nil {
		if d, ok := sla.Parse(svc.SLA.ResponseTime, body.Priority); ok {
			due := now.Add(d)
			req.SLAResponseDue = &due
		}
		if d, ok := sla.Parse(svc.SLA.ResolutionTime, body.Priority); ok {
			due := now.Add(d)
			req.SLAResolutionDue = &due
		}
	}

	created, err := h.RequestStore.Create(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
			"failed to create request", r.URL.Path, reqID)
		return
	}

	h.EventPublisher.PublishRequestCreated(events.RequestLifecyclePayload{
		RequestID: created.ID, TenantID: tenantID, DepartmentID: dept.ID,
		ServiceID: svc.ID, Title: created.Title, Status: created.Status,
		Timestamp: now,
	})

	if h.Dispatcher != nil {
		h.Dispatcher.Dispatch(r.Header.Get("Authorization"), tenantID, created, dept)
	} else {
		h.RequestStore.AppendEvent(created.ID, store.RequestEvent{
			Kind: "note", Detail: "dispatcher not configured — request awaiting manual handling",
		})
	}

	// Re-read so the response includes any dispatch timeline entries.
	fresh, err := h.RequestStore.GetByIDAndTenant(created.ID, tenantID)
	if err != nil {
		fresh = created
	}
	writeJSON(w, http.StatusCreated, fresh)
}

// listRequests handles GET /departments/{id}/requests.
func (h *TemplateHandlers) listRequests(w http.ResponseWriter, r *http.Request, dept *store.Department) {
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
	var statusFilter *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = &s
	}

	items, total, hasMore := h.RequestStore.ListByDepartment(tenantID, dept.ID, statusFilter, page, pageSize)
	data := make([]interface{}, 0, len(items))
	for i := range items {
		data = append(data, withSLAFlags(&items[i]))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": data,
		"meta": map[string]interface{}{
			"page": page, "page_size": pageSize, "total": total, "has_more": hasMore,
		},
	})
}

// HandleRequestByID handles GET /requests/{id} and POST /requests/{id}/cancel.
func (h *TemplateHandlers) HandleRequestByID(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	tenantID := middleware.TenantIDFromContext(r.Context())

	rest := strings.TrimPrefix(r.URL.Path, "/requests/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"request id required", r.URL.Path, reqID)
		return
	}
	id := parts[0]

	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		req, err := h.RequestStore.GetByIDAndTenant(id, tenantID)
		if err != nil {
			writeError(w, http.StatusNotFound, "about:blank", "Not Found",
				"request not found", r.URL.Path, reqID)
			return
		}
		writeJSON(w, http.StatusOK, withSLAFlags(req))

	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "cancel":
		req, err := h.RequestStore.GetByIDAndTenant(id, tenantID)
		if err != nil {
			writeError(w, http.StatusNotFound, "about:blank", "Not Found",
				"request not found", r.URL.Path, reqID)
			return
		}
		if store.TerminalRequestStatus(req.Status) {
			writeError(w, http.StatusConflict, "about:blank", "Conflict",
				"request already "+req.Status, r.URL.Path, reqID)
			return
		}
		h.RequestStore.Mutate(id, func(sr *store.ServiceRequest) {
			sr.Status = "cancelled"
			sr.Timeline = append(sr.Timeline, store.RequestEvent{
				At: time.Now().UTC(), Kind: "cancelled", Detail: "cancelled by requester"})
		})
		updated, _ := h.RequestStore.GetByIDAndTenant(id, tenantID)
		writeJSON(w, http.StatusOK, updated)

	default:
		writeError(w, http.StatusMethodNotAllowed, "about:blank", "Method Not Allowed",
			"unsupported request operation", r.URL.Path, reqID)
	}
}

// withSLAFlags decorates a request with computed SLA-breach booleans.
func withSLAFlags(req *store.ServiceRequest) map[string]interface{} {
	b, _ := json.Marshal(req)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	now := time.Now().UTC()
	if req.SLAResponseDue != nil && req.FirstResponseAt == nil && now.After(*req.SLAResponseDue) {
		m["sla_response_breached"] = true
	}
	if req.SLAResolutionDue != nil && req.CompletedAt == nil && !store.TerminalRequestStatus(req.Status) && now.After(*req.SLAResolutionDue) {
		m["sla_resolution_breached"] = true
	}
	return m
}
