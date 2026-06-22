// Package llm provides an OpenAI-compatible chat client used by the agent
// runtime to actually reason — drafting, deciding, generating — through the
// platform's LiteLLM gateway (e.g. Qwen). This is what makes an agent_task
// produce real work rather than a status flip.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls an OpenAI-compatible /v1/chat/completions endpoint.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// New creates a chat client. baseURL is the gateway root
// (e.g. "http://litellm.deep-research.svc.cluster.local:4000").
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Model returns the configured model id.
func (c *Client) Model() string { return c.model }

// Message is one chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Result is the outcome of a completion.
type Result struct {
	Content string
	Tokens  int
}

// Complete runs a chat completion. maxTokens must be generous: the gateway's
// models are reasoning models that spend tokens thinking before emitting
// content, so a small budget yields empty content (finish_reason "length").
func (c *Client) Complete(ctx context.Context, system, user string, maxTokens int) (*Result, error) {
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	msgs := []Message{}
	if system != "" {
		msgs = append(msgs, Message{Role: "system", Content: system})
	}
	msgs = append(msgs, Message{Role: "user", Content: user})

	body, err := json.Marshal(chatRequest{Model: c.model, Messages: msgs, MaxTokens: maxTokens, Temperature: 0.4})
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call chat endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("chat endpoint returned %d: %s", resp.StatusCode, string(snippet))
	}

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("chat endpoint returned no choices")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("model returned empty content (finish_reason %q) — raise max_tokens", out.Choices[0].FinishReason)
	}
	return &Result{Content: content, Tokens: out.Usage.TotalTokens}, nil
}
