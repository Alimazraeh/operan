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

func TestReviewStore_Create_Success(t *testing.T) {
	store, mockPool := newReviewTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("INSERT INTO reviews").
		WithArgs("tenant-1", "listing-1", 5, "Great", "Excellent agent", true).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), now, now))

	review := &Review{
		TenantID:         "tenant-1",
		ListingID:        "listing-1",
		Rating:           5,
		Title:            "Great",
		ReviewText:       "Excellent agent",
		VerifiedPurchase: true,
	}

	err := store.Create(context.Background(), review)
	assert.NoError(t, err)
	assert.NotEmpty(t, review.ID)
	assert.Equal(t, 5, review.Rating)
}

func TestReviewStore_Create_Fail(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectQuery("INSERT INTO reviews").
		WithArgs("tenant-1", "listing-1", 5, "Great", "Excellent", true).
		WillReturnError(errors.New("database error"))

	review := &Review{
		TenantID:         "tenant-1",
		ListingID:        "listing-1",
		Rating:           5,
		Title:            "Great",
		ReviewText:       "Excellent",
		VerifiedPurchase: true,
	}

	err := store.Create(context.Background(), review)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database error")
}

func TestReviewStore_GetByTenantAndListing_Success(t *testing.T) {
	store, mockPool := newReviewTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", "listing-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "rating", "title", "review_text", "verified_purchase",
			"helpful_count", "status", "created_at", "updated_at",
		}).AddRow(
			uuid.New().String(), "tenant-1", "listing-1", 5, "Great", "Excellent",
			true, 3, "active", now, now,
		))

	review, err := store.GetByTenantAndListing(context.Background(), "tenant-1", "listing-1")
	assert.NoError(t, err)
	assert.Equal(t, 5, review.Rating)
	assert.Equal(t, "Great", review.Title)
}

func TestReviewStore_GetByTenantAndListing_NotFound(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", "listing-1").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByTenantAndListing(context.Background(), "tenant-1", "listing-1")
	assert.Equal(t, ErrNotFound, err)
}

func TestReviewStore_GetByTenantAndListing_DBError(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("tenant-1", "listing-1").
		WillReturnError(errors.New("database error"))

	_, err := store.GetByTenantAndListing(context.Background(), "tenant-1", "listing-1")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database error")
}

func TestReviewStore_ListByListing_Success(t *testing.T) {
	store, mockPool := newReviewTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))
	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("listing-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "rating", "title", "review_text", "verified_purchase",
			"helpful_count", "status", "created_at", "updated_at",
		}).AddRow(
			uuid.New().String(), "tenant-1", "listing-1", 5, "Great", "Excellent",
			true, 5, "active", now, now,
		).AddRow(
			uuid.New().String(), "tenant-2", "listing-1", 4, "Good", "Works well",
			false, 1, "active", now, now,
		))

	result, err := store.ListByListing(context.Background(), "listing-1", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result.Reviews))
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 5, result.Reviews[0].Rating)
}

func TestReviewStore_ListByListing_Empty(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("listing-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "rating", "title", "review_text", "verified_purchase",
			"helpful_count", "status", "created_at", "updated_at",
		}))

	result, err := store.ListByListing(context.Background(), "listing-1", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.Reviews))
	assert.Equal(t, 0, result.Total)
}

func TestReviewStore_ListByListing_DBError(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("listing-1").
		WillReturnError(errors.New("database error"))

	result, err := store.ListByListing(context.Background(), "listing-1", 1, 20)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReviewStore_IncrementHelpful_Success(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectExec("UPDATE reviews SET helpful_count").
		WithArgs("review-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.IncrementHelpful(context.Background(), "review-1")
	assert.NoError(t, err)
}

func TestReviewStore_IncrementHelpful_Fail(t *testing.T) {
	store, mockPool := newReviewTestStore(t)

	mockPool.ExpectExec("UPDATE reviews SET helpful_count").
		WithArgs("review-1").
		WillReturnError(errors.New("database error"))

	err := store.IncrementHelpful(context.Background(), "review-1")
	assert.Error(t, err)
}

func TestReviewStore_ListByListing_Pagination(t *testing.T) {
	store, mockPool := newReviewTestStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs("listing-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
	mockPool.ExpectQuery("SELECT id, tenant_id, listing_id").
		WithArgs("listing-1", 10, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "listing_id", "rating", "title", "review_text", "verified_purchase",
			"helpful_count", "status", "created_at", "updated_at",
		}).AddRow(
			uuid.New().String(), "tenant-1", "listing-1", 3, "Average", "OK",
			false, 0, "active", now, now,
		))

	result, err := store.ListByListing(context.Background(), "listing-1", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Reviews))
	assert.Equal(t, 3, result.Reviews[0].Rating)
}