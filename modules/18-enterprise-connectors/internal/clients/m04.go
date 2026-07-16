package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/operan/enterprise-connectors/internal/connectors"
)

// M04Client is the HTTP client for Module 04 (Agent Registry) tool registration.
type M04Client struct {
	baseURL string
	client  *http.Client
}

// NewM04Client creates a new M04 client.
func NewM04Client(baseURL string) *M04Client {
	return &M04Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterTools registers connector tools with M04.
func (c *M04Client) RegisterTools(ctx context.Context, tenantID string, tools []connectors.ToolDefinition) error {
	// Build the registration payload
	var toolPayload []map[string]interface{}
	for _, t := range tools {
		toolPayload = append(toolPayload, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
			"returns":     t.Returns,
		})
	}

	payload := map[string]interface{}{
		"tools": toolPayload,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}

	url := c.baseURL + "/v1/agents/tools/batch"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("M04 unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("M04 registration failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// CheckHealth checks if M04 is available.
func (c *M04Client) CheckHealth(ctx context.Context) error {
	url := c.baseURL + "/health"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("M04 health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("M04 health check returned HTTP %d", resp.StatusCode)
	}
	return nil
}