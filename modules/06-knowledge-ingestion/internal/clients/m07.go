package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type M07Client struct {
	baseURL string
	timeout time.Duration
}

func NewM07Client(baseURL string, timeoutMs int) *M07Client {
	return &M07Client{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type Vector struct {
	ID       string         `json:"id"`
	Vector   []float64      `json:"vector"`
	Metadata map[string]any `json:"metadata"`
}

type M07StoreRequest struct {
	Namespace string    `json:"namespace"`
	Vectors   []Vector  `json:"vectors"`
}

func (c *M07Client) StoreVectors(ctx context.Context, namespace string, vectors []Vector) error {
	reqBody, err := json.Marshal(M07StoreRequest{Namespace: namespace, Vectors: vectors})
	if err != nil {
		return fmt.Errorf("m07 marshal: %w", err)
	}

	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/vectors", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("m07 request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("m07 store: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("m07 store: HTTP %d", resp.StatusCode)
	}
	return nil
}