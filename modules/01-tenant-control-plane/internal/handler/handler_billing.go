// Package handler implements HTTP request handlers for tenant-control-plane billing and quota endpoints.
package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Quota handlers ──────────────────────────────────────────────────────────

// GetTenantQuota handles GET /tenants/{id}/quota.
func GetTenantQuota(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		tenant, err := h.TenantStore.GetByID(id)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		resp := QuotaCheckResponse{
			TenantID:  id,
			QuotaType: "tenant_quota",
			Current:   tenant.Quota.MaxAgents,
			Limit:     tenant.Quota.MaxAgents,
			Allowed:   true,
			CheckedAt: time.Now(),
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// PatchTenantQuota handles PATCH /tenants/{id}/quota.
func PatchTenantQuota(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			MaxAgents              *int `json:"max_agents"`
			MaxWorkflowsPerDay     *int `json:"max_workflows_per_day"`
			MaxStorageGB           *int `json:"max_storage_gb"`
			MaxMonthlyTokens       *int `json:"max_monthly_tokens"`
			MaxConcurrentWorkflows *int `json:"max_concurrent_workflows"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		tenant, err := h.TenantStore.GetByID(id)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		if req.MaxAgents != nil && *req.MaxAgents < 0 {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "max_agents must be non-negative")
			return
		}
		if req.MaxWorkflowsPerDay != nil && *req.MaxWorkflowsPerDay < 0 {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "max_workflows_per_day must be non-negative")
			return
		}
		if req.MaxStorageGB != nil && *req.MaxStorageGB < 0 {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "max_storage_gb must be non-negative")
			return
		}
		if req.MaxMonthlyTokens != nil && *req.MaxMonthlyTokens < 0 {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "max_monthly_tokens must be non-negative")
			return
		}
		if req.MaxConcurrentWorkflows != nil && *req.MaxConcurrentWorkflows < 0 {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "max_concurrent_workflows must be non-negative")
			return
		}

		if req.MaxAgents != nil {
			tenant.Quota.MaxAgents = *req.MaxAgents
		}
		if req.MaxWorkflowsPerDay != nil {
			tenant.Quota.MaxWorkflowsPerDay = *req.MaxWorkflowsPerDay
		}
		if req.MaxStorageGB != nil {
			tenant.Quota.MaxStorageGB = *req.MaxStorageGB
		}
		if req.MaxMonthlyTokens != nil {
			tenant.Quota.MaxMonthlyTokens = *req.MaxMonthlyTokens
		}
		if req.MaxConcurrentWorkflows != nil {
			tenant.Quota.MaxConcurrentWorkflows = *req.MaxConcurrentWorkflows
		}

		tenant.UpdatedAt = time.Now()
		updated, err := h.TenantStore.Patch(id, store.TenantPatchRequest{
			Quota:            &tenant.Quota,
			ContactEmail:     tenant.ContactEmail,
			AdminEmail:       tenant.AdminEmail,
			CustomMetadata:   tenant.CustomMetadata,
			Name:             tenant.Name,
			DisplayName:      tenant.DisplayName,
			Status:           tenant.Status,
			Plan:             tenant.Plan,
			Region:           tenant.Region,
			IsolationLevel:   tenant.IsolationLevel,
		})
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "quota update failed", err.Error())
			return
		}

		resp := QuotaCheckResponse{
			TenantID:        id,
			QuotaType:       "tenant_quota",
			Current:         updated.Quota.MaxAgents,
			Limit:           updated.Quota.MaxAgents,
			Allowed:         true,
			ActionTaken:     "quota_updated",
			Reason:          "tenant quota updated via API",
			CheckedAt:       time.Now(),
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// ─── Billing handlers ────────────────────────────────────────────────────────

// GetBillingUsage handles GET /tenants/{id}/billing/usage.
func GetBillingUsage(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		_, err := h.TenantStore.GetByID(id)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		periodStart := time.Now().AddDate(0, 0, -30)
		resp := UsageResponse{
			ID:            "usage-" + id,
			TenantID:      id,
			PeriodStart:   periodStart,
			PeriodEnd:     time.Now(),
			Metrics:       map[string]interface{}{},
			UsageGB:       0,
			EstimatedCost: 0,
			Currency:    "USD",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// DownloadInvoice handles GET /tenants/{id}/billing/invoices/{invoice_id}/download.
func DownloadInvoice(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		invoiceID, ok := extractPathParam(r, "invoice_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "invoice id is required")
			return
		}

		_, err := h.BillingStore.GetByID(invoiceID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "invoice not found", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=invoice-"+invoiceID+".pdf")
		w.WriteHeader(http.StatusOK)
		// In production, this would stream the actual PDF content
		_, _ = w.Write([]byte("PDF content for invoice " + invoiceID))
	}
}

// ─── Payment Method handlers ─────────────────────────────────────────────────

// ListPaymentMethods handles GET /tenants/{id}/billing/payment-methods.
func ListPaymentMethods(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		page := 1
		pageSize := 20
		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := parsePositiveInt(p); err == nil {
				page = n
			}
		}
		if ps := r.URL.Query().Get("page_size"); ps != "" {
			if n, err := parsePositiveInt(ps); err == nil {
				pageSize = n
			}
		}

		items, total, hasMore := h.PaymentMethodStore.List(page, pageSize)

		resp := PaymentMethodListResponse{
			Items:    make([]*PaymentMethodResponse, len(items)),
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			HasMore:  hasMore,
		}
		for i, pm := range items {
			resp.Items[i] = paymentMethodResponse(pm)
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// CreatePaymentMethod handles POST /tenants/{id}/billing/payment-methods.
func CreatePaymentMethod(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			Type           string `json:"type" validate:"required"`
			LastFour       string `json:"last_four" validate:"required"`
			ExpiryMonth    int    `json:"expiry_month"`
			ExpiryYear     int    `json:"expiry_year"`
			BillingAddress string `json:"billing_address,omitempty"`
			SetAsDefault   bool   `json:"set_as_default,omitempty"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.Type == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "type is required")
			return
		}
		if req.LastFour == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "last_four is required")
			return
		}

		pm := &store.PaymentMethod{
			Type:           store.PaymentMethodType(req.Type),
			LastFour:       req.LastFour,
			ExpiryMonth:    req.ExpiryMonth,
			ExpiryYear:     req.ExpiryYear,
			BillingAddress: req.BillingAddress,
			IsDefault:      req.SetAsDefault,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		created, err := h.PaymentMethodStore.Create(pm)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "payment method creation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, paymentMethodResponse(created))
	}
}

