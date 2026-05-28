package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	TenantID    string                 `json:"tenant_id"`
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
	Auth   *authentik.Client
	Store  *ABACStore
	Pub    *events.Publisher
}

// NewABACHandler creates a new ABAC handler with a tenant-isolated store.
func NewABACHandler(auth *authentik.Client, publisher *events.Publisher, store *ABACStore) *ABACHandler {
	rbacHandler := &RBACHandler{Auth: auth}
	return &ABACHandler{
		RBACHandler: rbacHandler,
		Auth:        auth,
		Store:       store,
		Pub:         publisher,
	}
}

// ---------------------------------------------------------------------------
// Policy store (tenant-isolated in-memory)
// ---------------------------------------------------------------------------

// ABACStore provides tenant-isolated in-memory storage for ABAC policies.
type ABACStore struct {
	mu       sync.RWMutex
	policies map[string]map[string]ABACPolicy // tenantID -> policyID -> policy
}

// NewABACStore creates a new tenant-isolated ABAC store.
func NewABACStore() *ABACStore {
	return &ABACStore{
		policies: make(map[string]map[string]ABACPolicy),
	}
}

// Create adds a policy scoped to the given tenantID.
func (s *ABACStore) Create(tenantID string, policy ABACPolicy) error {
	if tenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	policy.TenantID = tenantID

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.policies[tenantID]; !ok {
		s.policies[tenantID] = make(map[string]ABACPolicy)
	}
	s.policies[tenantID][policy.ID] = policy
	return nil
}

// ListByTenant returns all policies for the given tenantID, sorted by name.
func (s *ABACStore) ListByTenant(tenantID string) []ABACPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantPolicies, ok := s.policies[tenantID]
	if !ok || len(tenantPolicies) == 0 {
		return []ABACPolicy{}
	}

	policies := make([]ABACPolicy, 0, len(tenantPolicies))
	for _, p := range tenantPolicies {
		policies = append(policies, p)
	}
	sortPoliciesByName(policies)
	return policies
}

// GetByID returns a policy by ID for the given tenantID.
func (s *ABACStore) GetByID(tenantID, policyID string) (ABACPolicy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantPolicies, ok := s.policies[tenantID]
	if !ok {
		return ABACPolicy{}, false
	}
	p, ok := tenantPolicies[policyID]
	return p, ok
}

// DeleteByID removes a policy by ID for the given tenantID.
func (s *ABACStore) DeleteByID(tenantID, policyID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantPolicies, ok := s.policies[tenantID]
	if !ok {
		return false
	}
	if _, ok := tenantPolicies[policyID]; !ok {
		return false
	}
	delete(tenantPolicies, policyID)
	if len(tenantPolicies) == 0 {
		delete(s.policies, tenantID)
	}
	return true
}

// EvaluateByResource returns all policies for the given tenantID that match
// the specified resource and action.
func (s *ABACStore) EvaluateByResource(tenantID, resource, action string) []ABACPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantPolicies, ok := s.policies[tenantID]
	if !ok {
		return nil
	}

	var matched []ABACPolicy
	for _, p := range tenantPolicies {
		if p.Resource == resource && p.Action == action {
			matched = append(matched, p)
		}
	}
	return matched
}

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

	// Check if Authentik is configured (UsersAPI not nil).
	if h.Auth == nil || h.Auth.UsersAPI == nil {
		result := ABACEvaluateResult{
			Allowed:     false,
			Reason:      "RBAC service not configured",
			RBACGranted: false,
			EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		writeJSON(w, result, http.StatusOK)
		return
	}

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
		h.Pub.PermissionRevoked(r.Context(), tenantID, req.ActorID, "user", authPermission, req.Resource, actorID, "denied — user not found in Authentik", correlationID, timestamp)

		writeJSON(w, result, http.StatusOK)
		return
	}

	hasPerm, err := h.Auth.RBACAPI.CheckPermission(ctx, authentik.CheckPermissionRequest{
		User:       req.ActorID,
		Permission: authPermission,
	})
	if err == nil && hasPerm {
		// Step 2a: RBAC granted — evaluate ABAC policies.
		abacResult := h.evaluateABAC(ctx, tenantID, &req, user)

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
			h.Pub.PermissionGranted(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, correlationID, timestamp)
		} else {
			result.Reason = "RBAC grant but ABAC policies denied: " + strings.Join(deniedPolicies, ", ")

			// Publish denial event
			h.Pub.PermissionRevoked(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, result.Reason, correlationID, timestamp)
		}

		writeJSON(w, result, http.StatusOK)
		return
	}

	// Fall back to checking user's groups and their roles' permissions.
	allowed, reason, _ := h.checkPermissionViaGroups(ctx, user, authPermission)

	if allowed {
		// RBAC granted via groups — evaluate ABAC policies.
		abacResult := h.evaluateABAC(ctx, tenantID, &req, user)

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

			h.Pub.PermissionGranted(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, correlationID, timestamp)
		} else {
			result.Reason = reason + " but ABAC policies denied: " + strings.Join(deniedPolicies, ", ")

			h.Pub.PermissionRevoked(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, result.Reason, correlationID, timestamp)
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
	h.Pub.PermissionRevoked(r.Context(), tenantID, req.ActorID, req.Resource, authPermission, req.Resource, actorID, reason, correlationID, timestamp)

	writeJSON(w, result, http.StatusOK)
}

