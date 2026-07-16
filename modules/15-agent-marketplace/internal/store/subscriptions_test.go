package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func TestSubscriptionStore_Create_Success(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("INSERT INTO tenant_subscriptions").
		WithArgs("tenant-1", "listing-1", "trial", pgxmock.AnyArg(), pgxmock.AnyArg(), "basic", false).
		WillReturnRows(pgxmock.NewRows([]string{"id", "started_at", "created_at"}).
			AddRow(uuid.New().String(), now, now))

	sub := &TenantSubscription{
		TenantID:       "tenant-1",
		ListingID:      "listing-1",
		Status:         "trial",
		SubscriptionTier: "basic",
	}

	err := store.Create(context.Background(), sub)
	assert.NoError(t, err)
	assert.NotEmpty(t, sub.ID)
}

func TestSubscriptionStore_Create_Fail(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("INSERT INTO tenant_subscriptions").
		WithArgs("tenant-1", "listing-1", "trial", pgxmock.AnyArg(), pgxmock.AnyArg(), "basic", false).
		WillReturnError(errors.New("database error"))

	sub := &TenantSubscription{
		TenantID:       "tenant-1",
		ListingID:      "listing-1",
		Status:         "trial",
		SubscriptionTier: "basic",
	}

	err := store.Create(context.Background(), sub)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database error")
}

func TestSubscriptionStore_GetByTenantAndListing_Success(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "status", "started_at", "expires_at",
			"auto_renew", "subscription_tier", "trial_used", "deployed", "deployed_at", "created_at",
		}).AddRow(
			uuid.New().String(), "tenant-1", "listing-1", "active", now,
			pgtype.Timestamptz{Valid: false},
			true, "basic", false, false, pgtype.Timestamptz{Valid: false}, now,
		))

	sub, err := store.GetByTenantAndListing(context.Background(), "tenant-1", "listing-1")
	assert.NoError(t, err)
	assert.Equal(t, "tenant-1", sub.TenantID)
	assert.Equal(t, "active", sub.Status)
}

func TestSubscriptionStore_GetByTenantAndListing_NotFound(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", "listing-1").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByTenantAndListing(context.Background(), "tenant-1", "listing-1")
	assert.Equal(t, ErrNotFound, err)
}

func TestSubscriptionStore_IsActive_Active(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("SELECT status, expires_at").
		WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("active", (*time.Time)(nil)))

	active, err := store.IsActive(context.Background(), "tenant-1", "listing-1")
	assert.NoError(t, err)
	assert.True(t, active)
}

func TestSubscriptionStore_IsActive_TrialExpiring(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	exp := time.Now().Add(7 * 24 * time.Hour)

	mockPool.ExpectQuery("SELECT status, expires_at").
		WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("trial", &exp))

	active, err := store.IsActive(context.Background(), "tenant-1", "listing-1")
	assert.NoError(t, err)
	assert.True(t, active)
}

func TestSubscriptionStore_IsActive_TrialExpired(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	exp := time.Now().Add(-24 * time.Hour)

	mockPool.ExpectQuery("SELECT status, expires_at").
		WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"status", "expires_at"}).
			AddRow("trial", &exp))

	active, err := store.IsActive(context.Background(), "tenant-1", "listing-1")
	assert.NoError(t, err)
	assert.False(t, active)
}

func TestSubscriptionStore_IsActive_NoSubscription(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("SELECT status, expires_at").
		WithArgs("tenant-1", "listing-1").
		WillReturnError(pgx.ErrNoRows)

	active, err := store.IsActive(context.Background(), "tenant-1", "listing-1")
	assert.NoError(t, err)
	assert.False(t, active)
}

func TestSubscriptionStore_IsActive_DBError(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("SELECT status, expires_at").
		WithArgs("tenant-1", "listing-1").
		WillReturnError(errors.New("database error"))

	active, err := store.IsActive(context.Background(), "tenant-1", "listing-1")
	assert.Error(t, err)
	assert.False(t, active)
}

func TestSubscriptionStore_UpdateDeployed_Success(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	now := time.Now()

	mockPool.ExpectExec("UPDATE tenant_subscriptions SET deployed").
		WithArgs(true, now, "tenant-1", "listing-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpdateDeployed(context.Background(), "tenant-1", "listing-1", true, now)
	assert.NoError(t, err)
}

func TestSubscriptionStore_UpdateDeployed_Fail(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	now := time.Now()

	mockPool.ExpectExec("UPDATE tenant_subscriptions SET deployed").
		WithArgs(true, now, "tenant-1", "listing-1").
		WillReturnError(errors.New("database error"))

	err := store.UpdateDeployed(context.Background(), "tenant-1", "listing-1", true, now)
	assert.Error(t, err)
}

func TestSubscriptionStore_ListByTenant_Success(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))
	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "status", "started_at", "expires_at",
			"auto_renew", "subscription_tier", "trial_used", "deployed", "deployed_at", "created_at",
		}).AddRow(
			uuid.New().String(), "tenant-1", "listing-1", "active", now,
			pgtype.Timestamptz{Valid: false},
			true, "basic", false, false, pgtype.Timestamptz{Valid: false}, now,
		).AddRow(
			uuid.New().String(), "tenant-1", "listing-2", "trial", now,
			pgtype.Timestamptz{Time: now, Valid: true},
			true, "pro", false, true, pgtype.Timestamptz{Time: now, Valid: true}, now,
		))

	result, err := store.ListByTenant(context.Background(), "tenant-1", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result.Subscriptions))
	assert.Equal(t, 2, result.Total)
}

func TestSubscriptionStore_ListByTenant_Empty(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "status", "started_at", "expires_at",
			"auto_renew", "subscription_tier", "trial_used", "deployed", "deployed_at", "created_at",
		}))

	result, err := store.ListByTenant(context.Background(), "tenant-1", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Subscriptions))
	assert.Equal(t, 0, result.Total)
}

func TestSubscriptionStore_ListByTenant_DBError(t *testing.T) {
	store, mockPool := newSubscriptionTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("tenant-1").
		WillReturnError(errors.New("database error"))

	result, err := store.ListByTenant(context.Background(), "tenant-1", 1, 20)
	assert.Error(t, err)
	assert.Nil(t, result)
}