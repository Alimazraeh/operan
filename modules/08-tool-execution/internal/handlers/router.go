package handlers

import "net/http"

// RegisterRoutes registers all Module 08 endpoints on the given ServeMux
// (Go 1.22+ pattern syntax with typed method + path wildcards).
func RegisterRoutes(mux *http.ServeMux, h *ToolHandlers) {
	// Tool registry (under /tools)
	mux.HandleFunc("POST /tools/register", h.RegisterTool)
	mux.HandleFunc("GET /tools", h.ListTools)
	mux.HandleFunc("GET /tools/{id}", h.GetTool)
	mux.HandleFunc("PATCH /tools/{id}", h.UpdateTool)
	mux.HandleFunc("GET /tools/{id}/versions", h.ListToolVersions)

	// Execution + cost (top-level, kept separate from /tools/{id} to avoid
	// ServeMux wildcard ambiguity). /execute and retry answer 410 — the echo
	// executor is gone; the governed door is /invoke.
	mux.HandleFunc("POST /execute", h.ExecuteTool)
	mux.HandleFunc("GET /executions", h.ListExecutions)
	mux.HandleFunc("GET /executions/{id}", h.GetExecution)
	mux.HandleFunc("POST /executions/{id}/retry", h.RetryExecution)
	mux.HandleFunc("GET /cost", h.GetToolCost)
}

// RegisterCapabilityRoutes registers the capability layer: the vocabulary,
// providers, bindings, the governed invoke funnel, and the audit trail.
func RegisterCapabilityRoutes(mux *http.ServeMux, h *CapabilityHandlers) {
	mux.HandleFunc("GET /capabilities", h.ListCapabilities)
	mux.HandleFunc("GET /providers", h.ListProviders)
	mux.HandleFunc("POST /providers", h.CreateProvider)
	mux.HandleFunc("GET /bindings", h.ListBindings)
	mux.HandleFunc("POST /bindings", h.PutBinding)
	mux.HandleFunc("POST /invoke", h.Invoke)
	mux.HandleFunc("GET /invocations", h.ListInvocations)
}
