package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Resource handlers ───────────────────────────────────────────────────────

// ListResources handles GET /tenants/{id}/resources.
func ListResources(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		items, total, hasMore := h.ResourceStore.ListByTenant(tenantID, 1, 20)

		resp := ResourceListResponse{
			Items:   make([]*ResourceResponse, len(items)),
			Total:   total,
			HasMore: hasMore,
		}
		for i, res := range items {
			resp.Items[i] = &ResourceResponse{
				ID:        res.ID,
				TenantID:  res.TenantID,
				Name:      res.Name,
				Type:      string(res.Type),
				Region:    string(res.Region),
				Spec: ResourceSpecResponse{
					Engine:      res.Spec.Engine,
					Size:        res.Spec.Size,
					VCPUs:       res.Spec.VCPUs,
					RAMGB:       res.Spec.RAMGB,
					StorageGB:   res.Spec.StorageGB,
					Replicas:    res.Spec.Replicas,
					ExtraConfig: res.Spec.ExtraConfig,
				},
				Status:    string(res.Status),
				Endpoint:  res.Endpoint,
				CreatedAt: res.CreatedAt,
				UpdatedAt: res.UpdatedAt,
			}
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// CreateResource handles POST /tenants/{id}/resources.
func CreateResource(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			Name   string            `json:"name"`
			Type   store.ResourceType `json:"type"`
			Region store.Region      `json:"region"`
			Spec   store.ResourceSpec `json:"spec"`
			Endpoint string          `json:"endpoint,omitempty"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		resource := &store.Resource{
			TenantID: tenantID,
			Name:     req.Name,
			Type:     req.Type,
			Region:   req.Region,
			Spec:     req.Spec,
			Endpoint: req.Endpoint,
		}

		created, err := h.ResourceStore.Create(resource)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "resource creation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, ResourceResponse{
			ID:        created.ID,
			TenantID:  created.TenantID,
			Name:      created.Name,
			Type:      string(created.Type),
			Region:    string(created.Region),
			Spec: ResourceSpecResponse{
				Engine:      created.Spec.Engine,
				Size:        created.Spec.Size,
				VCPUs:       created.Spec.VCPUs,
				RAMGB:       created.Spec.RAMGB,
				StorageGB:   created.Spec.StorageGB,
				Replicas:    created.Spec.Replicas,
				ExtraConfig: created.Spec.ExtraConfig,
			},
			Status:    string(created.Status),
			Endpoint:  created.Endpoint,
			CreatedAt: created.CreatedAt,
			UpdatedAt: created.UpdatedAt,
		})
	}
}

// GetResource handles GET /tenants/{id}/resources/{resource_id}.
func GetResource(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		resourceID, ok := extractPathParam(r, "resource_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "resource id is required")
			return
		}

		resource, err := h.ResourceStore.GetByIDAndTenant(resourceID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "resource not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, ResourceResponse{
			ID:        resource.ID,
			TenantID:  resource.TenantID,
			Name:      resource.Name,
			Type:      string(resource.Type),
			Region:    string(resource.Region),
			Spec: ResourceSpecResponse{
				Engine:      resource.Spec.Engine,
				Size:        resource.Spec.Size,
				VCPUs:       resource.Spec.VCPUs,
				RAMGB:       resource.Spec.RAMGB,
				StorageGB:   resource.Spec.StorageGB,
				Replicas:    resource.Spec.Replicas,
				ExtraConfig: resource.Spec.ExtraConfig,
			},
			Status:    string(resource.Status),
			Endpoint:  resource.Endpoint,
			CreatedAt: resource.CreatedAt,
			UpdatedAt: resource.UpdatedAt,
		})
	}
}

// PatchResource handles PATCH /tenants/{id}/resources/{resource_id}.
func PatchResource(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceID, ok := extractPathParam(r, "resource_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "resource id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.ResourcePatchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		resource, err := h.ResourceStore.Patch(resourceID, store.ResourcePatchRequest{
			Name:   req.Name,
			Spec:   req.Spec,
			Status: store.ResourceStatus(req.Status),
		})
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "resource not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, ResourceResponse{
			ID:        resource.ID,
			TenantID:  resource.TenantID,
			Name:      resource.Name,
			Type:      string(resource.Type),
			Region:    string(resource.Region),
			Spec: ResourceSpecResponse{
				Engine:      resource.Spec.Engine,
				Size:        resource.Spec.Size,
				VCPUs:       resource.Spec.VCPUs,
				RAMGB:       resource.Spec.RAMGB,
				StorageGB:   resource.Spec.StorageGB,
				Replicas:    resource.Spec.Replicas,
				ExtraConfig: resource.Spec.ExtraConfig,
			},
			Status:    string(resource.Status),
			Endpoint:  resource.Endpoint,
			CreatedAt: resource.CreatedAt,
			UpdatedAt: resource.UpdatedAt,
		})
	}
}

// DeleteResource handles DELETE /tenants/{id}/resources/{resource_id}.
func DeleteResource(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceID, ok := extractPathParam(r, "resource_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "resource id is required")
			return
		}

		err := h.ResourceStore.Delete(resourceID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "resource not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Billing handlers ────────────────────────────────────────────────────────

// ListInvoices handles GET /tenants/{id}/billing/invoices.
func ListInvoices(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		items, total, hasMore := h.BillingStore.GetByTenant(tenantID, 1, 20)

		resp := InvoiceListResponse{
			Items:   make([]*InvoiceResponse, len(items)),
			Total:   total,
			HasMore: hasMore,
		}
		for i, inv := range items {
			resp.Items[i] = &InvoiceResponse{
				ID:             inv.ID,
				TenantID:       inv.TenantID,
				SubscriptionID: inv.SubscriptionID,
				IssueDate:      inv.IssueDate,
				DueDate:        inv.DueDate,
				DueDateRaw:     inv.DueDateRaw,
				Amount:         inv.Amount,
				Currency:       inv.Currency,
				Status:         string(inv.Status),
				LineItems:      toInvoiceLineItems(inv.LineItems),
				PaidAt:         inv.PaidAt,
				CreatedAt:      inv.CreatedAt,
				UpdatedAt:      inv.UpdatedAt,
			}
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// GetInvoice handles GET /tenants/{id}/billing/invoices/{invoice_id}.
func GetInvoice(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		invoiceID, ok := extractPathParam(r, "invoice_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "invoice id is required")
			return
		}

		invoice, err := h.BillingStore.GetByIDAndTenant(invoiceID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "invoice not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, InvoiceResponse{
			ID:             invoice.ID,
			TenantID:       invoice.TenantID,
			SubscriptionID: invoice.SubscriptionID,
			IssueDate:      invoice.IssueDate,
			DueDate:        invoice.DueDate,
			DueDateRaw:     invoice.DueDateRaw,
			Amount:         invoice.Amount,
			Currency:       invoice.Currency,
			Status:         string(invoice.Status),
			LineItems:      toInvoiceLineItems(invoice.LineItems),
			PaidAt:         invoice.PaidAt,
			CreatedAt:      invoice.CreatedAt,
			UpdatedAt:      invoice.UpdatedAt,
		})
	}
}

// UpdateInvoice handles PATCH /tenants/{id}/billing/invoices/{invoice_id}.
func UpdateInvoice(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoiceID, ok := extractPathParam(r, "invoice_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "invoice id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.InvoiceUpdateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		invoice, err := h.BillingStore.Update(invoiceID, store.InvoiceUpdateRequest{
			Status: store.BillingStatus(req.Status),
		})
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "invoice not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, InvoiceResponse{
			ID:        invoice.ID,
			TenantID:  invoice.TenantID,
			Status:    string(invoice.Status),
			DueDate:   invoice.DueDate,
			DueDateRaw: invoice.DueDateRaw,
			Amount:    invoice.Amount,
			Currency:  invoice.Currency,
			PaidAt:    invoice.PaidAt,
			CreatedAt: invoice.CreatedAt,
			UpdatedAt: invoice.UpdatedAt,
		})
	}
}

// ─── Subscription handlers ───────────────────────────────────────────────────

// GetSubscription handles GET /tenants/{id}/subscriptions.
func GetSubscription(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		sub, err := h.SubscriptionStore.GetByTenant(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "subscription not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, SubscriptionResponse{
			ID:                 sub.ID,
			TenantID:           sub.TenantID,
			Plan:               string(sub.Plan),
			PlanName:           sub.PlanName,
			Status:             string(sub.Status),
			BillingCycle:       string(sub.BillingCycle),
			SeatCount:          sub.SeatCount,
			UnitPrice:          sub.UnitPrice,
			TotalAmount:        sub.TotalAmount,
			Currency:           sub.Currency,
			CurrentPeriodStart: sub.CurrentPeriodStart,
			CurrentPeriodEnd:   sub.CurrentPeriodEnd,
			NextBillingDate:    sub.NextBillingDate,
			CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
			CancelledAt:        sub.CancelledAt,
			CreatedAt:          sub.CreatedAt,
			UpdatedAt:          sub.UpdatedAt,
		})
	}
}

// ListSubscriptions handles GET /tenants/{id}/subscriptions (paginated list).
func ListSubscriptions(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		// Extract pagination params (default to page=1, page_size=20)
		page := 1
		pageSize := 20
		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := parsePositiveInt(p); err == nil && n > 0 {
				page = n
			}
		}
		if ps := r.URL.Query().Get("page_size"); ps != "" {
			if n, err := parsePositiveInt(ps); err == nil && n > 0 {
				pageSize = n
			}
		}

		// SubscriptionStore.List() returns all subscriptions, we filter by tenant
		allSubs := h.SubscriptionStore.List()

		// Filter by tenant
		var tenantSubs []*SubscriptionResponse
		for _, sub := range allSubs {
			if sub.TenantID == tenantID {
				tenantSubs = append(tenantSubs, &SubscriptionResponse{
					ID:                 sub.ID,
					TenantID:           sub.TenantID,
					Plan:               string(sub.Plan),
					PlanName:           sub.PlanName,
					Status:             string(sub.Status),
					BillingCycle:       string(sub.BillingCycle),
					SeatCount:          sub.SeatCount,
					UnitPrice:          sub.UnitPrice,
					TotalAmount:        sub.TotalAmount,
					Currency:           sub.Currency,
					CurrentPeriodStart: sub.CurrentPeriodStart,
					CurrentPeriodEnd:   sub.CurrentPeriodEnd,
					NextBillingDate:    sub.NextBillingDate,
					CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
					CancelledAt:        sub.CancelledAt,
					CreatedAt:          sub.CreatedAt,
					UpdatedAt:          sub.UpdatedAt,
				})
			}
		}

		total := len(tenantSubs)

		// Apply pagination
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		pagedItems := tenantSubs[start:end]

		resp := SubscriptionListResponse{
			Items: pagedItems,
			Total: total,
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// PatchSubscription handles PATCH /tenants/{id}/subscriptions.
func PatchSubscription(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		sub, err := h.SubscriptionStore.GetByTenant(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "subscription not found", err.Error())
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.SubscriptionUpdateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		updated, err := h.SubscriptionStore.Patch(sub.ID, store.SubscriptionUpdateRequest{
			Plan:         req.Plan,
			BillingCycle: req.BillingCycle,
			SeatCount:    req.SeatCount,
			CustomQuotas: req.CustomQuotas,
		})
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "subscription update failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, SubscriptionResponse{
			ID:                 updated.ID,
			TenantID:           updated.TenantID,
			Plan:               string(updated.Plan),
			PlanName:           updated.PlanName,
			Status:             string(updated.Status),
			BillingCycle:       string(updated.BillingCycle),
			SeatCount:          updated.SeatCount,
			UnitPrice:          updated.UnitPrice,
			TotalAmount:        updated.TotalAmount,
			Currency:           updated.Currency,
			CurrentPeriodStart: updated.CurrentPeriodStart,
			CurrentPeriodEnd:   updated.CurrentPeriodEnd,
			NextBillingDate:    updated.NextBillingDate,
			CancelAtPeriodEnd:  updated.CancelAtPeriodEnd,
			CancelledAt:        updated.CancelledAt,
			CreatedAt:          updated.CreatedAt,
			UpdatedAt:          updated.UpdatedAt,
		})
	}
}

// CancelSubscription handles POST /tenants/{id}/subscriptions/cancel.
func CancelSubscription(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		sub, err := h.SubscriptionStore.GetByTenant(tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "subscription not found", err.Error())
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.SubscriptionCancelRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		cancelled, err := h.SubscriptionStore.Cancel(sub.ID, store.SubscriptionCancelRequest{
			CancelAtPeriodEnd: req.CancelAtPeriodEnd,
			Reason:            req.Reason,
		})
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "subscription cancellation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, SubscriptionResponse{
			ID:                 cancelled.ID,
			TenantID:           cancelled.TenantID,
			Plan:               string(cancelled.Plan),
			PlanName:           cancelled.PlanName,
			Status:             string(cancelled.Status),
			BillingCycle:       string(cancelled.BillingCycle),
			SeatCount:          cancelled.SeatCount,
			UnitPrice:          cancelled.UnitPrice,
			TotalAmount:        cancelled.TotalAmount,
			Currency:           cancelled.Currency,
			CurrentPeriodStart: cancelled.CurrentPeriodStart,
			CurrentPeriodEnd:   cancelled.CurrentPeriodEnd,
			NextBillingDate:    cancelled.NextBillingDate,
			CancelAtPeriodEnd:  cancelled.CancelAtPeriodEnd,
			CancelledAt:        cancelled.CancelledAt,
			CreatedAt:          cancelled.CreatedAt,
			UpdatedAt:          cancelled.UpdatedAt,
		})
	}
}

// GetSubscriptionByID handles GET /tenants/{id}/subscriptions/{subscription_id}.
func GetSubscriptionByID(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		subID, ok := extractPathParam(r, "subscription_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "subscription id is required")
			return
		}

		sub, err := h.SubscriptionStore.GetByIDAndTenant(subID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "subscription not found", err.Error())
			return
		}

		resp := SubscriptionResponse{
			ID:                 sub.ID,
			TenantID:           sub.TenantID,
			Plan:               string(sub.Plan),
			PlanName:           sub.PlanName,
			Status:             string(sub.Status),
			BillingCycle:       string(sub.BillingCycle),
			SeatCount:          sub.SeatCount,
			UnitPrice:          sub.UnitPrice,
			TotalAmount:        sub.TotalAmount,
			Currency:           sub.Currency,
			CurrentPeriodStart: sub.CurrentPeriodStart,
			CurrentPeriodEnd:   sub.CurrentPeriodEnd,
			NextBillingDate:    sub.NextBillingDate,
			CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
			CancelledAt:        sub.CancelledAt,
			CreatedAt:          sub.CreatedAt,
			UpdatedAt:          sub.UpdatedAt,
		}
		if sub.CustomQuotas != nil {
			resp.CustomQuotas = &QuotaResponse{
				MaxAgents:              sub.CustomQuotas.MaxAgents,
				MaxWorkflowsPerDay:     sub.CustomQuotas.MaxWorkflowsPerDay,
				MaxStorageGB:           sub.CustomQuotas.MaxStorageGB,
				MaxMonthlyTokens:       sub.CustomQuotas.MaxMonthlyTokens,
				MaxConcurrentWorkflows: sub.CustomQuotas.MaxConcurrentWorkflows,
			}
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// UpdateSubscriptionByID handles PATCH /tenants/{id}/subscriptions/{subscription_id}.
func UpdateSubscriptionByID(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		subID, ok := extractPathParam(r, "subscription_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "subscription id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.SubscriptionUpdateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		_, err = h.SubscriptionStore.GetByIDAndTenant(subID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "subscription not found", err.Error())
			return
		}

		updated, err := h.SubscriptionStore.Patch(subID, store.SubscriptionUpdateRequest{
			Plan:         req.Plan,
			BillingCycle: req.BillingCycle,
			SeatCount:    req.SeatCount,
			CustomQuotas: req.CustomQuotas,
		})
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "subscription update failed", err.Error())
			return
		}

		resp := SubscriptionResponse{
			ID:                 updated.ID,
			TenantID:           updated.TenantID,
			Plan:               string(updated.Plan),
			PlanName:           updated.PlanName,
			Status:             string(updated.Status),
			BillingCycle:       string(updated.BillingCycle),
			SeatCount:          updated.SeatCount,
			UnitPrice:          updated.UnitPrice,
			TotalAmount:        updated.TotalAmount,
			Currency:           updated.Currency,
			CurrentPeriodStart: updated.CurrentPeriodStart,
			CurrentPeriodEnd:   updated.CurrentPeriodEnd,
			NextBillingDate:    updated.NextBillingDate,
			CancelAtPeriodEnd:  updated.CancelAtPeriodEnd,
			CancelledAt:        updated.CancelledAt,
			CreatedAt:          updated.CreatedAt,
			UpdatedAt:          updated.UpdatedAt,
		}
		if updated.CustomQuotas != nil {
			resp.CustomQuotas = &QuotaResponse{
				MaxAgents:              updated.CustomQuotas.MaxAgents,
				MaxWorkflowsPerDay:     updated.CustomQuotas.MaxWorkflowsPerDay,
				MaxStorageGB:           updated.CustomQuotas.MaxStorageGB,
				MaxMonthlyTokens:       updated.CustomQuotas.MaxMonthlyTokens,
				MaxConcurrentWorkflows: updated.CustomQuotas.MaxConcurrentWorkflows,
			}
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// UpgradeSubscription handles POST /tenants/{id}/subscriptions/{subscription_id}/upgrade.
func UpgradeSubscription(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		subID, ok := extractPathParam(r, "subscription_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "subscription id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req middleware.SubscriptionUpgradeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.TargetPlan == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "target_plan is required")
			return
		}

		sub, err := h.SubscriptionStore.GetByIDAndTenant(subID, tenantID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "subscription not found", err.Error())
			return
		}

		// Upgrade subscription plan
		_, err = h.SubscriptionStore.Patch(subID, store.SubscriptionUpdateRequest{
			Plan: req.TargetPlan,
		})
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "subscription upgrade failed", err.Error())
			return
		}

		// Also upgrade tenant's plan and quota
		quotas := store.PlanDefaults(req.TargetPlan)
		_, err = h.TenantStore.Patch(tenantID, store.TenantPatchRequest{
			Plan:  req.TargetPlan,
			Quota: &quotas,
		})
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "tenant plan upgrade failed", err.Error())
			return
		}

		resp := UpgradePlanResponse{
			TenantID:       tenantID,
			OldPlan:        string(sub.Plan),
			NewPlan:        string(req.TargetPlan),
			Status:         "upgraded",
			EffectiveDate:  time.Now(),
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}
