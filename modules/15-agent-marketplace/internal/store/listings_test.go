package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newListingTestStore(t *testing.T) (*ListingStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewListingStore(mockPool), mockPool
}

func newSubscriptionTestStore(t *testing.T) (*SubscriptionStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewSubscriptionStore(mockPool), mockPool
}

func newReviewTestStore(t *testing.T) (*ReviewStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewReviewStore(mockPool), mockPool
}

func TestListingStore_GetByID_Success(t *testing.T) {
	store, mockPool := newListingTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("550e8400-e29b-41d4-a716-446655440000").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"550e8400-e29b-41d4-a716-446655440000",
			"vendor-1", "Contract Agent", "Handles contract review",
			"agent", "vetted", "approved",
			"1.0.0", "{}", `["review_contracts"]`, `["en", "ar"]`,
			false, "free", 0, 0.0,
			4.5, 10, 25,
			"{}", now, now,
		))

	listing, err := store.GetByID(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)
	assert.Equal(t, "Contract Agent", listing.Name)
	assert.Equal(t, "agent", listing.Category)
	assert.Equal(t, "vetted", listing.ListingType)
}

func TestListingStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByID(context.Background(), "nonexistent")
	assert.Equal(t, ErrNotFound, err)
}

func TestListingStore_Create_Success(t *testing.T) {
	store, mockPool := newListingTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("INSERT INTO marketplace_listings").
		WithArgs("vendor-1", "Test Agent", "A test agent", "agent", "vetted", "draft",
			"1.0.0", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			false, pgxmock.AnyArg(), 0, 0.0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "rating_avg", "rating_count", "download_count", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), 0.0, 0, 0, now, now))

	listing := &Listing{
		VendorID:              "vendor-1",
		Name:                  "Test Agent",
		Description:           "A test agent",
		Category:              "agent",
		ListingType:           "vetted",
		Status:                "draft",
		Version:               "1.0.0",
		CompatibilityVersions: JSONB{String: "{}", Valid: true},
		Capabilities:          StringArray{String: `["review"]`, Valid: true},
		SupportedLanguages:    StringArray{String: `["en"]`, Valid: true},
	}

	err := store.Create(context.Background(), listing)
	assert.NoError(t, err)
	assert.NotEmpty(t, listing.ID)
}

func TestListingStore_Create_Fail(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("INSERT INTO marketplace_listings").
		WithArgs("vendor-1", "Test Agent", "A test agent", "agent", "vetted", "draft",
			"1.0.0", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			false, pgxmock.AnyArg(), 0, 0.0).
		WillReturnError(errors.New("database error"))

	listing := &Listing{
		VendorID:              "vendor-1",
		Name:                  "Test Agent",
		Description:           "A test agent",
		Category:              "agent",
		ListingType:           "vetted",
		Status:                "draft",
		Version:               "1.0.0",
		CompatibilityVersions: JSONB{String: "{}", Valid: true},
		Capabilities:          StringArray{String: `["review"]`, Valid: true},
		SupportedLanguages:    StringArray{String: `["en"]`, Valid: true},
	}

	err := store.Create(context.Background(), listing)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database error")
}

func TestListingStore_UpdateStatus_Success(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectExec("UPDATE marketplace_listings SET status").
		WithArgs("approved", "listing-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpdateStatus(context.Background(), "listing-1", "approved")
	assert.NoError(t, err)
}

func TestListingStore_UpdateStatus_NotFound(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectExec("UPDATE marketplace_listings SET status").
		WithArgs("approved", "nonexistent").
		WillReturnError(pgx.ErrNoRows)

	err := store.UpdateStatus(context.Background(), "nonexistent", "approved")
	assert.Equal(t, pgx.ErrNoRows, err)
}

func TestListingStore_UpdateRating_Success(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectExec("UPDATE marketplace_listings SET").
		WithArgs("listing-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpdateRating(context.Background(), "listing-1")
	assert.NoError(t, err)
}

func TestListingStore_IncrementDownloads_Success(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectExec("UPDATE marketplace_listings SET download_count").
		WithArgs("listing-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.IncrementDownloads(context.Background(), "listing-1")
	assert.NoError(t, err)
}

func TestListingStore_IncrementDownloads_Fail(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectExec("UPDATE marketplace_listings SET download_count").
		WithArgs("listing-1").
		WillReturnError(errors.New("database error"))

	err := store.IncrementDownloads(context.Background(), "listing-1")
	assert.Error(t, err)
}

func TestListingStore_List_Success(t *testing.T) {
	store, mockPool := newListingTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("agent").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("agent", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}).AddRow(
			"550e8400-1", "vendor-1", "Agent A", "Desc A", "agent", "vetted", "approved",
			"1.0", "{}", `["review"]`, `["en"]`, false, "free", 0, 0.0,
			4.5, 10, 5, "{}", now, now,
		))

	filter := &ListingFilter{
		Page:     1,
		PageSize: 20,
		Category: "agent",
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Listings))
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Page)
}

func TestListingStore_List_EmptyResult(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	result, err := store.List(context.Background(), &ListingFilter{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
	assert.Equal(t, 0, result.Total)
}

func TestListingStore_List_SearchFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("contract").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("contract", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	filter := &ListingFilter{
		Page:     1,
		PageSize: 20,
		Search:   "contract",
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_CategoryFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("template").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("template", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	filter := &ListingFilter{
		Page:     1,
		PageSize: 20,
		Category: "template",
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_RatingMinFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(4.0).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs(4.0, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	ratingMin := 4.0
	filter := &ListingFilter{
		Page:      1,
		PageSize:  20,
		RatingMin: &ratingMin,
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_PriceRangeFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	priceMin := 10.0
	priceMax := 100.0
	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(priceMin, priceMax).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs(priceMin, priceMax, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	filter := &ListingFilter{
		Page:     1,
		PageSize: 20,
		PriceMin: &priceMin,
		PriceMax: &priceMax,
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_RequiresSubscriptionFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	reqSub := false
	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(reqSub).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs(reqSub, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	filter := &ListingFilter{
		Page:               1,
		PageSize:           20,
		RequiresSubscription: &reqSub,
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_CapabilityFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("review_contracts").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("review_contracts", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	filter := &ListingFilter{
		Page:       1,
		PageSize:   20,
		Capability: "review_contracts",
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_LanguageFilter(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("ar").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, vendor_id, name").
		WithArgs("ar", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "vendor_id", "name", "description", "category", "listing_type", "status",
			"version", "compatibility_versions", "capabilities", "supported_languages",
			"requires_subscription", "subscription_tier", "trial_days", "price_usd",
			"rating_avg", "rating_count", "download_count", "metadata", "created_at", "updated_at",
		}))

	filter := &ListingFilter{
		Page:     1,
		PageSize: 20,
		Language: "ar",
	}

	result, err := store.List(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Listings))
}

func TestListingStore_List_DBError(t *testing.T) {
	store, mockPool := newListingTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WillReturnError(errors.New("database error"))

	filter := &ListingFilter{
		Page:     1,
		PageSize: 20,
		Category: "agent",
	}

	result, err := store.List(context.Background(), filter)
	assert.Error(t, err)
	assert.Nil(t, result)
}