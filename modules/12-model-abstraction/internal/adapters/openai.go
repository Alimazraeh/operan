package adapters

import (
	"context"
	"fmt"
)

// OpenAIAdapter implements ProviderAdapter for OpenAI's API.
type OpenAIAdapter struct {
	cfg ProviderConfig
}

// NewOpenAIAdapter creates a new OpenAI adapter.
func NewOpenAIAdapter(cfg ProviderConfig) *OpenAIAdapter {
	return &OpenAIAdapter{cfg: cfg}
}

// Chat forwards the request to OpenAI's chat completions endpoint.
func (a *OpenAIAdapter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var resp ChatResponse
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/chat/completions", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("openai chat error: %s", resp.Error.Message)
	}
	return &resp, nil
}

// Embeddings forwards the request to OpenAI's embeddings endpoint.
func (a *OpenAIAdapter) Embeddings(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	var resp EmbedResponse
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/embeddings", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("openai embeddings: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("openai embeddings error: %s", resp.Error.Message)
	}
	return &resp, nil
}

// HealthCheck pings the OpenAI API health endpoint.
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) error {
	// OpenAI doesn't have a public health endpoint, so we do a lightweight request.
	// Try listing models as a health check.
	req := map[string]any{}
	var resp map[string]any
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/models", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return fmt.Errorf("openai health check failed: %w", err)
	}
	if resp["object"] != "list" {
		return fmt.Errorf("unexpected health check response: %v", resp)
	}
	return nil
}