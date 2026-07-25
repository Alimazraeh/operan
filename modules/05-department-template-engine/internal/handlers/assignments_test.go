package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/ctxkeys"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func deptWithSeats(t *testing.T, h *TemplateHandlers) *store.Department {
	t.Helper()
	d, err := h.DepartmentStore.Create(&store.Department{
		TenantID: "t1", Name: "IT", Status: "operational",
		OrgChart: []store.Position{
			{ID: "pos-head", Title: "IT Manager", RoleType: "manager", HolderType: "vacant",
				AutonomyTier: "coordinate", ApprovalGateRefs: []string{"it-high-risk"},
				DecisionRights: []store.DecisionRight{{Decision: "approve changes", Authority: "decide"}}},
			{ID: "pos-eng", Title: "Engineer", RoleType: "specialist", HolderType: "ai_agent",
				ReportsTo: "pos-head", AgentID: "agent-1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func putHolder(h *TemplateHandlers, d *store.Department, pos string, body map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/departments/"+d.ID+"/org-chart/"+pos+"/holder", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.setPositionHolder(w, req, d, pos)
	return w
}

// A seat may only be bound to somebody who exists — a binding pointing at an
// id nobody can produce looks staffed and grants nothing.
func TestSetHolderRejectsUnknownPosition(t *testing.T) {
	h := &TemplateHandlers{DepartmentStore: store.NewDepartmentStore()}
	d := deptWithSeats(t, h)
	if w := putHolder(h, d, "pos-nope", map[string]string{"holder_type": "vacant"}); w.Code != 404 {
		t.Errorf("unknown position: status %d, want 404", w.Code)
	}
}

func TestSetHolderValidatesInput(t *testing.T) {
	h := &TemplateHandlers{DepartmentStore: store.NewDepartmentStore()}
	d := deptWithSeats(t, h)

	if w := putHolder(h, d, "pos-head", map[string]string{"holder_type": "wizard"}); w.Code != 422 {
		t.Errorf("bad holder_type: status %d, want 422", w.Code)
	}
	// human without a ref is refused before any identity lookup
	if w := putHolder(h, d, "pos-head", map[string]string{"holder_type": "human"}); w.Code != 422 {
		t.Errorf("human with no ref: status %d, want 422", w.Code)
	}
	// with no identity client configured, a human binding cannot be verified
	// and must not be accepted on trust
	if w := putHolder(h, d, "pos-head", map[string]string{
		"holder_type": "human", "human_ref": "someone"}); w.Code != 503 {
		t.Errorf("unverifiable human binding: status %d, want 503", w.Code)
	}
}

func TestSetHolderVacantAndAgentClearTheOther(t *testing.T) {
	h := &TemplateHandlers{DepartmentStore: store.NewDepartmentStore()}
	d := deptWithSeats(t, h)

	if w := putHolder(h, d, "pos-head", map[string]string{
		"holder_type": "ai_agent", "agent_id": "agent-9"}); w.Code != 200 {
		t.Fatalf("bind agent: status %d: %s", w.Code, w.Body.String())
	}
	got, _ := h.DepartmentStore.GetByIDAndTenant(d.ID, "t1")
	if got.OrgChart[0].AgentID != "agent-9" || got.OrgChart[0].HumanRef != "" {
		t.Errorf("agent binding wrong: %+v", got.OrgChart[0])
	}

	if w := putHolder(h, got, "pos-head", map[string]string{"holder_type": "vacant"}); w.Code != 200 {
		t.Fatalf("vacate: status %d", w.Code)
	}
	got2, _ := h.DepartmentStore.GetByIDAndTenant(d.ID, "t1")
	if got2.OrgChart[0].AgentID != "" || got2.OrgChart[0].HumanRef != "" ||
		got2.OrgChart[0].HolderType != "vacant" {
		t.Errorf("vacating left a holder: %+v", got2.OrgChart[0])
	}
}

// /me/assignments is the query that makes the org chart the authorization
// graph: it must return the caller's seats and the authority each carries.
func TestMeAssignmentsReturnsHeldSeatsWithAuthority(t *testing.T) {
	h := &TemplateHandlers{DepartmentStore: store.NewDepartmentStore()}
	d := deptWithSeats(t, h)
	// Bind the head seat to a person directly in the store (the endpoint's
	// identity check is covered above).
	d.OrgChart[0].HolderType = "human"
	d.OrgChart[0].HumanRef = "user-dana"
	h.DepartmentStore.UpdateByTenant(d.ID, "t1", map[string]interface{}{"org_chart": d.OrgChart})

	req := httptest.NewRequest("GET", "/me/assignments", nil)
	ctx := ctxkeys.WithTenantID(context.Background(), "t1")
	ctx = ctxkeys.WithUserID(ctx, "user-dana")
	w := httptest.NewRecorder()
	h.MeAssignments(w, req.WithContext(ctx))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []Assignment `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Data) != 1 {
		t.Fatalf("assignments = %d, want 1: %s", len(out.Data), w.Body.String())
	}
	a := out.Data[0]
	if a.PositionID != "pos-head" || a.DepartmentID != d.ID {
		t.Errorf("wrong seat: %+v", a)
	}
	if !a.IsDepartmentRoot {
		t.Error("a seat with no reports_to is the department root — that is what makes them the head")
	}
	if len(a.DecisionRights) != 1 || a.DecisionRights[0].Authority != "decide" {
		t.Errorf("authority not carried through: %+v", a.DecisionRights)
	}
	if len(a.ApprovalGateRefs) != 1 {
		t.Errorf("gate refs not carried through: %+v", a.ApprovalGateRefs)
	}

	// Somebody else holds nothing here.
	ctx2 := ctxkeys.WithUserID(ctxkeys.WithTenantID(context.Background(), "t1"), "user-someone-else")
	w2 := httptest.NewRecorder()
	h.MeAssignments(w2, req.WithContext(ctx2))
	json.Unmarshal(w2.Body.Bytes(), &out)
	if len(out.Data) != 0 {
		t.Errorf("a user with no seats got %d assignments", len(out.Data))
	}
}

func TestMeAssignmentsRequiresAuthentication(t *testing.T) {
	h := &TemplateHandlers{DepartmentStore: store.NewDepartmentStore()}
	req := httptest.NewRequest("GET", "/me/assignments", nil)
	ctx := ctxkeys.WithTenantID(context.Background(), "t1")
	w := httptest.NewRecorder()
	h.MeAssignments(w, req.WithContext(ctx))
	if w.Code != 401 {
		t.Errorf("status %d, want 401", w.Code)
	}
}
