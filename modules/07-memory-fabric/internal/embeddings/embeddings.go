// Package embeddings provides an OpenAI-compatible embeddings client used
// to vectorize memory content through the platform's LiteLLM gateway.
package embeddings

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

// Client calls an OpenAI-compatible /v1/embeddings endpoint.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// New creates an embeddings client. baseURL is the gateway root
// (e.g. "http://litellm.deep-research.svc.cluster.local:4000").
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Model returns the configured model identifier.
func (c *Client) Model() string { return c.model }

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// Embed returns one embedding vector per input text, in input order.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embeddingsRequest{Model: c.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embeddings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embeddings endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("embeddings endpoint returned %d: %s", resp.StatusCode, string(snippet))
	}

	var out embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings endpoint returned %d vectors for %d inputs", len(out.Data), len(texts))
	}

	vectors := make([][]float64, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vectors) {
			return nil, fmt.Errorf("embeddings response index %d out of range", d.Index)
		}
		vectors[d.Index] = d.Embedding
	}
	return vectors, nil
}
