package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionReplayCapture_SessionLimits(t *testing.T) {
	tests := []struct {
		name         string
		maxSessions  int
		maxRequests  int
		numSessions  int
		requestsPer  int
		wantMaxCache int
		wantMaxReqs  int
	}{
		{
			name:         "enforces max sessions",
			maxSessions:  3,
			maxRequests:  100,
			numSessions:  5,
			requestsPer:  5,
			wantMaxCache: 3, // only 3 sessions should remain
			wantMaxReqs:  100,
		},
		{
			name:         "enforces max requests per session",
			maxSessions:  100,
			maxRequests:  3,
			numSessions:  2,
			requestsPer:  10,
			wantMaxCache: 100,
			wantMaxReqs:  3, // only 3 requests per session
		},
		{
			name:         "both limits enforced",
			maxSessions:  2,
			maxRequests:  2,
			numSessions:  5,
			requestsPer:  10,
			wantMaxCache: 2,
			wantMaxReqs:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
				MaxSessions:     tt.maxSessions,
				MaxRequests:     tt.maxRequests,
				CleanupInterval: 0, // disable background cleanup for test
			})
			defer cap.Stop()

			for s := 0; s < tt.numSessions; s++ {
				sessionID := "session-" + string(rune('A'+s))
				for r := 0; r < tt.requestsPer; r++ {
					req := &ReplayRequest{
						Timestamp: time.Now().UTC(),
						Method:    "GET",
						Path:      "/api/test",
					}
					cap.SaveSession(req, sessionID)
				}
			}

			sessions := cap.ListSessions()
			if len(sessions) > tt.wantMaxCache {
				t.Errorf("got %d sessions, want at most %d", len(sessions), tt.wantMaxCache)
			}

			for _, sid := range sessions {
				session := cap.GetSession(sid)
				if session == nil {
					t.Errorf("session %s not found", sid)
					continue
				}
				if len(session.Requests) > tt.wantMaxReqs {
					t.Errorf("session %s has %d requests, want at most %d", sid, len(session.Requests), tt.wantMaxReqs)
				}
			}
		})
	}
}

func TestSessionReplayCapture_LRUOrder(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		MaxSessions:   3,
		MaxRequests:   100,
		CleanupInterval: 0,
	})
	defer cap.Stop()

	// Create 3 sessions
	for i := 0; i < 3; i++ {
		req := &ReplayRequest{
			Timestamp: time.Now().UTC(),
			Method:    "GET",
			Path:      "/api/test",
		}
		cap.SaveSession(req, "session-"+string(rune('A'+i)))
	}

	// Create a 4th session — should evict the oldest (A)
	req := &ReplayRequest{
		Timestamp: time.Now().UTC(),
		Method:    "GET",
		Path:      "/api/test",
	}
	cap.SaveSession(req, "session-D")

	sessions := cap.ListSessions()
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Oldest session A should be evicted
	oldest := cap.GetSession("session-A")
	if oldest != nil {
		t.Error("expected session-A to be evicted, but it still exists")
	}

	// Newer sessions should remain
	for _, id := range []string{"session-B", "session-C", "session-D"} {
		s := cap.GetSession(id)
		if s == nil {
			t.Errorf("expected session %s to exist", id)
		}
	}
}

func TestSessionReplayCapture_CleanupOldSessions(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		CleanupInterval: 0, // disable background cleanup
		MaxSessionAge:   1 * time.Minute,
	})
	defer cap.Stop()

	// Create an old session
	oldReq := &ReplayRequest{
		Timestamp: time.Now().UTC().Add(-2 * time.Minute),
		Method:    "GET",
		Path:      "/api/old",
	}
	cap.SaveSession(oldReq, "old-session")

	// Create a new session
	newReq := &ReplayRequest{
		Timestamp: time.Now().UTC(),
		Method:    "GET",
		Path:      "/api/new",
	}
	cap.SaveSession(newReq, "new-session")

	removed := cap.CleanupOldSessions(1 * time.Minute)
	if removed != 1 {
		t.Errorf("expected 1 session removed, got %d", removed)
	}

	// Old session should be gone
	if cap.GetSession("old-session") != nil {
		t.Error("expected old-session to be removed")
	}

	// New session should remain
	if cap.GetSession("new-session") == nil {
		t.Error("expected new-session to remain")
	}
}

func TestSessionReplayCapture_SetSessionUserLimits(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		MaxSessions:   2,
		MaxRequests:   100,
		CleanupInterval: 0,
	})
	defer cap.Stop()

	// Create 2 sessions via SetSessionUser
	cap.SetSessionUser("session-A", "user-1", "tenant-1")
	cap.SetSessionUser("session-B", "user-2", "tenant-2")

	// Create a 3rd session via SetSessionUser — should evict oldest
	cap.SetSessionUser("session-C", "user-3", "tenant-3")

	sessions := cap.ListSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// session-A should be evicted
	if cap.GetSession("session-A") != nil {
		t.Error("expected session-A to be evicted")
	}
}

