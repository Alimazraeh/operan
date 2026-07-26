package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListInvocationsForRequestFiltersByRequestID(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(invocationListResponse{
			Invocations: []Invocation{{CapabilityID: "identity.access.grant", Status: "completed", Simulated: true}},
			Total:       1,
		})
	}))
	defer srv.Close()

	c := &InvocationsClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.ListInvocationsForRequest(context.Background(), "tok", "smoke-tenant", "req-1", 50)
	if err != nil {
		t.Fatalf("ListInvocationsForRequest: %v", err)
	}
	if gotQuery != "request_id=req-1&limit=50" {
		t.Errorf("query = %q, want request_id=req-1&limit=50", gotQuery)
	}
	if len(got) != 1 || got[0].CapabilityID != "identity.access.grant" {
		t.Errorf("got = %+v", got)
	}
}
