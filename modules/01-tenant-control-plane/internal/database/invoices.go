package database

import (
	"context"
	"fmt"
	"time"
)

// InvoiceRow is the durable form of an invoice.
type InvoiceRow struct {
	ID             string
	TenantID       string
	SubscriptionID string
	IssueDate      time.Time
	DueDate        time.Time
	DueDateRaw     string
	Amount         float64
	Currency       string
	Status         string
	LineItems      []byte
	PaidAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertInvoice writes an invoice.
func (s *ControlPlaneStore) UpsertInvoice(ctx context.Context, i InvoiceRow) error {
	lineItems := i.LineItems
	if len(lineItems) == 0 {
		lineItems = []byte("[]")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_invoices
			(id, tenant_id, subscription_id, issue_date, due_date, due_date_raw,
			 amount, currency, status, line_items, paid_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			subscription_id = EXCLUDED.subscription_id,
			issue_date = EXCLUDED.issue_date,
			due_date = EXCLUDED.due_date,
			due_date_raw = EXCLUDED.due_date_raw,
			amount = EXCLUDED.amount,
			currency = EXCLUDED.currency,
			status = EXCLUDED.status,
			line_items = EXCLUDED.line_items,
			paid_at = EXCLUDED.paid_at,
			updated_at = EXCLUDED.updated_at
	`, i.ID, i.TenantID, i.SubscriptionID, i.IssueDate, i.DueDate, i.DueDateRaw,
		i.Amount, i.Currency, i.Status, lineItems, i.PaidAt, i.CreatedAt, i.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert invoice %s: %w", i.ID, err)
	}
	return nil
}

// DeleteInvoice removes an invoice.
func (s *ControlPlaneStore) DeleteInvoice(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_invoices WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete invoice %s: %w", id, err)
	}
	return nil
}

// LoadInvoices returns every invoice, for rehydration at boot.
func (s *ControlPlaneStore) LoadInvoices(ctx context.Context) ([]InvoiceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, subscription_id, issue_date, due_date, due_date_raw,
		       amount, currency, status, line_items, paid_at, created_at, updated_at
		FROM tctl_invoices ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load invoices: %w", err)
	}
	defer rows.Close()

	var out []InvoiceRow
	for rows.Next() {
		var i InvoiceRow
		if err := rows.Scan(&i.ID, &i.TenantID, &i.SubscriptionID, &i.IssueDate,
			&i.DueDate, &i.DueDateRaw, &i.Amount, &i.Currency, &i.Status,
			&i.LineItems, &i.PaidAt, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