func TestSessionReplayCapture_DeleteSession(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		MaxSessions:   5,
		CleanupInterval: 0,
	})
	defer cap.Stop()

	// Create 3 sessions
	for i := 0; i < 3; i++ {
		req := &ReplayRequest{
			Timestamp: time.Now().UTC(),
			Method:    "GET",
			Path:      "/api/test",
		}
		cap.SaveSession(req, "session-"+string(rune('A'+i)))
	}

	// Delete middle session
	cap.DeleteSession("session-B")

	sessions := cap.ListSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions after delete, got %d", len(sessions))
	}

	// session-B should be gone
	if cap.GetSession("session-B") != nil {
		t.Error("expected session-B to be deleted")
	}
}

func TestSessionReplayCapture_StopCleanupLoop(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		CleanupInterval: 1 * time.Millisecond, // very fast cleanup
		MaxSessionAge:   0,                     // evict all on cleanup
	})

	// Create a session
	req := &ReplayRequest{
		Timestamp: time.Now().UTC(),
		Method:    "GET",
		Path:      "/api/test",
	}
	cap.SaveSession(req, "session-stop-test")

	// Stop the cleanup loop
	cap.Stop()

	// Cleanup should still work (method is safe to call)
	removed := cap.CleanupOldSessions(0)
	_ = removed

	// Second stop should be a no-op
	cap.Stop() // should not panic
}

func TestSessionReplayCapture_MiddlewareIntegration(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		MaxSessions:   10,
		MaxRequests:   50,
		CleanupInterval: 0,
	})
	defer cap.Stop()

	middleware := SessionReplayMiddleware(cap)

	// Create a handler that echoes request info
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Create a request with session ID
	req := httptest.NewRequest("GET", "/api/tenant/test", nil)
	req.Header.Set("X-Session-ID", "test-session-123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace-ID", "trace-abc")

	// Inject tenant/user context (simulating middleware pipeline)
	ctx := req.Context()
	ctx = context.WithValue(ctx, TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, UserIDKey, "user-1")
	req = req.WithContext(ctx)

	// Record response
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	// Verify session was captured
	session := cap.GetSession("test-session-123")
	if session == nil {
		t.Fatal("expected session to be captured")
	}

	if session.UserID != "user-1" {
		t.Errorf("expected userID 'user-1', got '%s'", session.UserID)
	}

	if session.TenantID != "tenant-1" {
		t.Errorf("expected tenantID 'tenant-1', got '%s'", session.TenantID)
	}

	if len(session.Requests) != 1 {
		t.Errorf("expected 1 request in session, got %d", len(session.Requests))
	}
}

// Ensure SessionReplayCapture is thread-safe
func TestSessionReplayCapture_ConcurrentAccess(t *testing.T) {
	cap := NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{
		MaxSessions:   1000,
		MaxRequests:   1000,
		CleanupInterval: 0,
	})
	defer cap.Stop()

	done := make(chan bool)

	// 10 goroutines each creating 100 sessions
	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 100; i++ {
				sessionID := "session-goroutine-" + string(rune('A'+id)) + "-req-" + string(rune('A'+i))
				req := &ReplayRequest{
					Timestamp: time.Now().UTC(),
					Method:    "GET",
					Path:      "/api/concurrent",
				}
				cap.SaveSession(req, sessionID)
			}
			done <- true
		}(g)
	}

	for g := 0; g < 10; g++ {
		<-done
	}

	sessions := cap.ListSessions()
	if len(sessions) != 1000 {
		t.Errorf("expected 1000 sessions, got %d", len(sessions))
	}
}

// Ensure SanitizeQuery redacts sensitive parameters
func TestSanitizeQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no sensitive params",
			input:    "page=1&limit=10",
			expected: "limit=10&page=1", // order may vary
		},
		{
			name:     "redacts password",
			input:    "username=admin&password=secret",
			expected: "password=REDACTED&username=admin",
		},
		{
			name:     "redacts api_key",
			input:    "user=123&api_key=sk-12345",
			expected: "api_key=REDACTED&user=123",
		},
		{
			name:     "redacts token",
			input:    "action=list&token=abc123",
			expected: "action=list&token=REDACTED",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeQuery(tt.input)
			// Simple check: if input contains sensitive param, result should have REDACTED
			if containsSensitiveParam(tt.input) && !contains(result, "REDACTED") {
				t.Errorf("expected REDACTED in result for input %q, got %q", tt.input, result)
			}
		})
	}
}

func containsSensitiveParam(input string) bool {
	sensitive := []string{"password", "token", "secret", "key", "api_key", "authorization"}
	for _, s := range sensitive {
		if contains(input, s) {
			return true
		}
	}
	return false
}

func contains(input, substr string) bool {
	if len(substr) == 0 || len(input) < len(substr) {
		return false
	}
	for i := 0; i <= len(input)-len(substr); i++ {
		if input[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
