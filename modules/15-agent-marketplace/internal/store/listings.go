package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// PgxPool is the database interface used by all store types.
type PgxPool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Close()
}

var ErrNotFound = errors.New("record not found")

// ListingStore handles marketplace listings CRUD and search operations.
type ListingStore struct {
	pool PgxPool
}

func NewListingStore(pool PgxPool) *ListingStore {
	return &ListingStore{pool: pool}
}

func (s *ListingStore) Create(ctx context.Context, l *Listing) error {
	compatBytes, _ := json.Marshal(l.CompatibilityVersions)
	capBytes, _ := json.Marshal(l.Capabilities)
	langBytes, _ := json.Marshal(l.SupportedLanguages)
	_, _ = json.Marshal(l.Metadata) // metadata is stored via UPDATE later

	query := `
		INSERT INTO marketplace_listings (vendor_id, name, description, category, listing_type, status,
			version, compatibility_versions, capabilities, supported_languages,
			requires_subscription, subscription_tier, trial_days, price_usd)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, rating_avg, rating_count, download_count, created_at, updated_at`

	var id string
	var ratingAvg float64
	var ratingCount, downloadCount int
	var createdAt, updatedAt time.Time

	err := s.pool.QueryRow(ctx, query,
		l.VendorID, l.Name, l.Description, l.Category, l.ListingType, l.Status,
		l.Version, compatBytes, capBytes, langBytes,
		l.RequiresSubscription, l.SubscriptionTier, l.TrialDays, l.PriceUSD,
	).Scan(&id, &ratingAvg, &ratingCount, &downloadCount, &createdAt, &updatedAt)
	if err != nil {
		return err
	}

	l.ID = id
	l.RatingAvg = ratingAvg
	l.RatingCount = ratingCount
	l.DownloadCount = downloadCount
	l.CreatedAt = createdAt
	l.UpdatedAt = updatedAt
	return nil
}

func (s *ListingStore) GetByID(ctx context.Context, id string) (*Listing, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, vendor_id, name, description, category, listing_type, status,
			version, compatibility_versions, capabilities, supported_languages,
			requires_subscription, subscription_tier, trial_days, price_usd,
			rating_avg, rating_count, download_count, metadata, created_at, updated_at
		FROM marketplace_listings WHERE id = $1`, id)

	return scanListing(row)
}

func scanListing(row pgx.Row) (*Listing, error) {
	var l Listing
	var compat pgtype.Text
	var caps pgtype.Text
	var langs pgtype.Text
	var meta pgtype.Text

	err := row.Scan(&l.ID, &l.VendorID, &l.Name, &l.Description, &l.Category,
		&l.ListingType, &l.Status, &l.Version, &compat, &caps, &langs,
		&l.RequiresSubscription, &l.SubscriptionTier, &l.TrialDays, &l.PriceUSD,
		&l.RatingAvg, &l.RatingCount, &l.DownloadCount, &meta, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	l.CompatibilityVersions = JSONB{String: compat.String, Valid: compat.Valid}
	l.Capabilities = StringArray{String: caps.String, Valid: caps.Valid}
	l.SupportedLanguages = StringArray{String: langs.String, Valid: langs.Valid}
	l.Metadata = JSONB{String: meta.String, Valid: meta.Valid}
	return &l, nil
}

// List returns paginated listings matching the filter criteria.
func (s *ListingStore) List(ctx context.Context, f *ListingFilter) (*PaginatedListings, error) {
	whereClauses := []string{"status = 'approved'"}
	args := []interface{}{}
	argIndex := 1

	if f.Category != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, f.Category)
		argIndex++
	}
	if f.ListingType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("listing_type = $%d", argIndex))
		args = append(args, f.ListingType)
		argIndex++
	}
	if f.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, f.Status)
		argIndex++
	}
	if f.RequiresSubscription != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("requires_subscription = $%d", argIndex))
		args = append(args, *f.RequiresSubscription)
		argIndex++
	}
	if f.SubscriptionTier != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("subscription_tier = $%d", argIndex))
		args = append(args, f.SubscriptionTier)
		argIndex++
	}
	if f.Capability != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("capabilities @> ARRAY[$%d]::TEXT[]", argIndex))
		args = append(args, f.Capability)
		argIndex++
	}
	if f.Language != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("supported_languages @> ARRAY[$%d]::TEXT[]", argIndex))
		args = append(args, f.Language)
		argIndex++
	}
	if f.PriceMin != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("price_usd >= $%d", argIndex))
		args = append(args, *f.PriceMin)
		argIndex++
	}
	if f.PriceMax != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("price_usd <= $%d", argIndex))
		args = append(args, *f.PriceMax)
		argIndex++
	}
	if f.RatingMin != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("rating_avg >= $%d", argIndex))
		args = append(args, *f.RatingMin)
		argIndex++
	}
	if f.Search != "" {
		whereClauses = append(whereClauses, "to_tsvector('simple', name || ' ' || description) @@ plainto_tsquery('simple', $"+fmt.Sprint(argIndex)+")")
		args = append(args, f.Search)
		argIndex++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM marketplace_listings WHERE " + whereSQL
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Fetch page
	args = append(args, f.PageSize, f.Page)
	listQuery := fmt.Sprintf(`
		SELECT id, vendor_id, name, description, category, listing_type, status,
			version, compatibility_versions, capabilities, supported_languages,
			requires_subscription, subscription_tier, trial_days, price_usd,
			rating_avg, rating_count, download_count, metadata, created_at, updated_at
		FROM marketplace_listings WHERE %s
		ORDER BY rating_avg DESC, created_at DESC
		LIMIT $%d OFFSET $%d`, whereSQL, argIndex, argIndex+1)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]Listing, 0)
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, *l)
	}

	if listings == nil {
		listings = []Listing{}
	}

	return &PaginatedListings{
		Listings: listings,
		Page:     f.Page,
		PageSize: f.PageSize,
		Total:    total,
	}, nil
}

func (s *ListingStore) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE marketplace_listings SET status = $1, updated_at = NOW() WHERE id = $2",
		status, id)
	return err
}

// UpdateRating recalculates rating_avg and rating_count from reviews.
func (s *ListingStore) UpdateRating(ctx context.Context, listingID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE marketplace_listings SET
			rating_avg = COALESCE((
				SELECT AVG(rating)::NUMERIC(3,2) FROM reviews
				WHERE listing_id = $1 AND status = 'active'
			), 0),
			rating_count = (
				SELECT COUNT(*) FROM reviews
				WHERE listing_id = $1 AND status = 'active'
			),
			updated_at = NOW()
		WHERE id = $1`, listingID)
	return err
}

func (s *ListingStore) IncrementDownloads(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE marketplace_listings SET download_count = download_count + 1 WHERE id = $1",
		id)
	return err
}

// GetPool returns the underlying pgxpool for testing.
func (s *ListingStore) GetPool() PgxPool {
	return s.pool
}