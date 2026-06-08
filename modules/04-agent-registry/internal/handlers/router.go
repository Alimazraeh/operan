// Package handlers provides HTTP handlers and routing for the Agent Registry module.
package handlers

import (
	"net/http"
	"strings"

	mw "github.com/operan/modules/04-agent-registry/internal/middleware"
)

// adminRoles defines the roles permitted to perform write operations.
var adminRoles = []string{"admin", "registry_admin"}

// readerRoles defines the roles permitted to perform read operations.
var readerRoles = []string{"admin", "registry_admin", "registry_reader"}

// authWithRole wraps a handler with JWT authentication + Role-based access control.
func authWithRole(secret string, allowedRoles []string, handler http.HandlerFunc) http.HandlerFunc {
	jwtAuth := mw.JWTAuthWithSecret(secret)
	roleAuth := mw.RequireRole(allowedRoles...)
	return func(w http.ResponseWriter, r *http.Request) {
		jwtAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleAuth(http.HandlerFunc(handler)).ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	}
}

// RegisterRoutes registers all Agent Registry routes under /registry/agents.
func RegisterRoutes(h *AgentRegistryHandlers) *http.ServeMux {
	mux := http.NewServeMux()

	// ─── Health Check ──────────────────────────────────────────────────────
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		mw.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"module": "04-agent-registry",
		})
	})

	// ─── Agent List & Search ───────────────────────────────────────────────
	mux.HandleFunc("GET /registry/agents", mw.ExtractTenant(h.ListAgents))
	mux.HandleFunc("POST /registry/agents", authWithRole(h.JWTSecret, adminRoles, h.CreateAgent))
	mux.HandleFunc("POST /registry/agents/search", mw.ExtractTenant(h.SearchAgents))

	// ─── Agent GET/PATCH ───────────────────────────────────────────────────
	mux.HandleFunc("GET /registry/agents/", mw.ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		agentID := extractAgentIDFromPath(r.URL.Path)
		if agentID == "" {
			h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
			return
		}
		rem := strings.TrimPrefix(r.URL.Path, "/registry/agents/"+agentID)
		rem = strings.TrimPrefix(rem, "/")
		switch rem {
		case "":
			h.GetAgent(w, r)
		case "capabilities":
			h.ListAgentCapabilities(w, r)
		case "capabilities/index":
			h.IndexCapabilities(w, r)
		case "versions":
			h.ListAgentVersions(w, r)
		case "dependencies":
			h.ListDependencies(w, r)
		default:
			h.writeError(w, http.StatusNotFound, "not_found", "Not Found", "Resource not found")
		}
	}))

	mux.HandleFunc("PATCH /registry/agents/", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		agentID := extractAgentIDFromPath(r.URL.Path)
		if agentID == "" {
			h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
			return
		}
		rem := strings.TrimPrefix(r.URL.Path, "/registry/agents/"+agentID)
		rem = strings.TrimPrefix(rem, "/")
		switch rem {
		case "":
			h.UpdateAgent(w, r)
		case "capabilities":
			h.UpdateAgentCapabilities(w, r)
		default:
			h.writeError(w, http.StatusNotFound, "not_found", "Not Found", "Resource not found")
		}
	})))

	// ─── Agent DELETE ──────────────────────────────────────────────────────
	mux.HandleFunc("DELETE /registry/agents/", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
		agentID := extractAgentIDFromPath(r.URL.Path)
		if agentID == "" {
			h.writeError(w, http.StatusBadRequest, "invalid_path", "Bad Request", "agent_id is required")
			return
		}
		rem := strings.TrimPrefix(r.URL.Path, "/registry/agents/"+agentID)
		rem = strings.TrimPrefix(rem, "/")
		switch rem {
		case "":
			h.DeprecateAgent(w, r)
		case "dependencies":
			h.RemoveDependency(w, r)
		default:
			h.writeError(w, http.StatusNotFound, "not_found", "Not Found", "Resource not found")
		}
	})))

	// ─── Version GET/PATCH ─────────────────────────────────────────────────
	mux.HandleFunc("GET /registry/agents/*/versions", mw.ExtractTenant(h.ListAgentVersions))
	mux.HandleFunc("POST /registry/agents/*/versions", authWithRole(h.JWTSecret, adminRoles, h.CreateAgentVersion))
	mux.HandleFunc("GET /registry/agents/*/versions/", mw.ExtractTenant(h.GetAgentVersion))
	mux.HandleFunc("PATCH /registry/agents/*/versions/", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(h.UpdateAgentVersion)))

	// ─── Version Promotion ─────────────────────────────────────────────────
	mux.HandleFunc("POST /registry/agents/*/versions/*/promote", authWithRole(h.JWTSecret, adminRoles, h.PromoteVersion))

	// ─── Dependency CRUD ───────────────────────────────────────────────────
	mux.HandleFunc("GET /registry/agents/*/dependencies", mw.ExtractTenant(h.ListDependencies))
	mux.HandleFunc("POST /registry/agents/*/dependencies", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(h.AddDependency)))
	mux.HandleFunc("DELETE /registry/agents/*/dependencies", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(h.RemoveDependency)))

	// ─── Capability CRUD ───────────────────────────────────────────────────
	mux.HandleFunc("GET /registry/agents/*/capabilities", mw.ExtractTenant(h.ListAgentCapabilities))
	mux.HandleFunc("PATCH /registry/agents/*/capabilities", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(h.UpdateAgentCapabilities)))
	mux.HandleFunc("POST /registry/agents/*/capabilities/index", authWithRole(h.JWTSecret, adminRoles, mw.ExtractTenant(h.IndexCapabilities)))

	return mux
}
