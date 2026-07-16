package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/operan/enterprise-connectors/internal/clients"
	"github.com/operan/enterprise-connectors/internal/connectors"
	"github.com/operan/enterprise-connectors/internal/events"
	"github.com/operan/enterprise-connectors/internal/store"
)

// Engine handles sync execution for connectors.
type Engine struct {
	connectorStore *store.ConnectorStore
	syncStore      *store.SyncStore
	m04Client      *clients.M04Client
	eventPub       *events.Publisher
	registry       *connectors.Registry
}

// NewEngine creates a new sync engine.
func NewEngine(connectorStore *store.ConnectorStore, syncStore *store.SyncStore,
	m04Client *clients.M04Client, eventPub *events.Publisher,
	registry *connectors.Registry,
) *Engine {
	return &Engine{
		connectorStore: connectorStore,
		syncStore:      syncStore,
		m04Client:      m04Client,
		eventPub:       eventPub,
		registry:       registry,
	}
}

// RunSync executes a sync operation for a connector.
func (e *Engine) RunSync(ctx context.Context, connectorID string, syncType string) (*connectors.SyncResult, error) {
	if syncType == "" {
		syncType = "full"
	}

	conn, err := e.connectorStore.GetByID(ctx, uuid.MustParse(connectorID), "")
	if err != nil {
		return nil, fmt.Errorf("connector not found: %w", err)
	}

	// Create sync history record
	startedAt := time.Now()
	syncRecord := &store.ConnectorSyncHistory{
		TenantID:    conn.TenantID,
		ConnectorID: conn.ID,
		SyncType:    syncType,
		Status:      "running",
		StartedAt:   startedAt,
	}
	if err := e.syncStore.Create(ctx, syncRecord); err != nil {
		return nil, fmt.Errorf("create sync record: %w", err)
	}

	// Publish sync started event
	e.eventPub.PublishSyncStarted(conn.TenantID, conn.ID, conn.ConnectorType, syncType)

	// Get connector implementation
	connImpl, err := e.registry.Get(conn.ConnectorType)
	if err != nil {
		e.syncStore.UpdateError(ctx, syncRecord.ID, err.Error())
		e.connectorStore.UpdateStatus(ctx, conn.ID, conn.TenantID,
			"error", nil, errPtr("error"), errPtr(err.Error()))
		e.eventPub.PublishSyncFailed(conn.TenantID, conn.ID, conn.ConnectorType, err.Error())
		return nil, err
	}

	// Perform sync
	result, err := connImpl.Sync(ctx, conn.Credentials, conn.Config)
	if err != nil {
		completedAt := time.Now()
		e.syncStore.UpdateError(ctx, syncRecord.ID, err.Error())
		e.connectorStore.UpdateStatus(ctx, conn.ID, conn.TenantID,
			"error", &completedAt, errPtr("error"), errPtr(err.Error()))
		e.eventPub.PublishSyncFailed(conn.TenantID, conn.ID, conn.ConnectorType, err.Error())
		return nil, err
	}

	// Record sync completion
	completedAt := time.Now()
	durationMs := int(time.Since(startedAt).Milliseconds())
	e.syncStore.UpdateComplete(ctx, syncRecord.ID,
		result.ObjectsFetched, result.ObjectsUpdated, result.ObjectsFailed,
		durationMs, completedAt)

	// Update connector status
	status := "active"
	if result.ObjectsFailed > 0 {
		status = "error"
	}
	e.connectorStore.UpdateStatus(ctx, conn.ID, conn.TenantID,
		status, &completedAt, errPtr(status), nil)

	// Publish sync completed event
	e.eventPub.PublishSyncCompleted(conn.TenantID, conn.ID, conn.ConnectorType,
		result.ObjectsFetched, result.ObjectsUpdated, result.ObjectsFailed, durationMs)

	// Register tools with M04 if not already registered
	if !conn.ToolsRegistered {
		tools := connImpl.GetTools()
		regErr := e.m04Client.RegisterTools(ctx, conn.TenantID, tools)
		if regErr == nil {
			e.connectorStore.UpdateToolsRegistered(ctx, conn.ID, conn.TenantID, true)
			e.eventPub.PublishToolsRegistered(conn.TenantID, conn.ID, len(tools))
		}
	}

	return result, nil
}

// HealthCheck performs a health check for a connector.
func (e *Engine) HealthCheck(ctx context.Context, connectorID string) (*connectors.HealthCheckResult, error) {
	conn, err := e.connectorStore.GetByID(ctx, uuid.MustParse(connectorID), "")
	if err != nil {
		return nil, fmt.Errorf("connector not found: %w", err)
	}

	connImpl, err := e.registry.Get(conn.ConnectorType)
	if err != nil {
		return nil, err
	}

	return connImpl.ValidateCredentials(ctx, conn.Credentials)
}

func errPtr(s string) *string {
	return &s
}