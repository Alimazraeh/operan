package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"sync"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/config"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/handler"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/store"

	"github.com/segmentio/kafka-go"
)

func main() {
	cfg := config.Load()

	// Initialize stores
	users := store.NewUserStore()
	audit := store.NewAuditStore()

	// Initialize event publisher
	publisher := events.NewPublisher(cfg.EventBrokerURL)

	// Initialize Authentik client
	authClient := authentik.NewClient(cfg.AuthentikServerURL, cfg.AuthentikAdminToken)

	// Initialize handlers
	userHandler := handler.NewUserHandler(authClient, users, audit, publisher)
	roleHandler := handler.NewRoleHandler(authClient, publisher)
	serviceIDHandler := handler.NewServiceIdentityHandler(authClient, publisher)
	agentIDHandler := handler.NewAgentIdentityHandler(authClient, publisher)
	ssoHandler := handler.NewSSOHandler(authClient, publisher)
	scimHandler := handler.NewSCIMHandler(authClient, publisher)
	auditHandler := handler.NewAuditHandler(authClient)
	rbacHandler := handler.NewRBACHandler(authClient)
	ldapHandler := handler.NewLDAPHandler(authClient, publisher)
	adHandler := handler.NewADHandler(authClient, publisher)
	delegationHandler := handler.NewDelegationHandler(authClient, publisher)

	// Setup routes — base path: /api/v1/iam
	mux := http.NewServeMux()

	// Health check is registered later alongside the readiness probe.

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

	// /api/v1/iam/users/{id} → delegates (consolidated — PUT for roles handled here)
	mux.HandleFunc("/api/v1/iam/users/", func(w http.ResponseWriter, r *http.Request) {
		// Extract the remaining path after /api/v1/iam/users/
		remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/iam/users/")
		remaining = strings.TrimSuffix(remaining, "/")

		// Handle /api/v1/iam/users/{id}/roles
		if remaining != "" && strings.HasPrefix(remaining, "roles") {
			if r.Method == http.MethodPut {
				userHandler.SetRoles(w, r)
				return
			}
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

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
		sub := ""
		if len(r.URL.Path) > len("/api/v1/iam/scim/") {
			sub = r.URL.Path[len("/api/v1/iam/scim/"):]
			// strip trailing slash
			sub = strings.TrimSuffix(sub, "/")
		}

		switch sub {
		case "users", "":
			// /api/v1/iam/scim/users or /api/v1/iam/scim
			switch r.Method {
			case http.MethodGet:
				scimHandler.ListUsers(w, r)
			case http.MethodPost:
				scimHandler.Provision(w, r)
			case http.MethodPatch:
				scimHandler.UpdateUser(w, r)
			case http.MethodDelete:
				scimHandler.DeleteUser(w, r)
			default:
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			}
		case "provision":
			// /api/v1/iam/scim/provision
			switch r.Method {
			case http.MethodPost:
				scimHandler.Provision(w, r)
			default:
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			}
		case "bulk":
			// /api/v1/iam/scim/bulk — RFC 7644 Section 3.5
			switch r.Method {
			case http.MethodPost:
				scimHandler.BulkProvision(w, r)
			default:
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
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

	// ─── ABAC endpoints ───
	abacStore := handler.NewABACStore()
	abacHandler := handler.NewABACHandler(authClient, publisher, abacStore)

	mux.HandleFunc("/api/v1/iam/abac/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			abacHandler.Evaluate(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/iam/abac/policies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			abacHandler.CreatePolicy(w, r)
		case http.MethodGet:
			abacHandler.ListPolicies(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/iam/abac/policies/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			abacHandler.GetPolicy(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			abacHandler.DeletePolicy(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	// ─── MFA endpoints ───
	mfaHandler := handler.NewMFAHandler(authClient, publisher)

	mux.HandleFunc("/api/v1/iam/mfa/enroll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mfaHandler.Enroll(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/iam/mfa/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mfaHandler.Verify(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/iam/mfa/disable", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mfaHandler.Disable(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/iam/mfa/enrolled", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mfaHandler.ListDevices(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/iam/mfa/recovery-codes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mfaHandler.RegenerateRecoveryCodes(w, r)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	})

	// ─── LDAP endpoints ───
	mux.HandleFunc("/api/v1/iam/auth/ldap/", func(w http.ResponseWriter, r *http.Request) {
		sub := ""
		if len(r.URL.Path) > len("/api/v1/iam/auth/ldap/") {
			sub = r.URL.Path[len("/api/v1/iam/auth/ldap/"):]
			// strip trailing slash
			sub = sub[:len(sub)-1]
		}

		switch r.Method {
		case http.MethodPost:
			switch sub {
			case "configure":
				ldapHandler.Configure(w, r)
				return
			case "test":
				ldapHandler.Test(w, r)
				return
			}
		case http.MethodGet:
			if sub == "config" {
				ldapHandler.GetConfig(w, r)
				return
			}
		case http.MethodPatch:
			if sub == "config" {
				ldapHandler.UpdateConfig(w, r)
				return
			}
		case http.MethodDelete:
			if sub == "config" {
				ldapHandler.DeleteConfig(w, r)
				return
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	// ─── Active Directory endpoints ───
	mux.HandleFunc("/api/v1/iam/auth/ad/", func(w http.ResponseWriter, r *http.Request) {
		sub := ""
		if len(r.URL.Path) > len("/api/v1/iam/auth/ad/") {
			sub = r.URL.Path[len("/api/v1/iam/auth/ad/"):]
			// strip trailing slash
			sub = sub[:len(sub)-1]
		}

		switch r.Method {
		case http.MethodPost:
			switch sub {
			case "configure":
				adHandler.Configure(w, r)
				return
			case "test":
				adHandler.Test(w, r)
				return
			}
		case http.MethodGet:
			switch sub {
			case "config":
				adHandler.GetConfig(w, r)
				return
			}
		case http.MethodPatch:
			if sub == "config" {
				adHandler.UpdateConfig(w, r)
				return
			}
		case http.MethodDelete:
			if sub == "config" {
				adHandler.DeleteConfig(w, r)
				return
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	// ─── Delegated Admin Role endpoints ───
	mux.HandleFunc("/api/v1/iam/admin/delegation", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			delegationHandler.Create(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/iam/admin/delegations", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			delegationHandler.Create(w, r)
		case http.MethodGet:
			delegationHandler.List(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/iam/admin/delegations/", func(w http.ResponseWriter, r *http.Request) {
		remaining := ""
		if len(r.URL.Path) > len("/api/v1/iam/admin/delegations/") {
			remaining = r.URL.Path[len("/api/v1/iam/admin/delegations/"):]
			// strip trailing slash
			if len(remaining) > 0 {
				remaining = remaining[:len(remaining)-1]
			}
		}

		// Split path to extract role_id
		parts := []string{}
		if remaining != "" {
			parts = strings.Split(remaining, "/")
		}

		if len(parts) == 0 {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		roleID := parts[0]
		if roleID == "" {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		if len(parts) == 1 {
			// /api/v1/iam/admin/delegations/{role_id}
			switch r.Method {
			case http.MethodGet:
				delegationHandler.GetByID(w, r)
				return
			case http.MethodPatch:
				delegationHandler.Update(w, r)
				return
			case http.MethodDelete:
				delegationHandler.Delete(w, r)
				return
			}
		}

		// /api/v1/iam/admin/delegations/{role_id}/...
		if len(parts) >= 2 {
			action := parts[1]
			switch action {
			case "grant":
				if r.Method == http.MethodPost {
					delegationHandler.Grant(w, r)
					return
				}
			case "revoke":
				if r.Method == http.MethodPost {
					delegationHandler.Revoke(w, r)
					return
				}
			case "delegations":
				if r.Method == http.MethodGet {
					delegationHandler.ListDelegations(w, r)
					return
				}
			}
		}

		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	// Initialize JWKS cache for JWT validation
	jwksURL := cfg.AuthentikServerURL + "/.well-known/jwks.json"
	jwksHTTPClient := &http.Client{Timeout: 10 * time.Second}
	jwksCache := middleware.NewJWKSCache(jwksURL, jwksHTTPClient)

	// Initial refresh — populate cache on startup
	jwksCache.RefreshJWKS(context.Background(), jwksHTTPClient, jwksURL)
	log.Printf("JWKS cache initialized (last refresh: %s)", jwksCache.LastRefresh().Format(time.RFC3339))

	// Start background refresh every 55 minutes (before 1h TTL expires)
	jwksCache.RefreshPeriodically(context.Background(), 55*time.Minute)
	log.Println("JWKS background refresh started (55m interval)")

	// Health check — lightweight liveness probe
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy","module":"iam","version":"0.2.0"}`)
	})

	// Readiness check — verifies all dependencies are reachable
	type readinessCheck struct {
		Name   string `json:"name"`
		Status string `json:"status"` // "pass" | "fail"
	}

	readyTracker := &readiness{checks: map[string]func() bool{
		"authentik": func() bool {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.AuthentikServerURL+"/api/v1/health/", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return false
			}
			resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		},
		"event-broker": func() bool {
			if cfg.EventBrokerURL == "" {
				return true // No broker configured — always pass
			}
			// Kafka connection health check against the first broker address
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			addr := strings.TrimPrefix(cfg.EventBrokerURL, "kafka://")
			if i := strings.IndexByte(addr, ','); i >= 0 {
				addr = addr[:i]
			}
			conn, err := kafka.DialContext(ctx, "tcp", addr)
			if err != nil {
				return false
			}
			conn.Close()
			return true
		},
	}}

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		results := make([]readinessCheck, 0, len(readyTracker.checks))
		allReady := true
		for name, check := range readyTracker.checks {
			status := "fail"
			if check() {
				status = "pass"
			} else {
				allReady = false
			}
			results = append(results, readinessCheck{Name: name, Status: status})
		}
		w.Header().Set("Content-Type", "application/json")
		if !allReady {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"ready":false,"checks":`)
			encodeJSON(w, results)
			fmt.Fprint(w, `}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ready":true,"checks":`)
		encodeJSON(w, results)
		fmt.Fprint(w, `}`)
	})

	// ─── Middleware chain ───
	// Order (outermost → innermost): CORS → RequestID → Logging → TraceInjector → AuthValidator → TenantInjector → handlers
	var chain http.Handler = mux

	// 1. CORS — handle preflight before auth/rate-limit
	chain = middleware.CORS(middleware.DefaultCORSConfig())(chain)

	// 2. RequestID — generate/propagate request ID early
	chain = middleware.RequestIDMiddleware(chain)

	// 3. Logging — structured JSON logging with request ID
	chain = middleware.Logging(middleware.LoggingConfig{})(chain)

	// 4. TraceInjector — existing trace ID propagation
	chain = middleware.TraceInjector(chain)

	// 5. AuthValidator — JWT/HMAC validation + JWKS cache
	chain = middleware.AuthValidator(jwksCache, jwksURL, cfg.TokenSecret, chain)

	// 6. TenantInjector — extract tenant from header/context
	chain = middleware.TenantInjector(chain)

	// 7. RateLimiter — per-client token bucket
	rateLimiter := middleware.NewRateLimiterMiddleware(middleware.RateLimiterConfig{
		RequestsPerSecond: cfg.RateLimitRPS,
		BurstSize:         cfg.RateLimitBurst,
	})
	chain = rateLimiter.Chain(chain)

	// Liveness/readiness probes bypass auth and tenant middleware so
	// orchestrator health checks succeed without credentials.
	protected := chain
	chain = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/ready" {
			mux.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           chain,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Identity & Access Management module starting on %s", addr)
	log.Printf("Event broker: %s", cfg.EventBrokerURL)

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown on SIGTERM / SIGINT
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// ---- helpers ----

type readiness struct {
	mu     sync.RWMutex
	checks map[string]func() bool
}

func encodeJSON(w http.ResponseWriter, v interface{}) {
	// Inline JSON encode to avoid importing encoding/json in the main package when it might not be used elsewhere
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
	fmt.Fprint(w, strings.TrimSuffix(buf.String(), "\n"))
}
