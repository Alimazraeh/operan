package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// M03Client is an HTTP client for the Orchestration (M03).
type M03Client struct {
	baseURL    string
	httpClient *http.Client
}

// M03Workflow represents a workflow in the M03 Orchestration engine.
type M03Workflow struct {
	Name       string                 `json:"name"`
	TenantID   string                 `json:"tenant_id"`
	Steps      []map[string]interface{} `json:"steps"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// M03WorkflowResponse is a wrapper for M03 workflow API responses.
type M03WorkflowResponse struct {
	Success bool               `json:"success"`
	Data    *M03WorkflowResult `json:"data,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// M03WorkflowResult contains the result of workflow creation.
type M03WorkflowResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewM03Client creates a new M03 HTTP client.
func NewM03Client(baseURL string) *M03Client {
	return &M03Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateWorkflow creates a workflow in M03.
func (c *M03Client) CreateWorkflow(ctx context.Context, tenantID, bearerToken string, wf M03Workflow) (*M03WorkflowResult, error) {
	body, err := json.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/workflows", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("M03 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("M03 returned status %d", resp.StatusCode)
	}

	var result M03WorkflowResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode M03 response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("M03 error: %s", result.Error)
	}

	if result.Data == nil {
		return &M03WorkflowResult{Name: wf.Name}, nil
	}

	return result.Data, nil
}

// HealthCheck checks if M03 is reachable.
func (c *M03Client) HealthCheck(ctx context.Context) error {
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