package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/operan/model-abstraction/internal/adapters"
	"github.com/operan/model-abstraction/internal/config"
	"github.com/operan/model-abstraction/internal/store"
)

// WriteJSON is a helper that writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeJSON is an alias for WriteJSON (kept for internal use).
var writeJSON = WriteJSON

// buildProviderAdapter creates the appropriate provider adapter from a DB record.
//
// Secret Resolution: Provider API keys are currently loaded from the PROVIDER_API_KEYS
// environment variable (a JSON map keyed by provider name). In production, this should
// be wired to a secret manager (e.g., HashiCorp Vault, AWS Secrets Manager) using the
// api_key_secret_name field in model_providers. The field is reserved for that integration.
func buildProviderAdapter(p *store.ModelProvider, cfg *config.Config) (adapters.ProviderAdapter, error) {
	apiKey, ok := cfg.ProviderAPIKeys[p.Name]
	if !ok {
		return nil, fmt.Errorf("no API key configured for provider %q", p.Name)
	}

	provCfg := adapters.ProviderConfig{
		BaseURL:   p.BaseURL,
		APIKey:    apiKey,
		TimeoutMs: p.TimeoutMs,
	}

	switch p.Type {
	case "openai":
		return adapters.NewOpenAIAdapter(provCfg), nil
	case "anthropic":
		return adapters.NewAnthropicAdapter(provCfg), nil
	case "litellm":
		return adapters.NewLiteLLMAdapter(provCfg), nil
	case "ollama":
		return adapters.NewOllamaAdapter(provCfg), nil
	case "azure":
		return adapters.NewAzureOpenAIAdapter(provCfg), nil
	default:
		return nil, fmt.Errorf("unknown provider type %q", p.Type)
	}
}

// calcCost computes cost for a model call.
func calcCost(m *store.ModelRegistry, promptTokens, completionTokens int) float64 {
	promptCost := 0.0
	completionCost := 0.0
	if m.CostPerToken != nil {
		if pc, ok := m.CostPerToken["prompt"].(float64); ok {
			promptCost = pc
		}
		if cc, ok := m.CostPerToken["completion"].(float64); ok {
			completionCost = cc
		}
	}
	return float64(promptTokens)*promptCost + float64(completionTokens)*completionCost
}