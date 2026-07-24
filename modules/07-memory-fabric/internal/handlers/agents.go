package handlers

import (
	"net/http"

	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

// Default ephemeral window applied until per-agent configuration exists.
const (
	defaultEphemeralMaxTokens  = 8192
	defaultEphemeralTTLSeconds = 3600
)

// GetAgentMemory handles GET /agents/{id}.
func (h *MemoryHandlers) GetAgentMemory(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	agentID := r.PathValue("id")

	// An agent with no recorded memories has a valid EMPTY memory state —
	// return it rather than 404 so clients render "no memories yet".
	ids, latest, _ := h.Vectors.AgentMemoryIDs(tenantID, agentID)
	if ids == nil {
		ids = []string{}
	}

	mem := store.AgentMemory{
		AgentID:          agentID,
		TenantID:         tenantID,
		PersonalMemories: ids,
		EphemeralWindow: &store.EphemeralWindow{
			MaxTokens:  defaultEphemeralMaxTokens,
			TTLSeconds: defaultEphemeralTTLSeconds,
		},
		Status: "active",
	}
	if latest != nil {
		mem.LastUpdated = &latest.CreatedAt
	}
	writeJSON(w, http.StatusOK, mem)
}
