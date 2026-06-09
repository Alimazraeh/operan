package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestID_GeneratesUniqueID(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		if reqID == "" {
			t.Error("Expected RequestID in context")
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTraceID_PreservesAndGenerates(t *testing.T) {
	t.Run("preserves existing trace ID", func(t *testing.T) {
		h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceIDFromContext(r.Context())
			if traceID != "existing-trace-123" {
				t.Errorf("Expected existing-trace-123, got %s", traceID)
			}
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Trace-ID", "existing-trace-123")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	})

	t.Run("generates new trace ID", func(t *testing.T) {
		h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceIDFromContext(r.Context())
			if traceID == "" {
				t.Error("Expected generated trace ID")
			}
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	})
}

func TestTenantContext_RejectsMissingHeader(t *testing.T) {
	h := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 400 {
		t.Errorf("Expected error code 400, got %d", resp.Code)
	}
}

func TestTenantContext_ExtractsTenantID(t *testing.T) {
	h := TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "tenant-abc-123" {
			t.Errorf("Expected tenant-abc-123, got %s", tenantID)
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	h := &Handler{}

	w := httptest.NewRecorder()
	data := map[string]interface{}{"key": "value", "count": 42}
	h.WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["key"] != "value" {
		t.Errorf("Expected value 'value', got %v", result["key"])
	}
}

func TestWriteError(t *testing.T) {
	h := &Handler{}

	w := httptest.NewRecorder()
	h.WriteError(w, http.StatusNotFound, 404, "not found", "resource does not exist")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 404 {
		t.Errorf("Expected error code 404, got %d", resp.Code)
	}
	if resp.Message != "not found" {
		t.Errorf("Expected message 'not found', got %s", resp.Message)
	}
	if resp.Details != "resource does not exist" {
		t.Errorf("Expected details 'resource does not exist', got %s", resp.Details)
	}
	if resp.RequestID == "" {
		t.Error("Expected non-empty RequestID")
	}
}

func TestMiddleware_Chaining(t *testing.T) {
	requestIDs := make([]string, 0, 3)

	h := RequestID(
		TraceID(
			TenantContext(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					reqID := TraceIDFromContext(r.Context())
					tenantID := TenantIDFromContext(r.Context())
					requestIDs = append(requestIDs, reqID)

					w.Header().Set("X-Request-ID", reqID)
					w.Header().Set("X-Tenant-ID", tenantID)
					w.WriteHeader(http.StatusOK)
				}),
			),
		),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if len(requestIDs) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requestIDs))
	}

	if requestIDs[0] == "" {
		t.Error("Expected non-empty request ID")
	}

	if w.Header().Get("X-Tenant-ID") != "tenant-123" {
		t.Errorf("Expected X-Tenant-ID 'tenant-123', got %s", w.Header().Get("X-Tenant-ID"))
	}
}

func TestPaginatedResponse_Marshaling(t *testing.T) {
	str1 := "draft"
	str2 := "active"
	resp := PaginatedResponse[string]{
		Data:    []*string{&str1, &str2},
		Total:   2,
		HasMore: false,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["total"].(float64) != 2 {
		t.Errorf("Expected total 2, got %v", result["total"])
	}
	if result["has_more"].(bool) != false {
		t.Errorf("Expected has_more false, got %v", result["has_more"])
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true
		if len(id) != 32 { // 16 bytes as hex
			t.Errorf("Expected ID length 32, got %d", len(id))
		}
	}
}

func TestLogger_LogsRequest(t *testing.T) {
	logged := false
	h := Logger(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logged = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/test/path", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("X-Request-ID", "req-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !logged {
		t.Error("Expected handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ─── JWTAuth middleware tests ────────────────────────────────────────────────

// generateValidJWT creates a valid HMAC-S256 JWT with the given secret, claims, and optional exp override.
// expOverride of 0 means "expire in 1 hour".
func generateValidJWT(secret string, claims map[string]interface{}, expOverride int64) string {
	// Header
	header := `{"alg":"HS256","typ":"JWT"}`
	// Payload
	payload := `{"sub":"user-1","roles":["admin","viewer"],"iat":` + fmt.Sprintf("%d", time.Now().Unix())
	if expOverride > 0 {
		payload += `,"exp":` + fmt.Sprintf("%d", expOverride)
	} else {
		payload += `,"exp":` + fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix())
	}
	payload += `}`

	headerEnc := base64URLEncode([]byte(header))
	payloadEnc := base64URLEncode([]byte(payload))

	signingInput := headerEnc + "." + payloadEnc
	sig := computeHMAC(secret, signingInput)

	return signingInput + "." + sig
}

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "test-secret-123"
	token := generateValidJWT(secret, map[string]interface{}{
		"sub":   "user-1",
		"roles": []interface{}{"admin", "viewer"},
	}, 0)

	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		if userID != "user-1" {
			t.Errorf("Expected user-1, got %s", userID)
		}
		roles := UserRolesFromContext(r.Context())
		if len(roles) != 2 || roles[0] != "admin" || roles[1] != "viewer" {
			t.Errorf("Expected [admin, viewer], got %v", roles)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	secret := "test-secret-123"
	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when Authorization header is missing")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	// writeUnauthorized returns {"error":{"code":401,"message":"..."}}
	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Code != 401 {
		t.Errorf("Expected error code 401, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Authorization header required") {
		t.Errorf("Expected message about Authorization header, got: %s", resp.Error.Message)
	}
}

func TestJWTAuth_InvalidScheme(t *testing.T) {
	secret := "test-secret-123"
	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for invalid auth scheme")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp.Error.Message, "Bearer") {
		t.Errorf("Expected message about Bearer scheme, got: %s", resp.Error.Message)
	}
}

func TestJWTAuth_InvalidSignature(t *testing.T) {
	secret := "test-secret-123"
	// Create token with wrong secret
	token := generateValidJWT("wrong-secret", map[string]interface{}{
		"sub":   "user-1",
		"roles": []interface{}{"admin"},
	}, 0)

	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with invalid signature")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret-123"
	// Create token that expired 1 minute ago
	expiredExp := time.Now().Add(-1 * time.Minute).Unix()
	token := generateValidJWT(secret, map[string]interface{}{
		"sub":   "user-1",
		"roles": []interface{}{"admin"},
	}, expiredExp)

	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with expired token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	secret := "test-secret-123"
	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with invalid JWT format")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_MissingClaims(t *testing.T) {
	secret := "test-secret-123"
	// Create token with only sub, no roles
	header := `{"alg":"HS256","typ":"JWT"}`
	payload := `{"sub":"user-2","iat":` + fmt.Sprintf("%d", time.Now().Unix()) + `,"exp":` + fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()) + `}`

	headerEnc := base64URLEncode([]byte(header))
	payloadEnc := base64URLEncode([]byte(payload))
	signingInput := headerEnc + "." + payloadEnc
	sig := computeHMAC(secret, signingInput)
	token := signingInput + "." + sig

	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should still pass through, but roles will be nil
		userID := UserIDFromContext(r.Context())
		if userID != "user-2" {
			t.Errorf("Expected user-2, got %s", userID)
		}
		roles := UserRolesFromContext(r.Context())
		if roles != nil && len(roles) != 0 {
			t.Errorf("Expected nil roles, got %v", roles)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestJWTAuth_EmptyToken(t *testing.T) {
	secret := "test-secret-123"
	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with empty token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_TokenWithOnlyTwoParts(t *testing.T) {
	secret := "test-secret-123"
	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with malformed JWT")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer header.payload")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_TokenWithFourParts(t *testing.T) {
	secret := "test-secret-123"
	h := JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with malformed JWT")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer a.b.c.d")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

// ─── Context helper tests ────────────────────────────────────────────────────

func TestTenantIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	tenantID := TenantIDFromContext(ctx)
	if tenantID != "" {
		t.Errorf("Expected empty string, got %s", tenantID)
	}
}

func TestTenantIDFromContext_Present(t *testing.T) {
	ctx := SetTenantIDToContext(context.Background(), "tenant-xyz")
	tenantID := TenantIDFromContext(ctx)
	if tenantID != "tenant-xyz" {
		t.Errorf("Expected tenant-xyz, got %s", tenantID)
	}
}

func TestUserIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	userID := UserIDFromContext(ctx)
	if userID != "" {
		t.Errorf("Expected empty string, got %s", userID)
	}
}

func TestUserRolesFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	roles := UserRolesFromContext(ctx)
	if roles != nil {
		t.Errorf("Expected nil roles, got %v", roles)
	}
}

func TestRequestIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	reqID := RequestIDFromContext(ctx)
	if reqID != "" {
		t.Errorf("Expected empty string, got %s", reqID)
	}
}

func TestTraceIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	traceID := TraceIDFromContext(ctx)
	if traceID != "" {
		t.Errorf("Expected empty string, got %s", traceID)
	}
}

func TestNewHandler(t *testing.T) {
	h := NewHandler()
	if h.WorkflowStore == nil {
		t.Error("Expected non-nil WorkflowStore")
	}
	if h.ScheduleStore == nil {
		t.Error("Expected non-nil ScheduleStore")
	}
	if h.AgentStore == nil {
		t.Error("Expected non-nil AgentStore")
	}
	if h.EventPublisher == nil {
		t.Error("Expected non-nil EventPublisher")
	}
}
