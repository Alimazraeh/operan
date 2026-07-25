package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/operan/modules/02-identity-access/internal/auth"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// AuthHandler issues tokens for real, named users.
//
// This is the counterpart to the shared-admin login: that one always returns
// sub="admin-001", so every approval, request and audit entry in the platform
// is attributed to the same synthetic identity. Tokens minted here carry the
// user's own id, and the claims are otherwise identical — same signing secret,
// same issuer (operan-tenant-control-plane) — so no downstream module needs to
// change to accept them.
type AuthHandler struct {
	Users       store.UserStoreAPI
	TokenSecret string
	ExpiryMins  int
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Tenant   string `json:"tenant"`
}

// Login handles POST /api/v1/iam/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" || req.Tenant == "" {
		writeAuthError(w, http.StatusBadRequest, "tenant, email and password are required")
		return
	}
	if h.TokenSecret == "" {
		writeAuthError(w, http.StatusInternalServerError, "token secret not configured")
		return
	}

	// One answer for "no such user", "no credential set" and "wrong password",
	// so this endpoint cannot be used to discover which accounts exist.
	user, err := h.Users.GetByTenantAndEmail(req.Tenant, req.Email)
	if err != nil || user == nil {
		log.Printf("[IAM] login rejected for %s@%s: no such user", req.Email, req.Tenant)
		writeAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if user.Status == "deactivated" || user.Status == "suspended" {
		log.Printf("[IAM] login rejected for %s: account %s", user.ID, user.Status)
		writeAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		log.Printf("[IAM] login rejected for %s: %v", user.ID, err)
		writeAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, expiresAt, err := h.mintToken(user)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	log.Printf("[IAM] login ok: %s (%s) tenant=%s roles=%v", user.ID, user.Email, user.TenantID, user.RoleIDs)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":        token,
		"user_id":      user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"tenant":       user.TenantID,
		"roles":        orEmpty(user.RoleIDs),
		"expires_at":   expiresAt.Format(time.RFC3339),
	})
}

// mintToken produces the platform JWT. The claim set matches the shared-admin
// login exactly except that sub, email and roles describe a real user.
func (h *AuthHandler) mintToken(user *models.User) (string, time.Time, error) {
	expiry := h.ExpiryMins
	if expiry <= 0 {
		expiry = 480
	}
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expiry) * time.Minute)

	roles := orEmpty(user.RoleIDs)
	primary := "user"
	if len(roles) > 0 {
		primary = roles[0]
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       user.ID,
		"user_type": "user",
		"tenant_id": user.TenantID,
		"email":     user.Email,
		"role":      primary, // singular: Module 04 reads this
		"roles":     roles,   // array: Modules 08/09/11 read this
		// The platform's canonical issuer — M01/M10/M18 pin this exact value.
		"iss": "operan-tenant-control-plane",
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	})
	signed, err := tok.SignedString([]byte(h.TokenSecret))
	return signed, expiresAt, err
}

type setPasswordRequest struct {
	Password string `json:"password"`
}

// SetPassword handles POST /api/v1/iam/users/{id}/password — an administrator
// provisioning a credential. There is deliberately no self-service reset flow
// yet; see the package comment in internal/auth.
func (h *AuthHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	userID := extractUserIDFromPasswordPath(r.URL.Path)
	if userID == "" {
		writeAuthError(w, http.StatusBadRequest, "user id is required")
		return
	}
	var req setPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, err := h.Users.GetByID(userID); err != nil {
		writeAuthError(w, http.StatusNotFound, "user not found")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeAuthError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	setter, ok := h.Users.(interface{ SetPassword(id, hash string) error })
	if !ok {
		writeAuthError(w, http.StatusNotImplemented, "this deployment cannot store passwords")
		return
	}
	if err := setter.SetPassword(userID, hash); err != nil {
		writeAuthError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[IAM] password set for user %s", userID)
	w.WriteHeader(http.StatusNoContent)
}

// extractUserIDFromPasswordPath pulls {id} out of .../users/{id}/password.
func extractUserIDFromPasswordPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "users" && i+2 < len(parts) && parts[i+2] == "password" {
			return parts[i+1]
		}
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
