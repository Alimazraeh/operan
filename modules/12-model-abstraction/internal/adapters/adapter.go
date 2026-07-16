package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChatRequest matches OpenAI's chat completions API format.
type ChatRequest struct {
	Model            string           `json:"model"`
	Messages         []ChatMessage    `json:"messages"`
	Temperature      *float64         `json:"temperature,omitempty"`
	MaxTokens        int              `json:"max_tokens,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	Stream           bool             `json:"stream,omitempty"`
	User             string           `json:"user,omitempty"`
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatResponse matches OpenAI's chat completions response format.
type ChatResponse struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []ChatChoice     `json:"choices"`
	Usage   ChatUsage        `json:"usage"`
	Error   *APIError        `json:"error,omitempty"`
}

// ChatChoice represents a single choice in the response.
type ChatChoice struct {
	Index   int          `json:"index"`
	Message ChatMessage  `json:"message"`
	Finish  string       `json:"finish_reason"`
}

// ChatUsage records token usage for a completion.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// APIError represents an error response from a provider.
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// EmbedRequest matches OpenAI's embeddings API format.
type EmbedRequest struct {
	Model  string   `json:"model"`
	Input  []string `json:"input"`
	Format string   `json:"format,omitempty"`
	User   string   `json:"user,omitempty"`
}

// EmbedResponse matches OpenAI's embeddings response format.
type EmbedResponse struct {
	Model  string     `json:"model"`
	Object string     `json:"object"`
	Data   []EmbedDoc `json:"data"`
	Usage  EmbedUsage `json:"usage"`
	Error  *APIError  `json:"error,omitempty"`
}

// EmbedDoc represents a single embedding vector.
type EmbedDoc struct {
	Index    int      `json:"index"`
	Embedding []float64 `json:"embedding"`
	Object   string   `json:"object"`
}

// EmbedUsage records token usage for embeddings.
type EmbedUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ProviderAdapter is the interface all LLM provider adapters must implement.
type ProviderAdapter interface {
	// Chat sends a chat completion request to the provider.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// Embeddings sends an embedding request to the provider.
	Embeddings(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
	// HealthCheck verifies the provider is reachable.
	HealthCheck(ctx context.Context) error
}

// ProviderConfig holds the configuration needed for an adapter.
type ProviderConfig struct {
	BaseURL   string
	APIKey    string
	TimeoutMs int
}

// httpClient is the default HTTP client.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// httpClientWithTimeout creates a new HTTP client with a specific timeout.
func httpClientWithTimeout(timeoutMs int) *http.Client {
	return &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
}

// setHTTPClient allows tests to inject a mock HTTP client.
func setHTTPClient(c *http.Client) {
	httpClient = c
}

// resetHTTPClient restores the default HTTP client.
func resetHTTPClient() {
	httpClient = &http.Client{Timeout: 30 * time.Second}
}

// doJSONRequest performs an HTTP POST and unmarshals the JSON response.
func doJSONRequest(ctx context.Context, baseURL, path, apiKey string, reqBody any, timeoutMs int, response any) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := baseURL
	if len(path) > 0 && path[0] != '/' {
		url += "/"
	}
	url += path

	client := httpClientWithTimeout(timeoutMs)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
			return fmt.Errorf("provider error (%d): %s [%s]", resp.StatusCode, apiErr.Message, apiErr.Type)
		}
		return fmt.Errorf("provider error (%d): %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("unmarshal response: %w, body: %s", err, string(respBody))
	}

	return nil
}