package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SubscriptionStore handles tenant subscription operations.
type SubscriptionStore struct {
	pool PgxPool
}

func NewSubscriptionStore(pool PgxPool) *SubscriptionStore {
	return &SubscriptionStore{pool: pool}
}

func (s *SubscriptionStore) Create(ctx context.Context, sub *TenantSubscription) error {
	query := `
		INSERT INTO tenant_subscriptions (tenant_id, listing_id, status, expires_at,
			auto_renew, subscription_tier, trial_used)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, started_at, created_at`

	var id string
	var startedAt, createdAt time.Time
	err := s.pool.QueryRow(ctx, query,
		sub.TenantID, sub.ListingID, sub.Status, sub.ExpiresAt,
		sub.AutoRenew, sub.SubscriptionTier, sub.TrialUsed,
	).Scan(&id, &startedAt, &createdAt)
	if err != nil {
		return err
	}
	sub.ID = id
	sub.StartedAt = startedAt
	sub.CreatedAt = createdAt
	return nil
}

func (s *SubscriptionStore) GetByTenantAndListing(ctx context.Context, tenantID, listingID string) (*TenantSubscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, listing_id, status, started_at, expires_at,
			auto_renew, subscription_tier, trial_used, deployed, deployed_at, created_at
		FROM tenant_subscriptions WHERE tenant_id = $1 AND listing_id = $2`,
		tenantID, listingID)

	return scanSubscription(row)
}

func (s *SubscriptionStore) ListByTenant(ctx context.Context, tenantID string, page, pageSize int) (*PaginatedSubscriptions, error) {
	offset := (page - 1) * pageSize

	var total int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM tenant_subscriptions WHERE tenant_id = $1",
		tenantID).Scan(&total)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, listing_id, status, started_at, expires_at,
			auto_renew, subscription_tier, trial_used, deployed, deployed_at, created_at
		FROM tenant_subscriptions WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		tenantID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscriptions := make([]TenantSubscription, 0)
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, *sub)
	}

	if subscriptions == nil {
		subscriptions = []TenantSubscription{}
	}

	return &PaginatedSubscriptions{
		Subscriptions: subscriptions,
		Page:          page,
		PageSize:      pageSize,
		Total:         total,
	}, nil
}

func (s *SubscriptionStore) ListByListing(ctx context.Context, listingID string) ([]*TenantSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, listing_id, status, started_at, expires_at,
			auto_renew, subscription_tier, trial_used, deployed, deployed_at, created_at
		FROM tenant_subscriptions WHERE listing_id = $1`,
		listingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []*TenantSubscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, sub)
	}
	return subscriptions, nil
}

func (s *SubscriptionStore) UpdateDeployed(ctx context.Context, tenantID, listingID string, deployed bool, deployedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE tenant_subscriptions SET deployed = $1, deployed_at = $2
		WHERE tenant_id = $3 AND listing_id = $4`,
		deployed, deployedAt, tenantID, listingID)
	return err
}

// IsActive checks if a subscription is active or in trial.
func (s *SubscriptionStore) IsActive(ctx context.Context, tenantID, listingID string) (bool, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT status, expires_at FROM tenant_subscriptions
		WHERE tenant_id = $1 AND listing_id = $2 AND status IN ('active', 'trial')`,
		tenantID, listingID)

	var status string
	var expiresAt *time.Time
	err := row.Scan(&status, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	if status == "active" {
		return true, nil
	}
	if status == "trial" && expiresAt != nil && expiresAt.After(time.Now()) {
		return true, nil
	}
	return false, nil
}

func scanSubscription(row pgx.Row) (*TenantSubscription, error) {
	var sub TenantSubscription
	var expiresAt pgtype.Timestamptz
	var deployedAt pgtype.Timestamptz
	err := row.Scan(&sub.ID, &sub.TenantID, &sub.ListingID, &sub.Status,
		&sub.StartedAt, &expiresAt, &sub.AutoRenew, &sub.SubscriptionTier,
		&sub.TrialUsed, &sub.Deployed, &deployedAt, &sub.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if expiresAt.Valid {
		sub.ExpiresAt = &expiresAt.Time
	}
	if deployedAt.Valid {
		sub.DeployedAt = &deployedAt.Time
	}
	return &sub, nil
}

// GetPool returns the underlying pgxpool for testing.
func (s *SubscriptionStore) GetPool() PgxPool {
	return s.pool
}