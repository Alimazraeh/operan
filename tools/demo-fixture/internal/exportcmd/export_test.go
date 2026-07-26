package exportcmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/tools/demo-fixture/internal/apiclient"
	"github.com/operan/tools/demo-fixture/internal/fixture"
)

// newMockPlatform builds a single httptest.Server that answers every route
// Run touches, standing in for all five modules at once (their paths never
// collide, so one mux is enough and keeps the test readable).
func newMockPlatform(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/iam/admin/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(apiclient.AdminLoginResponse{Token: "admin-jwt", UserID: "admin-001"})
	})

	mux.HandleFunc("GET /v1/tenants", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Items []*apiclient.Tenant `json:"items"`
			Total int                 `json:"total"`
		}{
			Items: []*apiclient.Tenant{{
				ID: "t-1", Name: "smoke-tenant", DisplayName: "Smoke Tenant",
				Plan: "saas", Region: "me-east-1", IsolationLevel: "namespace",
			}},
			Total: 1,
		})
	})

	mux.HandleFunc("GET /api/v1/iam/users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Users []*apiclient.User `json:"users"`
			Total int               `json:"total"`
		}{
			Users: []*apiclient.User{{
				ID: "u-dana", Email: "dana@adri.nz", DisplayName: "Dana Q",
				RoleIDs: []string{"department_head"},
			}},
			Total: 1,
		})
	})

	mux.HandleFunc("GET /departments", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []*apiclient.DepartmentSummary `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}{
			Data: []*apiclient.DepartmentSummary{{
				ID: "d-1", Name: "IT Department", TemplateID: "it-medium-001",
			}},
		})
	})

	mux.HandleFunc("GET /departments/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "d-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(apiclient.Department{
			ID: "d-1", Name: "IT Department", TemplateID: "it-medium-001", Environment: "production",
			OrgChart: []apiclient.Position{
				{ID: "pos-it-manager-01", Title: "IT Manager", HolderType: "human", HumanRef: "u-dana"},
				{ID: "pos-triage-01", Title: "Triage", HolderType: "ai_agent", AgentID: "a-1"},
				{ID: "pos-vacant-01", Title: "Spare Seat", HolderType: "vacant"},
			},
			Services: []apiclient.ServiceOffering{{ID: "svc-1", Name: "Access Request"}},
		})
	})

	mux.HandleFunc("GET /registry/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "a-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(apiclient.Agent{
			ID: "a-1", Name: "Triage Agent", Role: "specialist", Capabilities: []string{"triage.classify"},
		})
	})

	mux.HandleFunc("GET /departments/{id}/requests", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []*apiclient.ServiceRequest `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}{
			Data: []*apiclient.ServiceRequest{{
				ID: "req-1", ServiceID: "svc-1", Title: "Grant read access", Priority: "normal", Status: "completed",
				Timeline: []apiclient.RequestEvent{{Kind: "created"}, {Kind: "completed"}},
			}},
		})
	})

	mux.HandleFunc("GET /invocations", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("request_id") != "req-1" {
			_ = json.NewEncoder(w).Encode(struct {
				Invocations []apiclient.Invocation `json:"invocations"`
			}{})
			return
		}
		_ = json.NewEncoder(w).Encode(struct {
			Invocations []apiclient.Invocation `json:"invocations"`
			Total       int                    `json:"total"`
		}{
			Invocations: []apiclient.Invocation{{CapabilityID: "identity.access.grant", Status: "completed", Simulated: true}},
			Total:       1,
		})
	})

	return httptest.NewServer(mux)
}

