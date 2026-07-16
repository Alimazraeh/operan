package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/operan/execution-sandbox/internal/events"
	"github.com/operan/execution-sandbox/internal/middleware"
	"github.com/operan/execution-sandbox/internal/policies"
	"github.com/operan/execution-sandbox/internal/sandbox"
	"github.com/operan/execution-sandbox/internal/store"
	"github.com/stretchr/testify/require"
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

	profileStore := store.NewProfileStore(pool)
	instanceStore := store.NewInstanceStore(pool)
	executor, _ := sandbox.NewExecutor("/tmp/test-sandbox-" + t.Name())
	policyClient := policies.NewPolicyClient("http://127.0.0.1:1")
	eventPub := events.NewPublisher("")

	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	router.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())

		profileHandler := NewProfileHandler(profileStore)
		instanceHandler := NewInstanceHandler(instanceStore)
		executeHandler := NewExecuteHandler(executor, profileStore, instanceStore, policyClient, eventPub)

		r.Get("/sandbox-profiles", profileHandler.ListProfiles)
		r.Post("/sandbox-profiles", profileHandler.CreateProfile)
		r.Get("/sandbox-profiles/{id}", profileHandler.GetProfile)
		r.Patch("/sandbox-profiles/{id}", profileHandler.UpdateProfile)
		r.Delete("/sandbox-profiles/{id}", profileHandler.DeleteProfile)
		r.Post("/sandboxes/execute", executeHandler.Execute)
		r.Get("/sandboxes/instances", instanceHandler.ListInstances)
		r.Get("/sandboxes/instances/{id}", instanceHandler.GetInstance)
		r.Post("/sandboxes/instances/{id}/cancel", instanceHandler.CancelInstance)
	})

	ts := httptest.NewServer(router)
	return ts, func() {
		ts.Close()
		pool.Close()
	}
}

func TestHealth(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	require.Equal(t, "ok", body["status"])
}

func TestCreateProfile_MissingAuth(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"name": "test-profile"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandbox-profiles", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateProfile_TenantMismatch(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"name": "test-profile"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandbox-profiles", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-2")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCreateProfile_InvalidJSON(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{invalid json`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandbox-profiles", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateProfile_MissingName(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"memory_mb": 256}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandbox-profiles", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestExecute_MissingAuth(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"profile_id": "550e8400-e29b-41d4-a716-446655440000", "tool_name": "echo"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandboxes/execute", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestExecute_MissingToolName(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"profile_id": "550e8400-e29b-41d4-a716-446655440000"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandboxes/execute", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestExecute_MissingProfileID(t *testing.T) {
	ts, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"tool_name": "echo"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/sandboxes/execute", body)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}