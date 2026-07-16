package sync

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/operan/enterprise-connectors/internal/connectors"
	"github.com/operan/enterprise-connectors/internal/events"
	"github.com/operan/enterprise-connectors/internal/store"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func TestEngine_New(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)
	syncStore := store.NewSyncStore(pool)
	eventPub := events.NewPublisher("")
	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})

	engine := NewEngine(connStore, syncStore, nil, eventPub, registry)
	require.NotNil(t, engine)
}

func TestEngine_RunSync_UnknownConnector(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)
	syncStore := store.NewSyncStore(pool)
	eventPub := events.NewPublisher("")
	registry := connectors.NewRegistry()
	engine := NewEngine(connStore, syncStore, nil, eventPub, registry)

	unknownID := uuid.New()
	_, err = engine.RunSync(context.Background(), unknownID.String(), "full")
	require.Error(t, err)
}

func TestEngine_HealthCheck_UnknownConnector(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)
	syncStore := store.NewSyncStore(pool)
	eventPub := events.NewPublisher("")
	registry := connectors.NewRegistry()
	engine := NewEngine(connStore, syncStore, nil, eventPub, registry)

	_, err = engine.HealthCheck(context.Background(), uuid.New().String())
	require.Error(t, err)
}

func TestSyncResult_Struct(t *testing.T) {
	result := &connectors.SyncResult{
		ObjectsFetched: 42,
		ObjectsUpdated: 40,
		ObjectsFailed:  2,
		Errors:         []string{"error 1", "error 2"},
	}
	require.Equal(t, 42, result.ObjectsFetched)
	require.Equal(t, 40, result.ObjectsUpdated)
	require.Equal(t, 2, result.ObjectsFailed)
	require.Len(t, result.Errors, 2)
}

func TestHealthCheckResult_Struct(t *testing.T) {
	result := &connectors.HealthCheckResult{
		Healthy: true,
		Message: "connection valid",
	}
	require.True(t, result.Healthy)
	require.Equal(t, "connection valid", result.Message)
}

func TestHealthCheckResult_Healthy_False(t *testing.T) {
	result := &connectors.HealthCheckResult{
		Healthy: false,
		Message: "missing credentials",
	}
	require.False(t, result.Healthy)
}

func TestConnectorStore_ModelDefaults(t *testing.T) {
	c := store.Connector{
		TenantID: "tenant-1",
		Name:     "test",
	}
	require.Equal(t, uuid.Nil, c.ID)
	require.False(t, c.ToolsRegistered)
	require.Equal(t, "", c.SyncFrequency)
}

func TestConnectorStore_ValidateConnectorTypes(t *testing.T) {
	// All connector types from the DB CHECK constraint
	validTypes := []string{"smtp", "salesforce", "hubspot", "m365", "sap", "generic_rest", "sharepoint", "slack", "custom"}
	for _, ctype := range validTypes {
		c := store.Connector{
			TenantID:      "tenant-1",
			Name:          "test",
			ConnectorType: ctype,
		}
		require.Equal(t, ctype, c.ConnectorType)
	}
}

func ptrInt(n int) *int { return &n }

func TestSyncHistoryModel_WithCompletion(t *testing.T) {
	now := time.Now()
	sh := store.ConnectorSyncHistory{
		TenantID:       "tenant-1",
		ConnectorID:    uuid.New(),
		SyncType:       "full",
		Status:         "completed",
		ObjectsFetched: 100,
		ObjectsUpdated: 95,
		ObjectsFailed:  5,
		DurationMs:     ptrInt(2500),
		StartedAt:      now,
		CompletedAt:    &now,
	}
	require.Equal(t, "completed", sh.Status)
	require.Equal(t, 100, sh.ObjectsFetched)
	require.Equal(t, 5, sh.ObjectsFailed)
	require.Equal(t, 2500, *sh.DurationMs)
}

func TestSyncHistoryModel_WithError(t *testing.T) {
	errMsg := "connection timeout after 30s"
	sh := store.ConnectorSyncHistory{
		TenantID:    "tenant-1",
		ConnectorID: uuid.New(),
		SyncType:    "full",
		Status:      "error",
		ErrorMessage: &errMsg,
	}
	require.Equal(t, "error", sh.Status)
	require.Equal(t, "connection timeout after 30s", *sh.ErrorMessage)
}