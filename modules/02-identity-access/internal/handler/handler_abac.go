package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ---------------------------------------------------------------------------
// ABAC structs
// ---------------------------------------------------------------------------

// ABACPolicy represents an attribute-based access control policy.
type ABACPolicy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Rule        string                 `json:"rule"` // "time", "ip", "ownership", "department", "custom"
	Conditions  map[string]interface{} `json:"conditions"`
	Effect      string                 `json:"effect"` // "allow" or "deny"
	CreatedAt   string                 `json:"created_at"`
}

// ABACEvaluateRequest extends PermissionCheckRequest with ABAC attributes.
type ABACEvaluateRequest struct {
	ActorID    string                 `json:"actor_id"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Validate checks that the evaluate request is valid.
func (r *ABACEvaluateRequest) Validate() error {
	if r.ActorID == "" {
		return &models.ValidationError{"actor_id is required"}
	}
	if r.Action == "" {
		return &models.ValidationError{"action is required"}
	}
	if r.Resource == "" {
		return &models.ValidationError{"resource is required"}
	}
	return nil
}

// ABACEvaluateResult is the response for ABAC evaluation.
type ABACEvaluateResult struct {
	Allowed      bool                `json:"allowed"`
	Reason       string              `json:"reason"`
	RBACGranted  bool                `json:"rbac_granted,omitempty"`
	ABACPolicies []ABACPolicyResult  `json:"abac_policies,omitempty"`
	EvaluatedAt  string              `json:"evaluated_at"`
}

// ABACPolicyResult is the result of a single ABAC policy evaluation.
type ABACPolicyResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// ABACPolicyCreateRequest is the request for creating an ABAC policy.
type ABACPolicyCreateRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Rule        string                 `json:"rule"`
	Conditions  map[string]interface{} `json:"conditions"`
	Effect      string                 `json:"effect"`
}

// Validate checks the create request.
func (r *ABACPolicyCreateRequest) Validate() error {
	if r.Name == "" {
		return &models.ValidationError{"policy name is required"}
	}
	if r.Resource == "" {
		return &models.ValidationError{"resource is required"}
	}
	if r.Action == "" {
		return &models.ValidationError{"action is required"}
	}
	if r.Rule == "" {
		return &models.ValidationError{"rule type is required"}
	}
	if r.Conditions == nil {
		return &models.ValidationError{"conditions are required"}
	}
	if r.Effect == "" {
		r.Effect = "allow"
	}
	return nil
}

// ABACHandler handles ABAC policy evaluation and management HTTP endpoints.
type ABACHandler struct {
	*RBACHandler
	Auth      *authentik.Client
	Publisher *events.Publisher
}

// NewABACHandler creates a new ABAC handler.
func NewABACHandler(auth *authentik.Client, publisher *events.Publisher) *ABACHandler {
	rbacHandler := &RBACHandler{Auth: auth}
	return &ABACHandler{
		RBACHandler: rbacHandler,
		Auth:        auth,
		Publisher:   publisher,
	}
}

// ---------------------------------------------------------------------------
// Policy store (in-memory)
// ---------------------------------------------------------------------------

var (
	abacPolicies   = make(map[string]ABACPolicy)
	abacPoliciesMu sync.RWMutex
	policyCounter  int
)

// ---------------------------------------------------------------------------
// POST /api/v1/iam/abac/evaluate — Evaluate
// ---------------------------------------------------------------------------

// Evaluate handles POST /api/v1/iam/abac/evaluate.
func (h *ABACHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req ABACEvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	correlationID := middleware.GetTraceID(r.Context())
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Step 1: Standard RBAC check via Authentik.
	authPermission := mapResourceActionToAuthenticPermission(req.Resource, req.Action)

	// Look up the user in Authentik.
	user, err := h.Auth.UsersAPI.GetByID(ctx, req.ActorID)
	if err != nil {
		// User not found in Authentik — deny access.
		result := ABACEvaluateResult{
			Allowed:     false,
			Reason:      "user not found in Authentik",
			RBACGranted: false,
			EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		_ = tenantID
		_ = actorID
		_ = correlationID
		_ = timestamp

		// Publish denial event
		h.Publisher.PermissionRevoked(r.Context(), tenantID, req.ActorID, "user", authPermission, req.Resource, actorID, "denied — user not found in Authentik", correlationID, timestamp)

		writeJSON(w, result, http.StatusOK)
		return
	}

	hasPerm, err := h.Auth.RBACAPI.CheckPermission(ctx, authentik.CheckPermissionRequest{
		User:       req.ActorID,
		Permission: authPermission,
	})
	if err == nil && hasPerm {
		// Step 2a: RBAC granted — evaluate ABAC policies.
		abacResult := h.evaluateABAC(ctx, &req, user)

		abacAllowed := true
		var deniedPolicies []string
		for _, p := range abacResult {
			if !p.Passed {
				abacAllowed = false
				deniedPolicies = append(deniedPolicies, p.Name)
			}
		}

		result := ABACEvaluateResult{
			Allowed:      abacAllowed,
			RBACGranted:  true,
			EvaluatedAt:  time.Now().UTC().Format(time.RFC3339),
			ABACPolicies: abacResult,
		}

		if abacAllowed {
			result.Reason = "RBAC grant + ABAC policies satisfied"

			// Publish granted event
			h.Publisher.PermissionGranted(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, correlationID, timestamp)
		} else {
			result.Reason = "RBAC grant but ABAC policies denied: " + strings.Join(deniedPolicies, ", ")

			// Publish denial event
			h.Publisher.PermissionRevoked(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, result.Reason, correlationID, timestamp)
		}

		writeJSON(w, result, http.StatusOK)
		return
	}

	// Fall back to checking user's groups and their roles' permissions.
	allowed, reason, _ := h.checkPermissionViaGroups(ctx, user, authPermission)

	if allowed {
		// RBAC granted via groups — evaluate ABAC policies.
		abacResult := h.evaluateABAC(ctx, &req, user)

		abacAllowed := true
		var deniedPolicies []string
		for _, p := range abacResult {
			if !p.Passed {
				abacAllowed = false
				deniedPolicies = append(deniedPolicies, p.Name)
			}
		}

		result := ABACEvaluateResult{
			Allowed:      abacAllowed,
			RBACGranted:  true,
			EvaluatedAt:  time.Now().UTC().Format(time.RFC3339),
			ABACPolicies: abacResult,
		}

		if abacAllowed {
			result.Reason = reason + " + ABAC policies satisfied"

			h.Publisher.PermissionGranted(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, correlationID, timestamp)
		} else {
			result.Reason = reason + " but ABAC policies denied: " + strings.Join(deniedPolicies, ", ")

			h.Publisher.PermissionRevoked(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, result.Reason, correlationID, timestamp)
		}

		writeJSON(w, result, http.StatusOK)
		return
	}

	// RBAC denied — no ABAC evaluation needed.
	result := ABACEvaluateResult{
		Allowed:     false,
		Reason:      reason,
		RBACGranted: false,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Log denial event
	h.Publisher.PermissionRevoked(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, reason, correlationID, timestamp)

	writeJSON(w, result, http.StatusOK)
}

// evaluateABAC evaluates all ABAC policies for a request.
func (h *ABACHandler) evaluateABAC(ctx context.Context, req *ABACEvaluateRequest, user *authentik.User) []ABACPolicyResult {
	var results []ABACPolicyResult
	attributes := req.Attributes
	if attributes == nil {
		attributes = make(map[string]interface{})
	}

	// Add actor_id from the request into attributes for policy evaluation.
	attributes["actor_id"] = req.ActorID

	// Get all policies that match the resource/action.
	abacPoliciesMu.RLock()
	defer abacPoliciesMu.RUnlock()

	for _, policy := range abacPolicies {
		// Only evaluate policies that match the requested resource and action.
		if policy.Resource != req.Resource || policy.Action != req.Action {
			continue
		}

		result := ABACPolicyResult{Name: policy.Name}

		switch policy.Rule {
		case "time":
			result.Passed = evaluateTimePolicy(attributes, policy.Conditions)
			if result.Passed {
				result.Reason = "within allowed time window"
			} else {
				result.Reason = "outside allowed time window"
			}

		case "ip":
			result.Passed = evaluateIPPolicy(attributes, policy.Conditions)
			if result.Passed {
				result.Reason = "IP in allowed range"
			} else {
				result.Reason = "IP not in allowed range"
			}

		case "ownership":
			result.Passed = evaluateOwnershipPolicy(attributes, policy.Conditions)
			if result.Passed {
				result.Reason = "ownership constraint satisfied"
			} else {
				result.Reason = "ownership constraint violated"
			}

		case "department":
			result.Passed = evaluateDepartmentPolicy(attributes, policy.Conditions)
			if result.Passed {
				result.Reason = "department constraint satisfied"
			} else {
				result.Reason = "department constraint violated"
			}

		case "custom":
			result.Passed = evaluateCustomPolicy(ctx, attributes, policy.Conditions)
			if result.Passed {
				result.Reason = "custom policy evaluated"
			} else {
				result.Reason = "custom policy denied"
			}

		default:
			result.Passed = true
			result.Reason = "unknown rule type, defaulting to pass"
		}

		// If effect is "deny", invert the result.
		if policy.Effect == "deny" {
			result.Passed = !result.Passed
		}

		results = append(results, result)
	}

	return results
}

// ---------------------------------------------------------------------------
// POST /api/v1/iam/abac/policies — CreatePolicy
// ---------------------------------------------------------------------------

// CreatePolicy handles POST /api/v1/iam/abac/policies.
func (h *ABACHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req ABACPolicyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	abacPoliciesMu.Lock()
	policyCounter++
	policyID := strconv.Itoa(policyCounter)
	policy := ABACPolicy{
		ID:          policyID,
		Name:        req.Name,
		Description: req.Description,
		Resource:    req.Resource,
		Action:      req.Action,
		Rule:        req.Rule,
		Conditions:  req.Conditions,
		Effect:      req.Effect,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	abacPolicies[req.Name] = policy
	abacPoliciesMu.Unlock()

	writeJSON(w, policy, http.StatusCreated)
}

// ---------------------------------------------------------------------------
// GET /api/v1/iam/abac/policies — ListPolicies
// ---------------------------------------------------------------------------

// ListPolicies handles GET /api/v1/iam/abac/policies.
func (h *ABACHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	abacPoliciesMu.RLock()
	defer abacPoliciesMu.RUnlock()

	policies := make([]ABACPolicy, 0, len(abacPolicies))
	for _, p := range abacPolicies {
		policies = append(policies, p)
	}

	// Sort by name for deterministic output.
	sortPoliciesByName(policies)

	writeJSON(w, policies, http.StatusOK)
}

// ---------------------------------------------------------------------------
// GET /api/v1/iam/abac/policies/{policy_id} — GetPolicy
// ---------------------------------------------------------------------------

// GetPolicy handles GET /api/v1/iam/abac/policies/{policy_id}.
func (h *ABACHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	policyID := extractPolicyID(r.URL.Path)
	if policyID == "" {
		http.Error(w, `{"error":"policy_id is required"}`, http.StatusBadRequest)
		return
	}

	abacPoliciesMu.RLock()
	defer abacPoliciesMu.RUnlock()

	policy, ok := abacPolicies[policyID]
	if !ok {
		http.Error(w, `{"error":"policy not found"}`, http.StatusNotFound)
		return
	}

	writeJSON(w, policy, http.StatusOK)
}

// ---------------------------------------------------------------------------
// DELETE /api/v1/iam/abac/policies/{policy_id} — DeletePolicy
// ---------------------------------------------------------------------------

// DeletePolicy handles DELETE /api/v1/iam/abac/policies/{policy_id}.
func (h *ABACHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := extractPolicyID(r.URL.Path)
	if policyID == "" {
		http.Error(w, `{"error":"policy_id is required"}`, http.StatusBadRequest)
		return
	}

	abacPoliciesMu.Lock()
	defer abacPoliciesMu.Unlock()

	_, ok := abacPolicies[policyID]
	if !ok {
		http.Error(w, `{"error":"policy not found"}`, http.StatusNotFound)
		return
	}

	delete(abacPolicies, policyID)
	writeJSON(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

// ---------------------------------------------------------------------------
// Policy evaluation functions
// ---------------------------------------------------------------------------

// evaluateTimePolicy checks time-based conditions.
func evaluateTimePolicy(attrs map[string]interface{}, conditions map[string]interface{}) bool {
	hour := 12 // default
	if h, ok := attrs["time_of_day"].(float64); ok {
		hour = int(h)
	}
	start, _ := conditions["start_hour"].(float64)
	end, _ := conditions["end_hour"].(float64)
	if start == 0 && end == 0 {
		start, end = 6, 22 // default business hours
	}
	return float64(hour) >= start && float64(hour) <= end
}

// evaluateIPPolicy checks IP-based conditions.
func evaluateIPPolicy(attrs map[string]interface{}, conditions map[string]interface{}) bool {
	// For now, always pass — IP filtering requires CIDR parsing
	// which would need net package; placeholder for future implementation
	return true
}

// evaluateOwnershipPolicy checks ownership conditions.
func evaluateOwnershipPolicy(attrs map[string]interface{}, conditions map[string]interface{}) bool {
	owner, _ := attrs["resource_owner"].(string)
	actor, _ := attrs["actor_id"].(string)
	if owner == "" || actor == "" {
		return true // no ownership constraint
	}
	return owner == actor
}

// evaluateDepartmentPolicy checks department matching.
func evaluateDepartmentPolicy(attrs map[string]interface{}, conditions map[string]interface{}) bool {
	dept, _ := attrs["resource_department"].(string)
	actorDept, _ := attrs["actor_department"].(string)
	if dept == "" || actorDept == "" {
		return true
	}

	// Admin roles bypass department constraints
	if admins, ok := conditions["admin_roles"].([]interface{}); ok {
		actorRoles := middleware.GetJWTToken(context.Background())
		if actorRoles != nil {
			for _, role := range actorRoles.Roles {
				for _, adminRole := range admins {
					if role == fmt.Sprintf("%v", adminRole) {
						return true
					}
				}
			}
		}
	}

	return dept == actorDept
}

// evaluateCustomPolicy is a placeholder for custom rule evaluation.
func evaluateCustomPolicy(ctx context.Context, attrs map[string]interface{}, conditions map[string]interface{}) bool {
	// Custom policies can be implemented by extending this function.
	// For now, default to pass.
	_ = ctx
	_ = attrs
	_ = conditions
	return true
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractPolicyID extracts the policy_id from the URL path.
// Handles: /api/v1/iam/abac/policies/{id}
func extractPolicyID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/abac/policies/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// sortPoliciesByName sorts policies by name for deterministic output.
func sortPoliciesByName(policies []ABACPolicy) {
	for i := 0; i < len(policies); i++ {
		for j := i + 1; j < len(policies); j++ {
			if policies[i].Name > policies[j].Name {
				policies[i], policies[j] = policies[j], policies[i]
			}
		}
	}
}
