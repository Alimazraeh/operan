package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func makeJWT(secret string, claims map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pj, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(pj)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + payload + "." + sig
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestJWTAuth(t *testing.T) {
	secret := "test-secret"
	h := RequestID(JWTAuth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserIDFromContext(r.Context()) != "user-1" {
			t.Errorf("user id not in context: %q", UserIDFromContext(r.Context()))
		}
		if roles := UserRolesFromContext(r.Context()); len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("roles not in context: %v", roles)
		}
		w.WriteHeader(http.StatusOK)
	})))

	token := makeJWT(secret, map[string]interface{}{
		"sub": "user-1", "roles": []string{"admin"},
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	})

	// valid
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("valid token = %d, want 200", w.Code)
	}

	// missing header
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing auth = %d, want 401", w.Code)
	}

	// bad scheme
	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Authorization", "Basic abc")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("bad scheme = %d, want 401", w.Code)
	}

	// tampered signature
	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Authorization", "Bearer "+token+"tamper")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("tampered = %d, want 401", w.Code)
	}

	// expired
	expired := makeJWT(secret, map[string]interface{}{"sub": "u", "exp": float64(time.Now().Add(-time.Hour).Unix())})
	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Authorization", "Bearer "+expired)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expired = %d, want 401", w.Code)
	}
}

func TestTenantContext(t *testing.T) {
	h := RequestID(TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if TenantIDFromContext(r.Context()) != "t1" {
			t.Errorf("tenant not set: %q", TenantIDFromContext(r.Context()))
		}
		w.WriteHeader(http.StatusOK)
	})))

	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("X-Tenant-ID", "t1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("with tenant = %d, want 200", w.Code)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing tenant = %d, want 400", w.Code)
	}
}

func TestRateLimit(t *testing.T) {
	h := RequestID(TenantContext(RateLimit(2, time.Minute)(okHandler())))
	call := func() int {
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		r.Header.Set("X-Tenant-ID", "t1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if call() != http.StatusOK || call() != http.StatusOK {
		t.Error("first two requests should pass")
	}
	if call() != http.StatusTooManyRequests {
		t.Error("third request should be rate-limited")
	}
}

func TestTraceID_PropagatesAndGenerates(t *testing.T) {
	var seen string
	h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = TenantIDFromContext(r.Context()) // unused tenant; ensure no panic
		_ = seen
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("X-Trace-Id", "trace-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("X-Trace-Id") != "trace-123" {
		t.Errorf("trace id not propagated: %q", w.Header().Get("X-Trace-Id"))
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("X-Trace-Id") == "" {
		t.Error("trace id should be generated when absent")
	}
}
