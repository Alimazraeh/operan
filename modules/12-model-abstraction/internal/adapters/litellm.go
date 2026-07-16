package adapters

import (
	"context"
	"fmt"
)

// LiteLLMAdapter implements ProviderAdapter for LiteLLM proxy.
// LiteLLM exposes an OpenAI-compatible API, so we reuse the same mechanism as OpenAI
// but allow a custom base URL and model name transformation.
type LiteLLMAdapter struct {
	cfg ProviderConfig
}

// NewLiteLLMAdapter creates a new LiteLLM adapter.
func NewLiteLLMAdapter(cfg ProviderConfig) *LiteLLMAdapter {
	return &LiteLLMAdapter{cfg: cfg}
}

// Chat forwards the request to LiteLLM's OpenAI-compatible endpoint.
func (a *LiteLLMAdapter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var resp ChatResponse
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/chat/completions", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("litellm chat: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("litellm chat error: %s", resp.Error.Message)
	}
	return &resp, nil
}

// Embeddings forwards the request to LiteLLM's embeddings endpoint.
func (a *LiteLLMAdapter) Embeddings(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	var resp EmbedResponse
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/embeddings", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("litellm embeddings: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("litellm embeddings error: %s", resp.Error.Message)
	}
	return &resp, nil
}

// HealthCheck pings LiteLLM.
func (a *LiteLLMAdapter) HealthCheck(ctx context.Context) error {
	req := map[string]any{}
	var resp map[string]any
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/models", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return fmt.Errorf("litellm health check failed: %w", err)
	}
	if resp["object"] != "list" {
		return fmt.Errorf("unexpected litellm health response: %v", resp)
	}
	return nil
}