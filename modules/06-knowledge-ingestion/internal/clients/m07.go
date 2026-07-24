// Module 07 (Memory Fabric) vector-store client.
//
// Current M07 contract: POST /vectors with semantic content; M07 generates
// the embeddings itself (LiteLLM gateway). Callers forward identity headers
// so writes stay tenant-scoped.
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

// VectorItem mirrors Module 07's vector-write item (subset we use).
type VectorItem struct {
	DocumentID      string         `json:"document_id"`
	EmbeddingType   string         `json:"embedding_type"`
	SemanticContent string         `json:"semantic_content"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type m07StoreRequest struct {
	TenantID string       `json:"tenant_id"`
	Items    []VectorItem `json:"items"`
}

// StoreVectors ingests knowledge chunks; M07 embeds them server-side, so
// this can legitimately take tens of seconds for large batches.
func (c *M07Client) StoreVectors(ctx context.Context, jwtToken, tenantID string, items []VectorItem) error {
	reqBody, err := json.Marshal(m07StoreRequest{TenantID: tenantID, Items: items})
	if err != nil {
		return fmt.Errorf("marshal store request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/vectors", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build store request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}
	req.Header.Set("X-Tenant-ID", tenantID)

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("store vectors: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("store vectors: M07 returned %d", resp.StatusCode)
	}
	return nil
}
