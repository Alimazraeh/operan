package policies

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PolicyCheckRequest is sent to M10's evaluation endpoint. The shape is
// M10's contract (internal/handler/evaluate.go): resource + action_type are
// required; tenancy travels in the header, not the body.
type PolicyCheckRequest struct {
	AgentID    string `json:"agent_id,omitempty"`
	Resource   string `json:"resource"`
	ActionType string `json:"action_type"`
}

// PolicyCheckResult is the response from M10 policy engine.
type PolicyCheckResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// PolicyClient talks to M10's policy engine.
type PolicyClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPolicyClient creates a new PolicyClient.
func NewPolicyClient(baseURL string) *PolicyClient {
	return &PolicyClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CanExecute checks M10 policy to see if a tool execution is allowed.
func (c *PolicyClient) CanExecute(ctx context.Context, tenantID, agentID, toolName string) (*PolicyCheckResult, error) {
	reqBody := PolicyCheckRequest{
		AgentID:    agentID,
		Resource:   "tool:" + toolName,
		ActionType: "execute",
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal policy request: %w", err)
	}

	url := c.baseURL + "/policies/evaluate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create policy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network unreachable — return denied with nil error (fail-safe)
		return &PolicyCheckResult{Allowed: false, Reason: "policy engine unreachable"}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
		// M10 unavailable — fail open is not safe; default to denied
		return &PolicyCheckResult{Allowed: false, Reason: "policy engine unavailable"}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return &PolicyCheckResult{Allowed: false, Reason: fmt.Sprintf("unexpected status: %d", resp.StatusCode)}, nil
	}

	var result PolicyCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Deny on an unreadable answer. Returning (nil, err) here used to let
		// the caller's nil-check skip the deny branch entirely — allow-on-
		// malformed-response, the exact opposite of the intent.
		return &PolicyCheckResult{Allowed: false, Reason: "policy response unreadable"},
			fmt.Errorf("decode policy response: %w", err)
	}

	return &result, nil
}
