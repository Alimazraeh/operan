package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// M12Client wraps HTTP calls to Module 12 (Model Abstraction Layer).
type M12Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewM12Client creates an HTTP client for M12.
func NewM12Client(baseURL string) *M12Client {
	return &M12Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// EmbedRequest is the body sent to M12's /v1/models/embeddings.
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse is the response from M12's /v1/models/embeddings.
type EmbedResponse struct {
	Model  string    `json:"model"`
	Object string    `json:"object"`
	Data   []EmbedItem `json:"data"`
}

// EmbedItem is a single embedding vector entry.
type EmbedItem struct {
	Index    int     `json:"index"`
	Object   string  `json:"object"`
	Embedding []float64 `json:"embedding"`
}

// EmbedArabic calls M12 to get embeddings for Arabic text.
func (c *M12Client) EmbedArabic(ctx context.Context, tenantID, model, text, jwtToken string) (*EmbedResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("M12_BASE_URL not configured")
	}

	reqBody := EmbedRequest{
		Model: model,
		Input: text,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	url := c.baseURL + "/v1/models/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call M12 embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("M12 server error %d: %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("M12 error %d: %s", resp.StatusCode, string(body))
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("decode M12 response: %w", err)
	}

	return &embedResp, nil
}