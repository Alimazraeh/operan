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
	// ServeMux wildcard ambiguity).
	mux.HandleFunc("POST /execute", h.ExecuteTool)
	mux.HandleFunc("GET /executions", h.ListExecutions)
	mux.HandleFunc("GET /executions/{id}", h.GetExecution)
	mux.HandleFunc("POST /executions/{id}/retry", h.RetryExecution)
	mux.HandleFunc("GET /cost", h.GetToolCost)
}
