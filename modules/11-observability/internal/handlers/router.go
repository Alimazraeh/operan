package handlers

import "net/http"

// RegisterRoutes registers all Module 11 endpoints on the given ServeMux
// (Go 1.22+ pattern syntax with typed method + path wildcards).
//
// Note: GET /health here is the contract's tenant system-health endpoint
// (auth-required). The unauthenticated service liveness probe is /healthz,
// registered on the root mux in main.
func RegisterRoutes(mux *http.ServeMux, h *ObservabilityHandlers) {
	// Metrics
	mux.HandleFunc("POST /metrics", h.RecordMetric)
	mux.HandleFunc("GET /metrics", h.ListMetrics)

	// Spans & traces
	mux.HandleFunc("GET /spans", h.ListSpans)
	mux.HandleFunc("GET /traces/{id}", h.GetTrace)

	// Alerts
	mux.HandleFunc("GET /alerts", h.ListAlerts)
	mux.HandleFunc("POST /alerts/{id}/resolve", h.ResolveAlert)

	// System health (tenant-scoped, per contract)
	mux.HandleFunc("GET /health", h.GetHealthStatus)
	mux.HandleFunc("GET /health/{componentId}", h.GetComponentHealth)
}
