package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RESTConnector handles generic REST API integration.
type RESTConnector struct{}

// Name returns the connector name.
func (c *RESTConnector) Name() string {
	return "Generic REST"
}

// Type returns the connector type.
func (c *RESTConnector) Type() string {
	return "generic_rest"
}

// ValidateConfig validates the REST configuration.
func (c *RESTConnector) ValidateConfig(config map[string]interface{}) error {
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
	allowedAuthTypes := []string{"api_key", "bearer", "basic", "none"}
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

// ValidateCredentials validates the REST credentials.
func (c *RESTConnector) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error) {
	authType, _ := credentials["auth_type"].(string)

	switch authType {
	case "api_key":
		if _, ok := credentials["api_key"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "api_key is required"}, nil
		}
	case "bearer":
		if _, ok := credentials["bearer_token"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "bearer_token is required"}, nil
		}
	case "basic":
		if _, ok := credentials["username"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "username is required"}, nil
		}
		if _, ok := credentials["password"].(string); !ok {
			return &HealthCheckResult{Healthy: false, Message: "password is required"}, nil
		}
	}

	// Test with a GET request to base_url
	baseURL, _ := credentials["base_url"].(string)
	if baseURL != "" && ctx.Err() == nil {
		url := baseURL
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

		switch authType {
		case "api_key":
			if apiKey, ok := credentials["api_key"].(string); ok {
				req.Header.Set("X-API-Key", apiKey)
			}
		case "bearer":
			if token, ok := credentials["bearer_token"].(string); ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "basic":
			if user, ok := credentials["username"].(string); ok {
				if pass, ok := credentials["password"].(string); ok {
					req.SetBasicAuth(user, pass)
				}
			}
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("connection failed: %v", err)}, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}, nil
		}
	}

	return &HealthCheckResult{Healthy: true, Message: "REST API connection valid"}, nil
}

// Sync performs a sync based on configured endpoints.
func (c *RESTConnector) Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error) {
	baseURL, _ := config["base_url"].(string)
	endpointsRaw, ok := config["endpoints"].([]interface{})
	authType, _ := credentials["auth_type"].(string)

	if baseURL == "" || !ok {
		return nil, fmt.Errorf("missing base_url or endpoints config")
	}

	result := &SyncResult{}
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

		path, _ := epMap["path"].(string)
		label, _ := epMap["label"].(string)
		method, _ := epMap["method"].(string)
		if path == "" {
			result.Errors = append(result.Errors, "endpoint missing path")
			result.ObjectsFailed++
			continue
		}
		if method == "" {
			method = "GET"
		}

		url := baseURL + path
		var body io.Reader
		if method == "POST" || method == "PUT" {
			if data, ok := epMap["request_body"].(string); ok {
				body = strings.NewReader(data)
			}
		}

		req, _ := http.NewRequestWithContext(ctx, method, url, body)
		req.Header.Set("Content-Type", "application/json")

		// Apply auth
		switch authType {
		case "api_key":
			if apiKey, ok := credentials["api_key"].(string); ok {
				req.Header.Set("X-API-Key", apiKey)
			}
		case "bearer":
			if token, ok := credentials["bearer_token"].(string); ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "basic":
			if user, ok := credentials["username"].(string); ok {
				if pass, ok := credentials["password"].(string); ok {
					req.SetBasicAuth(user, pass)
				}
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
		resBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(resBody, &respBody); err == nil {
			if records, ok := respBody["records"]; ok {
				if recArr, ok := records.([]interface{}); ok {
					result.ObjectsFetched += len(recArr)
					result.ObjectsUpdated += len(recArr)
				}
			} else if _, ok := respBody["data"]; ok {
				if dataArr, ok := respBody["data"].([]interface{}); ok {
					result.ObjectsFetched += len(dataArr)
					result.ObjectsUpdated += len(dataArr)
				} else {
					result.ObjectsFetched++
					result.ObjectsUpdated++
				}
			} else {
				result.ObjectsFetched++
				result.ObjectsUpdated++
			}
		}
	}

	return result, nil
}

// GetTools returns the tool definitions for generic REST operations.
func (c *RESTConnector) GetTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "rest_query_endpoint",
			Description: "Query a configured REST endpoint",
			Parameters: map[string]interface{}{
				"endpoint": map[string]string{"type": "string", "description": "Endpoint path"},
				"params":   map[string]string{"type": "object", "description": "Query parameters"},
			},
			Returns: map[string]interface{}{
				"data":    map[string]string{"type": "object", "description": "Response data"},
				"status":  map[string]string{"type": "integer", "description": "HTTP status code"},
			},
		},
		{
			Name:        "rest_create_resource",
			Description: "Create a resource via POST to a configured REST endpoint",
			Parameters: map[string]interface{}{
				"endpoint": map[string]string{"type": "string", "description": "Endpoint path"},
				"data":     map[string]string{"type": "object", "description": "Request body"},
			},
			Returns: map[string]interface{}{
				"id":     map[string]string{"type": "string", "description": "Created resource ID"},
				"status": map[string]string{"type": "integer", "description": "HTTP status code"},
			},
		},
		{
			Name:        "rest_update_resource",
			Description: "Update a resource via PUT/PATCH to a configured REST endpoint",
			Parameters: map[string]interface{}{
				"endpoint": map[string]string{"type": "string", "description": "Endpoint path"},
				"data":     map[string]string{"type": "object", "description": "Fields to update"},
			},
			Returns: map[string]interface{}{
				"success": map[string]string{"type": "boolean", "description": "Update success"},
			},
		},
		{
			Name:        "rest_delete_resource",
			Description: "Delete a resource via DELETE to a configured REST endpoint",
			Parameters: map[string]interface{}{
				"endpoint": map[string]string{"type": "string", "description": "Endpoint path"},
			},
			Returns: map[string]interface{}{
				"success": map[string]string{"type": "boolean", "description": "Delete success"},
			},
		},
	}
}