package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetQueueFiltersByUserAndType(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(queueResponse{
			Items: []QueueItem{{ItemID: "appr-1", ItemType: "approval", Title: "Grant replay-test read access"}},
			Total: 1,
		})
	}))
	defer srv.Close()

	c := &SupervisionClient{BaseURL: srv.URL, Doer: NewDoer()}
	items, err := c.GetQueue(context.Background(), "dana-jwt", "smoke-tenant", "u-2")
	if err != nil {
		t.Fatalf("GetQueue: %v", err)
	}
	if gotQuery != "type=approval&user_id=u-2" {
		t.Errorf("query = %q, want type=approval&user_id=u-2", gotQuery)
	}
	if len(items) != 1 || items[0].ItemID != "appr-1" {
		t.Errorf("items = %+v", items)
	}
}

func TestApproveAttributesToTheCallerToken(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(Approval{ID: "appr-1", Status: "approved"})
	}))
	defer srv.Close()

	c := &SupervisionClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.Approve(context.Background(), "dana-jwt", "smoke-tenant", "appr-1", "looks good")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if gotPath != "/approvals/appr-1/approve" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer dana-jwt" {
		t.Errorf("Authorization = %q — approval must be attributed via the caller's own token, not a body field", gotAuth)
	}
	if got.Status != "approved" {
		t.Errorf("got.Status = %q", got.Status)
	}
}
