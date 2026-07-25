// Package policyclient asks Module 10 whether an invocation may proceed.
//
// This is the call that finally puts the policy engine in the run path — M10
// shipped as an evaluation API that nothing consulted. Every answer other
// than an explicit allow is a deny: a policy engine that is down, slow, or
// speaking a shape we do not recognise must stop actions, not wave them
// through. That is the deliberate trade — fail closed was decided in the
// plan, and a stalled run beats an ungoverned side effect.
package policyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client for M10 at baseURL. Empty baseURL means "no policy
// engine configured", which Check reports as a deny with that exact reason.
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 5 * time.Second}}
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type evalRequest struct {
	AgentID    string `json:"agent_id,omitempty"`
	Resource   string `json:"resource"`
	ActionType string `json:"action_type"`
}

// Check evaluates "may this actor perform this capability" against M10.
// Resource is "capability:<id>"; the action type is the capability's side
// effect, so a tenant can write one policy covering every destructive verb.
// The caller's authorization is forwarded, so M10's own audit sees the same
// identity the invocation ran under — the policy engine must never be asked
// anonymously about a named actor's action.
func (c *Client) Check(ctx context.Context, authorization, tenantID, actorID, capabilityID, sideEffect string) Decision {
	if c.baseURL == "" {
		return Decision{Allowed: false, Reason: "no policy engine configured"}
	}
	body, err := json.Marshal(evalRequest{
		AgentID: actorID, Resource: "capability:" + capabilityID, ActionType: sideEffect,
	})
	if err != nil {
		return Decision{Allowed: false, Reason: "policy request not encodable"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/policies/evaluate", bytes.NewReader(body))
	if err != nil {
		return Decision{Allowed: false, Reason: "policy request not buildable"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Decision{Allowed: false, Reason: "policy engine unreachable: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Decision{Allowed: false, Reason: fmt.Sprintf("policy engine answered %d", resp.StatusCode)}
	}
	var d Decision
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return Decision{Allowed: false, Reason: "policy engine answered in an unrecognised shape"}
	}
	if d.Allowed && d.Reason == "" {
		d.Reason = "allowed by policy"
	}
	return d
}
