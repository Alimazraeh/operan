package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type M12Client struct {
	baseURL string
	timeout time.Duration
}

func NewM12Client(baseURL string, timeoutMs int) *M12Client {
	return &M12Client{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbedResponse struct {
	Embedding  []float64 `json:"embedding"`
	TokenCount int       `json:"token_count"`
}

func (c *M12Client) EmbedChunk(ctx context.Context, model, text, jwtToken string) ([]float64, int, error) {
	reqBody, err := json.Marshal(EmbedRequest{Model: model, Input: []string{text}})
	if err != nil {
		return nil, 0, fmt.Errorf("m12 marshal: %w", err)
	}

	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/models/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, 0, fmt.Errorf("m12 request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("m12 embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("m12 embed: HTTP %d", resp.StatusCode)
	}

	var result EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("m12 decode: %w", err)
	}
	return result.Embedding, result.TokenCount, nil
}