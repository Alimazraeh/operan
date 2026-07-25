package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/operan/modules/08-tool-execution/internal/funnel"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// CapabilityHandlers exposes the capability layer: the vocabulary, a tenant's
// providers and bindings, the governed invoke funnel, and the invocation
// audit trail.
type CapabilityHandlers struct {
	Funnel      *funnel.Funnel
	MaxPageSize int
}

// ListCapabilities handles GET /capabilities — the product vocabulary.
func (h *CapabilityHandlers) ListCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": h.Funnel.Capabilities.List(),
	})
}

// ListProviders handles GET /providers.
func (h *CapabilityHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": h.Funnel.Providers.List(tenantID),
	})
}

// CreateProvider handles POST /providers. Credentials are never accepted
// inline — a provider carries a credential_ref, not a secret.
func (h *CapabilityHandlers) CreateProvider(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	var p store.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	switch p.Kind {
	case "simulated", "mcp", "native", "http":
	default:
		writeError(w, r, http.StatusBadRequest, "kind must be simulated, mcp, native or http")
		return
	}
	if p.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required")
		return
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	p.TenantID = tenantID
	if p.Status == "" {
		p.Status = "active"
	}
	h.Funnel.Providers.Put(&p)
	writeJSON(w, http.StatusCreated, p)
}

// ListBindings handles GET /bindings.
func (h *CapabilityHandlers) ListBindings(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bindings": h.Funnel.Bindings.List(tenantID),
	})
}

// PutBinding handles POST /bindings — create or update the join between a
// capability and the provider that performs it.
func (h *CapabilityHandlers) PutBinding(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	var b store.CapabilityBinding
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, ok := h.Funnel.Capabilities.Get(b.CapabilityID); !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "unknown capability "+b.CapabilityID)
		return
	}
	provider, ok := h.Funnel.Providers.Get(tenantID, b.ProviderID)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "unknown provider "+b.ProviderID)
		return
	}
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	b.TenantID = tenantID
	if b.ProviderTool == "" {
		b.ProviderTool = b.CapabilityID
	}
	// A binding through a simulated provider is simulated no matter what the
	// caller claims — the flag is derived, not asserted.
	b.Simulated = provider.Kind == "simulated"
	h.Funnel.Bindings.Put(&b)
	writeJSON(w, http.StatusCreated, b)
}

// Invoke handles POST /invoke — the single governed door.
func (h *CapabilityHandlers) Invoke(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	var req funnel.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	inv, err := h.Funnel.Invoke(r.Context(), r.Header.Get("Authorization"), tenantID, req)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// Refusals are recorded outcomes, not transport errors: the caller gets
	// 200 with the invocation stating exactly which stage stopped it and why.
	writeJSON(w, http.StatusOK, inv)
}

// ListInvocations handles GET /invocations — the audit trail.
func (h *CapabilityHandlers) ListInvocations(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	q := r.URL.Query()
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n := atoiOr(v, 50); n > 0 {
			limit = n
		}
	}
	items := h.Funnel.Invocations.List(tenantID, q.Get("capability_id"), q.Get("status"), q.Get("request_id"), limit)
	if items == nil {
		items = []*store.Invocation{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"invocations": items,
		"total":       len(items),
	})
}

func atoiOr(s string, fallback int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}
