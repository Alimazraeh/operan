// Admin login — password file auth for bootstrap access.
// POST /api/v1/iam/admin/login
// Validates the password against a file and returns a signed JWT.
package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/operan/modules/02-identity-access/internal/config"
)

// AdminLoginHandler handles password-file-based admin bootstrap login.
type AdminLoginHandler struct {
	config *config.Config
}

func NewAdminLoginHandler(cfg *config.Config) *AdminLoginHandler {
	return &AdminLoginHandler{config: cfg}
}

func (h *AdminLoginHandler) WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AdminLoginHandler) WriteError(w http.ResponseWriter, status int, code int, message string, detail string) {
	h.WriteJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"detail":  detail,
		},
	})
}

// AdminLoginRequest is the body for POST /api/v1/iam/admin/login.
type AdminLoginRequest struct {
	Password string `json:"password"`
	Tenant   string `json:"tenant,omitempty"`
}

// AdminLoginResponse is the JWT token + user info returned on success.
type AdminLoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// Login handles POST /api/v1/iam/admin/login.
func (h *AdminLoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "failed to read body")
		return
	}
	defer r.Body.Close()

	var req AdminLoginRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
		return
	}
	if req.Password == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "password required", "password field is required")
		return
	}

	// Admin password: check env var first (ADMIN_PASSWORD), then file (IAM_ADMIN_PASSWORD_FILE), then default
	var storedPassword string

	if envPwd := os.Getenv("ADMIN_PASSWORD"); envPwd != "" {
		storedPassword = envPwd
	} else if filePwd := os.Getenv("IAM_ADMIN_PASSWORD_FILE"); filePwd != "" {
		content, err := os.ReadFile(filePwd)
		if err != nil {
			h.WriteError(w, http.StatusServiceUnavailable, 503, "password file not found",
				fmt.Sprintf("cannot read %s — set ADMIN_PASSWORD env var to set login password", filePwd))
			return
		}
		storedPassword = strings.TrimSpace(string(content))
	} else {
		storedPassword = "operan-admin-2026"
	}

	if req.Password != storedPassword {
		h.WriteError(w, http.StatusUnauthorized, 401, "invalid password", "password does not match the stored value")
		return
	}

	// Defaults
	tenantID := req.Tenant
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	userID := "admin-001"
	email := "admin@operan"
	now := time.Now().UTC()

	// Token expiry
	expiryMinutes := h.config.TokenExpiry
	if expiryMinutes <= 0 {
		expiryMinutes = 480 // 8 hours default
	}

	// Check for token secret (used to sign JWT)
	tokenSecret := h.config.TokenSecret
	if tokenSecret == "" {
		h.WriteError(w, http.StatusInternalServerError, 500, "token secret not configured",
			"IAM_TOKEN_SECRET must be set in the environment to issue JWTs")
		return
	}

	// Mint JWT
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        userID,
		"user_type":  "user",
		"tenant_id":  tenantID,
		"email":      email,
		"role":       "admin",
		"roles":      []string{"admin"},
		// The platform's canonical issuer — M01/M10/M18 pin this exact value,
		// M17/M06 accept the "operan-" prefix, M08/M11 don't check issuer.
		"iss":        "operan-tenant-control-plane",
		"iat":        now.Unix(),
		"exp":        now.Add(time.Duration(expiryMinutes) * time.Minute).Unix(),
	})

	tokenStr, err := jwtToken.SignedString([]byte(tokenSecret))
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, "failed to sign token", err.Error())
		return
	}

	// Write admin ID for easy retrieval
	adminIDFile := os.Getenv("IAM_ADMIN_USERID_FILE")
	if adminIDFile == "" {
		adminIDFile = "/etc/operan/admin-id"
	}
	if err := os.MkdirAll(filepath.Dir(adminIDFile), 0755); err == nil {
		os.WriteFile(adminIDFile, []byte(userID+"\n"), 0600)
	}

	h.WriteJSON(w, http.StatusOK, AdminLoginResponse{
		Token:  tokenStr,
		UserID: userID,
		Email:  email,
	})
}

// GeneratePassword generates a cryptographically secure random password and writes it to the password file.
func (h *AdminLoginHandler) GeneratePassword(w http.ResponseWriter, r *http.Request) {
	// Generate 24 random bytes → 48 hex chars → "admin-" + 48 chars = 54 char password
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, "random read failed", err.Error())
		return
	}
	password := "admin-" + fmt.Sprintf("%x", bytes)

	passFile := os.Getenv("IAM_ADMIN_PASSWORD_FILE")
	if passFile == "" {
		passFile = "/etc/operan/admin.pass"
	}
	// Create the parent directory of the password file
	if err := os.MkdirAll(filepath.Dir(passFile), 0755); err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, "directory creation failed", err.Error())
		return
	}
	if err := os.WriteFile(passFile, []byte(password+"\n"), 0600); err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, "file write failed", err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"password": password,
		"file":     passFile,
		"message":  "password written — keep this file secure",
	})
}