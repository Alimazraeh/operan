package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// JWTValidator tests use the placeholder implementation.
	// When jwt/v5 is available, replace with:
	//   "github.com/golang-jwt/jwt/v5"
	// _ "github.com/golang-jwt/jwt/v5"
)

// ─── RequestID tests ─────────────────────────────────────────────────────────

func TestRequestID_GeneratesUniqueID(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First request
		reqID1 := r.Context().Value(ctxKeyRequestID).(string)
		if reqID1 == "" {
			t.Error("expected non-empty request ID in context")
		}
		if len(reqID1) != 32 {
			t.Errorf("expected request ID length 32, got %d", len(reqID1))
		}

		// Second request should have different ID
		w.Header().Set("X-Request-ID-First", reqID1)
	}))

	// Request 1
	req1 := httptest.NewRequest("GET", "/test", nil)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	id1 := w1.Header().Get("X-Request-ID")

	// Request 2
	req2 := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	id2 := w2.Header().Get("X-Request-ID")

	if id1 == "" {
		t.Error("expected X-Request-ID header in response")
	}
	if id1 == id2 {
		t.Error("expected different request IDs for different requests")
	}
	if len(id1) != 32 {
		t.Errorf("expected request ID length 32, got %d", len(id1))
	}
}

func TestRequestID_ContextValue(t *testing.T) {
	var capturedID string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.Context().Value(ctxKeyRequestID).(string)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedID == "" {
		t.Error("expected request ID in context")
	}
}

// ─── TraceID tests ───────────────────────────────────────────────────────────

func TestTraceID_GeneratesID(t *testing.T) {
	var capturedID string
	h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.Context().Value(ctxKeyTraceID).(string)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedID == "" {
		t.Error("expected trace ID in context")
	}
	if len(capturedID) != 32 {
		t.Errorf("expected trace ID length 32, got %d", len(capturedID))
	}
	if w.Header().Get("X-Trace-Id") == "" {
		t.Error("expected X-Trace-Id header in response")
	}
}

func TestTraceID_PropagatesFromHeader(t *testing.T) {
	var capturedID string
	h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.Context().Value(ctxKeyTraceID).(string)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Trace-Id", "existing-trace-1234567890123456789012")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedID != "existing-trace-1234567890123456789012" {
		t.Errorf("expected propagated trace ID, got %s", capturedID)
	}
	if w.Header().Get("X-Trace-Id") != "existing-trace-1234567890123456789012" {
		t.Error("expected propagated trace ID in response header")
	}
}

// ─── TenantContext tests ─────────────────────────────────────────────────────

func TestTenantContext_InjectsTenantID(t *testing.T) {
	var capturedTenantID string
	h := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(ctxKeyTenantID)
		if v != nil {
			capturedTenantID = v.(string)
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedTenantID != "tenant-abc-123" {
		t.Errorf("expected tenant_id 'tenant-abc-123', got %s", capturedTenantID)
	}
}

func TestTenantContext_NoHeader(t *testing.T) {
	var capturedTenantID interface{}
	h := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = r.Context().Value(ctxKeyTenantID)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedTenantID != nil {
		t.Errorf("expected nil tenant_id when no header, got %v", capturedTenantID)
	}
}

// ─── generateID tests ────────────────────────────────────────────────────────

func TestGenerateID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("generated duplicate ID at iteration %d", i)
		}
		ids[id] = true
		if len(id) != 32 {
			t.Errorf("expected ID length 32, got %d", len(id))
			break
		}
		// Check it's valid hex
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("expected hex character, got %c", c)
				break
			}
		}
	}
}

func TestGenerateID_Length(t *testing.T) {
	id := generateID()
	if len(id) != 32 {
		t.Errorf("expected ID length 32, got %d", len(id))
	}
}

// ─── WriteJSON tests ─────────────────────────────────────────────────────────

