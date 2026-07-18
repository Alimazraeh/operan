// Bootstrap tenant — creates the first tenant on a clean install.
// POST /api/v1/tenants/bootstrap
// Returns the created tenant or an error if tenants already exist.
package handler

import (
	"net/http"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// BootstrapTenant handles POST /api/v1/tenants/bootstrap.
// This endpoint creates the initial default-tenant if no tenants exist.
func BootstrapTenant(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if any tenants exist
		count := h.TenantStore.CountTotal()
		if count > 0 {
			// Return the first tenant
			tenants, _, _ := h.TenantStore.List(1, 1, nil)
			if len(tenants) > 0 {
				h.WriteJSON(w, http.StatusOK, tenantResponse(tenants[0]))
				return
			}
		}

		// Create the default tenant
		defaultTenant := &store.Tenant{
			Name:        "default-tenant",
			DisplayName: "Default Tenant",
			Plan:        store.PlanSaaS,
			Region:      store.RegionMEAST1,
			IsolationLevel: store.IsolationNamespace,
			Status:      store.TenantStatusActive,
			Quota:       store.PlanDefaults(store.PlanSaaS),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}

		created, err := h.TenantStore.Create(defaultTenant)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "bootstrap failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, tenantResponse(created))
	}
}

// BootstrapResponse is the response for POST /api/v1/tenants/bootstrap.
type BootstrapResponse struct {
	Tenant  *store.Tenant `json:"tenant"`
	Message string        `json:"message"`
}