package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// InvocationsClient talks to Module 08 (tool-execution), read-only. Base
// path is bare (internal/handlers/router.go registers "/invocations"
// directly, mounted at root in main.go:131). Export uses this to capture
// the governed capability invocations behind a historical request, so the
// fixture documents that the demo once produced real execution, not just a
// request record. Restore never calls this module — invoking a capability
// is something only a live workflow run does, not something a seed script
// replays directly.
type InvocationsClient struct {
	BaseURL string // e.g. http://tool-execution.operan.svc.cluster.local:8008
	Doer    *Doer
}

// Invocation mirrors store.Invocation (store/invocation.go:53-72) — only
// the fields export copies into fixture.HistoricalInvocation. Input/Output
// are deliberately not modeled here: HistoricalInvocation never carries raw
// payloads (see its doc comment).
type Invocation struct {
	CapabilityID   string `json:"capability_id"`
	ProviderKind   string `json:"provider_kind"`
	Status         string `json:"status"`
	Simulated      bool   `json:"simulated"`
	PolicyDecision string `json:"policy_decision"`
}

type invocationListResponse struct {
	Invocations []Invocation `json:"invocations"`
	Total       int          `json:"total"`
}

// ListInvocationsForRequest calls GET /invocations?request_id=&limit=.
func (c *InvocationsClient) ListInvocationsForRequest(ctx context.Context, token, tenantID, requestID string, limit int) ([]Invocation, error) {
	reqURL := fmt.Sprintf("%s/invocations?request_id=%s&limit=%d", c.BaseURL, url.QueryEscape(requestID), limit)
	var out invocationListResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, reqURL, token, tenantID, nil, &out)
	if err != nil {
		return nil, err
	}
	return out.Invocations, nil
}
