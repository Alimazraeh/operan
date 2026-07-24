package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func seedOperationalDepartment(t *testing.T, h *TemplateHandlers) *store.Department {
	t.Helper()
	h.DepartmentStore = store.NewDepartmentStore()
	h.RequestStore = store.NewRequestStore()
	dept, err := h.DepartmentStore.Create(&store.Department{
		TenantID: "tenant-1",
		Name:     "IT Department — Test",
		Category: "it",
		Status:   "operational",
		Services: []store.ServiceOffering{{
			ID:   "svc-desk",
			Name: "Service Desk",
			SLA:  &store.SLA{ResponseTime: "15m (P1) / 4h (P3)", ResolutionTime: "4h (P1) / 2d (P3)"},
		}},
	})
	if err != nil {
		t.Fatalf("seed department: %v", err)
	}
	return dept
}

func TestCreateAndListRequests(t *testing.T) {
	h := newTestHandlers(t)
	dept := seedOperationalDepartment(t, h)

	req, _ := testRequest(http.MethodPost, "/departments/"+dept.ID+"/requests", map[string]any{
		"service_id": "svc-desk", "title": "VPN down for finance team", "priority": "P1",
	})
	w := httptest.NewRecorder()
	h.HandleDepartmentByID(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create request: %d %s", w.Code, w.Body.String())
	}
	var created store.ServiceRequest
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Status != "open" || created.ServiceName != "Service Desk" {
		t.Errorf("created = %+v", created)
	}
	// P1 → response SLA 15m from now.
	if created.SLAResponseDue == nil || time.Until(*created.SLAResponseDue) > 16*time.Minute {
		t.Errorf("P1 response SLA wrong: %v", created.SLAResponseDue)
	}
	// Dispatcher is nil in tests → honest timeline note.
	found := false
	for _, ev := range created.Timeline {
		if ev.Kind == "note" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dispatcher-not-configured note, timeline = %+v", created.Timeline)
	}

	// List through the department nested route.
	lreq, _ := testRequest(http.MethodGet, "/departments/"+dept.ID+"/requests", nil)
	lw := httptest.NewRecorder()
	h.HandleDepartmentByID(lw, lreq)
	if lw.Code != http.StatusOK {
		t.Fatalf("list: %d %s", lw.Code, lw.Body.String())
	}
	var listResp struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	json.Unmarshal(lw.Body.Bytes(), &listResp)
	if listResp.Meta.Total != 1 || len(listResp.Data) != 1 {
		t.Fatalf("list total = %+v", listResp.Meta)
	}

	// Detail + cancel.
	dreq, _ := testRequest(http.MethodGet, "/requests/"+created.ID, nil)
	dw := httptest.NewRecorder()
	h.HandleRequestByID(dw, dreq)
	if dw.Code != http.StatusOK {
		t.Fatalf("detail: %d", dw.Code)
	}
	creq, _ := testRequest(http.MethodPost, "/requests/"+created.ID+"/cancel", nil)
	cw := httptest.NewRecorder()
	h.HandleRequestByID(cw, creq)
	if cw.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", cw.Code, cw.Body.String())
	}
	var cancelled store.ServiceRequest
	json.Unmarshal(cw.Body.Bytes(), &cancelled)
	if cancelled.Status != "cancelled" {
		t.Errorf("status after cancel = %s", cancelled.Status)
	}
	// Cancel again → 409.
	creq2, _ := testRequest(http.MethodPost, "/requests/"+created.ID+"/cancel", nil)
	cw2 := httptest.NewRecorder()
	h.HandleRequestByID(cw2, creq2)
	if cw2.Code != http.StatusConflict {
		t.Errorf("double cancel = %d, want 409", cw2.Code)
	}
}

func TestCreateRequestValidation(t *testing.T) {
	h := newTestHandlers(t)
	dept := seedOperationalDepartment(t, h)

	// Unknown service.
	req, _ := testRequest(http.MethodPost, "/departments/"+dept.ID+"/requests", map[string]any{
		"service_id": "nope", "title": "x",
	})
	w := httptest.NewRecorder()
	h.HandleDepartmentByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown service = %d, want 400", w.Code)
	}

	// Missing title.
	req2, _ := testRequest(http.MethodPost, "/departments/"+dept.ID+"/requests", map[string]any{
		"service_id": "svc-desk",
	})
	w2 := httptest.NewRecorder()
	h.HandleDepartmentByID(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("missing title = %d, want 400", w2.Code)
	}

	// Non-operational department.
	h.DepartmentStore.UpdateStatus(dept.ID, "tenant-1", "archived")
	req3, _ := testRequest(http.MethodPost, "/departments/"+dept.ID+"/requests", map[string]any{
		"service_id": "svc-desk", "title": "x",
	})
	w3 := httptest.NewRecorder()
	h.HandleDepartmentByID(w3, req3)
	if w3.Code != http.StatusConflict {
		t.Errorf("archived dept = %d, want 409", w3.Code)
	}
}
