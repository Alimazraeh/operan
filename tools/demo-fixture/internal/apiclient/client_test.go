package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallSendsHeadersAndBody(t *testing.T) {
	var gotAuth, gotTenant, gotMethod string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc123"}`))
	}))
	defer srv.Close()

	d := &Doer{HTTP: srv.Client()}
	var out struct {
		ID string `json:"id"`
	}
	_, err := d.Call(context.Background(), http.MethodPost, srv.URL+"/things", "my-token", "smoke-tenant",
		map[string]string{"name": "widget"}, &out)
	if err != nil {
		t.Fatalf("Call: unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer my-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer my-token")
	}
	if gotTenant != "smoke-tenant" {
		t.Errorf("X-Tenant-ID header = %q, want %q", gotTenant, "smoke-tenant")
	}
	if gotBody["name"] != "widget" {
		t.Errorf("request body name = %q, want %q", gotBody["name"], "widget")
	}
	if out.ID != "abc123" {
		t.Errorf("decoded response id = %q, want %q", out.ID, "abc123")
	}
}

func TestCallOmitsAuthHeaderWhenTokenEmpty(t *testing.T) {
	var sawAuthHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			sawAuthHeader = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := &Doer{HTTP: srv.Client()}
	_, err := d.Call(context.Background(), http.MethodPost, srv.URL+"/login", "", "", map[string]string{"password": "x"}, nil)
	if err != nil {
		t.Fatalf("Call: unexpected error: %v", err)
	}
	if sawAuthHeader {
		t.Error("expected no Authorization header for an unauthenticated call, but one was sent")
	}
}

func TestCallReturnsAPIErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"tenant name \"smoke-tenant\" already exists"}`))
	}))
	defer srv.Close()

	d := &Doer{HTTP: srv.Client()}
	_, err := d.Call(context.Background(), http.MethodPost, srv.URL+"/v1/tenants", "tok", "", map[string]string{}, nil)
	if err == nil {
		t.Fatal("Call: expected an error for a 409 response, got nil")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Call: expected *APIError, got %T: %v", err, err)
	}
	if ae.Status != http.StatusConflict {
		t.Errorf("APIError.Status = %d, want %d", ae.Status, http.StatusConflict)
	}
	if !IsConflict(err) {
		t.Error("IsConflict(err) = false, want true")
	}
	if IsNotFound(err) {
		t.Error("IsNotFound(err) = true, want false")
	}
}

func TestCallReturnsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	d := &Doer{HTTP: srv.Client()}
	_, err := d.Call(context.Background(), http.MethodGet, srv.URL+"/things/nope", "tok", "", nil, nil)
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(err) = false, want true (err: %v)", err)
	}
}

func TestCallHandlesEmptyResponseBody(t *testing.T) {
	// M02's SetPassword and a few other endpoints answer 204 with no body;
	// decoding into `out` must not choke on that.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := &Doer{HTTP: srv.Client()}
	var out map[string]interface{}
	_, err := d.Call(context.Background(), http.MethodPost, srv.URL+"/x", "tok", "", nil, &out)
	if err != nil {
		t.Fatalf("Call: unexpected error on empty 204 body: %v", err)
	}
}
