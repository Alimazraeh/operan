package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// M04Client is an HTTP client for the M04 Agent Registry.
type M04Client struct {
	BaseURL string
	Client  *http.Client
}

// AgentInfo represents agent metadata from M04.
type AgentInfo struct {
	AgentID      string `json:"agent_id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	IsActive     bool   `json:"is_active"`
	DepartmentID string `json:"department_id"`
}

// NewM04Client creates a new M04 client.
func NewM04Client(baseURL string) *M04Client {
	return &M04Client{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ValidateAgent checks if an agent exists and is active in the registry.
func (c *M04Client) ValidateAgent(ctx context.Context, agentID, tenantID string) (*AgentInfo, error) {
	url := fmt.Sprintf("%s/agents/%s?tenant_id=%s", c.BaseURL, agentID, tenantID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("M04 unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("M04 service unavailable (503)")
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("M04 internal error (500)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("M04 returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Agent *AgentInfo `json:"agent"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Agent == nil {
		return nil, fmt.Errorf("empty agent response")
	}

	return result.Agent, nil
}