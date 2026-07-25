// Package llm provides an OpenAI-compatible chat client used by the agent
// runtime to actually reason — drafting, deciding, generating — through the
// platform's LiteLLM gateway (e.g. Qwen). This is what makes an agent_task
// produce real work rather than a status flip.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
			// The gateway returns the model's thinking separately. It is not
			// the work product and is never presented as one, but its length
			// is the evidence for why a budget was not enough.
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens      int `json:"total_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// DefaultMaxTokens is the budget for one agent step.
//
// Measured against the deployed gateway (Qwen3.6-35B) on the IT change
// management SOP's first step: 2000 returns finish_reason "length" with empty
// content, because reasoning tokens are spent before any content is emitted;
// the same call completes at 4000, using 2234. The old hardcoded 2000 sat just
// under what a real step costs, so the flagship SOP failed on its first node
// and took the whole request down with it.
const DefaultMaxTokens = 6000

// Result is the outcome of a completion.
type Result struct {
	Content string
	Tokens  int
	// Truncated reports that the model was cut off mid-answer. The content is
	// real but incomplete, and callers must say so rather than presenting a
	// half-written draft as finished work.
	Truncated bool
}

// Complete runs a chat completion.
//
// maxTokens must be generous: the gateway's models are reasoning models that
// spend tokens thinking before emitting any content, so a small budget yields
// empty content with finish_reason "length". That is a budget failure, not a
// model refusal, so it is retried once at double the budget before giving up.
func (c *Client) Complete(ctx context.Context, system, user string, maxTokens int) (*Result, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	res, err := c.complete(ctx, system, user, maxTokens)
	if err == nil || !isBudgetExhausted(err) {
		return res, err
	}
	// One retry, at double. If the model cannot get started in twice the
	// budget the problem is the prompt, not the ceiling, and doubling again
	// only burns time and tokens.
	retried, retryErr := c.complete(ctx, system, user, maxTokens*2)
	if retryErr != nil {
		return nil, fmt.Errorf("%w (retried at %d tokens: %v)", err, maxTokens*2, retryErr)
	}
	return retried, nil
}

// budgetExhaustedError marks the one failure worth retrying: the model spent
// its whole budget reasoning and emitted nothing.
type budgetExhaustedError struct {
	maxTokens      int
	reasoningChars int
}

func (e *budgetExhaustedError) Error() string {
	return fmt.Sprintf("model spent its whole %d-token budget reasoning (%d characters) and emitted no content",
		e.maxTokens, e.reasoningChars)
}

func isBudgetExhausted(err error) bool {
	var e *budgetExhaustedError
	return errors.As(err, &e)
}

func (c *Client) complete(ctx context.Context, system, user string, maxTokens int) (*Result, error) {
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
	choice := out.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)
	truncated := choice.FinishReason == "length"
	if content == "" {
		if truncated {
			return nil, &budgetExhaustedError{
				maxTokens:      maxTokens,
				reasoningChars: len(choice.Message.ReasoningContent),
			}
		}
		return nil, fmt.Errorf("model returned empty content (finish_reason %q)", choice.FinishReason)
	}
	return &Result{Content: content, Tokens: out.Usage.TotalTokens, Truncated: truncated}, nil
}
