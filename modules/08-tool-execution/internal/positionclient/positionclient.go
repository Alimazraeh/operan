// Package positionclient asks Module 05 what autonomy tier a seat actually
// holds.
//
// The funnel's authority stage exists to answer one question: is the seat
// this actor claims to sit in actually cleared for this verb? Before this
// client existed, the funnel took the caller's word for its own tier — any
// authenticated request could simply assert "coordinate" and clear any
// authority check. This client closes that gap by resolving the seat's real
// tier from Module 05's org chart at invoke time, live, every time: nothing
// is cached across invocations and nothing is trusted from the request body.
//
// Fail-closed, same trade as policyclient: a department engine that is down,
// slow, or speaking a shape we do not recognise must resolve to "no
// authority", not to the caller's claim and not to a default tier. An actor
// whose real tier cannot be established is exactly as unauthorised as one
// who holds no tier at all — a stalled run beats a forged one.
package positionclient

import (
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

// New returns a client for M05 at baseURL. Empty baseURL means "no
// department engine configured", which Resolve reports as unresolved with
// that exact reason.
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 5 * time.Second}}
}

// Resolution is the answer to "what autonomy tier does this seat actually
// hold." Tier is empty whenever the seat could not be positively resolved —
// unreachable engine, unknown department or position, or a position that
// carries no tier — and an empty Tier ranks below every real tier via
// store.AutonomyRank, so an unresolved seat is refused write verbs exactly
// like a seat that openly holds no authority. Reason is always populated so
// the funnel can record why, whether resolution succeeded or not.
type Resolution struct {
	Tier   string `json:"tier,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// orgChartResponse mirrors the subset of M05's
// GET /departments/{id}/org-chart response this client needs.
type orgChartResponse struct {
	Positions []struct {
		ID           string `json:"id"`
		AutonomyTier string `json:"autonomy_tier"`
	} `json:"positions"`
}

// Resolve looks up positionID's autonomy tier within departmentID's org
// chart. The caller's authorization is forwarded, so M05 sees the same
// identity the invocation ran under, matching the pattern policyclient uses
// for the same reason. Note M05's routes are bare paths — no /api/v1 prefix.
func (c *Client) Resolve(ctx context.Context, authorization, tenantID, departmentID, positionID string) Resolution {
	if c.baseURL == "" {
		return Resolution{Reason: "no department engine configured"}
	}
	if positionID == "" {
		return Resolution{Reason: "request names no position — the acting seat is unidentified"}
	}
	if departmentID == "" {
		return Resolution{Reason: "request names no department — the acting seat cannot be located"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/departments/"+departmentID+"/org-chart", nil)
	if err != nil {
		return Resolution{Reason: "position request not buildable"}
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Resolution{Reason: "department engine unreachable: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Resolution{Reason: fmt.Sprintf("department engine answered %d for department %q", resp.StatusCode, departmentID)}
	}
	var body orgChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Resolution{Reason: "department engine answered in an unrecognised shape"}
	}

	for _, p := range body.Positions {
		if p.ID != positionID {
			continue
		}
		if p.AutonomyTier == "" {
			return Resolution{Reason: fmt.Sprintf("position %q carries no autonomy tier in the org chart", positionID)}
		}
		return Resolution{Tier: p.AutonomyTier, Reason: "resolved from department org chart"}
	}
	return Resolution{Reason: fmt.Sprintf("position %q not found in department %q org chart", positionID, departmentID)}
}
