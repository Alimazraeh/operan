package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// M365Connector handles Microsoft 365 integration.
type M365Connector struct{}

// Name returns the connector name.
func (c *M365Connector) Name() string {
	return "Microsoft 365"
}

// Type returns the connector type.
func (c *M365Connector) Type() string {
	return "m365"
}

// ValidateConfig validates the M365 configuration.
func (c *M365Connector) ValidateConfig(config map[string]interface{}) error {
	if _, ok := config["tenant_id"].(string); !ok {
		return fmt.Errorf("tenant_id is required")
	}
	if _, ok := config["client_id"].(string); !ok {
		return fmt.Errorf("client_id is required")
	}
	return nil
}

// ValidateCredentials validates the M365 OAuth2 credentials.
func (c *M365Connector) ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error) {
	clientID, ok := credentials["client_id"].(string)
	if !ok || clientID == "" {
		return &HealthCheckResult{Healthy: false, Message: "client_id is required"}, nil
	}
	clientSecret, ok := credentials["client_secret"].(string)
	if !ok || clientSecret == "" {
		return &HealthCheckResult{Healthy: false, Message: "client_secret is required"}, nil
	}
	tenantID, ok := credentials["tenant_id"].(string)
	if !ok || tenantID == "" {
		return &HealthCheckResult{Healthy: false, Message: "tenant_id is required"}, nil
	}

	if ctx.Err() != nil {
		return &HealthCheckResult{Healthy: false, Message: "context cancelled"}, nil
	}

	// Validate by calling Microsoft Graph API
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(tokenURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &HealthCheckResult{Healthy: false, Message: fmt.Sprintf("invalid credentials (HTTP %d)", resp.StatusCode)}, nil
	}

	return &HealthCheckResult{Healthy: true, Message: fmt.Sprintf("M365 connected (tenant: %s)", tenantID)}, nil
}

// Sync performs a sync of M365 resources.
func (c *M365Connector) Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error) {
	tenantID, _ := credentials["tenant_id"].(string)
	clientID, _ := credentials["client_id"].(string)
	clientSecret, _ := credentials["client_secret"].(string)

	if tenantID == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("missing required credentials")
	}

	result := &SyncResult{}
	client := &http.Client{Timeout: 30 * time.Second}

	token, err := c.getAccessToken(ctx, client, tenantID, clientID, clientSecret)
	if err != nil {
		result.ObjectsFailed++
		result.Errors = append(result.Errors, fmt.Sprintf("failed to get access token: %v", err))
		return result, nil
	}

	if ctx.Err() == nil {
		events, err := c.syncCalendarEvents(ctx, client, token)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("calendar sync failed: %v", err))
			result.ObjectsFailed++
		} else {
			result.ObjectsFetched += events
			result.ObjectsUpdated += events
		}
	}

	if ctx.Err() == nil {
		files, err := c.syncSharePointFiles(ctx, client, token)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("SharePoint sync failed: %v", err))
			result.ObjectsFailed++
		} else {
			result.ObjectsFetched += files
			result.ObjectsUpdated += files
		}
	}

	return result, nil
}

func (c *M365Connector) getAccessToken(ctx context.Context, client *http.Client, tenantID, clientID, clientSecret string) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Post(tokenURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("token request failed: HTTP %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.AccessToken, nil
}

func (c *M365Connector) syncCalendarEvents(ctx context.Context, client *http.Client, token string) (int, error) {
	url := "https://graph.microsoft.com/v1.0/me/events?$top=100"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var respBody struct {
		Value []map[string]interface{} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return 0, err
	}
	return len(respBody.Value), nil
}

func (c *M365Connector) syncSharePointFiles(ctx context.Context, client *http.Client, token string) (int, error) {
	url := "https://graph.microsoft.com/v1.0/me/drive/root/children?$top=100"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var respBody struct {
		Value []map[string]interface{} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return 0, err
	}
	return len(respBody.Value), nil
}

// GetTools returns the tool definitions for M365 operations.
func (c *M365Connector) GetTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "m365_send_email",
			Description: "Send an email via Microsoft 365 Outlook",
			Parameters: map[string]interface{}{
				"to":      map[string]string{"type": "string", "description": "Recipient email address"},
				"subject": map[string]string{"type": "string", "description": "Email subject"},
				"body":    map[string]string{"type": "string", "description": "Email body"},
			},
			Returns: map[string]interface{}{
				"message_id": map[string]string{"type": "string", "description": "Outlook message ID"},
				"status":     map[string]string{"type": "string", "description": "Send status"},
			},
		},
		{
			Name:        "m365_create_calendar_event",
			Description: "Create a calendar event in Microsoft 365",
			Parameters: map[string]interface{}{
				"subject":   map[string]string{"type": "string", "description": "Event subject"},
				"start":     map[string]string{"type": "string", "description": "Start time (ISO 8601)"},
				"end":       map[string]string{"type": "string", "description": "End time (ISO 8601)"},
				"attendees": map[string]string{"type": "array", "description": "List of attendee email addresses"},
			},
			Returns: map[string]interface{}{
				"event_id": map[string]string{"type": "string", "description": "Event ID"},
			},
		},
		{
			Name:        "m365_list_sharepoint_files",
			Description: "List files in SharePoint/OneDrive",
			Parameters: map[string]interface{}{
				"path":    map[string]string{"type": "string", "description": "SharePoint folder path"},
				"max_age": map[string]string{"type": "integer", "description": "Max age in hours to filter"},
			},
			Returns: map[string]interface{}{
				"files": map[string]string{"type": "array", "description": "List of file records"},
			},
		},
		{
			Name:        "m365_read_sharepoint_doc",
			Description: "Read a document from SharePoint/OneDrive",
			Parameters: map[string]interface{}{
				"file_id": map[string]string{"type": "string", "description": "SharePoint file ID"},
			},
			Returns: map[string]interface{}{
				"content": map[string]string{"type": "string", "description": "File content"},
			},
		},
	}
}