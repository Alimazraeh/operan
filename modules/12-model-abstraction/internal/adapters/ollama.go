package adapters

import (
	"context"
	"fmt"
)

// OllamaAdapter implements ProviderAdapter for local Ollama instances.
type OllamaAdapter struct {
	cfg ProviderConfig
}

// NewOllamaAdapter creates a new Ollama adapter.
func NewOllamaAdapter(cfg ProviderConfig) *OllamaAdapter {
	return &OllamaAdapter{cfg: cfg}
}

// Chat forwards the request to Ollama's chat endpoint.
func (a *OllamaAdapter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	ollamaReq := map[string]any{
		"model":  req.Model,
		"messages": req.Messages,
		"stream": false,
	}
	if req.Temperature != nil {
		ollamaReq["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		ollamaReq["max_tokens"] = req.MaxTokens
	}
	if req.TopP != nil {
		ollamaReq["top_p"] = *req.TopP
	}
	if len(req.Stop) > 0 {
		ollamaReq["stop"] = req.Stop
	}

	var resp map[string]any
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/api/chat", a.cfg.APIKey, ollamaReq, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}

	// Ollama returns {message: {role, content}, done: bool, prompt_eval_count, eval_count}
	msg, _ := resp["message"].(map[string]any)
	content, _ := msg["content"].(string)
	promptCount, _ := resp["prompt_eval_count"].(float64)
	evalCount, _ := resp["eval_count"].(float64)

	return &ChatResponse{
		ID:    resp["id"].(string),
		Model: resp["model"].(string),
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				Finish: "stop",
			},
		},
		Usage: ChatUsage{
			PromptTokens:     int(promptCount),
			CompletionTokens: int(evalCount),
			TotalTokens:      int(promptCount + evalCount),
		},
	}, nil
}

// Embeddings sends an embedding request to Ollama.
func (a *OllamaAdapter) Embeddings(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	ollamaReq := map[string]any{
		"model":  req.Model,
		"input":  req.Input,
		"stream": false,
	}

	var resp map[string]any
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/api/embeddings", a.cfg.APIKey, ollamaReq, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("ollama embeddings: %w", err)
	}

	embeddings, _ := resp["embeddings"].([]any)
	var docs []EmbedDoc
	for i, e := range embeddings {
		emb, _ := e.([]any)
		var vec []float64
		for _, v := range emb {
			f, _ := v.(float64)
			vec = append(vec, f)
		}
		docs = append(docs, EmbedDoc{
			Index:     i,
			Embedding: vec,
			Object:    "embedding",
		})
	}

	// Ollama doesn't report token counts in embeddings; estimate.
	totalTokens := 0
	for _, input := range req.Input {
		totalTokens += len(input) / 4
	}

	return &EmbedResponse{
		Model:  req.Model,
		Object: "list",
		Data:   docs,
		Usage: EmbedUsage{
			PromptTokens:  totalTokens,
			TotalTokens:   totalTokens,
		},
	}, nil
}

// HealthCheck verifies Ollama is reachable.
func (a *OllamaAdapter) HealthCheck(ctx context.Context) error {
	// Ollama has a /api/tags endpoint for listing models — free, no tokens consumed.
	req := map[string]any{}
	var resp map[string]any
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/api/tags", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	return nil
}