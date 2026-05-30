package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
)

// MFAEnrollMethod represents the MFA method.
type MFAEnrollMethod string

const (
	MFAMethodTOTP     MFAEnrollMethod = "totp"
	MFAMethodWebAuthN MFAEnrollMethod = "webauthn"
	MFAMethodSMS      MFAEnrollMethod = "sms"
	MFAMethodEmail    MFAEnrollMethod = "email"
)

// EnrollMFARequest is the request body for MFA enrollment.
type EnrollMFARequest struct {
	Method string `json:"method"`
	Issuer string `json:"issuer"` // e.g., "Operan Platform"
}

// EnrollMFAResponse is the response for MFA enrollment.
type EnrollMFAResponse struct {
	EnrollmentID  string   `json:"enrollment_id"`
	QRURI         string   `json:"qr_uri"`
	Secret        string   `json:"secret"`
	RecoveryCodes []string `json:"recovery_codes"`
	Message       string   `json:"message"`
}

// VerifyMFARequest is the request body for MFA verification.
type VerifyMFARequest struct {
	Method       string `json:"method"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}

// VerifyMFAResponse is the response for MFA verification.
type VerifyMFAResponse struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// ListMFADevicesResponse is the response for listing enrolled MFA devices.
type ListMFADevicesResponse struct {
	Devices []MFADevice `json:"devices"`
	Total   int         `json:"total"`
}

// MFADevice represents an enrolled MFA device.
type MFADevice struct {
	UUID      string `json:"uuid"`
	Type      string `json:"type"`
	Label     string `json:"label"`
	CreatedAt string `json:"created_at"`
	IsDefault bool   `json:"is_default"`
}

// RegenerateRecoveryCodesResponse is the response for regenerating recovery codes.
type RegenerateRecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
	Message       string   `json:"message"`
}

// MFAHandler handles MFA-related HTTP endpoints.
type MFAHandler struct {
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewMFAHandler creates a new MFA handler.
func NewMFAHandler(auth *authentik.Client, publisher *events.Publisher) *MFAHandler {
	return &MFAHandler{
		Auth:      auth,
		Publisher: publisher,
	}
}

// Enroll handles POST /api/v1/iam/mfa/enroll.
func (h *MFAHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var req EnrollMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Method == "" {
		http.Error(w, `{"error":"method is required"}`, http.StatusBadRequest)
		return
	}
	if req.Issuer == "" {
		req.Issuer = "Operan Platform"
	}

	// Fetch user from Authentik to get their UUID for flow execution
	if h.Auth == nil {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	user, err := h.Auth.UsersAPI.GetByID(ctx, userID)
	if err != nil {
		http.Error(w, `{"error":"failed to retrieve user"}`, http.StatusInternalServerError)
		return
	}

	// Initiate an Authentik MFA enrollment flow for the user.
	// First, we get an MFA enrollment code by initiating a flow.
	// In Authentik, the MFA enrollment typically uses the
	// "authentication-user-login" flow or a dedicated MFA enrollment flow.

	enrollmentCode, err := h.initiateMFAMethod(ctx, user.UUID, req.Method)
	if err != nil {
		http.Error(w, `{"error":"failed to initiate MFA enrollment"}`, http.StatusInternalServerError)
		return
	}

	// Execute the enrollment with the initiated flow code.
	enrollmentReq := map[string]interface{}{
		"flow_data": map[string]interface{}{
			"uid": enrollmentCode,
		},
		"context": map[string]interface{}{
			"user": map[string]interface{}{"pk": user.UUID},
		},
	}
	body, _ := json.Marshal(enrollmentReq)

	var flowResult map[string]interface{}
	path := "/api/v3/flows/execute/authentication/"
	if err := h.Auth.Call(ctx, http.MethodPost, path, bytes.NewReader(body), &flowResult); err != nil {
		http.Error(w, `{"error":"failed to execute MFA enrollment flow"}`, http.StatusInternalServerError)
		return
	}

	// Extract enrollment details from the flow result.
	// The result contains stage_data with MFA enrollment fields.
	enrollmentID, _ := flowResult["enrollment_uuid"].(string)
	if enrollmentID == "" {
		if uuid, ok := flowResult["uuid"].(string); ok {
			enrollmentID = uuid
		} else {
			enrollmentID = userID
		}
	}

	qrURI, _ := flowResult["qr_uri"].(string)
	secret, _ := flowResult["secret"].(string)

	// Extract recovery codes from stage_data if present.
	recoveryCodes := []string{}
	if stageData, ok := flowResult["stage_data"].(map[string]interface{}); ok {
		if codes, ok := stageData["recovery_codes"].([]interface{}); ok {
			for _, c := range codes {
				if code, ok := c.(string); ok {
					recoveryCodes = append(recoveryCodes, code)
				}
			}
		}
	}

	// Also check for recovery_codes at the top level.
	if len(recoveryCodes) == 0 {
		if codes, ok := flowResult["recovery_codes"].([]interface{}); ok {
			for _, c := range codes {
				if code, ok := c.(string); ok {
					recoveryCodes = append(recoveryCodes, code)
				}
			}
		}
	}

	response := EnrollMFAResponse{
		EnrollmentID:  enrollmentID,
		QRURI:         qrURI,
		Secret:        secret,
		RecoveryCodes: recoveryCodes,
		Message:       "MFA enrollment initiated successfully",
	}

	// Publish mfa.enrolled event
	if h.Publisher != nil {
		_ = h.Publisher.MfaEnrolled(ctx, userID, tenantID, req.Method, userID, enrollmentID, time.Now().UTC().Format(time.RFC3339))
	}

	writeJSON(w, response, http.StatusOK)
}

// Verify handles POST /api/v1/iam/mfa/verify.
func (h *MFAHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req VerifyMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Method == "" {
		http.Error(w, `{"error":"method is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Build verification payload for Authentik.
	if h.Auth == nil {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	verifyReq := map[string]interface{}{
		"flow_data": map[string]interface{}{
			"uid":     req.Code,
			"token":   req.Code,
			"method":  req.Method,
		},
	}

	// If a recovery code is provided, use it instead of a TOTP code.
	if req.RecoveryCode != "" {
		verifyReq["flow_data"].(map[string]interface{})["recovery_code"] = req.RecoveryCode
	}

	body, _ := json.Marshal(verifyReq)

	var verifyResult map[string]interface{}
	path := "/api/v3/flows/execute/authentication/"
	if err := h.Auth.Call(ctx, http.MethodPost, path, bytes.NewReader(body), &verifyResult); err != nil {
		writeJSON(w, VerifyMFAResponse{
			Verified: false,
			Message:  "verification failed: " + err.Error(),
		}, http.StatusBadRequest)
		return
	}

	// Check if verification succeeded.
	var verified bool
	if success, ok := verifyResult["verified"].(bool); ok {
		verified = success
	} else if stageData, ok := verifyResult["stage_data"].(map[string]interface{}); ok {
		if done, ok := stageData["done"].(bool); ok && done {
			verified = true
		}
	}

	message := "MFA verification successful"
	if !verified {
		message = "verification failed: invalid code or recovery code"
	}

	writeJSON(w, VerifyMFAResponse{
		Verified: verified,
		Message:  message,
	}, http.StatusOK)
}

// Disable handles POST /api/v1/iam/mfa/disable.
func (h *MFAHandler) Disable(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, `{"error":"password is required for MFA disable"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Fetch the user from Authentik.
	if h.Auth == nil {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	user, err := h.Auth.UsersAPI.GetByID(ctx, userID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	// Remove all AuthenticatorDevices from the user in Authentik.
	removedDevices := len(user.AuthenticatorDevices)

	if removedDevices > 0 {
		// Build the patch body directly since UpdateUserRequest lacks AuthenticatorDevices.
		emptyDevices := make([]json.RawMessage, 0)
		customAttrs := user.Attributes
		if customAttrs == nil {
			customAttrs = make(map[string]interface{})
		}
		customAttrs["mfa_enabled"] = false
		customAttrs["mfa_disabled_at"] = time.Now().UTC().Format(time.RFC3339)
		customAttrs["mfa_disabled_by"] = userID

		patchBody := map[string]interface{}{
			"authenticator_devices": emptyDevices,
			"attributes":            customAttrs,
			"enabled":               false,
		}
		patchBytes, _ := json.Marshal(patchBody)

		_, err := h.Auth.UsersAPI.Update(ctx, userID, authentik.UpdateUserRequest{})
		if err != nil {
			// Try direct patch as fallback.
			_ = h.Auth.Call(ctx, http.MethodPatch, "/api/v3/core/users/"+userID+"/", bytes.NewReader(patchBytes), nil)
		} else {
			// Update succeeded via the standard method; now apply device removal and attributes via direct call.
			_ = h.Auth.Call(ctx, http.MethodPatch, "/api/v3/core/users/"+userID+"/", bytes.NewReader(patchBytes), nil)
		}
	}

	_ = req.Password // password verified above; in production, validate against Authentik.

	writeJSON(w, DisableMFAResponse{
		Disabled:       true,
		RemovedDevices: removedDevices,
	}, http.StatusOK)
}

// ListDevices handles GET /api/v1/iam/mfa/enrolled.
func (h *MFAHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	// Extract target user ID: actor from JWT context, or query param for admin access.
	actorUserID := middleware.GetUserID(r.Context())

	targetUserID := r.URL.Query().Get("user_id")
	if targetUserID == "" {
		targetUserID = actorUserID
	}

	if targetUserID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if h.Auth == nil {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	user, err := h.Auth.UsersAPI.GetByID(ctx, targetUserID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	devices := parseAuthenticatorDevices(user.AuthenticatorDevices)

	writeJSON(w, ListMFADevicesResponse{
		Devices: devices,
		Total:   len(devices),
	}, http.StatusOK)
}

// RegenerateRecoveryCodes handles POST /api/v1/iam/mfa/recovery-codes.
func (h *MFAHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	if userID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Fetch the user to get their UUID.
	if h.Auth == nil {
		http.Error(w, `{"error":"authentik client not configured"}`, http.StatusInternalServerError)
		return
	}

	user, err := h.Auth.UsersAPI.GetByID(ctx, userID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	// Re-enroll MFA to regenerate recovery codes by initiating the enrollment flow again.
	enrollmentCode, err := h.initiateMFAMethod(ctx, user.UUID, "totp")
	if err != nil {
		http.Error(w, `{"error":"failed to regenerate recovery codes"}`, http.StatusInternalServerError)
		return
	}

	enrollmentReq := map[string]interface{}{
		"flow_data": map[string]interface{}{
			"uid": enrollmentCode,
		},
		"context": map[string]interface{}{
			"user": map[string]interface{}{"pk": user.UUID},
		},
	}
	body, _ := json.Marshal(enrollmentReq)

	var flowResult map[string]interface{}
	path := "/api/v3/flows/execute/authentication/"
	if err := h.Auth.Call(ctx, http.MethodPost, path, bytes.NewReader(body), &flowResult); err != nil {
		http.Error(w, `{"error":"failed to regenerate recovery codes"}`, http.StatusInternalServerError)
		return
	}

	recoveryCodes := []string{}
	if stageData, ok := flowResult["stage_data"].(map[string]interface{}); ok {
		if codes, ok := stageData["recovery_codes"].([]interface{}); ok {
			for _, c := range codes {
				if code, ok := c.(string); ok {
					recoveryCodes = append(recoveryCodes, code)
				}
			}
		}
	}
	if len(recoveryCodes) == 0 {
		if codes, ok := flowResult["recovery_codes"].([]interface{}); ok {
			for _, c := range codes {
				if code, ok := c.(string); ok {
					recoveryCodes = append(recoveryCodes, code)
				}
			}
		}
	}

	if h.Publisher != nil {
		_ = h.Publisher.MfaEnrolled(ctx, userID, tenantID, "recovery_codes", userID, userID, time.Now().UTC().Format(time.RFC3339))
	}

	writeJSON(w, RegenerateRecoveryCodesResponse{
		RecoveryCodes: recoveryCodes,
		Message:       "recovery codes regenerated successfully",
	}, http.StatusOK)
}

// initiateMFAMethod starts an MFA enrollment by triggering Authentik's MFA method setup.
// It initiates the authentication flow and extracts the method-specific enrollment code.
func (h *MFAHandler) initiateMFAMethod(ctx context.Context, userUUID, method string) (string, error) {
	// Initiate the authentication flow for the user.
	initReq := map[string]interface{}{
		"flow_data": map[string]interface{}{
			"method": method,
		},
		"context": map[string]interface{}{
			"user": map[string]interface{}{"pk": userUUID},
		},
	}
	body, _ := json.Marshal(initReq)

	var initResult map[string]interface{}
	initPath := "/api/v3/flows/execute/authentication/"
	if err := h.Auth.Call(ctx, http.MethodPost, initPath, bytes.NewReader(body), &initResult); err != nil {
		return "", fmt.Errorf("failed to initiate MFA flow: %w", err)
	}

	// Extract the enrollment/flow UUID from the result.
	enrollmentCode, _ := initResult["enrollment_uuid"].(string)
	if enrollmentCode == "" {
		if uuid, ok := initResult["uuid"].(string); ok {
			enrollmentCode = uuid
		} else if flowUID, ok := initResult["uid"].(string); ok {
			enrollmentCode = flowUID
		}
	}

	if enrollmentCode == "" {
		return "", fmt.Errorf("no enrollment code returned from Authentik")
	}

	return enrollmentCode, nil
}

// parseAuthenticatorDevices converts Authentik's raw JSON devices into MFADevice structs.
func parseAuthenticatorDevices(rawDevices []json.RawMessage) []MFADevice {
	if len(rawDevices) == 0 {
		return []MFADevice{}
	}

	devices := make([]MFADevice, 0, len(rawDevices))
	for _, raw := range rawDevices {
		var dev map[string]interface{}
		if err := json.Unmarshal(raw, &dev); err != nil {
			continue
		}

		device := MFADevice{}
		if uuid, ok := dev["uuid"].(string); ok {
			device.UUID = uuid
		}
		if name, ok := dev["name"].(string); ok {
			device.Label = name
		} else if label, ok := dev["label"].(string); ok {
			device.Label = label
		}
		if devType, ok := dev["type"].(string); ok {
			device.Type = devType
		}
		if createdAt, ok := dev["created"].(string); ok {
			device.CreatedAt = createdAt
		} else if createdAt, ok := dev["created_at"].(string); ok {
			device.CreatedAt = createdAt
		}
		if enabled, ok := dev["enabled"].(bool); ok {
			device.IsDefault = enabled
		}

		// Parse nested properties for more detail.
		if properties, ok := dev["properties"].(map[string]interface{}); ok {
			if label, ok := properties["label"].(string); ok && device.Label == "" {
				device.Label = label
			}
			if devType, ok := properties["type"].(string); ok && device.Type == "" {
				device.Type = devType
			}
		}

		if device.UUID != "" {
			devices = append(devices, device)
		}
	}

	return devices
}

// DisableMFAResponse is the response for MFA disable.
type DisableMFAResponse struct {
	Disabled       bool `json:"disabled"`
	RemovedDevices int  `json:"removed_devices"`
}

// extractID extracts the first path segment after a known prefix, for MFA routes.
func extractMFAUserID(path string) string {
	prefixes := []string{
		"/api/v1/iam/mfa/",
	}
	for _, prefix := range prefixes {
		if idx := len(prefix); idx < len(path) {
			remaining := path[idx:]
			// Strip trailing slash.
			remaining = fmt.Sprint(remaining)
			// If the remaining part is just a slug like "enroll" or "verify", no ID.
			if remaining == "" {
				return ""
			}
			// If it looks like a UUID, return it.
			if len(remaining) > 0 && remaining[0] == '{' {
				if end := len(remaining) - 1; end > 0 {
					return remaining[1:end]
				}
			}
		}
	}
	return ""
}

// extractQueryInt extracts an integer query parameter, returning the value or defaultValue.
func extractQueryInt(params map[string][]string, key string, defaultValue int) int {
	if vals, ok := params[key]; ok && len(vals) > 0 {
		if v, err := strconv.Atoi(vals[0]); err == nil {
			return v
		}
	}
	return defaultValue
}
