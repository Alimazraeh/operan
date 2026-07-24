package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/draft"
	"github.com/operan/modules/03-agent-orchestration/internal/llm"
)

// AgentHandler makes an agent actually reason. The actual memory-recall +
// prompt assembly + completion lives in the shared draft.Engine (also used
// by the workflow node handler) so a draft behaves identically wherever it
// runs.
type AgentHandler struct {
	engine *draft.Engine
}

// NewAgentHandler constructs an AgentHandler. A nil llm disables reasoning
// (the endpoint then reports it is unconfigured rather than faking output).
func NewAgentHandler(client *llm.Client, memoryURL string) *AgentHandler {
	if client == nil {
		return &AgentHandler{}
	}
	return &AgentHandler{engine: draft.NewEngine(client, memoryURL)}
}

// Engine exposes the shared draft engine for the workflow node handler.
func (h *AgentHandler) Engine() *draft.Engine { return h.engine }

type draftRequest struct {
	AgentID       string `json:"agent_id"`
	Role          string `json:"role"`
	Instruction   string `json:"instruction"`
	MemoryQuery   string `json:"memory_query"`
	EmbeddingType string `json:"embedding_type"`
	// DepartmentID grounds the draft in the department's own memory
	// (charter, service portfolio) in addition to the agent's personal one.
	DepartmentID string `json:"department_id"`
	MaxTokens    int    `json:"max_tokens"`
}

type draftResponse struct {
	Output      string   `json:"output"`
	Model       string   `json:"model"`
	MemoryUsed  []string `json:"memory_used"`
	Tokens      int      `json:"tokens"`
	GeneratedAt string   `json:"generated_at"`
}

// Draft handles POST /agent/draft. It is the real "agent_task": memory →
// reasoning → output. Caller credentials (Authorization + X-Tenant-ID) are
// forwarded to the Memory Fabric so retrieval stays tenant-scoped.
func (h *AgentHandler) Draft(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.WriteError(w, http.StatusServiceUnavailable, 503, "LLM gateway not configured (set LLM_BASE_URL)")
		return
	}
	var req draftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Instruction == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "instruction is required")
		return
	}

	out, err := h.engine.Draft(r.Context(), draft.Input{
		Role:          req.Role,
		Instruction:   req.Instruction,
		MemoryQuery:   req.MemoryQuery,
		EmbeddingType: req.EmbeddingType,
		DepartmentID:  req.DepartmentID,
		Authorization: r.Header.Get("Authorization"),
		TenantID:      r.Header.Get("X-Tenant-ID"),
		MaxTokens:     req.MaxTokens,
	})
	if err != nil {
		h.WriteError(w, http.StatusBadGateway, 502, "agent reasoning failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(draftResponse{
		Output: out.Text, Model: out.Model, MemoryUsed: out.MemoryUsed,
		Tokens: out.Tokens, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// WriteError mirrors the error shape used by the other handlers.
func (h *AgentHandler) WriteError(w http.ResponseWriter, status, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{"code": code, "message": message},
	})
}
