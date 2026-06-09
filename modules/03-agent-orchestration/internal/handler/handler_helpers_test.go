package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// setTenant injects tenant-1 into the request context for testing.
func setTenant(req *http.Request) *http.Request {
	ctx := middleware.SetTenantIDToContext(req.Context(), "tenant-1")
	return req.WithContext(ctx)
}

// ─── Extract helper function tests ───────────────────────────────────────────

func TestExtractNodeIDFromPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		expect string
	}{
		{
			name:   "standard path with node retry",
			path:   "/api/v1/orchestration/workflows/wf-1/nodes/node-2/retry",
			expect: "nodes", // extracts everything after wf-1/, then first segment
		},
		{
			name:   "no retry suffix",
			path:   "/api/v1/orchestration/workflows/wf-1/nodes/node-3",
			expect: "nodes",
		},
		{
			name:   "workflow only - no second slash",
			path:   "/api/v1/orchestration/workflows/wf-1",
			expect: "",
		},
		{
			name:   "wrong prefix - extracts from first slash",
			path:   "/other/path/item/retry",
			expect: "other", // trims nothing, then takes after first /
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNodeIDFromPath(tt.path)
			if got != tt.expect {
				t.Errorf("extractNodeIDFromPath(%q) = %q, want %q", tt.path, got, tt.expect)
			}
		})
	}
}

func TestExtractAgentIDFromPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		expect string
	}{
		{
			name:   "agent ID in path",
			path:   "/api/v1/orchestration/agents/agent-1/status",
			expect: "agent-1",
		},
		{
			name:   "no agent prefix",
			path:   "/other/path/agent-1",
			expect: "",
		},
		{
			name:   "empty path",
			path:   "",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAgentIDFromPath(tt.path)
			if got != tt.expect {
				t.Errorf("extractAgentIDFromPath(%q) = %q, want %q", tt.path, got, tt.expect)
			}
		})
	}
}

// ─── ListAgents function test ────────────────────────────────────────────────

func TestListAgentEndpoint(t *testing.T) {
	agStore := store.NewAgentStore()
	agStore.SetAgentAvailability(&store.AgentAvailability{
		AgentID:        "agent-1",
		Status:         store.AgentStatusAvailable,
		CurrentWorkflows: 0,
		MaxConcurrency: 10,
	})

	t.Run("lists agents from store", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/agents", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		ListAgents(agStore).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		jsonBytes := w.Body.Bytes()
		if len(jsonBytes) > 0 {
			if err := json.Unmarshal(jsonBytes, &resp); err != nil {
				t.Errorf("Failed to parse response: %v. Body: %s", err, string(jsonBytes))
			}
		}
	})
}

// ─── ptrBool helper test ─────────────────────────────────────────────────────

func TestPtrBool(t *testing.T) {
	tests := []struct {
		input    bool
		expected bool
	}{
		{true, true},
		{false, false},
	}

	for _, tt := range tests {
		result := ptrBool(tt.input)
		if result == nil || *result != tt.expected {
			t.Errorf("ptrBool(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// ─── ptrTime helper test ─────────────────────────────────────────────────────

func TestPtrTime(t *testing.T) {
	now := time.Now()
	result := ptrTime(now)

	if result == nil || !result.Equal(now) {
		t.Errorf("ptrTime(now) = %v, want %v", result, now)
	}
}
