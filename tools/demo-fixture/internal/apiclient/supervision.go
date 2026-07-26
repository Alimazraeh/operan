package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// SupervisionClient talks to Module 09 (human-supervision). Base path is
// bare (internal/handlers/router.go registers "/approvals", "/queue"
// directly, mounted at root in main.go:88) — not /approvals/queue, not
// /api/v1. Every route requires Authorization: Bearer plus X-Tenant-ID.
type SupervisionClient struct {
	BaseURL string // e.g. http://human-supervision.operan.svc.cluster.local:8009
	Doer    *Doer
}

// QueueItem mirrors store.QueueItem as populated for an approval entry
// (handlers/queue.go:26-38). Module 09 has no endpoint that lists Approval
// records directly with their request_id (see ApprovalCorrelation doc
// comment below) — Title is the only field available over HTTP that can be
// matched back to the request that raised the gate.
type QueueItem struct {
	ItemID    string `json:"item_id"`
	ItemType  string `json:"item_type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type queueResponse struct {
	Items []QueueItem `json:"items"`
	Total int         `json:"total"`
}

// GetQueue calls GET /queue?user_id=&type=approval.
func (c *SupervisionClient) GetQueue(ctx context.Context, token, tenantID, userID string) ([]QueueItem, error) {
	u := fmt.Sprintf("%s/queue?type=approval&user_id=%s", c.BaseURL, url.QueryEscape(userID))
	var out queueResponse
	_, err := c.Doer.Call(ctx, http.MethodGet, u, token, tenantID, nil, &out)
	if err != nil {
		return nil, err
	}
	return out.Items, nil
}

// ApproveRequest mirrors approveRequest (handlers/approvals.go:148-158).
// ApproverID is accepted by the server but ignored — the actor is always
// taken from the caller's JWT (actorFromToken) — so approving as a
// particular fixture user requires logging in as that user first (see
// IAMClient.Login) and passing their token here, not just naming them.
type ApproveRequest struct {
	Comment string `json:"comment,omitempty"`
}

// Approval mirrors store.Approval (store/models.go:116-138) — only the
// fields this tool reads.
type Approval struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
}

// Approve calls POST /approvals/{id}/approve. The token passed here must
// belong to the seat holder expected to approve — the server attributes
// the decision to whoever the token identifies, never to a body field.
func (c *SupervisionClient) Approve(ctx context.Context, approverToken, tenantID, approvalID, comment string) (*Approval, error) {
	var out Approval
	req := ApproveRequest{Comment: comment}
	_, err := c.Doer.Call(ctx, http.MethodPost, c.BaseURL+"/approvals/"+approvalID+"/approve", approverToken, tenantID, req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
