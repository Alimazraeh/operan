package connectors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	// Create a mock connector
	mock := &mockConnector{connectorType: "test", name: "Test Connector"}
	registry.Register(mock)

	got, err := registry.Get("test")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "Test Connector", got.Name())
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&mockConnector{connectorType: "smtp", name: "SMTP"})
	registry.Register(&mockConnector{connectorType: "salesforce", name: "Salesforce"})
	registry.Register(&mockConnector{connectorType: "hubspot", name: "HubSpot"})

	types := registry.List()
	require.Len(t, types, 3)
	require.Contains(t, types, "smtp")
	require.Contains(t, types, "salesforce")
	require.Contains(t, types, "hubspot")
}

func TestRegistry_GetUnknownType(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockConnector{connectorType: "smtp"})

	_, err := registry.Get("unknown")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found in registry")
}

func TestRegistry_ListTools(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&mockConnector{
		connectorType: "smtp",
		name:          "SMTP",
		tools:         []ToolDefinition{{Name: "send_email", Description: "Send an email"}},
	})
	registry.Register(&mockConnector{
		connectorType: "salesforce",
		name:          "Salesforce",
		tools:         []ToolDefinition{{Name: "query_records", Description: "Query records"}},
	})

	allTools := registry.ListTools()
	require.Len(t, allTools, 2)
	names := make(map[string]bool)
	for _, t := range allTools {
		names[t.Name] = true
	}
	require.True(t, names["send_email"])
	require.True(t, names["query_records"])
}

func TestSMTP_Connector_ValidateConfig_MissingHost(t *testing.T) {
	conn := &SMTPConnector{}
	err := conn.ValidateConfig(map[string]interface{}{})
	require.Error(t, err)
	// ValidateConfig for SMTP requires host, port, from_address
	require.Contains(t, err.Error(), "host")
}

func TestSalesforceConnector_ValidateConfig_MissingClientID(t *testing.T) {
	conn := &SalesforceConnector{}
	// Missing instance_url, client_id, and client_secret
	err := conn.ValidateConfig(map[string]interface{}{})
	require.Error(t, err)
	// First missing field reported is instance_url, then client_id
	require.Contains(t, err.Error(), "instance_url")
}

// --- mock connector helpers ---

type mockConnector struct {
	connectorType string
	name          string
	healthResult  *HealthCheckResult
	syncResult    *SyncResult
	tools         []ToolDefinition
}

func (m *mockConnector) Name() string                  { return m.name }
func (m *mockConnector) Type() string                  { return m.connectorType }
func (m *mockConnector) ValidateConfig(_ map[string]interface{}) error { return nil }
func (m *mockConnector) ValidateCredentials(_ context.Context, _ map[string]interface{}) (*HealthCheckResult, error) {
	return m.healthResult, nil
}
func (m *mockConnector) Sync(_ context.Context, _, _ map[string]interface{}) (*SyncResult, error) {
	return m.syncResult, nil
}
func (m *mockConnector) GetTools() []ToolDefinition { return m.tools }