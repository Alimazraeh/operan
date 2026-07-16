package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// AzureOpenAIAdapter implements ProviderAdapter for Azure OpenAI Service.
// Azure OpenAI uses a different URL structure: {base_url}/openai/deployments/{deployment}/chat/completions?api-version={version}
type AzureOpenAIAdapter struct {
	cfg       ProviderConfig
	APIVersion string
}

// NewAzureOpenAIAdapter creates a new Azure OpenAI adapter.
func NewAzureOpenAIAdapter(cfg ProviderConfig) *AzureOpenAIAdapter {
	return &AzureOpenAIAdapter{
		cfg:        cfg,
		APIVersion: "2024-02-01",
	}
}

// Chat forwards the request to Azure OpenAI's chat completions endpoint.
func (a *AzureOpenAIAdapter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Azure uses the deployment name (model_name in registry) as the deployment ID.
	deploymentName := req.Model

	// Build the URL with api-version query parameter.
	baseURL, err := url.Parse(a.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("azure parse base URL: %w", err)
	}
	path := fmt.Sprintf("/openai/deployments/%s/chat/completions?api-version=%s", deploymentName, a.APIVersion)
	fullURL := baseURL.ResolveReference(&url.URL{Path: path}).String()

	// Build request body.
	body := map[string]any{
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if len(req.Stop) > 0 {
		body["stop"] = req.Stop
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("azure marshal request: %w", err)
	}

	reqObj, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("azure create request: %w", err)
	}
	reqObj.Header.Set("Content-Type", "application/json")
	// Azure uses "api-key" instead of "Bearer" for the header.
	reqObj.Header.Set("api-key", a.cfg.APIKey)

	resp, err := httpClient.Do(reqObj)
	if err != nil {
		return nil, fmt.Errorf("azure HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("azure error (%d): %s [%s]", resp.StatusCode, apiErr.Message, apiErr.Type)
		}
		return nil, fmt.Errorf("azure error (%d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("azure unmarshal response: %w, body: %s", err, string(respBody))
	}
	return &chatResp, nil
}

// Embeddings sends an embedding request to Azure OpenAI.
func (a *AzureOpenAIAdapter) Embeddings(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	deploymentName := req.Model

	baseURL, err := url.Parse(a.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("azure parse base URL: %w", err)
	}
	path := fmt.Sprintf("/openai/deployments/%s/embeddings?api-version=%s", deploymentName, a.APIVersion)
	fullURL := baseURL.ResolveReference(&url.URL{Path: path}).String()

	body := map[string]any{
		"input": req.Input,
	}
	if req.Format != "" {
		body["encoding_format"] = req.Format
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("azure marshal embeddings request: %w", err)
	}

	reqObj, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("azure create embeddings request: %w", err)
	}
	reqObj.Header.Set("Content-Type", "application/json")
	reqObj.Header.Set("api-key", a.cfg.APIKey)

	resp, err := httpClient.Do(reqObj)
	if err != nil {
		return nil, fmt.Errorf("azure embeddings HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure read embeddings response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("azure embeddings error (%d): %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("azure embeddings error (%d): %s", resp.StatusCode, string(respBody))
	}

	var embedResp EmbedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("azure unmarshal embeddings response: %w", err)
	}
	return &embedResp, nil
}

// HealthCheck verifies Azure OpenAI is reachable.
func (a *AzureOpenAIAdapter) HealthCheck(ctx context.Context) error {
	// Try to list deployments as a health check.
	baseURL, err := url.Parse(a.cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("azure parse base URL: %w", err)
	}
	path := fmt.Sprintf("/openai/models?api-version=%s", a.APIVersion)
	fullURL := baseURL.ResolveReference(&url.URL{Path: path}).String()

	reqObj, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("azure health check request: %w", err)
	}
	reqObj.Header.Set("api-key", a.cfg.APIKey)

	resp, err := httpClient.Do(reqObj)
	if err != nil {
		return fmt.Errorf("azure health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("azure health check returned status %d", resp.StatusCode)
	}
	return nil
}