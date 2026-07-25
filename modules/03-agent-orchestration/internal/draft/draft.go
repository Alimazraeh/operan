// Package draft is the reusable agent-reasoning engine: memory recall from
// Module 07 (agent-personal + department scope) plus prompt assembly plus an
// LLM completion. Shared by the /agent/draft HTTP handler and the workflow
// node handler so a "draft" behaves identically wherever it runs.
package draft

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/llm"
)

// Engine performs grounded drafts.
type Engine struct {
	LLM       *llm.Client
	MemoryURL string // Module 07 base URL; empty disables recall
	httpc     *http.Client
}

func NewEngine(l *llm.Client, memoryURL string) *Engine {
	return &Engine{
		LLM:       l,
		MemoryURL: strings.TrimRight(memoryURL, "/"),
		httpc:     &http.Client{Timeout: 20 * time.Second},
	}
}

// Input is one draft invocation.
type Input struct {
	Role          string
	Instruction   string
	MemoryQuery   string
	EmbeddingType string // defaults to agent_personal
	DepartmentID  string // adds department-scope grounding when set
	Authorization string // forwarded to Module 07
	TenantID      string
	MaxTokens     int
}

// Output is the draft result.
type Output struct {
	Text       string
	Model      string
	MemoryUsed []string
	Tokens     int
	// Truncated reports that the model ran out of budget mid-answer. The text
	// is real but incomplete, and it must be labelled as such wherever it is
	// shown — an unfinished draft presented as finished work is the kind of
	// quiet lie this platform exists to avoid.
	Truncated bool
}

// Draft recalls memory, assembles the prompt, and completes.
func (e *Engine) Draft(ctx context.Context, in Input) (*Output, error) {
	if in.EmbeddingType == "" {
		in.EmbeddingType = "agent_personal"
	}
	memQuery := in.MemoryQuery
	if memQuery == "" {
		memQuery = in.Instruction
	}

	memories := e.recall(ctx, in, memQuery, in.EmbeddingType, nil)

	var deptContext []string
	if in.DepartmentID != "" {
		deptContext = e.recall(ctx, in, memQuery, "department",
			map[string]interface{}{"department_id": in.DepartmentID})
	}

	system := "You are " + fallback(in.Role, "a department agent") +
		", an AI agent operating inside a customer's Operan department. " +
		"Use the department context and the agent's remembered facts to ground your work. " +
		"Output only the work product requested — no preamble, no explanation of your reasoning."

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
	b.WriteString("Task: " + in.Instruction)

	res, err := e.LLM.Complete(ctx, system, b.String(), in.MaxTokens)
	if err != nil {
		return nil, err
	}
	return &Output{
		Text:       res.Content,
		Model:      e.LLM.Model(),
		MemoryUsed: append(deptContext, memories...),
		Tokens:     res.Tokens,
		Truncated:  res.Truncated,
	}, nil
}

// recall queries Module 07 /search. Best effort: outages yield no context.
func (e *Engine) recall(ctx context.Context, in Input, query, embeddingType string, filters map[string]interface{}) []string {
	if e.MemoryURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"query": query, "embedding_type": embeddingType, "relevance_threshold": 0.25, "top_n": 5,
	}
	if len(filters) > 0 {
		payload["filters"] = filters
	}
	body, _ := json.Marshal(payload)
	rctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, e.MemoryURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", in.Authorization)
	req.Header.Set("X-Tenant-ID", in.TenantID)
	resp, err := e.httpc.Do(req)
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
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	json.Unmarshal(data, &parsed)
	out := make([]string, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		if it.Content != "" {
			out = append(out, it.Content)
		}
	}
	return out
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
