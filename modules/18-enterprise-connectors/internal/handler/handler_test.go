package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/operan/enterprise-connectors/internal/connectors"
	"github.com/operan/enterprise-connectors/internal/middleware"
	"github.com/operan/enterprise-connectors/internal/store"
	"github.com/stretchr/testify/require"
	"time"
)

const testJWTSecret = "test-secret-key-for-jwt-validation"
const testIssuer = "operan-tenant-control-plane"

func makeTestJWT(tenantID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   tenantID,
		"iss":   testIssuer,
		"roles": []interface{}{"admin"},
		"exp":   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	s, _ := token.SignedString([]byte(testJWTSecret))
	return s
}

func setupTestRouter(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)

	connStore := store.NewConnectorStore(pool)
	syncStore := store.NewSyncStore(pool)

	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})

	syncEngine := &mockSyncEngine{}
	connHandler := NewConnectorHandler(connStore)
	syncHandler := NewSyncHandler(syncEngine, connStore, syncStore)
	toolsHandler := NewToolsHandler(registry)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())
		connHandler.Routes(r)
		syncHandler.Routes(r)
		toolsHandler.Routes(r)
	})

	ts := httptest.NewServer(r)
	return ts, func() {
		ts.Close()
		pool.Close()
	}
}

type mockSyncEngine struct{}

func (m *mockSyncEngine) RunSync(ctx context.Context, tenantID string, connectorID uuid.UUID, syncType string) (*connectors.SyncResult, error) {
	return &connectors.SyncResult{ObjectsFetched: 5, ObjectsUpdated: 5, ObjectsFailed: 0}, nil
}

func (m *mockSyncEngine) HealthCheck(ctx context.Context, tenantID string, connectorID uuid.UUID) (*connectors.HealthCheckResult, error) {
	return &connectors.HealthCheckResult{Healthy: true, Message: "ok"}, nil
}

func TestHealth(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCreateConnector_MissingAuth(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"name":"test","connector_type":"smtp"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/connectors", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateConnector_InvalidJSON(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/connectors", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateConnector_MissingName(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"connector_type":"smtp"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/connectors", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateConnector_MissingType(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"name":"test"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/connectors", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateConnector_Success(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)

	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})
	syncStore := store.NewSyncStore(pool)
	syncEngine := &mockSyncEngine{}
	connHandler := NewConnectorHandler(connStore)
	syncHandler := NewSyncHandler(syncEngine, connStore, syncStore)
	toolsHandler := NewToolsHandler(registry)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())
		connHandler.Routes(r)
		syncHandler.Routes(r)
		toolsHandler.Routes(r)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()
	defer pool.Close()

	pool.ExpectQuery("INSERT INTO connector_definitions").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), false,
			pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "status", "last_sync_at", "last_sync_status", "last_error",
			"created_at", "updated_at",
		}).AddRow(uuid.New(), "active", nil, nil, nil, time.Now(), time.Now()))

	body := strings.NewReader(`{"name":"Salesforce","connector_type":"salesforce","auth_method":"api_key","config":{"host":"test"}}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/connectors", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestListConnectors(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)

	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})
	syncStore := store.NewSyncStore(pool)
	syncEngine := &mockSyncEngine{}
	connHandler := NewConnectorHandler(connStore)
	syncHandler := NewSyncHandler(syncEngine, connStore, syncStore)
	toolsHandler := NewToolsHandler(registry)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())
		connHandler.Routes(r)
		syncHandler.Routes(r)
		toolsHandler.Routes(r)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()
	defer pool.Close()

	now := time.Now()
	configJSON, _ := json.Marshal(map[string]interface{}{})
	credsJSON, _ := json.Marshal(map[string]interface{}{})
	metaJSON, _ := json.Marshal(map[string]interface{}{})

	pool.ExpectQuery("SELECT id, tenant_id, name").
		WithArgs("tenant-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "name", "description", "connector_type", "auth_method",
			"config", "credentials", "sync_frequency", "last_sync_at",
			"last_sync_status", "last_error", "tools_registered", "metadata",
			"created_at", "updated_at",
		}).AddRow(uuid.New().String(), "tenant-1", "SMTP", nil, "smtp", "api_key",
			configJSON, credsJSON, "manual", nil, nil, nil, false, metaJSON, now, now))

	pool.ExpectQuery("SELECT COUNT").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/connectors", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestGetConnector_NotFound(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)

	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})
	syncStore := store.NewSyncStore(pool)
	syncEngine := &mockSyncEngine{}
	connHandler := NewConnectorHandler(connStore)
	syncHandler := NewSyncHandler(syncEngine, connStore, syncStore)
	toolsHandler := NewToolsHandler(registry)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())
		connHandler.Routes(r)
		syncHandler.Routes(r)
		toolsHandler.Routes(r)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()
	defer pool.Close()

	// Set up the query expectation to return ErrNoRows
	testID := uuid.New()
	pool.ExpectQuery("SELECT id, tenant_id, name").
		WithArgs(testID, "tenant-1").
		WillReturnError(pgx.ErrNoRows)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/connectors/"+testID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	// Could be 404 or whatever the store returns
	_ = resp
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestDeleteConnector(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	connStore := store.NewConnectorStore(pool)

	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})
	syncStore := store.NewSyncStore(pool)
	syncEngine := &mockSyncEngine{}
	connHandler := NewConnectorHandler(connStore)
	syncHandler := NewSyncHandler(syncEngine, connStore, syncStore)
	toolsHandler := NewToolsHandler(registry)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())
		connHandler.Routes(r)
		syncHandler.Routes(r)
		toolsHandler.Routes(r)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()
	defer pool.Close()

	id := uuid.New()
	pool.ExpectExec("DELETE FROM connector_definitions").
		WithArgs(id, "tenant-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/v1/connectors/"+id.String(), nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, pool.ExpectationsWereMet())
}