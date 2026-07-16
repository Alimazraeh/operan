package adapters

import (
	"context"
	"fmt"
	"strings"
)

// AnthropicAdapter implements ProviderAdapter for Anthropic's API.
type AnthropicAdapter struct {
	cfg ProviderConfig
}

// NewAnthropicAdapter creates a new Anthropic adapter.
func NewAnthropicAdapter(cfg ProviderConfig) *AnthropicAdapter {
	return &AnthropicAdapter{cfg: cfg}
}

// anthropicMessage maps our ChatMessage to Anthropic's format.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicChatRequest is Anthropic's native format.
type anthropicChatRequest struct {
	Model         string               `json:"model"`
	Messages      []anthropicMessage   `json:"messages"`
	MaxTokens     int                  `json:"max_tokens"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	System        string               `json:"system,omitempty"`
}

// anthropicChatResponse is Anthropic's native response.
type anthropicChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Stop    string `json:"stop_reason"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	CContent []struct {
		Type  string `json:"type"`
		Text  string `json:"text,omitempty"`
	} `json:"content"`
}

// Chat transforms the unified request to Anthropic's format and calls their API.
func (a *AnthropicAdapter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	anthReq := anthropicChatRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
	}
	if req.Temperature != nil {
		anthReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		anthReq.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		anthReq.StopSequences = req.Stop
	}

	// Separate system message if present.
	for _, m := range req.Messages {
		if m.Role == "system" {
			anthReq.System = m.Content
			continue
		}
		anthReq.Messages = append(anthReq.Messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	var resp anthropicChatResponse
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/messages", a.cfg.APIKey, anthReq, a.cfg.TimeoutMs, &resp); err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}
	if resp.Error.Message != "" {
		return nil, fmt.Errorf("anthropic chat error: %s", resp.Error.Message)
	}

	// Extract text content.
	var textContent string
	for _, c := range resp.CContent {
		if c.Type == "text" {
			textContent += c.Text
		}
	}

	return &ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: textContent,
				},
				Finish: resp.Stop,
			},
		},
		Usage: ChatUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

// Embeddings is not supported by Anthropic's chat models directly.
func (a *AnthropicAdapter) Embeddings(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	return nil, fmt.Errorf("anthropic chat models do not support embeddings; use text-embeddings model")
}

// HealthCheck verifies Anthropic is reachable.
// NOTE: Anthropic has no dedicated health endpoint; this sends a minimal request
// (1 output token) which costs a fraction of a cent. For production, consider
// using an AMQP health-check queue or a reverse-proxy health endpoint instead.
func (a *AnthropicAdapter) HealthCheck(ctx context.Context) error {
	// Use the cheapest model with a 1-token response to minimize cost.
	req := map[string]any{
		"model":    "claude-haiku-3-20240307",
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}
	var resp anthropicChatResponse
	if err := doJSONRequest(ctx, a.cfg.BaseURL, "/v1/messages", a.cfg.APIKey, req, a.cfg.TimeoutMs, &resp); err != nil {
		return fmt.Errorf("anthropic health check failed: %w", err)
	}
	if resp.Error.Message != "" {
		// Rate limit or invalid request doesn't mean the service is down.
		if !strings.Contains(resp.Error.Message, "rate_limit") {
			return fmt.Errorf("anthropic health check returned error: %s", resp.Error.Message)
		}
	}
	return nil
}