func TestRunAssemblesFullFixtureFromMockedAPI(t *testing.T) {
	srv := newMockPlatform(t)
	defer srv.Close()

	cfg := Config{
		TenantControlPlaneURL: srv.URL,
		IdentityAccessURL:     srv.URL,
		AgentRegistryURL:      srv.URL,
		DepartmentsURL:        srv.URL,
		ToolExecutionURL:      srv.URL,
		AdminPassword:         "operan-admin-2026",
		TenantName:            "smoke-tenant",
		TemplateID:            "it-medium-001",
		DepartmentName:        "IT Department",
		Out:                   &strings.Builder{},
	}
	clients := NewClients(cfg)

	f, err := Run(context.Background(), cfg, clients)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if f.Metadata.Provenance != "live-export" {
		t.Errorf("Metadata.Provenance = %q, want live-export", f.Metadata.Provenance)
	}
	if f.Metadata.ExportedAt == "" {
		t.Error("Metadata.ExportedAt is empty")
	}

	if f.Tenant.Name != "smoke-tenant" || f.Tenant.Plan != "saas" || f.Tenant.Region != "me-east-1" {
		t.Errorf("Tenant = %+v", f.Tenant)
	}

	if len(f.Users) != 1 || f.Users[0].Email != "dana@adri.nz" || f.Users[0].Ref != "dana" {
		t.Fatalf("Users = %+v", f.Users)
	}

	if len(f.Agents) != 1 || f.Agents[0].ID != "a-1" || f.Agents[0].Ref != "triage-agent" {
		t.Fatalf("Agents = %+v", f.Agents)
	}

	if f.Department.TemplateID != "it-medium-001" || f.Department.Environment != "production" {
		t.Errorf("Department = %+v", f.Department)
	}
	if len(f.Department.SeatBindings) != 2 {
		// The vacant position must not produce a binding.
		t.Fatalf("SeatBindings = %+v, want exactly 2 (vacant seat excluded)", f.Department.SeatBindings)
	}
	byPosition := map[string]fixture.SeatBinding{}
	for _, sb := range f.Department.SeatBindings {
		byPosition[sb.PositionID] = sb
	}
	if got := byPosition["pos-it-manager-01"]; got.HolderType != "human" || got.UserRef != "dana" {
		t.Errorf("pos-it-manager-01 binding = %+v", got)
	}
	if got := byPosition["pos-triage-01"]; got.HolderType != "ai_agent" || got.AgentRef != "triage-agent" {
		t.Errorf("pos-triage-01 binding = %+v", got)
	}
	if _, vacantBound := byPosition["pos-vacant-01"]; vacantBound {
		t.Errorf("pos-vacant-01 should have no seat binding recorded, got one")
	}

	if len(f.History) != 1 {
		t.Fatalf("History = %+v, want 1 entry", f.History)
	}
	if f.History[0].ServiceID != "svc-1" || len(f.History[0].Invocations) != 1 {
		t.Errorf("History[0] = %+v", f.History[0])
	}
	if f.History[0].Invocations[0].CapabilityID != "identity.access.grant" {
		t.Errorf("History[0].Invocations[0] = %+v", f.History[0].Invocations[0])
	}

	if f.Replay == nil {
		t.Fatal("Replay was not derived, expected a spec derived from history")
	}
	if f.Replay.ServiceID != "svc-1" {
		t.Errorf("Replay.ServiceID = %q, want svc-1", f.Replay.ServiceID)
	}
	if f.Replay.ApproverRef != "dana" {
		t.Errorf("Replay.ApproverRef = %q, want dana", f.Replay.ApproverRef)
	}
}

func TestRunErrorsWhenTemplateIDMissing(t *testing.T) {
	srv := newMockPlatform(t)
	defer srv.Close()
	cfg := Config{
		TenantControlPlaneURL: srv.URL, IdentityAccessURL: srv.URL, AgentRegistryURL: srv.URL,
		DepartmentsURL: srv.URL, ToolExecutionURL: srv.URL,
		TenantName: "smoke-tenant", Out: &strings.Builder{},
	}
	_, err := Run(context.Background(), cfg, NewClients(cfg))
	if err == nil {
		t.Fatal("Run: expected an error when TemplateID is empty, got nil")
	}
}

func TestRunErrorsWhenDepartmentNotFound(t *testing.T) {
	srv := newMockPlatform(t)
	defer srv.Close()
	cfg := Config{
		TenantControlPlaneURL: srv.URL, IdentityAccessURL: srv.URL, AgentRegistryURL: srv.URL,
		DepartmentsURL: srv.URL, ToolExecutionURL: srv.URL,
		AdminPassword: "x", TenantName: "smoke-tenant", TemplateID: "does-not-exist", Out: &strings.Builder{},
	}
	_, err := Run(context.Background(), cfg, NewClients(cfg))
	if err == nil {
		t.Fatal("Run: expected an error when no department matches the template id, got nil")
	}
}

func TestRunFallsBackWhenTenantNotRegisteredInM01(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/iam/admin/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(apiclient.AdminLoginResponse{Token: "admin-jwt"})
	})
	mux.HandleFunc("GET /v1/tenants", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Items []*apiclient.Tenant `json:"items"`
			Total int                 `json:"total"`
		}{}) // no tenants registered at all
	})
	mux.HandleFunc("GET /api/v1/iam/users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Users []*apiclient.User `json:"users"`
			Total int               `json:"total"`
		}{})
	})
	mux.HandleFunc("GET /departments", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []*apiclient.DepartmentSummary `json:"data"`
		}{Data: []*apiclient.DepartmentSummary{{ID: "d-1", Name: "IT Department", TemplateID: "it-medium-001"}}})
	})
	mux.HandleFunc("GET /departments/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(apiclient.Department{ID: "d-1", Name: "IT Department", TemplateID: "it-medium-001", Environment: "production"})
	})
	mux.HandleFunc("GET /departments/{id}/requests", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []*apiclient.ServiceRequest `json:"data"`
		}{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var logs strings.Builder
	cfg := Config{
		TenantControlPlaneURL: srv.URL, IdentityAccessURL: srv.URL, AgentRegistryURL: srv.URL,
		DepartmentsURL: srv.URL, ToolExecutionURL: srv.URL,
		AdminPassword: "x", TenantName: "smoke-tenant", TemplateID: "it-medium-001", Out: &logs,
	}
	f, err := Run(context.Background(), cfg, NewClients(cfg))
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if f.Tenant.Name != "smoke-tenant" {
		t.Errorf("Tenant.Name = %q, want smoke-tenant even without an M01 record", f.Tenant.Name)
	}
	if !strings.Contains(logs.String(), "no tenant control-plane record") {
		t.Errorf("expected a warning about the missing M01 record, got log output:\n%s", logs.String())
	}
}
