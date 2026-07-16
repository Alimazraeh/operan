package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"

	"github.com/operan/agent-marketplace/internal/clients"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/store"
)

func setupDeployTest(t *testing.T) (*Deployer, pgxmock.PgxPoolIface, *httptest.Server, *httptest.Server) {
	t.Helper()

	// Mock M04 server
	m04Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":     "agent-123",
				"name":   "Test Agent",
				"role":   "Analyst",
				"tenant": "tenant-1",
			},
		})
	}))

	// Mock M03 server
	m03Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":   "workflow-456",
				"name": "Test Workflow",
			},
		})
	}))

	// Mock DB
	mockPool, err := pgxmock.NewPool()
	assert.NoError(t, err)

	listingStore := store.NewListingStore(mockPool)
	subStore := store.NewSubscriptionStore(mockPool)
	evtPub := events.NewPublisher("")

	m04Client := clients.NewM04Client(m04Server.URL)
	m03Client := clients.NewM03Client(m03Server.URL)

	deployer := NewDeployer(m04Client, m03Client, listingStore, subStore, evtPub, "test-token")

	return deployer, mockPool, m04Server, m03Server
}

func TestDeploy_Success(t *testing.T) {
	deployer, mockPool, m04srv, m03srv := setupDeployTest(t)
	defer m04srv.Close()
	defer m03srv.Close()
	now := time.Now()

	// Mock GetByID
	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-1", "vendor-1", "Contract Review", "Reviews contracts",
			"agent", "vetted", "approved",
			"1.0.0", "{}", `["review"]`, `["en"]`, false, "free", 0, 0.0,
			4.5, 10, 5, `{"agents":[{"name":"Contract Reviewer","role":"Analyst","capabilities":["review"],"tools":[]}]}`,
			now, now))

	// Mock IsActive
	mockPool.ExpectQuery("SELECT status, expires_at").WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("active", (*time.Time)(nil)))

	// Mock UpdateDeployed
	mockPool.ExpectExec("UPDATE tenant_subscriptions SET deployed").
		WithArgs(true, pgxmock.AnyArg(), "tenant-1", "listing-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Mock IncrementDownloads
	mockPool.ExpectExec("UPDATE marketplace_listings SET download_count").
		WithArgs("listing-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-1")

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, len(result.CreatedAgents))
	assert.Equal(t, 1, len(result.CreatedWorkflows))
	assert.Empty(t, result.Errors)
}

func TestDeploy_MultipleAgents(t *testing.T) {
	deployer, mockPool, m04srv, m03srv := setupDeployTest(t)
	defer m04srv.Close()
	defer m03srv.Close()
	now := time.Now()

	// Mock GetByID with 2 agents in metadata
	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-2").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-2", "vendor-1", "Multi-Agent Suite", "Two agents",
			"agent", "vetted", "approved",
			"1.0.0", "{}", `["review","analyze"]`, `["en"]`, false, "free", 0, 0.0,
			4.5, 10, 5, `{"agents":[{"name":"Agent A","role":"Analyst","capabilities":["review"],"tools":[]},{"name":"Agent B","role":"Writer","capabilities":["analyze"],"tools":["write"]}]}`,
			now, now))

	// Mock IsActive
	mockPool.ExpectQuery("SELECT status, expires_at").WithArgs("tenant-1", "listing-2").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("active", (*time.Time)(nil)))

	// Mock UpdateDeployed + IncrementDownloads
	mockPool.ExpectExec("UPDATE tenant_subscriptions SET deployed").
		WithArgs(true, pgxmock.AnyArg(), "tenant-1", "listing-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mockPool.ExpectExec("UPDATE marketplace_listings SET download_count").
		WithArgs("listing-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-2")

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, len(result.CreatedAgents))
	assert.Equal(t, 2, len(result.CreatedWorkflows))
}

func TestDeploy_FallbackToSingleAgent(t *testing.T) {
	deployer, mockPool, m04srv, m03srv := setupDeployTest(t)
	defer m04srv.Close()
	defer m03srv.Close()
	now := time.Now()

	// Mock GetByID with empty metadata (no agents array)
	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-3").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-3", "vendor-1", "Standalone Agent", "Single agent",
			"agent", "vetted", "approved",
			"1.0.0", "{}", `[]`, `["en"]`, false, "free", 0, 0.0,
			0.0, 0, 0, "{}",
			now, now))

	// Mock IsActive
	mockPool.ExpectQuery("SELECT status, expires_at").WithArgs("tenant-1", "listing-3").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("active", (*time.Time)(nil)))

	// Mock UpdateDeployed + IncrementDownloads
	mockPool.ExpectExec("UPDATE tenant_subscriptions SET deployed").
		WithArgs(true, pgxmock.AnyArg(), "tenant-1", "listing-3").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mockPool.ExpectExec("UPDATE marketplace_listings SET download_count").
		WithArgs("listing-3").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-3")

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, len(result.CreatedAgents))
	assert.Equal(t, 1, len(result.CreatedWorkflows))
}

