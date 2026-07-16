package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type M19Client struct {
	baseURL string
	timeout time.Duration
}

func NewM19Client(baseURL string, timeoutMs int) *M19Client {
	return &M19Client{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type NormalizeRequest struct {
	Text string `json:"text"`
}

type NormalizeResponse struct {
	Normalized string `json:"normalized"`
}

func (c *M19Client) NormalizeText(ctx context.Context, text string) (string, error) {
	reqBody, err := json.Marshal(NormalizeRequest{Text: text})
	if err != nil {
		return text, nil
	}

	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/normalize", bytes.NewReader(reqBody))
	if err != nil {
		return text, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Fallback: return original text when M19 unavailable
		return text, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback: return original text
		return text, nil
	}

	var result NormalizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return text, nil
	}
	return result.Normalized, nil
}