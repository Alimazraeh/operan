package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// M12Client wraps HTTP calls to the M12 Model Abstraction Layer.
type M12Client struct {
	BaseURL    string
	httpClient *http.Client
}

// NewM12Client creates a client for the M12 service.
func NewM12Client(baseURL string) *M12Client {
	return &M12Client{
		BaseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CompletionRequest is the body sent to M12's completions endpoint.
type CompletionRequest struct {
	Model      string                 `json:"model"`
	Messages   []CompletionMessage    `json:"messages,omitempty"`
	Prompt     string                 `json:"prompt,omitempty"`
	MaxTokens  int                    `json:"max_tokens,omitempty"`
	Temperature float64               `json:"temperature,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// CompletionMessage represents a chat message.
type CompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse is the response from M12's completions endpoint.
type CompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []Choice `json:"choices"`
	CostUSD float64 `json:"cost_usd"`
	LatencyMs int    `json:"latency_ms"`
	Error   string `json:"error,omitempty"`
}

// Choice represents a single completion choice.
type Choice struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

// Completions calls M12's completion endpoint with the resolved model.
func (c *M12Client) Completions(req CompletionRequest) (*CompletionResponse, error) {
	url := c.BaseURL + "/v1/models/completions"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(nil, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("m12 returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var completion CompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if completion.Error != "" {
		return nil, fmt.Errorf("m12 error: %s", completion.Error)
	}

	return &completion, nil
}