package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/operan/policy-governance/internal/engine"
	"github.com/operan/policy-governance/internal/middleware"
	"github.com/operan/policy-governance/internal/store"
)

// SetupRouter creates and configures the HTTP router.
func SetupRouter(
	policyStore *store.PolicyStore,
	groupStore *store.GroupStore,
	auditStore *store.AuditStore,
	evaluateEngine *engine.Engine,
	eventPub engine.EventPublisher,
	jwtSecret, jwtIssuer string,
) http.Handler {
	// Handlers
	policyHandler := NewPolicyHandler(policyStore)
	groupHandler := NewGroupHandler(groupStore)
	evaluateHandler := NewEvaluateHandler(evaluateEngine, auditStore, eventPub)
	auditHandler := NewAuditHandler(auditStore)

	r := chi.NewRouter()
	r.Use(middleware.SetupCORS())
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.TraceID)

	// Health — unauthenticated
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// JWT + tenant middleware
	r.Group(func(r chi.Router) {
		// Routes are authenticated with JWT + tenant header
		r.Use(middleware.JWTMiddleware(middleware.NewAuthValidator(jwtSecret, jwtIssuer)))
		r.Use(middleware.TenantMiddleware())

		// Policy CRUD
		r.Post("/policies", policyHandler.CreatePolicy)
		r.Get("/policies", policyHandler.ListPolicies)
		r.Get("/policies/{id}", policyHandler.GetPolicy)
		r.Patch("/policies/{id}", policyHandler.UpdatePolicy)
		r.Delete("/policies/{id}", policyHandler.DeletePolicy)

		// Policy evaluation
		r.Post("/policies/evaluate", evaluateHandler.EvaluatePolicy)

		// Policy group CRUD
		r.Get("/policy-groups", groupHandler.ListGroups)
		r.Post("/policy-groups", groupHandler.CreateGroup)
		r.Get("/policy-groups/{id}", groupHandler.GetGroup)
		r.Patch("/policy-groups/{id}", groupHandler.UpdateGroup)
		r.Delete("/policy-groups/{id}", groupHandler.DeleteGroup)

		// Audit log
		r.Get("/audit", auditHandler.ListAudits)
	})

	return r
}

// Ensure json import is used
var _ = json.Marshal