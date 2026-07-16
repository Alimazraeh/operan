package policies

import (
	"context"
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