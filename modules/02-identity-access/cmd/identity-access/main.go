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
	rbacHandler := handler.NewRBACHandler(users, roles, audit)

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	})

	// User routes
	mux.HandleFunc("/tenants/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tenants/" || r.URL.Path == "/tenants" {
			http.NotFound(w, r)
			return
		}

		parts := splitPath(r.URL.Path)
		if len(parts) < 5 {
			http.NotFound(w, r)
			return
		}

		// paths: /tenants/{id}/iam/...
		if parts[2] != "iam" {
			http.NotFound(w, r)
			return
		}

		switch parts[4] {
		case "roles", "service-identities", "agent-identities", "sso", "scim", "audit", "rbac":
			// Handle sub-routes
			handleSubRoute(w, r, parts, userHandler, roleHandler, serviceIDHandler, agentIDHandler, ssoHandler, scimHandler, auditHandler, rbacHandler)
		case "users":
			// Handle user routes
			switch r.Method {
			case http.MethodPost:
				userHandler.Create(w, r)
			case http.MethodGet:
				userHandler.List(w, r)
			default:
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			}
		default:
			http.NotFound(w, r)
		}
	})

	// Wrap with middleware chain
	var chain http.Handler = mux
	chain = middleware.TraceInjector(chain)
	chain = middleware.AuthValidator(chain)
	chain = middleware.TenantInjector(chain)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Identity & Access Management module starting on %s", addr)
	log.Printf("Event broker: %s", cfg.EventBrokerURL)

	if err := http.ListenAndServe(addr, chain); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleSubRoute dispatches to the appropriate handler based on the URL path.
func handleSubRoute(w http.ResponseWriter, r *http.Request, parts []string,
	userHandler *handler.UserHandler, roleHandler *handler.RoleHandler,
	serviceIDHandler *handler.ServiceIdentityHandler, agentIDHandler *handler.AgentIdentityHandler,
	ssoHandler *handler.SSOHandler, scimHandler *handler.SCIMHandler,
	auditHandler *handler.AuditHandler, rbacHandler *handler.RBACHandler) {

	if len(parts) < 6 {
		http.NotFound(w, r)
		return
	}

	resource := parts[4]

	switch resource {
	case "users":
		handleUserRoutes(w, r, parts, userHandler)
	case "roles":
		handleRoleRoutes(w, r, parts, roleHandler)
	case "service-identities":
		handleServiceIdentityRoutes(w, r, parts, serviceIDHandler)
	case "agent-identities":
		handleAgentIdentityRoutes(w, r, parts, agentIDHandler)
	case "sso":
		handleSSORoutes(w, r, parts, ssoHandler)
	case "scim":
		handleSCIMRoutes(w, r, scimHandler)
	case "audit":
		handleAuditRoutes(w, r, parts, auditHandler)
	case "rbac":
		handleRBACRoutes(w, r, rbacHandler)
	default:
		http.NotFound(w, r)
	}
}

// handleUserRoutes dispatches user sub-routes.
func handleUserRoutes(w http.ResponseWriter, r *http.Request, parts []string, userHandler *handler.UserHandler) {
	// /tenants/{id}/iam/users/{user_id}/roles
	if len(parts) >= 7 && parts[5] == "users" && parts[6] == "roles" {
		if r.Method == http.MethodPost {
			userHandler.SetRoles(w, r)
			return
		}
	}

	// /tenants/{id}/iam/users/{user_id}
	if len(parts) >= 6 && parts[5] == "users" {
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
		return
	}
}

// handleRoleRoutes dispatches role sub-routes.
func handleRoleRoutes(w http.ResponseWriter, r *http.Request, parts []string, roleHandler *handler.RoleHandler) {
	if len(parts) >= 6 && parts[5] == "roles" {
		switch r.Method {
		case http.MethodGet:
			roleHandler.GetByID(w, r)
		case http.MethodPost:
			roleHandler.Create(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleServiceIdentityRoutes dispatches service identity sub-routes.
func handleServiceIdentityRoutes(w http.ResponseWriter, r *http.Request, parts []string, serviceIDHandler *handler.ServiceIdentityHandler) {
	if len(parts) >= 6 && parts[5] == "service-identities" {
		switch r.Method {
		case http.MethodGet:
			serviceIDHandler.GetByID(w, r)
		case http.MethodPost:
			serviceIDHandler.Create(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleAgentIdentityRoutes dispatches agent identity sub-routes.
func handleAgentIdentityRoutes(w http.ResponseWriter, r *http.Request, parts []string, agentIDHandler *handler.AgentIdentityHandler) {
	if len(parts) >= 6 && parts[5] == "agent-identities" {
		switch r.Method {
		case http.MethodGet:
			agentIDHandler.GetByAgent(w, r)
		case http.MethodPost:
			agentIDHandler.Register(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleSSORoutes dispatches SSO sub-routes.
func handleSSORoutes(w http.ResponseWriter, r *http.Request, parts []string, ssoHandler *handler.SSOHandler) {
	if len(parts) >= 6 && parts[5] == "sso" {
		switch r.Method {
		case http.MethodPost:
			if len(parts) >= 7 && parts[6] == "configure" {
				ssoHandler.Configure(w, r)
				return
			}
		case http.MethodGet:
			if len(parts) >= 7 && parts[6] == "config" {
				ssoHandler.GetConfig(w, r)
				return
			}
		}
	}
}

// handleSCIMRoutes dispatches SCIM sub-routes.
func handleSCIMRoutes(w http.ResponseWriter, r *http.Request, scimHandler *handler.SCIMHandler) {
	if r.URL.Path != "" && len(r.URL.Path) > 10 && r.URL.Path[10:14] == "scim" {
		if r.Method == http.MethodPost {
			scimHandler.Provision(w, r)
			return
		}
	}
}

// handleAuditRoutes dispatches audit sub-routes.
func handleAuditRoutes(w http.ResponseWriter, r *http.Request, parts []string, auditHandler *handler.AuditHandler) {
	if len(parts) >= 6 && parts[5] == "audit" {
		switch r.Method {
		case http.MethodGet:
			if len(parts) >= 8 && parts[6] == "trails" && parts[7] != "" {
				auditHandler.GetByID(w, r)
			} else {
				auditHandler.GetTrails(w, r)
			}
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleRBACRoutes dispatches RBAC sub-routes.
func handleRBACRoutes(w http.ResponseWriter, r *http.Request, rbacHandler *handler.RBACHandler) {
	if r.URL.Path != "" && len(r.URL.Path) > 9 && r.URL.Path[9:13] == "rbac" {
		if r.Method == http.MethodPost {
			rbacHandler.Evaluate(w, r)
			return
		}
	}
}

// splitPath splits a URL path into segments (copied from handler_users.go for now).
func splitPath(path string) []string {
	if path == "/" {
		return []string{""}
	}
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return splitString(path, '/')
}

// splitString splits a string by a separator (copied from handler_users.go).
func splitString(s string, sep rune) []string {
	var result []string
	var current string
	for _, r := range s {
		if r == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	result = append(result, current)
	return result
}
