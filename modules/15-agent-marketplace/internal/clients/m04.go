package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// M04Client is an HTTP client for the Agent Registry (M04).
type M04Client struct {
	baseURL    string
	httpClient *http.Client
}

// M04Agent represents an agent in the M04 Agent Registry.
type M04Agent struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	Tools        []string `json:"tools"`
	TenantID     string   `json:"tenant_id"`
}

// M04Response is a generic wrapper for M04 API responses.
type M04Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewM04Client creates a new M04 HTTP client.
func NewM04Client(baseURL string) *M04Client {
	return &M04Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterAgent registers a new agent in M04.
func (c *M04Client) RegisterAgent(ctx context.Context, tenantID, bearerToken string, agent M04Agent) (*M04Agent, error) {
	body, err := json.Marshal(agent)
	if err != nil {
		return nil, fmt.Errorf("marshal agent: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/agents", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("M04 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("M04 returned status %d", resp.StatusCode)
	}

	var result M04Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode M04 response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("M04 error: %s", result.Error)
	}

	agentData, ok := result.Data.(map[string]interface{})
	if !ok {
		return &M04Agent{
			Name:         agent.Name,
			Role:         agent.Role,
			Capabilities: agent.Capabilities,
			Tools:        agent.Tools,
			TenantID:     tenantID,
		}, nil
	}

	out := &M04Agent{
		Name:         agent.Name,
		Role:         agent.Role,
		Capabilities: agent.Capabilities,
		Tools:        agent.Tools,
		TenantID:     tenantID,
	}
	if id, ok := agentData["id"].(string); ok {
		out.ID = id
	}
	return out, nil
}

// HealthCheck checks if M04 is reachable.
func (c *M04Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}