package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
)

func TestAuditHandler_GetSessionReplay_AuthBacked(t *testing.T) {
	h := NewAuditHandler(ssoAuthClient(t))

	rr := httptest.NewRecorder()
	h.GetSessionReplay(rr, tenantReq(http.MethodGet, "/api/v1/iam/audit/session-replay/sess-123", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("GetSessionReplay() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// missing id -> 400
	rr = httptest.NewRecorder()
	h.GetSessionReplay(rr, tenantReq(http.MethodGet, "/api/v1/iam/audit/session-replay/", ""))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GetSessionReplay(no id) = %d, want 400", rr.Code)
	}
}

func TestABACHandler_Evaluate_AuthBacked(t *testing.T) {
	h := NewABACHandler(ssoAuthClient(t), events.NewPublisher(""), NewABACStore())

	rr := httptest.NewRecorder()
	body := `{"actor_id":"user-1","action":"read","resource":"document"}`
	h.Evaluate(rr, userTenantReq(http.MethodPost, "/api/v1/iam/abac/evaluate", body))
	if rr.Code != http.StatusOK {
		t.Errorf("Evaluate() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	// validation error
	rr = httptest.NewRecorder()
	h.Evaluate(rr, userTenantReq(http.MethodPost, "/api/v1/iam/abac/evaluate", `{"action":"read"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Evaluate(no actor) = %d, want 400", rr.Code)
	}
}

func TestSessionReplayHandler_RealCapture(t *testing.T) {
	h := &SessionReplayHandler{
		Capture:    middleware.NewSessionReplayCapture(),
		Publisher:  nil,
		AuditStore: NewAuditStore(),
	}

	rr := httptest.NewRecorder()
	h.ListSessions(rr, userTenantReq(http.MethodGet, "/api/v1/iam/session-replay/sessions", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("ListSessions() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.GetSessionRequests(rr, userTenantReq(http.MethodGet, "/api/v1/iam/session-replay/sessions/sess-1/requests", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Errorf("GetSessionRequests() = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.DeleteSession(rr, userTenantReq(http.MethodDelete, "/api/v1/iam/session-replay/sessions/sess-1", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent && rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteSession() = %d", rr.Code)
	}
}
