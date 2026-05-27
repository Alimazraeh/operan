package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/operan/modules/02-identity-access/internal/config"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/handler"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/store"
)

func main() {
	cfg := config.Load()

	// Initialize stores
	users := store.NewUserStore()
	roles := store.NewRoleStore()
	serviceIDs := store.NewServiceIdentityStore()
	agentIDs := store.NewAgentIdentityStore()
	ssoConfigs := store.NewSSOConfigStore()
	audit := store.NewAuditStore()

	// Initialize event publisher
	publisher := events.NewPublisher(cfg.EventBrokerURL)

	// Initialize handlers
	userHandler := handler.NewUserHandler(users, audit, publisher)
	roleHandler := handler.NewRoleHandler(roles, audit, publisher)
	serviceIDHandler := handler.NewServiceIdentityHandler(serviceIDs, audit, publisher)
	agentIDHandler := handler.NewAgentIdentityHandler(agentIDs, audit, publisher)
	ssoHandler := handler.NewSSOHandler(ssoConfigs, audit, publisher)
	scimHandler := handler.NewSCIMHandler(users, audit, publisher)
	auditHandler := handler.NewAuditHandler(audit)
	rbacHandler := handler.NewRBACHandler(users, roles, serviceIDs, agentIDs, audit)

	// Setup routes — base path: /api/v1/iam
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	})

	// POST /api/v1/iam/users
	mux.HandleFunc("/api/v1/iam/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			userHandler.Create(w, r)
		case http.MethodGet:
			userHandler.List(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// /api/v1/iam/users/{id} → delegates
	mux.HandleFunc("/api/v1/iam/users/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.GetByID(w, r)
		case http.MethodPatch:
			userHandler.Update(w, r)
		case http.MethodDelete:
			userHandler.Deactivate(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// PUT /api/v1/iam/users/{id}/roles
	mux.HandleFunc("/api/v1/iam/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			// only hit this if path is /api/v1/iam/users/{id}/roles
			if len(r.URL.Path) > len("/api/v1/iam/users/") {
				remaining := r.URL.Path[len("/api/v1/iam/users"):]
				if remaining == "/roles" {
					userHandler.SetRoles(w, r)
					return
				}
			}
		}
		// fall through to other methods
		switch r.Method {
		case http.MethodGet:
			userHandler.GetByID(w, r)
		case http.MethodPatch:
			userHandler.Update(w, r)
		case http.MethodDelete:
			userHandler.Deactivate(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// POST /api/v1/iam/roles
	mux.HandleFunc("/api/v1/iam/roles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			roleHandler.Create(w, r)
		case http.MethodGet:
			roleHandler.List(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// DELETE /api/v1/iam/roles/{id}
	mux.HandleFunc("/api/v1/iam/roles/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			roleHandler.GetByID(w, r)
		case http.MethodDelete:
			roleHandler.Delete(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// POST /api/v1/iam/service-identities
	mux.HandleFunc("/api/v1/iam/service-identities", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			serviceIDHandler.Create(w, r)
		case http.MethodGet:
			serviceIDHandler.List(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// GET /api/v1/iam/service-identities/{id}
	mux.HandleFunc("/api/v1/iam/service-identities/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			serviceIDHandler.GetByID(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	// POST /api/v1/iam/agent-identities
	mux.HandleFunc("/api/v1/iam/agent-identities", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			agentIDHandler.Register(w, r)
		case http.MethodGet:
			agentIDHandler.List(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// GET /api/v1/iam/agent-identities/agent/{agent_id}
	mux.HandleFunc("/api/v1/iam/agent-identities/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			agentIDHandler.GetByAgent(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	// POST /api/v1/iam/auth/sso/configure
	mux.HandleFunc("/api/v1/iam/auth/sso/", func(w http.ResponseWriter, r *http.Request) {
		// Determine sub-route
		sub := ""
		if len(r.URL.Path) > len("/api/v1/iam/auth/sso/") {
			sub = r.URL.Path[len("/api/v1/iam/auth/sso/"):]
			// strip trailing slash
			sub = sub[:len(sub)-1]
		}

		switch r.Method {
		case http.MethodPost:
			switch sub {
			case "configure":
				ssoHandler.Configure(w, r)
				return
			case "test":
				ssoHandler.Test(w, r)
				return
			}
		case http.MethodGet:
			if sub == "config" {
				ssoHandler.GetConfig(w, r)
				return
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	// SCIM endpoints
	mux.HandleFunc("/api/v1/iam/scim/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			scimHandler.ListUsers(w, r)
		case http.MethodPost:
			scimHandler.Provision(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// GET /api/v1/iam/audit/trails
	mux.HandleFunc("/api/v1/iam/audit/", func(w http.ResponseWriter, r *http.Request) {
		sub := ""
		if len(r.URL.Path) > len("/api/v1/iam/audit/") {
			sub = r.URL.Path[len("/api/v1/iam/audit/"):]
			sub = sub[:len(sub)-1]
		}

		switch r.Method {
		case http.MethodGet:
			if sub == "trails" {
				// Check if trail_id is present
				if len(r.URL.Path) > len("/api/v1/iam/audit/trails/") {
					auditHandler.GetByID(w, r)
					return
				}
				auditHandler.GetTrails(w, r)
				return
			}
			if sub == "session-replay" {
				if len(r.URL.Path) > len("/api/v1/iam/audit/session-replay/") {
					auditHandler.GetSessionReplay(w, r)
					return
				}
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	// POST /api/v1/iam/rbac/evaluate
	mux.HandleFunc("/api/v1/iam/rbac/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			rbacHandler.Evaluate(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	// Wrap with middleware chain
	var chain http.Handler = mux
	chain = middleware.TraceInjector(chain)
	chain = middleware.AuthValidator(cfg.TokenSecret, chain)
	chain = middleware.TenantInjector(chain)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Identity & Access Management module starting on %s", addr)
	log.Printf("Event broker: %s", cfg.EventBrokerURL)

	if err := http.ListenAndServe(addr, chain); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
