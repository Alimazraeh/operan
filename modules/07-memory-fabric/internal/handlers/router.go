package handlers

import "net/http"

// RegisterRoutes registers all Module 07 endpoints on the given ServeMux
// (Go 1.22+ pattern syntax with typed method + path wildcards).
func RegisterRoutes(mux *http.ServeMux, h *MemoryHandlers) {
	// Vectors
	mux.HandleFunc("POST /vectors", h.IngestVectors)
	mux.HandleFunc("GET /vectors", h.ListVectors)
	mux.HandleFunc("GET /vectors/{id}", h.GetVector)
	mux.HandleFunc("PUT /vectors/{id}", h.UpdateVector)
	mux.HandleFunc("DELETE /vectors/{id}", h.DeleteVector)

	// Semantic search
	mux.HandleFunc("POST /search", h.SearchMemory)

	// Agent memory state
	mux.HandleFunc("GET /agents/{id}", h.GetAgentMemory)

	// Garbage collection
	mux.HandleFunc("POST /gc", h.TriggerGC)

	// Retention policies
	mux.HandleFunc("GET /retention-policies", h.ListRetentionPolicies)
	mux.HandleFunc("POST /retention-policies", h.CreateRetentionPolicy)
}