func TestDeploy_ListingNotFound(t *testing.T) {
	deployer, mockPool, m04srv, m03srv := setupDeployTest(t)
	defer m04srv.Close()
	defer m03srv.Close()

	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	result, err := deployer.Deploy(context.Background(), "tenant-1", "nonexistent")

	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Errors[0], "listing not found")
}

func TestDeploy_DeactivatedListing(t *testing.T) {
	deployer, mockPool, m04srv, m03srv := setupDeployTest(t)
	defer m04srv.Close()
	defer m03srv.Close()
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-deactivated").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-deactivated", "vendor-1", "Old Agent", "Deactivated",
			"agent", "vetted", "deactivated",
			"1.0.0", "{}", `[]`, `["en"]`, false, "free", 0, 0.0,
			0.0, 0, 0, "{}",
			now, now))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-deactivated")

	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Errors[0], "deactivated")
}

func TestDeploy_ExpiredSubscription(t *testing.T) {
	deployer, mockPool, m04srv, m03srv := setupDeployTest(t)
	defer m04srv.Close()
	defer m03srv.Close()
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-premium").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-premium", "vendor-1", "Premium Agent", "Requires subscription",
			"agent", "vetted", "approved",
			"1.0.0", "{}", `[]`, `["en"]`, true, "pro", 14, 99.99,
			4.5, 5, 2, "{}",
			now, now))

	// Mock IsActive returning false (expired trial)
	expiredTime := time.Now().Add(-24 * time.Hour)
	mockPool.ExpectQuery("SELECT status, expires_at").WithArgs("tenant-1", "listing-premium").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("trial", &expiredTime))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-premium")

	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Errors[0], "no active subscription")
}

func TestDeploy_M03Failure_Rollback(t *testing.T) {
	// M03 server that fails on call
	m03FailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer m03FailServer.Close()

	// M04 server that succeeds
	m04Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":   "agent-123",
				"name": "Test Agent",
			},
		})
	}))
	defer m04Server.Close()

	mockPool, err := pgxmock.NewPool()
	assert.NoError(t, err)

	listingStore := store.NewListingStore(mockPool)
	subStore := store.NewSubscriptionStore(mockPool)
	evtPub := events.NewPublisher("")

	m04Client := clients.NewM04Client(m04Server.URL)
	m03Client := clients.NewM03Client(m03FailServer.URL)

	deployer := NewDeployer(m04Client, m03Client, listingStore, subStore, evtPub, "test-token")
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-1", "vendor-1", "Test Agent", "Desc",
			"agent", "vetted", "approved",
			"1.0.0", "{}", `[]`, `["en"]`, false, "free", 0, 0.0,
			0.0, 0, 0, "{}",
			now, now))

	mockPool.ExpectQuery("SELECT status, expires_at").WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("active", (*time.Time)(nil)))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-1")

	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Errors[0], "M03 workflow creation failed")
	assert.Equal(t, 1, len(result.CreatedAgents)) // Agent was registered in M04
	assert.Equal(t, 0, len(result.CreatedWorkflows))
}

func TestDeploy_CompatibilityCheckFail(t *testing.T) {
	// M04 server that fails health check
	m04FailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer m04FailServer.Close()

	mockPool, err := pgxmock.NewPool()
	assert.NoError(t, err)

	listingStore := store.NewListingStore(mockPool)
	subStore := store.NewSubscriptionStore(mockPool)
	evtPub := events.NewPublisher("")

	m04Client := clients.NewM04Client(m04FailServer.URL)
	m03Client := clients.NewM03Client("http://localhost:9999")

	deployer := NewDeployer(m04Client, m03Client, listingStore, subStore, evtPub, "test-token")
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, vendor_id, name").WithArgs("listing-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"listing-1", "vendor-1", "Test Agent", "Desc",
			"agent", "vetted", "approved",
			"1.0.0", `{"m04_min": "99.0"}`, `[]`, `["en"]`, false, "free", 0, 0.0,
			0.0, 0, 0, "{}",
			now, now))

	mockPool.ExpectQuery("SELECT status, expires_at").WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("active", (*time.Time)(nil)))

	result, err := deployer.Deploy(context.Background(), "tenant-1", "listing-1")

	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Errors[0], "compatibility check failed")
}

