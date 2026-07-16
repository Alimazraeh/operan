package connectors

import "testing"

func ptrInt(i int) *int { return &i }

func TestHubSpotConnector_GetTools(t *testing.T) {
	c := &HubSpotConnector{}
	tools := c.GetTools()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
}

func TestHubSpotConnector_Type(t *testing.T) {
	c := &HubSpotConnector{}
	if c.Type() != "hubspot" {
		t.Errorf("expected type 'hubspot', got '%s'", c.Type())
	}
}

func TestM365Connector_GetTools(t *testing.T) {
	c := &M365Connector{}
	tools := c.GetTools()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
}

func TestM365Connector_ValidateConfig_MissingTenantID(t *testing.T) {
	c := &M365Connector{}
	err := c.ValidateConfig(map[string]interface{}{
		"client_id": "test",
	})
	if err == nil {
		t.Error("expected error for missing tenant_id")
	}
}

func TestM365Connector_ValidateConfig_Valid(t *testing.T) {
	c := &M365Connector{}
	err := c.ValidateConfig(map[string]interface{}{
		"tenant_id": "tenant-123",
		"client_id": "test",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestM365Connector_Type(t *testing.T) {
	c := &M365Connector{}
	if c.Type() != "m365" {
		t.Errorf("expected type 'm365', got '%s'", c.Type())
	}
}

func TestSAPConnector_Type(t *testing.T) {
	c := &SAPConnector{}
	if c.Type() != "sap" {
		t.Errorf("expected type 'sap', got '%s'", c.Type())
	}
}

func TestSAPConnector_ValidateConfig_MissingBaseURL(t *testing.T) {
	c := &SAPConnector{}
	err := c.ValidateConfig(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing base_url")
	}
}

func TestSAPConnector_ValidateConfig_MissingEndpoints(t *testing.T) {
	c := &SAPConnector{}
	err := c.ValidateConfig(map[string]interface{}{
		"base_url":  "https://sap.example.com",
		"auth_type": "api_key",
	})
	if err == nil {
		t.Error("expected error for missing endpoints")
	}
}

func TestSAPConnector_ValidateConfig_Valid(t *testing.T) {
	c := &SAPConnector{}
	err := c.ValidateConfig(map[string]interface{}{
		"base_url":  "https://sap.example.com",
		"auth_type": "api_key",
		"endpoints": []interface{}{
			map[string]interface{}{"path": "/api/ERP", "label": "ERP"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSAPConnector_GetTools(t *testing.T) {
	c := &SAPConnector{}
	tools := c.GetTools()
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}
}

func TestRESTConnector_Type(t *testing.T) {
	c := &RESTConnector{}
	if c.Type() != "generic_rest" {
		t.Errorf("expected type 'generic_rest', got '%s'", c.Type())
	}
}

func TestRESTConnector_ValidateConfig_MissingBaseURL(t *testing.T) {
	c := &RESTConnector{}
	err := c.ValidateConfig(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing base_url")
	}
}

func TestRESTConnector_ValidateConfig_Valid(t *testing.T) {
	c := &RESTConnector{}
	err := c.ValidateConfig(map[string]interface{}{
		"base_url":  "https://api.example.com",
		"auth_type": "bearer",
		"endpoints": []interface{}{
			map[string]interface{}{"path": "/data", "label": "Data"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRESTConnector_GetTools(t *testing.T) {
	c := &RESTConnector{}
	tools := c.GetTools()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
}

func TestSalesforceConnector_GetTools(t *testing.T) {
	c := &SalesforceConnector{}
	tools := c.GetTools()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
}

func TestSalesforceConnector_Type(t *testing.T) {
	c := &SalesforceConnector{}
	if c.Type() != "salesforce" {
		t.Errorf("expected type 'salesforce', got '%s'", c.Type())
	}
}