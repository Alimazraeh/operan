package policies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanExecute_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"allowed": true, "reason": "policy check passed"}`))
	}))
	defer server.Close()

	client := NewPolicyClient(server.URL)
	result, err := client.CanExecute(context.Background(), "tenant-1", "agent-1", "echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected allowed=true")
	}
	if result.Reason != "policy check passed" {
		t.Errorf("expected 'policy check passed', got '%s'", result.Reason)
	}
}

func TestCanExecute_Denied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"allowed": false, "reason": "tool not permitted"}`))
	}))
	defer server.Close()

	client := NewPolicyClient(server.URL)
	result, err := client.CanExecute(context.Background(), "tenant-1", "agent-1", "rm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected allowed=false")
	}
	if result.Reason != "tool not permitted" {
		t.Errorf("expected 'tool not permitted', got '%s'", result.Reason)
	}
}

func TestCanExecute_Unavailable(t *testing.T) {
	// Point to a port that's not listening
	client := NewPolicyClient("http://127.0.0.1:1")
	result, err := client.CanExecute(context.Background(), "tenant-1", "agent-1", "echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected allowed=false when M10 unreachable")
	}
	if result.Reason != "policy engine unreachable" {
		t.Errorf("expected 'policy engine unreachable', got '%s'", result.Reason)
	}
}

func TestCanExecute_BadGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewPolicyClient(server.URL)
	result, err := client.CanExecute(context.Background(), "tenant-1", "agent-1", "echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected allowed=false on BadGateway")
	}
}

func TestCanExecute_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewPolicyClient(server.URL)
	result, err := client.CanExecute(context.Background(), "tenant-1", "agent-1", "echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected allowed=false on ServiceUnavailable")
	}
}

// The endpoint and request shape must match M10's real contract
// (POST /policies/evaluate with resource + action_type). The client used to
// call /v1/policies/check, which M10 does not expose — so every check 404'd
// and every sandbox execution would have been denied in a live deployment.
func TestCanExecute_UsesM10EvaluateContract(t *testing.T) {
	var gotPath, gotTenant string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTenant = r.Header.Get("X-Tenant-ID")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]interface{}{"allowed": true})
	}))
	defer srv.Close()

	res, err := NewPolicyClient(srv.URL).CanExecute(context.Background(), "t1", "agent-1", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed {
		t.Error("expected allowed")
	}
	if gotPath != "/policies/evaluate" {
		t.Errorf("path = %q, want /policies/evaluate", gotPath)
	}
	if gotTenant != "t1" {
		t.Errorf("X-Tenant-ID = %q, want t1", gotTenant)
	}
	if gotBody["resource"] != "tool:bash" || gotBody["action_type"] != "execute" {
		t.Errorf("body = %v, want resource=tool:bash action_type=execute", gotBody)
	}
}

// A malformed answer must deny. Previously this returned (nil, err), and the
// caller's nil-check skipped the deny branch — allow-on-malformed-response.
func TestCanExecute_MalformedResponseDenies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not json</html>"))
	}))
	defer srv.Close()

	res, err := NewPolicyClient(srv.URL).CanExecute(context.Background(), "t1", "a", "bash")
	if err == nil {
		t.Error("expected an error for a malformed response")
	}
	if res == nil {
		t.Fatal("must return a decision, not nil — a nil result is what let execution proceed")
	}
	if res.Allowed {
		t.Error("a malformed policy response must deny")
	}
}
