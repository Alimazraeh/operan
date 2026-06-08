package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/events"
)

func TestABACHandler_Evaluate_PolicyEngine(t *testing.T) {
	st := NewABACStore()
	// Seed one policy per rule type, all matching document/read.
	for _, p := range []ABACPolicy{
		{ID: "p-time", Name: "time", Resource: "document", Action: "read", Rule: "time",
			Conditions: map[string]interface{}{"start_hour": float64(0), "end_hour": float64(23)}, Effect: "allow"},
		{ID: "p-ip", Name: "ip", Resource: "document", Action: "read", Rule: "ip",
			Conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8"}}, Effect: "allow"},
		{ID: "p-own", Name: "own", Resource: "document", Action: "read", Rule: "ownership",
			Conditions: map[string]interface{}{"owner_field": "actor_id"}, Effect: "allow"},
		{ID: "p-dept", Name: "dept", Resource: "document", Action: "read", Rule: "department",
			Conditions: map[string]interface{}{"allowed_departments": []interface{}{"eng"}}, Effect: "allow"},
		{ID: "p-custom", Name: "custom", Resource: "document", Action: "read", Rule: "custom",
			Conditions: map[string]interface{}{"expression": "true"}, Effect: "allow"},
		{ID: "p-unknown", Name: "unknown", Resource: "document", Action: "read", Rule: "weird",
			Conditions: map[string]interface{}{}, Effect: "allow"},
	} {
		if err := st.Create("t1", p); err != nil {
			t.Fatalf("seed policy %s: %v", p.ID, err)
		}
	}

	h := NewABACHandler(ssoAuthClient(t), events.NewPublisher(""), st)

	rr := httptest.NewRecorder()
	body := `{"actor_id":"user-1","action":"read","resource":"document","attributes":{"ip":"10.1.2.3","department":"eng","actor_id":"user-1","owner_id":"user-1"}}`
	h.Evaluate(rr, userTenantReq(http.MethodPost, "/api/v1/iam/abac/evaluate", body))
	if rr.Code != http.StatusOK {
		t.Errorf("Evaluate() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
	// Response should reference ABAC policy evaluation.
	if !strings.Contains(rr.Body.String(), "abac") && !strings.Contains(rr.Body.String(), "allowed") {
		t.Errorf("Evaluate() body unexpected: %s", rr.Body.String())
	}
}
