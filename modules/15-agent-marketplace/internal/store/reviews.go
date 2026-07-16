package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReviewStore handles review and rating operations.
type ReviewStore struct {
	pool PgxPool
}

func NewReviewStore(pool PgxPool) *ReviewStore {
	return &ReviewStore{pool: pool}
}

func (s *ReviewStore) Create(ctx context.Context, r *Review) error {
	query := `
		INSERT INTO reviews (tenant_id, listing_id, rating, title, review_text, verified_purchase)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	var id string
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx, query,
		r.TenantID, r.ListingID, r.Rating, r.Title, r.ReviewText, r.VerifiedPurchase,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	r.ID = id
	r.CreatedAt = createdAt
	r.UpdatedAt = updatedAt
	return nil
}

func (s *ReviewStore) GetByTenantAndListing(ctx context.Context, tenantID, listingID string) (*Review, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, listing_id, rating, title, review_text, verified_purchase,
			helpful_count, status, created_at, updated_at
		FROM reviews WHERE tenant_id = $1 AND listing_id = $2`,
		tenantID, listingID)

	return scanReview(row)
}

func (s *ReviewStore) ListByListing(ctx context.Context, listingID string, page, pageSize int) (*PaginatedReviews, error) {
	offset := (page - 1) * pageSize

	var total int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM reviews WHERE listing_id = $1 AND status = 'active'",
		listingID).Scan(&total)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, listing_id, rating, title, review_text, verified_purchase,
			helpful_count, status, created_at, updated_at
		FROM reviews WHERE listing_id = $1 AND status = 'active'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		listingID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviews := make([]Review, 0)
	for rows.Next() {
			r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, *r)
	}

	if reviews == nil {
		reviews = []Review{}
	}

	return &PaginatedReviews{
		Reviews: reviews,
		Page:    page,
		PageSize: pageSize,
		Total:   total,
	}, nil
}

func (s *ReviewStore) IncrementHelpful(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE reviews SET helpful_count = helpful_count + 1 WHERE id = $1",
		id)
	return err
}

func scanReview(row pgx.Row) (*Review, error) {
	var r Review
	err := row.Scan(&r.ID, &r.TenantID, &r.ListingID, &r.Rating, &r.Title,
		&r.ReviewText, &r.VerifiedPurchase, &r.HelpfulCount, &r.Status,
		&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

// GetPool returns the underlying pgxpool for testing.
func (s *ReviewStore) GetPool() PgxPool {
	return s.pool
}