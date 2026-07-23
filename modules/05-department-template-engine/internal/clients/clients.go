// Package clients holds the thin HTTP clients the deploy orchestrator uses to
// provision into other modules: agent registration in Module 04 and memory
// provisioning in Module 07. Caller credentials (Authorization + X-Tenant-ID)
// are forwarded verbatim — the Module 03 pattern — so all writes happen under
// the deploying user's identity and tenant.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	maxRetries     = 2
)

// Caller carries the forwarded identity for provisioning calls.
type Caller struct {
	Authorization string // full header value, e.g. "Bearer eyJ..."
	TenantID      string
}

func doJSON(ctx context.Context, method, url string, caller Caller, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", caller.Authorization)
		req.Header.Set("X-Tenant-ID", caller.TenantID)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("%s %s: upstream %d: %s", method, url, resp.StatusCode, truncate(respBody, 200))
			continue // retry server errors
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("%s %s: upstream %d: %s", method, url, resp.StatusCode, truncate(respBody, 200))
		}
		if out != nil {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("%s %s: decode response: %w", method, url, err)
			}
		}
		return nil
	}
	return lastErr
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

// ─── Module 04: Agent Registry ───────────────────────────────────────────────

type RegistryClient struct {
	BaseURL string
}

// CreateAgentRequest mirrors Module 04's create shape (subset we use).
type CreateAgentRequest struct {
	Name            string   `json:"name"`
	Role            string   `json:"role"`
	Description     string   `json:"description,omitempty"`
	TenantID        string   `json:"tenant_id"`
	DepartmentID    *string  `json:"department_id,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
	Tools           []string `json:"tools,omitempty"`
	EscalationRules []string `json:"escalation_rules,omitempty"`
	Objectives      []struct {
		Description string  `json:"description"`
		Metric      string  `json:"metric,omitempty"`
		Weight      float64 `json:"weight,omitempty"`
	} `json:"objectives,omitempty"`
}

// CreatedAgent is the subset of Module 04's response we need.
type CreatedAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// CreateAgent registers one agent; Module 04 assigns the id.
func (c *RegistryClient) CreateAgent(ctx context.Context, caller Caller, req CreateAgentRequest) (*CreatedAgent, error) {
	var out CreatedAgent
	if err := doJSON(ctx, http.MethodPost, c.BaseURL+"/registry/agents", caller, req, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, fmt.Errorf("registry returned no agent id")
	}
	return &out, nil
}

// Ping performs a cheap authorized read so the pipeline can fail fast on
// missing admin role before any provisioning happens.
func (c *RegistryClient) Ping(ctx context.Context, caller Caller) error {
	client := &http.Client{Timeout: requestTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/registry/agents?page_size=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", caller.Authorization)
	req.Header.Set("X-Tenant-ID", caller.TenantID)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("registry pre-flight: %d", resp.StatusCode)
	}
	return nil
}

// ─── Module 07: Memory Fabric ────────────────────────────────────────────────

type MemoryClient struct {
	BaseURL string
}

// VectorItem mirrors Module 07's vector write item (subset we use).
type VectorItem struct {
	DocumentID      string                 `json:"document_id"`
	EmbeddingType   string                 `json:"embedding_type"`
	SemanticContent string                 `json:"semantic_content"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// StoreVectors ingests items into the tenant's memory fabric and returns the
// document ids provisioned.
func (c *MemoryClient) StoreVectors(ctx context.Context, caller Caller, items []VectorItem) ([]string, error) {
	payload := map[string]interface{}{
		"tenant_id": caller.TenantID,
		"items":     items,
	}
	if err := doJSON(ctx, http.MethodPost, c.BaseURL+"/vectors", caller, payload, nil); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.DocumentID)
	}
	return ids, nil
}
