package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SAPConnector handles SAP integration via generic REST API.
type SAPConnector struct{}

// Name returns the connector name.
func (c *SAPConnector) Name() string {
	return "SAP (REST)"
}

// Type returns the connector type.
func (c *SAPConnector) Type() string {
	return "sap"
}

// ValidateConfig validates the SAP configuration.
func (c *SAPConnector) ValidateConfig(config map[string]interface{}) error {
	if _, ok := config["base_url"].(string); !ok {
		return fmt.Errorf("base_url is required")
	}
	endpoints, ok := config["endpoints"].([]interface{})
	if !ok || len(endpoints) == 0 {
		return fmt.Errorf("at least one endpoint is required")
	}
	authType, ok := config["auth_type"].(string)
	if !ok || authType == "" {
		return fmt.Errorf("auth_type is required")
	}
	allowedAuthTypes := []string{"api_key", "basic", "saml", "oauth2"}
	valid := false
	for _, at := range allowedAuthTypes {
		if authType == at {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("auth_type must be one of: %v", allowedAuthTypes)
	}
	return nil
}

// ValidateCredentials validates the SAP credentials.
func (c *SAPConnector) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error) {
	authType, _ := credentials["auth_type"].(string)

	switch authType {
	case "api_key":
		if _, ok := credentials["api_key"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "api_key is required"}, nil
		}
	case "basic":
		if _, ok := credentials["username"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "username is required"}, nil
		}
		if _, ok := credentials["password"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "password is required"}, nil
		}
	case "oauth2":
		if _, ok := credentials["client_id"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "client_id is required"}, nil
		}
		if _, ok := credentials["client_secret"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "client_secret is required"}, nil
		}
	case "saml":
		// SAML is handled differently; just check that we have an assertion
		if _, ok := credentials["saml_assertion"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "saml_assertion is required"}, nil
		}
	default:
		return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("unknown auth_type: %s", authType)}, nil
	}

	return &HealthCheckResult{Healthy: true, Message: fmt.Sprintf("SAP credentials valid (%s auth)", authType)}, nil
}

// Sync performs a configurable sync based on the defined endpoints.
func (c *SAPConnector) Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error) {
	baseURL, _ := config["base_url"].(string)
	endpointsRaw, ok := config["endpoints"].([]interface{})
	if !ok || len(endpointsRaw) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}

	result := &SyncResult{}
	authType, _ := credentials["auth_type"].(string)

	client := &http.Client{Timeout: 30 * time.Second}

	for _, epRaw := range endpointsRaw {
		if ctx.Err() != nil {
			break
		}
		epMap, ok := epRaw.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "invalid endpoint format")
			result.ObjectsFailed++
			continue
		}

		endpoint, _ := epMap["path"].(string)
		label, _ := epMap["label"].(string)
		if endpoint == "" {
			result.Errors = append(result.Errors, "endpoint missing path")
			result.ObjectsFailed++
			continue
		}

		url := baseURL + endpoint
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

		// Apply auth
		switch authType {
		case "api_key":
			if apiKey, ok := credentials["api_key"].(string); ok {
				req.Header.Set("x-api-key", apiKey)
			}
		case "basic":
			if user, ok := credentials["username"].(string); ok {
				if pass, ok := credentials["password"].(string); ok {
					req.SetBasicAuth(user, pass)
				}
			}
		case "oauth2":
			if token, ok := credentials["access_token"].(string); ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "saml":
			if assertion, ok := credentials["saml_assertion"].(string); ok {
				req.Header.Set("Authorization", "SAML "+assertion)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] request failed: %v", label, err))
			result.ObjectsFailed++
			continue
		}
		if resp.StatusCode >= 400 {
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] HTTP %d", label, resp.StatusCode))
			resp.Body.Close()
			result.ObjectsFailed++
			continue
		}

		var respBody map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(body, &respBody); err == nil {
			// Count records in response
			if records, ok := respBody["records"]; ok {
				if recArr, ok := records.([]interface{}); ok {
					result.ObjectsFetched += len(recArr)
					result.ObjectsUpdated += len(recArr)
				}
			} else {
				result.ObjectsFetched++
				result.ObjectsUpdated++
			}
		}
	}

	return result, nil
}

// GetTools returns the tool definitions for SAP operations.
func (c *SAPConnector) GetTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "sap_query_erp_data",
			Description: "Query ERP data from SAP via configured REST endpoint",
			Parameters: map[string]interface{}{
				"endpoint": map[string]string{"type": "string", "description": "SAP endpoint path"},
				"params":   map[string]string{"type": "object", "description": "Query parameters"},
			},
			Returns: map[string]interface{}{
				"records": map[string]string{"type": "array", "description": "Query result records"},
			},
		},
		{
			Name:        "sap_create_business_object",
			Description: "Create a business object in SAP (e.g., sales order, purchase order)",
			Parameters: map[string]interface{}{
				"object_type": map[string]string{"type": "string", "description": "SAP object type"},
				"data":        map[string]string{"type": "object", "description": "Object data"},
			},
			Returns: map[string]interface{}{
				"id":    map[string]string{"type": "string", "description": "Created object ID"},
				"error": map[string]string{"type": "string", "description": "Error message if failed"},
			},
		},
		{
			Name:        "sap_update_business_object",
			Description: "Update an existing business object in SAP",
			Parameters: map[string]interface{}{
				"object_id":  map[string]string{"type": "string", "description": "SAP object ID"},
				"object_type": map[string]string{"type": "string", "description": "SAP object type"},
				"data":       map[string]string{"type": "object", "description": "Fields to update"},
			},
			Returns: map[string]interface{}{
				"success": map[string]string{"type": "boolean", "description": "Update success"},
			},
		},
	}
}