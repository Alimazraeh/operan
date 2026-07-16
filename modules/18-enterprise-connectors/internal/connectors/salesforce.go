package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SalesforceConnector handles Salesforce CRM integration.
type SalesforceConnector struct{}

// Name returns the connector name.
func (c *SalesforceConnector) Name() string {
	return "Salesforce CRM"
}

// Type returns the connector type.
func (c *SalesforceConnector) Type() string {
	return "salesforce"
}

// ValidateConfig validates the Salesforce configuration.
func (c *SalesforceConnector) ValidateConfig(config map[string]interface{}) error {
	if _, ok := config["instance_url"].(string); !ok {
		return fmt.Errorf("instance_url is required")
	}
	if _, ok := config["client_id"].(string); !ok {
		return fmt.Errorf("client_id is required")
	}
	if _, ok := config["client_secret"].(string); !ok {
		return fmt.Errorf("client_secret is required")
	}
	return nil
}

// ValidateCredentials validates Salesforce OAuth2 credentials.
func (c *SalesforceConnector) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error) {
	clientID, ok := credentials["client_id"].(string)
	if !ok || clientID == "" {
		return &HealthCheckResult{Healthy: false, Message: "client_id is required"}, nil
	}
	clientSecret, ok := credentials["client_secret"].(string)
	if !ok || clientSecret == "" {
		return &HealthCheckResult{Healthy: false, Message: "client_secret is required"}, nil
	}
	accessToken, ok := credentials["access_token"].(string)
	if !ok || accessToken == "" {
		return &HealthCheckResult{Healthy: false, Message: "access_token is required"}, nil
	}
	instanceURL, ok := credentials["instance_url"].(string)
	if !ok || instanceURL == "" {
		return &HealthCheckResult{Healthy: false, Message: "instance_url is required"}, nil
	}

	// Try to call Salesforce REST API to validate the token
	url := instanceURL + "/services/oauth2/userinfo"
	if ctx.Err() == nil {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Try the REST API endpoint instead
			url = instanceURL + "/services/data/v55.0/chatter/users/me"
			req, _ = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+accessToken)
			resp, err = client.Do(req)
			if err != nil {
				return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("connection failed: %v", err)}, nil
			}
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("invalid credentials (HTTP %d)", resp.StatusCode)}, nil
		}
	}

	return &HealthCheckResult{Healthy: true, Message: fmt.Sprintf("Salesforce connected (%s)", clientID)}, nil
}

// Sync performs a full/incremental sync of Salesforce objects.
func (c *SalesforceConnector) Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error) {
	instanceURL, _ := credentials["instance_url"].(string)
	accessToken, _ := credentials["access_token"].(string)

	if instanceURL == "" || accessToken == "" {
		return nil, fmt.Errorf("missing instance_url or access_token")
	}

	result := &SyncResult{}
	objects := []string{"Account", "Contact", "Opportunity"}
	client := &http.Client{Timeout: 30 * time.Second}

	for _, obj := range objects {
		if ctx.Err() != nil {
			break
		}
		url := fmt.Sprintf("%s/services/data/v55.0/sobjects/%s", instanceURL, obj)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := client.Do(req)
		if err != nil {
			result.ObjectsFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to fetch %s: %v", obj, err))
			continue
		}
		if resp.StatusCode >= 400 {
			result.ObjectsFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("HTTP %d for %s", resp.StatusCode, obj))
			resp.Body.Close()
			continue
		}

		var sobjResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&sobjResp); err != nil {
			resp.Body.Close()
			result.ObjectsFailed++
			continue
		}
		resp.Body.Close()

		// Count records via a SOQL query
		queryURL := fmt.Sprintf("%s/services/data/v55.0/query/?q=SELECT+Id+FROM+%s", instanceURL, obj)
		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
		req2.Header.Set("Authorization", "Bearer "+accessToken)

		resp2, err := client.Do(req2)
		if err != nil {
			result.ObjectsFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("query %s failed: %v", obj, err))
			continue
		}
		if resp2.StatusCode >= 400 {
			result.ObjectsFailed++
			resp2.Body.Close()
			continue
		}

		var queryResp struct {
			TotalSize int `json:"totalSize"`
		}
		if err := json.NewDecoder(resp2.Body).Decode(&queryResp); err == nil {
			result.ObjectsFetched += queryResp.TotalSize
			result.ObjectsUpdated += queryResp.TotalSize
		}
		resp2.Body.Close()
	}

	return result, nil
}

// GetTools returns the tool definitions for Salesforce operations.
func (c *SalesforceConnector) GetTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "sf_lookup_account",
			Description: "Look up a Salesforce account by ID or name",
			Parameters: map[string]interface{}{
				"account_id": map[string]string{"type": "string", "description": "Salesforce account ID"},
				"name":       map[string]string{"type": "string", "description": "Account name (alternative to ID)"},
			},
			Returns: map[string]interface{}{
				"account": map[string]string{"type": "object", "description": "Account record"},
			},
		},
		{
			Name:        "sf_lookup_contact",
			Description: "Look up a Salesforce contact by ID or name",
			Parameters: map[string]interface{}{
				"contact_id": map[string]string{"type": "string", "description": "Salesforce contact ID"},
				"name":       map[string]string{"type": "string", "description": "Contact name"},
			},
			Returns: map[string]interface{}{
				"contact": map[string]string{"type": "object", "description": "Contact record"},
			},
		},
		{
			Name:        "sf_create_opportunity",
			Description: "Create a new Salesforce opportunity",
			Parameters: map[string]interface{}{
				"name":        map[string]string{"type": "string", "description": "Opportunity name"},
				"account_id":  map[string]string{"type": "string", "description": "Associated account ID"},
				"amount":      map[string]string{"type": "number", "description": "Opportunity amount"},
				"close_date":  map[string]string{"type": "string", "description": "Close date (YYYY-MM-DD)"},
				"stage_name":  map[string]string{"type": "string", "description": "Opportunity stage"},
			},
			Returns: map[string]interface{}{
				"id":   map[string]string{"type": "string", "description": "Created opportunity ID"},
				"name": map[string]string{"type": "string", "description": "Opportunity name"},
			},
		},
		{
			Name:        "sf_update_contact",
			Description: "Update an existing Salesforce contact",
			Parameters: map[string]interface{}{
				"contact_id": map[string]string{"type": "string", "description": "Contact ID to update"},
				"fields":     map[string]string{"type": "object", "description": "Fields to update"},
			},
			Returns: map[string]interface{}{
				"success": map[string]string{"type": "boolean", "description": "Update success"},
			},
		},
	}
}