// Package capability calls Module 08's governed invoke funnel — the door
// through which an SOP step actually performs a business verb.
package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client invokes capabilities on Module 08.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client, or nil when no base URL is configured — a nil client
// keeps action nodes on the recorded pass-through, which is the honest state
// for a deployment without the capability service.
func New(baseURL string) *Client {
	if baseURL == "" {
		return nil
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

// Actor mirrors M08's actor: who performs the verb, under which seat's
// authority.
type Actor struct {
	Type         string `json:"type"`
	ID           string `json:"id,omitempty"`
	PositionID   string `json:"position_id,omitempty"`
	AutonomyTier string `json:"autonomy_tier,omitempty"`
}

// Correlation ties the invocation back to the run that caused it.
type Correlation struct {
	RequestID    string `json:"request_id,omitempty"`
	WorkflowID   string `json:"workflow_id,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	DepartmentID string `json:"department_id,omitempty"`
}

// InvokeRequest is M08's funnel request.
type InvokeRequest struct {
	CapabilityID string                 `json:"capability_id"`
	Input        map[string]interface{} `json:"input"`
	Actor        Actor                  `json:"actor"`
	Correlation  Correlation            `json:"correlation"`
}

// Invocation is the recorded outcome — completed or refused, both are
// first-class answers, not transport errors.
type Invocation struct {
	ID           string                 `json:"id"`
	CapabilityID string                 `json:"capability_id"`
	SideEffect   string                 `json:"side_effect"`
	ProviderKind string                 `json:"provider_kind"`
	Output       map[string]interface{} `json:"output"`
	ExternalRef  *struct {
		System string `json:"system"`
		Kind   string `json:"kind"`
		ID     string `json:"id"`
		URL    string `json:"url"`
	} `json:"external_ref"`
	Simulated      bool   `json:"simulated"`
	PolicyDecision string `json:"policy_decision"`
	Status         string `json:"status"`
	Error          string `json:"error"`
}

// Invoke performs one capability through the funnel, with the caller's
// authorization forwarded so M08 sees the same identity the run carries.
func (c *Client) Invoke(ctx context.Context, authorization, tenantID string, req InvokeRequest) (*Invocation, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode invoke request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/invoke", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build invoke request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	if authorization != "" {
		httpReq.Header.Set("Authorization", authorization)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("capability service unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("capability service answered %d: %s", resp.StatusCode, bound(buf.String(), 200))
	}
	var inv Invocation
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return nil, fmt.Errorf("decode invocation: %w", err)
	}
	return &inv, nil
}

func bound(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
