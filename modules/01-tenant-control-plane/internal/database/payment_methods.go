package database

import (
	"context"
	"fmt"
	"time"
)

// PaymentMethodRow is the durable form of a payment method.
type PaymentMethodRow struct {
	ID             string
	TenantID       string
	Type           string
	LastFour       string
	ExpiryMonth    int
	ExpiryYear     int
	BillingAddress string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertPaymentMethod writes a payment method.
func (s *ControlPlaneStore) UpsertPaymentMethod(ctx context.Context, p PaymentMethodRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_payment_methods
			(id, tenant_id, type, last_four, expiry_month, expiry_year,
			 billing_address, is_default, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			last_four = EXCLUDED.last_four,
			expiry_month = EXCLUDED.expiry_month,
			expiry_year = EXCLUDED.expiry_year,
			billing_address = EXCLUDED.billing_address,
			is_default = EXCLUDED.is_default,
			updated_at = EXCLUDED.updated_at
	`, p.ID, p.TenantID, p.Type, p.LastFour, p.ExpiryMonth, p.ExpiryYear,
		p.BillingAddress, p.IsDefault, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert payment method %s: %w", p.ID, err)
	}
	return nil
}

// DeletePaymentMethod removes a payment method.
func (s *ControlPlaneStore) DeletePaymentMethod(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_payment_methods WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete payment method %s: %w", id, err)
	}
	return nil
}

// LoadPaymentMethods returns every payment method, for rehydration at boot.
func (s *ControlPlaneStore) LoadPaymentMethods(ctx context.Context) ([]PaymentMethodRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, type, last_four, expiry_month, expiry_year,
		       billing_address, is_default, created_at, updated_at
		FROM tctl_payment_methods ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load payment methods: %w", err)
	}
	defer rows.Close()

	var out []PaymentMethodRow
	for rows.Next() {
		var p PaymentMethodRow
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Type, &p.LastFour,
			&p.ExpiryMonth, &p.ExpiryYear, &p.BillingAddress, &p.IsDefault,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan payment method: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
