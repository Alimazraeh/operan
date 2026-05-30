// Package handlers provides the central route registration for Module 05.
package handlers

import "net/http"

// RegisterRoutes registers all Module 05 endpoints on the given ServeMux.
// It expects a mux created with http.NewServeMux() (Go 1.22+).
func RegisterRoutes(mux *http.ServeMux, h *TemplateHandlers) {
	// ─── Standard Template CRUD ───────────────────────────────────────────
	mux.HandleFunc("GET /templates", h.ListTemplates)
	mux.HandleFunc("POST /templates", h.CreateTemplate)
	mux.HandleFunc("GET /templates/", h.GetTemplate)
	mux.HandleFunc("PATCH /templates/", h.UpdateTemplate)
	mux.HandleFunc("DELETE /templates/", h.DeleteTemplate)

	// ─── Custom Templates (under /templates/custom) ───────────────────────
	mux.HandleFunc("GET /templates/custom", h.ListCustomTemplates)
	mux.HandleFunc("POST /templates/custom", h.CreateCustomTemplate)
	mux.HandleFunc("GET /templates/custom/", h.GetCustomTemplate)
	mux.HandleFunc("PATCH /templates/custom/", h.UpdateCustomTemplate)
	mux.HandleFunc("DELETE /templates/custom/", h.DeleteCustomTemplate)

	// ─── Deploy, Deployments, Versions, Clone (nested under /templates/{id}) ─
	// These use /templates/ prefix and extract template ID from path.
	mux.HandleFunc("POST /templates/", h.HandleTemplateNested)
}
