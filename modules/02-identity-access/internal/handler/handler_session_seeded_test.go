package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/middleware"
)

func TestSessionReplayHandler_SeededCapture(t *testing.T) {
	cap := middleware.NewSessionReplayCapture()
	// Seed a real session into the capture store.
	captured := cap.Capture(httptest.NewRequest(http.MethodGet, "/api/v1/iam/users", nil))
	cap.SaveSession(captured, "sess-1")
	cap.SetSessionUser("sess-1", "user-1", "t1")

	h := &SessionReplayHandler{Capture: cap, Publisher: nil, AuditStore: NewAuditStore()}

	// ListSessions iterates the seeded session.
	rr := httptest.NewRecorder()
	h.ListSessions(rr, userTenantReq(http.MethodGet, "/api/v1/iam/session-replay/sessions", ""))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "sess-1") {
		t.Errorf("ListSessions() = %d, body: %s", rr.Code, rr.Body.String())
	}

	// GetSessionRequests for the seeded session.
	rr = httptest.NewRecorder()
	h.GetSessionRequests(rr, userTenantReq(http.MethodGet, "/api/v1/iam/session-replay/sessions/sess-1/requests", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("GetSessionRequests() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Not-found session.
	rr = httptest.NewRecorder()
	h.GetSessionRequests(rr, userTenantReq(http.MethodGet, "/api/v1/iam/session-replay/sessions/missing/requests", ""))
	if rr.Code != http.StatusNotFound {
		t.Errorf("GetSessionRequests(missing) = %d, want 404", rr.Code)
	}

	// DeleteSession removes the seeded session.
	rr = httptest.NewRecorder()
	h.DeleteSession(rr, userTenantReq(http.MethodDelete, "/api/v1/iam/session-replay/sessions/sess-1", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("DeleteSession() = %d. Body: %s", rr.Code, rr.Body.String())
	}
}
