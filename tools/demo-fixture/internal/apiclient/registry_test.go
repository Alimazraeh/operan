package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAgentSendsCallerSuppliedID(t *testing.T) {
	var gotPath string
	var gotBody CreateAgentRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Agent{ID: gotBody.ID, Name: gotBody.Name, Status: "active"})
	}))
	defer srv.Close()

	c := &RegistryClient{BaseURL: srv.URL, Doer: NewDoer()}
	const fixedID = "3a0c0c3c-c849-4b74-883a-9ccf85b14b5c"
	got, err := c.CreateAgent(context.Background(), "admin-jwt", "smoke-tenant", CreateAgentRequest{
		ID: fixedID, Name: "Triage Agent", Role: "specialist",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if gotPath != "/registry/agents" {
		t.Errorf("path = %q, want /registry/agents", gotPath)
	}
	if gotBody.ID != fixedID {
		t.Errorf("request body ID = %q, want %q", gotBody.ID, fixedID)
	}
	if gotBody.TenantID != "smoke-tenant" {
		t.Errorf("request body TenantID = %q, want smoke-tenant (CreateAgent must stamp it from the tenantID arg)", gotBody.TenantID)
	}
	if got.ID != fixedID {
		t.Errorf("got.ID = %q, want %q", got.ID, fixedID)
	}
}

func TestCreateAgentConflictIsDetectable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"agent already exists"}`))
	}))
	defer srv.Close()

	c := &RegistryClient{BaseURL: srv.URL, Doer: NewDoer()}
	_, err := c.CreateAgent(context.Background(), "tok", "smoke-tenant", CreateAgentRequest{ID: "x", Name: "n", Role: "r"})
	if !IsConflict(err) {
		t.Fatalf("expected IsConflict(err) = true, got: %v", err)
	}
}

func TestGetAgentPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(Agent{ID: "a-1", Name: "Triage"})
	}))
	defer srv.Close()

	c := &RegistryClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.GetAgent(context.Background(), "tok", "smoke-tenant", "a-1")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if gotPath != "/registry/agents/a-1" {
		t.Errorf("path = %q, want /registry/agents/a-1", gotPath)
	}
	if got.Name != "Triage" {
		t.Errorf("got.Name = %q", got.Name)
	}
}
