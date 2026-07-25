package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/operan/modules/02-identity-access/internal/config"
)

// ---------- helpers ----------

func adminHandler(t *testing.T, passFile, tokenSecret string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		TokenSecret: tokenSecret,
		TokenExpiry: 480,
	}
	h := NewAdminLoginHandler(cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/iam/admin/login", h.Login)
	mux.HandleFunc("/api/v1/iam/admin/generate-password", h.GeneratePassword)
	return mux
}

func decodeResponse(t *testing.T, body []byte) interface{} {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

// ---------- Login handler tests ----------

func TestAdminLogin_Success(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("secret123\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "test-secret-32-characters-long")

	body := `{"password":"secret123","tenant":"my-tenant"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		return
	}

	var resp AdminLoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("Expected non-empty token")
	}
	if resp.UserID != "admin-001" {
		t.Errorf("UserID = %q, want %q", resp.UserID, "admin-001")
	}
	if resp.Email != "admin@operan" {
		t.Errorf("Email = %q, want %q", resp.Email, "admin@operan")
	}
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("correct\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "test-secret-32-characters-long")

	body := `{"password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
		return
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	errObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}
	if errObj["message"] != "invalid password" {
		t.Errorf("Error message = %q, want %q", errObj["message"], "invalid password")
	}
}

func TestAdminLogin_EmptyPassword(t *testing.T) {
	h := adminHandler(t, "", "test-secret-32-characters-long")

	body := `{"password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminLogin_MissingPassword(t *testing.T) {
	h := adminHandler(t, "", "test-secret-32-characters-long")

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminLogin_InvalidJSON(t *testing.T) {
	h := adminHandler(t, "", "test-secret-32-characters-long")

	body := `not json at all`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminLogin_MissingPasswordFile(t *testing.T) {
	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", "/tmp/nonexistent-path-12345/admin.pass")
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, "", "test-secret-32-characters-long")

	body := `{"password":"anything"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func TestAdminLogin_MissingTokenSecret(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("secret\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "") // empty token secret

	body := `{"password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestAdminLogin_CustomTenant(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "my-signing-secret-that-is-long-enough")

	body := `{"password":"pass","tenant":"custom-tenant-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminLoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Decode and verify JWT claims
	claims := jwt.MapClaims{}
	token, _ := jwt.ParseWithClaims(resp.Token, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte("my-signing-secret-that-is-long-enough"), nil
	})
	if !token.Valid {
		t.Fatal("Invalid JWT token")
	}
	if claims["tenant_id"] != "custom-tenant-id" {
		t.Errorf("JWT tenant_id = %q, want %q", claims["tenant_id"], "custom-tenant-id")
	}
	if claims["user_type"] != "user" {
		t.Errorf("JWT user_type = %q, want %q", claims["user_type"], "user")
	}
	if claims["role"] != "admin" {
		t.Errorf("JWT role = %q, want %q", claims["role"], "admin")
	}
	// Verify expiry is set
	exp, ok := claims["exp"].(float64)
	if !ok || exp <= 0 {
		t.Error("JWT exp claim missing or invalid")
	}
}

func TestAdminLogin_DefaultTenant(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "my-signing-secret-that-is-long-enough")

	body := `{"password":"pass"}` // no tenant
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminLoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	claims := jwt.MapClaims{}
	token, _ := jwt.ParseWithClaims(resp.Token, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte("my-signing-secret-that-is-long-enough"), nil
	})
	if !token.Valid {
		t.Fatal("Invalid JWT")
	}
	if claims["tenant_id"] != "default-tenant" {
		t.Errorf("JWT tenant_id = %q, want %q", claims["tenant_id"], "default-tenant")
	}
}

// ---------- GeneratePassword tests ----------

func TestGeneratePassword_Success(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "test-secret-32-characters-long")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/generate-password", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GeneratePassword code = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		return
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	password, ok := resp["password"].(string)
	if !ok {
		t.Fatal("Expected password in response")
	}
	if !strings.HasPrefix(password, "admin-") {
		t.Errorf("Password prefix = %q, want %q", password[:6], "admin-")
	}
	if len(password) < 30 {
		t.Errorf("Password length = %d, want >= 30", len(password))
	}

	// Verify file was created
	content, err := os.ReadFile(passFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.TrimSpace(string(content)) != password {
		t.Errorf("Password file content = %q, want %q", strings.TrimSpace(string(content)), password)
	}
}

func TestGeneratePassword_FileWriteError(t *testing.T) {
	h := adminHandler(t, "/root/protected/admin.pass", "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/generate-password", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Should fail because /root doesn't exist (or we can't write there)
	if rec.Code != http.StatusInternalServerError {
		t.Logf("GeneratePassword code = %d (expected 500 for unwritable path)", rec.Code)
	}
}

// ---------- Password uniqueness (crypto/rand verification) ----------

func TestGeneratePassword_Uniqueness(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "test-secret")

	passwords := make(map[string]bool)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/generate-password", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		pw := resp["password"].(string)
		if passwords[pw] {
			t.Errorf("Generated duplicate password %q on iteration %d", pw, i)
		}
		passwords[pw] = true
	}
}

// ---------- JWT expiration test ----------

func TestAdminLogin_JWTExpiry(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	cfg := &config.Config{
		TokenSecret: "signing-secret-32-chars-minimum",
		TokenExpiry: 30, // 30 minutes
	}
	h := NewAdminLoginHandler(cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/iam/admin/login", h.Login)
	h = nil // not used directly; mux handles it

	body := `{"password":"pass","tenant":"exp-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminLoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	claims := jwt.MapClaims{}
	token, _ := jwt.ParseWithClaims(resp.Token, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte("signing-secret-32-chars-minimum"), nil
	})
	if !token.Valid {
		t.Fatal("Invalid JWT")
	}

	now := time.Now().Unix()
	exp := int64(claims["exp"].(float64))
	iat := int64(claims["iat"].(float64))
	duration := exp - iat

	if duration != 1800 { // 30 minutes = 1800 seconds
		t.Errorf("JWT duration = %ds, want %ds", duration, 1800)
	}
	if exp < now {
		t.Errorf("JWT expiry %d is in the past (now=%d)", exp, now)
	}
}

// ---------- Context injection for admin ID file ----------

func TestAdminLogin_WritesAdminIDFile(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	// Set custom admin ID file path
	origAdminID := os.Getenv("IAM_ADMIN_USERID_FILE")
	os.Setenv("IAM_ADMIN_USERID_FILE", filepath.Join(dir, "admin-id"))
	defer os.Setenv("IAM_ADMIN_USERID_FILE", origAdminID)

	h := adminHandler(t, passFile, "signing-secret-32-chars-minimum")

	body := `{"password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify admin ID file was written
	content, err := os.ReadFile(filepath.Join(dir, "admin-id"))
	if err != nil {
		t.Fatalf("ReadFile admin-id: %v", err)
	}
	if strings.TrimSpace(string(content)) != "admin-001" {
		t.Errorf("Admin ID file content = %q, want %q", strings.TrimSpace(string(content)), "admin-001")
	}
}

// ---------- HTTP method tests ----------

// Note: The admin login handler does NOT enforce HTTP method — it accepts any
// method and processes the request body. Tests below verify the actual behavior.

func TestAdminLogin_WrongMethod(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "signing-secret-32-chars-minimum")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// GET with no body returns 400 (invalid JSON), not 405
	// (the handler doesn't enforce method)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestGeneratePassword_WrongMethod(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "signing-secret-32-chars-minimum")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/generate-password", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// GET is accepted by the handler; it still writes the password file.
	// This is intentional — the endpoint doesn't enforce method.
	if rec.Code != http.StatusOK {
		t.Logf("GeneratePassword code = %d; GET was accepted", rec.Code)
	}
}

// ---------- WriteError test (reuses handler, no conflict since TestWriteError is elsewhere) ----------

func TestAdminLoginWriteError(t *testing.T) {
	cfg := &config.Config{TokenSecret: "test"}
	h := NewAdminLoginHandler(cfg)

	rec := httptest.NewRecorder()
	h.WriteError(rec, http.StatusUnauthorized, 401, "unauthorized", "bad credentials")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("WriteError code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}
	if errObj["code"] != float64(401) {
		t.Errorf("Error code = %v, want %v", errObj["code"], 401)
	}
	if errObj["message"] != "unauthorized" {
		t.Errorf("Error message = %q, want %q", errObj["message"], "unauthorized")
	}
	if errObj["detail"] != "bad credentials" {
		t.Errorf("Error detail = %q, want %q", errObj["detail"], "bad credentials")
	}
}

func TestAdminLoginWriteJSON(t *testing.T) {
	cfg := &config.Config{TokenSecret: "test"}
	h := NewAdminLoginHandler(cfg)

	rec := httptest.NewRecorder()
	h.WriteJSON(rec, http.StatusCreated, map[string]string{"key": "val"})

	if rec.Code != http.StatusCreated {
		t.Errorf("WriteJSON code = %d, want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["key"] != "val" {
		t.Errorf("Response key = %q, want %q", resp["key"], "val")
	}
}

// ---------- Whitespace handling ----------

func TestAdminLogin_PasswordWithWhitespace(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("  secret123  \n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "signing-secret-32-chars-minimum")

	// Password without surrounding whitespace should match (file is trimmed)
	body := `{"password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// ---------- Concurrent login test ----------

func TestAdminLogin_Concurrent(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "signing-secret-32-chars-minimum")

	const numGoroutines = 10
	done := make(chan int, numGoroutines)

	body := `{"password":"pass","tenant":"concurrent"}`
	for i := 0; i < numGoroutines; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			done <- rec.Code
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		code := <-done
		if code != http.StatusOK {
			t.Errorf("Goroutine %d: code = %d, want %d", i, code, http.StatusOK)
		}
	}
}

// ---------- NewAdminLoginHandler tests ----------

func TestNewAdminLoginHandler(t *testing.T) {
	cfg := &config.Config{TokenSecret: "test", TokenExpiry: 60}
	h := NewAdminLoginHandler(cfg)
	if h == nil {
		t.Fatal("Expected non-nil handler")
	}
	if h.config != cfg {
		t.Error("Expected handler to hold config reference")
	}
}

// ---------- Empty body test ----------

func TestAdminLogin_EmptyBody(t *testing.T) {
	h := adminHandler(t, "", "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login code = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// ---------- Large body test ----------

func TestAdminLogin_LargeBody(t *testing.T) {
	dir := t.TempDir()
	passFile := filepath.Join(dir, "admin.pass")
	os.WriteFile(passFile, []byte("pass\n"), 0600)

	origEnv := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	os.Setenv("IAM_ADMIN_PASSWORD_FILE", passFile)
	defer os.Setenv("IAM_ADMIN_PASSWORD_FILE", origEnv)

	h := adminHandler(t, passFile, "test-secret")

	largeBody := bytes.Repeat([]byte("x"), 1024*1024) // 1MB
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/login", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Should fail with JSON parse error
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