func TestCheckCompatibility_Healthy(t *testing.T) {
	m04Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer m04Server.Close()

	m03Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer m03Server.Close()

	m04Client := clients.NewM04Client(m04Server.URL)
	m03Client := clients.NewM03Client(m03Server.URL)

	listing := &store.Listing{
		CompatibilityVersions: store.JSONB{String: `{"m04_min": "1.0", "m03_min": "1.0"}`, Valid: true},
	}

	err := CheckCompatibility(m04Client, m03Client, listing)
	assert.NoError(t, err)
}

func TestCheckCompatibility_M04Down(t *testing.T) {
	m04Client := clients.NewM04Client("http://localhost:1")
	m03Client := clients.NewM03Client("http://localhost:1")

	listing := &store.Listing{
		CompatibilityVersions: store.JSONB{String: `{"m04_min": "1.0"}`, Valid: true},
	}

	err := CheckCompatibility(m04Client, m03Client, listing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "M04 health check failed")
}

func TestParseListing_WithAgents(t *testing.T) {
	listing := &store.Listing{
		Name:                  "Suite",
		Category:              "agent",
		Metadata:              store.JSONB{String: `{"agents":[{"name":"A1","role":"Analyst","capabilities":["review"],"tools":["read"]},{"name":"A2","role":"Writer","capabilities":["write"],"tools":[]}]}`, Valid: true},
		CompatibilityVersions: store.JSONB{String: `{"m04_min": "1.0"}`, Valid: true},
		Capabilities:          store.StringArray{String: `["review","write"]`, Valid: true},
	}

	ml, agents := parseListing(listing)
	assert.Equal(t, 2, len(agents))
	assert.Equal(t, "A1", agents[0].Name)
	assert.Equal(t, "Analyst", agents[0].Role)
	assert.Equal(t, "review", agents[0].Capabilities[0])
	assert.Equal(t, "read", agents[0].Tools[0])
	assert.Equal(t, "A2", agents[1].Name)
	assert.Equal(t, "Writer", agents[1].Role)
	assert.Equal(t, "write", agents[1].Capabilities[0])
	assert.Empty(t, agents[1].Tools)
	assert.Equal(t, "1.0", ml.CompatibilityVersions["m04_min"])
	assert.Empty(t, ml.Metadata["agents"]) // agents removed from top-level metadata
}

func TestParseListing_FallbackSingleAgent(t *testing.T) {
	listing := &store.Listing{
		Name:                  "Standalone",
		Category:              "agent",
		Metadata:              store.JSONB{String: "{}", Valid: true},
		CompatibilityVersions: store.JSONB{Valid: false},
		Capabilities:          store.StringArray{String: `["review"]`, Valid: true},
	}

	ml, agents := parseListing(listing)
	assert.Equal(t, 1, len(agents))
	assert.Equal(t, "Standalone", agents[0].Name)
	assert.Equal(t, "Marketplace-agent", agents[0].Role)
	assert.Equal(t, "review", agents[0].Capabilities[0])
	assert.Equal(t, "Standalone", ml.Name)
	assert.Equal(t, "agent", ml.Category)
}

func TestParseListing_EmptyMetadata(t *testing.T) {
	listing := &store.Listing{
		Name:     "Test",
		Category: "agent",
		Metadata: store.JSONB{Valid: false},
	}

	_, agents := parseListing(listing)
	assert.Equal(t, 1, len(agents))
	assert.Equal(t, "Test", agents[0].Name)
	assert.Equal(t, "Marketplace-agent", agents[0].Role)
}

func TestParseListing_InvalidJSON(t *testing.T) {
	listing := &store.Listing{
		Name:     "Test",
		Category: "agent",
		Metadata: store.JSONB{String: "{invalid json", Valid: true},
	}

	_, agents := parseListing(listing)
	assert.Equal(t, 1, len(agents))
	assert.Equal(t, "Test", agents[0].Name)
}

func TestParseListing_MissingAgentFields(t *testing.T) {
	listing := &store.Listing{
		Name:     "Suite",
		Category: "agent",
		Metadata: store.JSONB{String: `{"agents":[{"capabilities":["review"]}]}`, Valid: true},
	}

	_, agents := parseListing(listing)
	assert.Equal(t, 1, len(agents))
	assert.Empty(t, agents[0].Name)
	assert.Empty(t, agents[0].Role)
	assert.Equal(t, "review", agents[0].Capabilities[0])
}