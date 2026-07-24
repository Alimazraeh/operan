package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/llm"
)

// AgentHandler makes an agent actually reason: it gathers the agent's
// relevant memories from the Memory Fabric (Module 07) and calls an LLM
// (via the platform gateway) to produce real work product — the thing a
// human then reviews at a supervision gate.
type AgentHandler struct {
	llm       *llm.Client
	memoryURL string // base URL of Module 07, e.g. http://memory-fabric...:8007
	http      *http.Client
}

// NewAgentHandler constructs an AgentHandler. A nil llm disables reasoning
// (the endpoint then reports it is unconfigured rather than faking output).
func NewAgentHandler(client *llm.Client, memoryURL string) *AgentHandler {
	return &AgentHandler{llm: client, memoryURL: strings.TrimRight(memoryURL, "/"),
		http: &http.Client{Timeout: 20 * time.Second}}
}

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
	if h.llm == nil {
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
	if req.EmbeddingType == "" {
		req.EmbeddingType = "agent_personal"
	}

	// 1) Gather context from the agent's memory (Module 07), tenant-scoped
	//    by forwarding the caller's credentials.
	memQuery := req.MemoryQuery
	if memQuery == "" {
		memQuery = req.Instruction
	}
	memories := h.recallMemory(r, memQuery, req.EmbeddingType, nil)

	// Department context: the charter and service portfolio provisioned at
	// deploy time live in Module 07 as embedding_type "department", tagged
	// with the department id.
	var deptContext []string
	if req.DepartmentID != "" {
		deptContext = h.recallMemory(r, memQuery, "department",
			map[string]interface{}{"department_id": req.DepartmentID})
	}

	// 2) Build the agent's prompt from its role + what it remembers.
	system := fmt.Sprintf(
		"You are %s, an AI agent operating inside a customer's Operan department. "+
			"Use the department context and the agent's remembered facts to ground your work. "+
			"Output only the work product requested — no preamble, no explanation of your reasoning.",
		fallback(req.Role, "a department agent"))
	var b strings.Builder
	if len(deptContext) > 0 {
		b.WriteString("Your department's charter and services:\n")
		for _, m := range deptContext {
			b.WriteString("- " + m + "\n")
		}
		b.WriteString("\n")
	}
	if len(memories) > 0 {
		b.WriteString("What you remember about this customer:\n")
		for _, m := range memories {
			b.WriteString("- " + m + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Task: " + req.Instruction)
	memories = append(deptContext, memories...)

	// 3) Reason.
	res, err := h.llm.Complete(r.Context(), system, b.String(), req.MaxTokens)
	if err != nil {
		h.WriteError(w, http.StatusBadGateway, 502, "agent reasoning failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(draftResponse{
		Output: res.Content, Model: h.llm.Model(), MemoryUsed: memories,
		Tokens: res.Tokens, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// recallMemory queries Module 07 /search, forwarding caller credentials.
// Best effort: a memory outage yields no context, not a failure.
func (h *AgentHandler) recallMemory(r *http.Request, query, embeddingType string, filters map[string]interface{}) []string {
	if h.memoryURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"query": query, "embedding_type": embeddingType, "relevance_threshold": 0.25, "top_n": 5,
	}
	if len(filters) > 0 {
		payload["filters"] = filters
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.memoryURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Tenant-ID", r.Header.Get("X-Tenant-ID"))
	resp, err := h.http.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var parsed struct {
		Items []struct {
			Content string `json:"content"`
		} `json:"items"`
	}
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &parsed)
	out := make([]string, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		if it.Content != "" {
			out = append(out, it.Content)
		}
	}
	return out
}

// WriteError mirrors the error shape used by the other handlers.
func (h *AgentHandler) WriteError(w http.ResponseWriter, status, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{"code": code, "message": message},
	})
}

func fallback(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
