package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func ptrString(s string) *string { return &s }

func TestCreateConnector(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	now := time.Now()

	pool.ExpectQuery("INSERT INTO connector_definitions").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg(), "salesforce", "oauth2",
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), false, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "status", "last_sync_at", "last_sync_status", "last_error",
			"created_at", "updated_at",
		}).AddRow(id, "inactive", nil, nil, nil, now, now))

	conn := &Connector{
		TenantID:      "tenant-1",
		Name:          "Salesforce",
		ConnectorType: "salesforce",
		AuthMethod:    "oauth2",
		Config:        map[string]interface{}{"instance_url": "https://test"},
	}
	err = store.Create(context.Background(), conn)
	require.NoError(t, err)
	require.Equal(t, id, conn.ID)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestGetByID_Success(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	now := time.Now()
	configJSON, _ := json.Marshal(map[string]interface{}{"instance_url": "https://test"})
	credsJSON, _ := json.Marshal(map[string]interface{}{})
	metaJSON, _ := json.Marshal(map[string]interface{}{})

	pool.ExpectQuery("SELECT id, tenant_id, name").
		WithArgs(id, "tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "connector_type", "auth_method",
			"config", "credentials", "sync_frequency", "last_sync_at",
			"last_sync_status", "last_error", "tools_registered", "metadata",
			"created_at", "updated_at",
		}).AddRow(id, "tenant-1", "Salesforce", nil, "salesforce", "oauth2",
			configJSON, credsJSON, "manual", nil, nil, nil, false, metaJSON, now, now))

	conn, err := store.GetByID(context.Background(), id, "tenant-1")
	require.NoError(t, err)
	require.Equal(t, id, conn.ID)
	require.Equal(t, "Salesforce", conn.Name)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestGetByID_NotFound(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	pool.ExpectQuery("SELECT id, tenant_id, name").
		WithArgs(id, "tenant-1").
		WillReturnError(pgx.ErrNoRows)

	_, err = store.GetByID(context.Background(), id, "tenant-1")
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestListConnectors(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	now := time.Now()
	configJSON, _ := json.Marshal(map[string]interface{}{"host": "smtp.test.com"})
	credsJSON, _ := json.Marshal(map[string]interface{}{})
	metaJSON, _ := json.Marshal(map[string]interface{}{})

	// LIST query runs first
	pool.ExpectQuery("SELECT id, tenant_id, name").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "connector_type", "auth_method",
			"config", "credentials", "sync_frequency", "last_sync_at",
			"last_sync_status", "last_error", "tools_registered", "metadata",
			"created_at", "updated_at",
		}).AddRow(id, "tenant-1", "SMTP", nil, "smtp", "api_key",
			configJSON, credsJSON, "manual", nil, nil, nil, false, metaJSON, now, now))

	// COUNT query runs second
	pool.ExpectQuery("SELECT COUNT.*FROM connector_definitions").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	connectors, total, err := store.List(context.Background(), "tenant-1", "", "", 1, 20)
	require.NoError(t, err)
	require.Len(t, connectors, 1)
	require.Equal(t, 1, total)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestListConnectorsWithTypeFilter(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	now := time.Now()
	configJSON, _ := json.Marshal(map[string]interface{}{})
	credsJSON, _ := json.Marshal(map[string]interface{}{})
	metaJSON, _ := json.Marshal(map[string]interface{}{})

	// LIST query runs first
	pool.ExpectQuery("SELECT id, tenant_id, name").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "connector_type", "auth_method",
			"config", "credentials", "sync_frequency", "last_sync_at",
			"last_sync_status", "last_error", "tools_registered", "metadata",
			"created_at", "updated_at",
		}).AddRow(id, "tenant-1", "SF", nil, "salesforce", "oauth2",
			configJSON, credsJSON, "manual", nil, nil, nil, false, metaJSON, now, now))

	// COUNT query runs second
	pool.ExpectQuery("SELECT COUNT.*AND connector_type").
		WithArgs("tenant-1", "salesforce").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	connectors, total, err := store.List(context.Background(), "tenant-1", "salesforce", "", 1, 20)
	require.NoError(t, err)
	require.Len(t, connectors, 1)
	require.Equal(t, 1, total)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestUpdateStatus(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	now := time.Now()
	statusStr := "success"

	pool.ExpectExec("UPDATE connector_definitions SET status").
		WithArgs("active", &now, &statusStr, pgxmock.AnyArg(), id, "tenant-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = store.UpdateStatus(context.Background(), id, "tenant-1", "active", &now, &statusStr, nil)
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestUpdateToolsRegistered(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	pool.ExpectExec("UPDATE connector_definitions SET tools_registered").
		WithArgs(true, id, "tenant-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = store.UpdateToolsRegistered(context.Background(), id, "tenant-1", true)
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestDeleteConnector(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	store := NewConnectorStore(pool)

	id := uuid.New()
	pool.ExpectExec("DELETE FROM connector_definitions").
		WithArgs(id, "tenant-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = store.Delete(context.Background(), id, "tenant-1")
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestConnector_ModelDefaults(t *testing.T) {
	c := Connector{TenantID: "tenant-1", Name: "test"}
	if c.ID != uuid.Nil {
		t.Errorf("expected zero UUID")
	}
	if c.Status != "" {
		t.Errorf("expected empty status")
	}
	if c.ToolsRegistered {
		t.Error("expected ToolsRegistered false")
	}
}

// A connector must never carry secret material into a response body.
// Regression guard: Credentials was `json:"credentials,omitempty"`, so
// GET /v1/connectors returned every client_secret and access_token in
// cleartext.
func TestConnectorJSONNeverIncludesCredentials(t *testing.T) {
	c := Connector{
		ID:            uuid.New(),
		TenantID:      "t1",
		Name:          "Jira Production",
		ConnectorType: "generic_rest",
		Config:        map[string]interface{}{"endpoint": "https://example.invalid"},
		Credentials: map[string]interface{}{
			"client_secret": "super-secret-value",
			"access_token":  "tok-should-not-leak",
			"password":      "hunter2",
		},
	}
	blob, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	body := string(blob)
	for _, forbidden := range []string{
		"super-secret-value", "tok-should-not-leak", "hunter2", "credentials",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("serialized connector leaks %q: %s", forbidden, body)
		}
	}
	// The non-secret fields must still be there.
	if !strings.Contains(body, "Jira Production") {
		t.Errorf("connector JSON lost its name: %s", body)
	}
}