func TestWriteJSON_StatusAndContent(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()

	h.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected body {\"status\":\"ok\"}, got %s", w.Body.String())
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()

	h.WriteJSON(w, http.StatusNoContent, nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

// ─── WriteError tests ────────────────────────────────────────────────────────

func TestWriteError_Structure(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()

	h.WriteError(w, http.StatusNotFound, 404, "not found", "resource does not exist")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)

	if errResp.Code != 404 {
		t.Errorf("expected code 404, got %d", errResp.Code)
	}
	if errResp.Message != "not found" {
		t.Errorf("expected message 'not found', got %s", errResp.Message)
	}
	if errResp.Details != "resource does not exist" {
		t.Errorf("expected details 'resource does not exist', got %s", errResp.Details)
	}
	if errResp.RequestID == "" {
		t.Error("expected non-empty request_id in error response")
	}
}

func TestWriteError_ContentType(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()

	h.WriteError(w, http.StatusBadRequest, 400, "bad request", "")

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

// ─── Logger tests ────────────────────────────────────────────────────────────

func TestLogger_NoPanic(t *testing.T) {
	h := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test-path", nil)
	w := httptest.NewRecorder()

	// Just verify middleware doesn't panic
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestLogger_CustomStatusCode(t *testing.T) {
	h := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/missing", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ─── PaginatedResponse tests ─────────────────────────────────────────────────

func TestPaginatedResponse_JSONMarshal(t *testing.T) {
	resp := PaginatedResponse[string]{
		Data:    []*string{ptrStr("item1"), ptrStr("item2")},
		Total:   2,
		HasMore: false,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var unmarshaled PaginatedResponse[string]
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(unmarshaled.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(unmarshaled.Data))
	}
	if unmarshaled.Total != 2 {
		t.Errorf("expected total 2, got %d", unmarshaled.Total)
	}
	if unmarshaled.HasMore {
		t.Error("expected has_more false")
	}
}

func ptrStr(s string) *string { return &s }

// ─── TenantPatchRequest tests ────────────────────────────────────────────────

func TestTenantPatchRequest_JSONRoundTrip(t *testing.T) {
	original := TenantPatchRequest{
		Name:           "new-name",
		Status:         "active",
		Plan:           "enterprise",
		CustomMetadata: map[string]interface{}{"key": "value"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded TenantPatchRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.Name != "new-name" {
		t.Errorf("expected name 'new-name', got %s", decoded.Name)
	}
	if decoded.Status != "active" {
		t.Errorf("expected status 'active', got %s", decoded.Status)
	}
}

// ─── ErrorResponse JSON tests ────────────────────────────────────────────────

func TestErrorResponse_JSONFields(t *testing.T) {
	resp := ErrorResponse{
		Code:      500,
		Message:   "internal error",
		Details:   "something went wrong",
		RequestID: "req-123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON structure has expected keys
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := raw["code"]; !ok {
		t.Error("expected 'code' field in JSON")
	}
	if _, ok := raw["message"]; !ok {
		t.Error("expected 'message' field in JSON")
	}
	if _, ok := raw["details"]; !ok {
		t.Error("expected 'details' field in JSON")
	}
	if _, ok := raw["request_id"]; !ok {
		t.Error("expected 'request_id' field in JSON")
	}
}

// ─── Context key tests ───────────────────────────────────────────────────────

func TestContextKey_String(t *testing.T) {
	keys := []contextKey{
		ctxKeyTenantID,
		ctxKeyTraceID,
		ctxKeyRequestID,
		ctxKeyUserID,
		ctxKeyAuthType,
	}
	for _, key := range keys {
		s := string(key)
		if s == "" {
			t.Errorf("context key %s should not be empty", key)
		}
	}
}

// ─── Middleware composition test ─────────────────────────────────────────────

func TestMiddlewareComposition(t *testing.T) {
	var requestID, traceID, tenantID string
	h := RequestID(TraceID(TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = r.Context().Value(ctxKeyRequestID).(string)
		traceID = r.Context().Value(ctxKeyTraceID).(string)
		tenantID = r.Context().Value(ctxKeyTenantID).(string)
	}))))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-123")
	req.Header.Set("X-Trace-Id", "my-trace-id")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if requestID == "" {
		t.Error("expected request ID from composed middleware")
	}
	if traceID != "my-trace-id" {
		t.Errorf("expected propagated trace ID, got %s", traceID)
	}
	if tenantID != "tenant-123" {
		t.Errorf("expected tenant ID 'tenant-123', got %s", tenantID)
	}
}

// ─── LoggingResponseWriter tests ─────────────────────────────────────────────

func TestLoggingResponseWriter_WriteHeader(t *testing.T) {
	lw := &loggingResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	lw.WriteHeader(http.StatusCreated)
	if lw.statusCode != http.StatusCreated {
		t.Errorf("expected status code 201, got %d", lw.statusCode)
	}
}

func TestLoggingResponseWriter_DefaultStatusCode(t *testing.T) {
	lw := &loggingResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	// If WriteHeader is never called, statusCode should remain Ok
	if lw.statusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", lw.statusCode)
	}
}

// ─── JWTValidator tests (placeholder implementation) ─────────────────────────
// The JWTValidator uses a placeholder implementation until the golang-jwt/v5
// dependency is available. These tests validate the placeholder behavior.

func TestJWTValidator_MissingAuthHeader(t *testing.T) {
	// Placeholder: when no Authorization header, falls through to TenantContext
	h := JWTValidator("secret", "issuer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK (fallthrough), got %d", w.Code)
	}
}

func TestJWTValidator_InvalidAuthScheme(t *testing.T) {
	h := JWTValidator("secret", "issuer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid scheme, got %d", w.Code)
	}
}

func TestJWTValidator_BearerTokenPresent(t *testing.T) {
	// Placeholder: when Bearer token present but not validated, falls through
	h := JWTValidator("secret", "issuer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token-here")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK (placeholder fallthrough), got %d", w.Code)
	}
}

func TestJWTValidator_TenantContextPrecedence(t *testing.T) {
	// JWTValidator falls through to TenantContext for header-based auth
	h := JWTValidator("secret", "issuer")(TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r.Context())
		if tenantID != "tenant-from-header" {
			t.Errorf("expected tenant_id from header, got %q", tenantID)
		}
	})))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	req.Header.Set("X-Tenant-ID", "tenant-from-header")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// ─── GetTenantID tests ────────────────────────────────────────────────────────

func TestGetTenantID_FromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKeyTenantID, "test-tenant-123")
	id := GetTenantID(ctx)
	if id != "test-tenant-123" {
		t.Errorf("expected 'test-tenant-123', got %q", id)
	}
}

func TestGetTenantID_Empty(t *testing.T) {
	ctx := context.Background()
	id := GetTenantID(ctx)
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}
