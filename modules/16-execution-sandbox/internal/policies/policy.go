package policies

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PolicyCheckRequest is sent to M10 to check if a tool execution is allowed.
type PolicyCheckRequest struct {
	TenantID string `json:"tenant_id"`
	AgentID  string `json:"agent_id"`
	ToolName string `json:"tool_name"`
	Action   string `json:"action"`
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
		TenantID: tenantID,
		AgentID:  agentID,
		ToolName: toolName,
		Action:   "execute",
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal policy request: %w", err)
	}

	url := c.baseURL + "/v1/policies/check"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create policy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("decode policy response: %w", err)
	}

	return &result, nil
}