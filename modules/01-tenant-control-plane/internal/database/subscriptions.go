package database

import (
	"context"
	"fmt"
	"time"
)

// SubscriptionRow is the durable form of a subscription.
type SubscriptionRow struct {
	ID                 string
	TenantID           string
	Plan               string
	PlanName           string
	Status             string
	BillingCycle       string
	SeatCount          int
	UnitPrice          float64
	TotalAmount        float64
	Currency           string
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	NextBillingDate    time.Time
	CancelAtPeriodEnd  bool
	CancelledAt        *time.Time
	CustomQuotas       []byte
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// UpsertSubscription writes a subscription.
func (s *ControlPlaneStore) UpsertSubscription(ctx context.Context, r SubscriptionRow) error {
	quotas := r.CustomQuotas
	if len(quotas) == 0 {
		quotas = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_subscriptions
			(id, tenant_id, plan, plan_name, status, billing_cycle, seat_count,
			 unit_price, total_amount, currency, current_period_start,
			 current_period_end, next_billing_date, cancel_at_period_end,
			 cancelled_at, custom_quotas, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			plan = EXCLUDED.plan,
			plan_name = EXCLUDED.plan_name,
			status = EXCLUDED.status,
			billing_cycle = EXCLUDED.billing_cycle,
			seat_count = EXCLUDED.seat_count,
			unit_price = EXCLUDED.unit_price,
			total_amount = EXCLUDED.total_amount,
			currency = EXCLUDED.currency,
			current_period_start = EXCLUDED.current_period_start,
			current_period_end = EXCLUDED.current_period_end,
			next_billing_date = EXCLUDED.next_billing_date,
			cancel_at_period_end = EXCLUDED.cancel_at_period_end,
			cancelled_at = EXCLUDED.cancelled_at,
			custom_quotas = EXCLUDED.custom_quotas,
			updated_at = EXCLUDED.updated_at
	`, r.ID, r.TenantID, r.Plan, r.PlanName, r.Status, r.BillingCycle, r.SeatCount,
		r.UnitPrice, r.TotalAmount, r.Currency, r.CurrentPeriodStart, r.CurrentPeriodEnd,
		r.NextBillingDate, r.CancelAtPeriodEnd, r.CancelledAt, quotas,
		r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert subscription %s: %w", r.ID, err)
	}
	return nil
}

// DeleteSubscription removes a subscription.
func (s *ControlPlaneStore) DeleteSubscription(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_subscriptions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete subscription %s: %w", id, err)
	}
	return nil
}

// LoadSubscriptions returns every subscription, for rehydration at boot.
func (s *ControlPlaneStore) LoadSubscriptions(ctx context.Context) ([]SubscriptionRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, plan, plan_name, status, billing_cycle, seat_count,
		       unit_price, total_amount, currency, current_period_start,
		       current_period_end, next_billing_date, cancel_at_period_end,
		       cancelled_at, custom_quotas, created_at, updated_at
		FROM tctl_subscriptions ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load subscriptions: %w", err)
	}
	defer rows.Close()

	var out []SubscriptionRow
	for rows.Next() {
		var r SubscriptionRow
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Plan, &r.PlanName, &r.Status,
			&r.BillingCycle, &r.SeatCount, &r.UnitPrice, &r.TotalAmount, &r.Currency,
			&r.CurrentPeriodStart, &r.CurrentPeriodEnd, &r.NextBillingDate,
			&r.CancelAtPeriodEnd, &r.CancelledAt, &r.CustomQuotas,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
