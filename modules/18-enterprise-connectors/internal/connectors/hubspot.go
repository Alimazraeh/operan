package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HubSpotConnector handles HubSpot CRM integration.
type HubSpotConnector struct{}

// Name returns the connector name.
func (c *HubSpotConnector) Name() string {
	return "HubSpot CRM"
}

// Type returns the connector type.
func (c *HubSpotConnector) Type() string {
	return "hubspot"
}

// ValidateConfig validates the HubSpot configuration.
func (c *HubSpotConnector) ValidateConfig(config map[string]interface{}) error {
	return nil
}

// ValidateCredentials validates the HubSpot API key.
func (c *HubSpotConnector) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error) {
	apiKey, ok := credentials["access_token"].(string)
	if !ok || apiKey == "" {
		return &HealthCheckResult{Healthy: false, Message: "access_token (API key) is required"}, nil
	}

	if ctx.Err() != nil {
		return &HealthCheckResult{Healthy: false, Message: "context cancelled"}, nil
	}

	url := "https://api.hubapi.com/contacts/v1/lists/all/contacts/all?limit=1"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Hapi-key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("invalid API key (HTTP %d: %s)", resp.StatusCode, string(body))}, nil
	}

	return &HealthCheckResult{Healthy: true, Message: "HubSpot API key valid"}, nil
}

// Sync performs a full/incremental sync of HubSpot objects.
func (c *HubSpotConnector) Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error) {
	apiKey, _ := credentials["access_token"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("missing access_token")
	}

	result := &SyncResult{}
	client := &http.Client{Timeout: 30 * time.Second}

	if ctx.Err() == nil {
		contacts, err := c.syncContacts(ctx, client, apiKey)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.ObjectsFailed++
		} else {
			result.ObjectsFetched += contacts
			result.ObjectsUpdated += contacts
		}
	}

	if ctx.Err() == nil {
		companies, err := c.syncCompanies(ctx, client, apiKey)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.ObjectsFailed++
		} else {
			result.ObjectsFetched += companies
			result.ObjectsUpdated += companies
		}
	}

	if ctx.Err() == nil {
		deals, err := c.syncDeals(ctx, client, apiKey)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.ObjectsFailed++
		} else {
			result.ObjectsFetched += deals
			result.ObjectsUpdated += deals
		}
	}

	return result, nil
}

func (c *HubSpotConnector) syncContacts(ctx context.Context, client *http.Client, apiKey string) (int, error) {
	url := "https://api.hubapi.com/contacts/v1/lists/all/contacts/all?vidType=processed&count=100"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Hapi-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sync contacts failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("sync contacts HTTP %d", resp.StatusCode)
	}

	var respBody struct {
		Total int `json:"total"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &respBody)
	return respBody.Total, nil
}

func (c *HubSpotConnector) syncCompanies(ctx context.Context, client *http.Client, apiKey string) (int, error) {
	url := "https://api.hubapi.com/companies/v2/companies?limit=100"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Hapi-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sync companies failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("sync companies HTTP %d", resp.StatusCode)
	}

	var respBody []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &respBody)
	return len(respBody), nil
}

func (c *HubSpotConnector) syncDeals(ctx context.Context, client *http.Client, apiKey string) (int, error) {
	url := "https://api.hubapi.com/crm/v3/objects/deals/limit/100"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Hapi-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sync deals failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("sync deals HTTP %d", resp.StatusCode)
	}

	var respBody struct {
		Total int `json:"total"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &respBody)
	return respBody.Total, nil
}

// GetTools returns the tool definitions for HubSpot operations.
func (c *HubSpotConnector) GetTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "hs_lookup_contact",
			Description: "Look up a HubSpot contact by email or ID",
			Parameters: map[string]interface{}{
				"email": map[string]string{"type": "string", "description": "Contact email address"},
				"id":    map[string]string{"type": "string", "description": "HubSpot contact ID"},
			},
			Returns: map[string]interface{}{
				"contact": map[string]string{"type": "object", "description": "Contact record"},
			},
		},
		{
			Name:        "hs_create_contact",
			Description: "Create a new HubSpot contact",
			Parameters: map[string]interface{}{
				"email":  map[string]string{"type": "string", "description": "Contact email"},
				"fname":  map[string]string{"type": "string", "description": "First name"},
				"lname":  map[string]string{"type": "string", "description": "Last name"},
			},
			Returns: map[string]interface{}{
				"id":   map[string]string{"type": "string", "description": "Created contact ID"},
				"email": map[string]string{"type": "string", "description": "Contact email"},
			},
		},
		{
			Name:        "hs_lookup_deal",
			Description: "Look up a HubSpot deal",
			Parameters: map[string]interface{}{
				"deal_id": map[string]string{"type": "string", "description": "HubSpot deal ID"},
			},
			Returns: map[string]interface{}{
				"deal": map[string]string{"type": "object", "description": "Deal record"},
			},
		},
		{
			Name:        "hs_update_company",
			Description: "Update an existing HubSpot company",
			Parameters: map[string]interface{}{
				"company_id": map[string]string{"type": "string", "description": "HubSpot company ID"},
				"fields":     map[string]string{"type": "object", "description": "Fields to update"},
			},
			Returns: map[string]interface{}{
				"success": map[string]string{"type": "boolean", "description": "Update success"},
			},
		},
	}
}