// SetDefaultPaymentMethod handles POST /tenants/{id}/billing/payment-methods/{pm_id}/set-default.
func SetDefaultPaymentMethod(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		pmID, ok := extractPathParam(r, "pm_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "payment method id is required")
			return
		}

		pm, err := h.PaymentMethodStore.GetByID(pmID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "payment method not found", err.Error())
			return
		}

		pm.IsDefault = true
		updated, err := h.PaymentMethodStore.Update(pm)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "failed to update payment method", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, paymentMethodResponse(updated))
	}
}

// GetBillingMethod handles GET /tenants/{id}/billing/payment-methods/{method_id}.
func GetBillingMethod(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		methodID, ok := extractPathParam(r, "method_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "method id is required")
			return
		}

		pm, err := h.PaymentMethodStore.GetByID(methodID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "payment method not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, paymentMethodResponse(pm))
	}
}

// UpgradePlan handles POST /tenants/{id}/billing/upgrade-plan.
func UpgradePlan(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			NewPlan store.Plan `json:"new_plan" validate:"required"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.NewPlan == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "new_plan is required")
			return
		}

		tenant, err := h.TenantStore.GetByID(id)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "tenant not found", err.Error())
			return
		}

		oldPlan := string(tenant.Plan)
		tenant.Plan = req.NewPlan
		tenant.Quota = store.PlanDefaults(req.NewPlan)

		_, err = h.TenantStore.Patch(id, store.TenantPatchRequest{
			Plan: req.NewPlan,
		})
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "plan upgrade failed", err.Error())
			return
		}

		resp := UpgradePlanResponse{
			TenantID:       id,
			OldPlan:        oldPlan,
			NewPlan:        string(req.NewPlan),
			Status:         "upgraded",
			ProratedCharge: 0,
			EffectiveDate:  time.Now(),
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func paymentMethodResponse(pm *store.PaymentMethod) *PaymentMethodResponse {
	return &PaymentMethodResponse{
		ID:             pm.ID,
		TenantID:       pm.TenantID,
		Type:           string(pm.Type),
		LastFour:       pm.LastFour,
		ExpiryMonth:    pm.ExpiryMonth,
		ExpiryYear:     pm.ExpiryYear,
		BillingAddress: pm.BillingAddress,
		IsDefault:      pm.IsDefault,
		CreatedAt:      pm.CreatedAt,
		UpdatedAt:      pm.UpdatedAt,
	}
}
