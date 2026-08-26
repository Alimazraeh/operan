package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateTenantHitsExactPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody CreateTenantRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Tenant{ID: "tid-1", Name: gotBody.Name, Status: "provisioning"})
	}))
	defer srv.Close()

	c := &TenantClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.CreateTenant(context.Background(), "admin-token", CreateTenantRequest{
		Name: "smoke-tenant", Plan: "saas", Region: "me-east-1", IsolationLevel: "namespace",
	})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if gotPath != "/v1/tenants" {
		t.Errorf("path = %q, want /v1/tenants", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody.Name != "smoke-tenant" || gotBody.Plan != "saas" {
		t.Errorf("request body = %+v", gotBody)
	}
	if got.ID != "tid-1" {
		t.Errorf("got.ID = %q, want tid-1", got.ID)
	}
}

func TestCreateTenantConflictIsDetectable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"tenant name \"smoke-tenant\" already exists"}`))
	}))
	defer srv.Close()

	c := &TenantClient{BaseURL: srv.URL, Doer: NewDoer()}
	_, err := c.CreateTenant(context.Background(), "tok", CreateTenantRequest{Name: "smoke-tenant"})
	if !IsConflict(err) {
		t.Fatalf("expected IsConflict(err) = true, got err: %v", err)
	}
}

func TestFindTenantByNamePaginates(t *testing.T) {
	// FindTenantByName pages at a fixed size of 50 (see tenant.go); generate
	// enough fixture tenants to span 3 real pages, with the target on the
	// last one, and drive the real (non-duplicated) method end to end.
	const total = 120
	all := make([]*Tenant, total)
	for i := range all {
		all[i] = &Tenant{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("tenant-%d", i)}
	}
	all[total-1].Name = "smoke-tenant"

	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page, pageSize := 1, 50
		fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page)
		fmt.Sscanf(r.URL.Query().Get("page_size"), "%d", &pageSize)
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > len(all) {
			start = len(all)
		}
		if end > len(all) {
			end = len(all)
		}
		_ = json.NewEncoder(w).Encode(tenantListResponse{Items: all[start:end], Total: total})
	}))
	defer srv.Close()

	c := &TenantClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindTenantByName(context.Background(), "tok", "smoke-tenant")
	if err != nil {
		t.Fatalf("FindTenantByName: %v", err)
	}
	if found == nil || found.Name != "smoke-tenant" {
		t.Fatalf("expected to find smoke-tenant, got %+v", found)
	}
	if callCount != 3 {
		t.Errorf("expected exactly 3 page requests for 120 items at page_size 50, got %d", callCount)
	}
}

func TestFindTenantByNameNotFoundReturnsNilNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tenantListResponse{Items: []*Tenant{{ID: "1", Name: "other"}}, Total: 1})
	}))
	defer srv.Close()

	c := &TenantClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindTenantByName(context.Background(), "tok", "smoke-tenant")
	if err != nil {
		t.Fatalf("FindTenantByName: unexpected error: %v", err)
	}
	if found != nil {
		t.Fatalf("expected nil (not found), got %+v", found)
	}
}