// evaluateABAC evaluates all ABAC policies for a request, scoped to the tenant.
func (h *ABACHandler) evaluateABAC(ctx context.Context, tenantID string, req *ABACEvaluateRequest, user *authentik.User) []ABACPolicyResult {
	var results []ABACPolicyResult
	attributes := req.Attributes
	if attributes == nil {
		attributes = make(map[string]interface{})
	}

	// Add actor_id from the request into attributes for policy evaluation.
	attributes["actor_id"] = req.ActorID

	// Get tenant-scoped policies that match the resource/action.
	policies := h.Store.EvaluateByResource(tenantID, req.Resource, req.Action)

	for _, policy := range policies {
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
			result.Passed = evaluateDepartmentPolicy(ctx, attributes, policy.Conditions)
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
	tenantID := middleware.GetTenantID(r.Context())

	var req ABACPolicyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	policyID := uuid.New().String()
	policy := ABACPolicy{
		ID:          policyID,
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Resource:    req.Resource,
		Action:      req.Action,
		Rule:        req.Rule,
		Conditions:  req.Conditions,
		Effect:      req.Effect,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if err := h.Store.Create(tenantID, policy); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	writeJSON(w, policy, http.StatusCreated)
}

// ---------------------------------------------------------------------------
// GET /api/v1/iam/abac/policies — ListPolicies
// ---------------------------------------------------------------------------

// ListPolicies handles GET /api/v1/iam/abac/policies.
func (h *ABACHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	policies := h.Store.ListByTenant(tenantID)
	writeJSON(w, policies, http.StatusOK)
}

// ---------------------------------------------------------------------------
// GET /api/v1/iam/abac/policies/{policy_id} — GetPolicy
// ---------------------------------------------------------------------------

// GetPolicy handles GET /api/v1/iam/abac/policies/{policy_id}.
func (h *ABACHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	policyID := extractPolicyID(r.URL.Path)
	if policyID == "" {
		http.Error(w, `{"error":"policy_id is required"}`, http.StatusBadRequest)
		return
	}

	policy, ok := h.Store.GetByID(tenantID, policyID)
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
	tenantID := middleware.GetTenantID(r.Context())
	policyID := extractPolicyID(r.URL.Path)
	if policyID == "" {
		http.Error(w, `{"error":"policy_id is required"}`, http.StatusBadRequest)
		return
	}

	if !h.Store.DeleteByID(tenantID, policyID) {
		http.Error(w, `{"error":"policy not found"}`, http.StatusNotFound)
		return
	}

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

// evaluateIPPolicy checks IP-based conditions using CIDR matching.
//
// Conditions schema:
//
//	"allowed_cidrs":  []string{"10.0.0.0/8", "192.168.1.0/24"}  // if set, IP must match one
//	"denied_cidrs":   []string{"10.0.1.0/24"}                     // optional, evaluated first (deny wins)
//
// Attributes:
//
//	"client_ip": "10.0.2.5"  // the request client IP
func evaluateIPPolicy(attrs map[string]interface{}, conditions map[string]interface{}) bool {
	clientIPStr, _ := attrs["client_ip"].(string)
	if clientIPStr == "" {
		// No client IP provided — allow by default
		return true
	}

	clientIP := net.ParseIP(clientIPStr)
	if clientIP == nil {
		// Invalid IP — reject
		return false
	}

	// 1. Evaluate deny list first (deny always wins)
	if deniedCIDRs, ok := conditions["denied_cidrs"].([]interface{}); ok {
		for _, c := range deniedCIDRs {
			cidrStr, _ := c.(string)
			if cidrStr == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(cidrStr)
			if err != nil {
				continue // skip malformed CIDRs
			}
			if ipNet.Contains(clientIP) {
				return false // IP is denied
			}
		}
	}

	// 2. Evaluate allow list
	if allowedCIDRs, ok := conditions["allowed_cidrs"].([]interface{}); ok {
		for _, c := range allowedCIDRs {
			cidrStr, _ := c.(string)
			if cidrStr == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(cidrStr)
			if err != nil {
				continue // skip malformed CIDRs
			}
			if ipNet.Contains(clientIP) {
				return true // IP is allowed
			}
		}
		// Allowed list present but IP not in it — deny
		return false
	}

	// 3. No allow list — deny by default (only deny list was present)
	return false
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
func evaluateDepartmentPolicy(ctx context.Context, attrs map[string]interface{}, conditions map[string]interface{}) bool {
	dept, _ := attrs["resource_department"].(string)
	actorDept, _ := attrs["actor_department"].(string)
	if dept == "" || actorDept == "" {
		return true
	}

	// Admin roles bypass department constraints
	if admins, ok := conditions["admin_roles"].([]interface{}); ok {
		actorRoles := middleware.GetJWTToken(ctx)
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
	path = strings.TrimPrefix(path, "/api/v1/iam/abac/policies")
	path = strings.TrimSuffix(path, "/")
	if path == "" || path == "/api/v1/iam/abac/policies" {
		return ""
	}
	return path
}

// sortPoliciesByName sorts policies by name for deterministic output.
func sortPoliciesByName(policies []ABACPolicy) {
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})
}
