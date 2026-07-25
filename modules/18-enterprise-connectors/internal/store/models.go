package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxPool is the interface wrapping pgxpool.Pool for testability.
type PgxPool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Close()
}

// Connector represents a row in the connector_definitions table.
type Connector struct {
	ID            uuid.UUID              `json:"id"`
	TenantID      string                 `json:"tenant_id"`
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	ConnectorType string                 `json:"connector_type"`
	Status        string                 `json:"status"`
	AuthMethod    string                 `json:"auth_method"`
	Config        map[string]interface{} `json:"config"`
	// Credentials never leave the process in a response body. Persistence
	// marshals this map directly (see store.Create/scan), and the sync engine
	// reads the field, so `json:"-"` costs nothing and closes the leak:
	// GET /v1/connectors previously returned every stored client_secret,
	// password, access_token and api_key in cleartext.
	Credentials     map[string]interface{} `json:"-"`
	SyncFrequency   string                 `json:"sync_frequency"`
	LastSyncAt      *time.Time             `json:"last_sync_at,omitempty"`
	LastSyncStatus  *string                `json:"last_sync_status,omitempty"`
	LastError       *string                `json:"last_error,omitempty"`
	ToolsRegistered bool                   `json:"tools_registered"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ConnectorSyncHistory represents a row in the connector_sync_history table.
type ConnectorSyncHistory struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       string     `json:"tenant_id"`
	ConnectorID    uuid.UUID  `json:"connector_id"`
	SyncType       string     `json:"sync_type"`
	Status         string     `json:"status"`
	ObjectsFetched int        `json:"objects_fetched"`
	ObjectsUpdated int        `json:"objects_updated"`
	ObjectsFailed  int        `json:"objects_failed"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	DurationMs     *int       `json:"duration_ms,omitempty"`
}
