// Package handler handles secrets and status endpoints.
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/config"
	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

var startTime = time.Now()

// ─── Secret handlers ─────────────────────────────────────────────────────────

// ListSecrets handles GET /tenants/{id}/secrets.
func ListSecrets(h *middleware.Handler) http.HandlerFunc {
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

		items, total, hasMore := h.SecretStore.List(tenantID, 1, 20)

		resp := SecretListResponse{
			Items:   make([]*SecretMetadataResponse, len(items)),
			Total:   total,
			HasMore: hasMore,
		}
		for i, s := range items {
			resp.Items[i] = &SecretMetadataResponse{
				ID:           s.ID,
				Key:          s.Key,
				Description:  s.Description,
				Tags:         s.Tags,
				CreatedAt:    s.CreatedAt,
				UpdatedAt:    s.UpdatedAt,
				Version:      s.Version,
				VersionCount: s.VersionCount,
			}
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// CreateSecret handles POST /tenants/{id}/secrets.
func CreateSecret(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
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
			Key         string   `json:"key"`
			Value       string   `json:"value"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.Key == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "key is required")
			return
		}

		secret, err := h.SecretStore.Create(tenantID, req.Key, req.Value, req.Description, req.Tags)
		if err != nil {
			h.WriteError(w, http.StatusConflict, 409, "secret creation failed", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, SecretResponse{
			ID:             secret.ID,
			Key:            secret.Key,
			Value:          secret.Value,
			EncryptedValue: secret.EncryptedValue,
			Description:    secret.Description,
			Tags:           secret.Tags,
			Version:        secret.Version,
			CreatedAt:      secret.CreatedAt,
			UpdatedAt:      secret.UpdatedAt,
		})
	}
}

// GetSecret handles GET /tenants/{id}/secrets/{secret_id}.
func GetSecret(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secretID, ok := extractPathParam(r, "secret_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "secret id is required")
			return
		}

		secret, err := h.SecretStore.GetByID(secretID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "secret not found", err.Error())
			return
		}

		// Count versions
		verCount := secret.Version
		items, _, _ := h.SecretStore.List(secret.ID, 1, 1000)
		for _, s := range items {
			if s.Version > verCount {
				verCount = s.Version
			}
		}

		h.WriteJSON(w, http.StatusOK, SecretResponse{
			ID:             secret.ID,
			Key:            secret.Key,
			Value:          secret.Value,
			EncryptedValue: secret.EncryptedValue,
			Description:    secret.Description,
			Tags:           secret.Tags,
			Version:        secret.Version,
			VersionCount:   verCount,
			CreatedAt:      secret.CreatedAt,
			UpdatedAt:      secret.UpdatedAt,
		})
	}
}

// UpdateSecret handles PATCH /tenants/{id}/secrets/{secret_id}.
func UpdateSecret(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secretID, ok := extractPathParam(r, "secret_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "secret id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		secret, err := h.SecretStore.Update(secretID, req.Description, req.Tags)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "secret not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, SecretMetadataResponse{
			ID:          secret.ID,
			Key:         secret.Key,
			Description: secret.Description,
			Tags:        secret.Tags,
			CreatedAt:   secret.CreatedAt,
			UpdatedAt:   secret.UpdatedAt,
			Version:     secret.Version,
		})
	}
}

// DeleteSecret handles DELETE /tenants/{id}/secrets/{secret_id}.
func DeleteSecret(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secretID, ok := extractPathParam(r, "secret_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "secret id is required")
			return
		}

		err := h.SecretStore.Delete(secretID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "secret not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// RotateSecret handles POST /tenants/{id}/secrets/{secret_id}/rotate.
func RotateSecret(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secretID, ok := extractPathParam(r, "secret_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "secret id is required")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request body", "failed to read request body")
			return
		}

		var req struct {
			NewValue string `json:"new_value"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid JSON", err.Error())
			return
		}

		if req.NewValue == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "new_value is required")
			return
		}

		secret, err := h.SecretStore.Rotate(secretID, req.NewValue)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "secret not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, SecretResponse{
			ID:             secret.ID,
			Key:            secret.Key,
			Value:          secret.Value,
			EncryptedValue: secret.EncryptedValue,
			Description:    secret.Description,
			Tags:           secret.Tags,
			Version:        secret.Version,
			CreatedAt:      secret.CreatedAt,
			UpdatedAt:      secret.UpdatedAt,
		})
	}
}

// ─── Status endpoint ─────────────────────────────────────────────────────────

// GetModuleStatus handles GET /status.
func GetModuleStatus(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := ModuleStatusResponse{
			Service:   "tenant-control-plane",
			Version:   config.ParseConfig().Version,
			Status:    "running",
			Uptime:    time.Since(startTime).String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Health:    "healthy",
		}

		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// extractPathParam extracts a path parameter from the request URL path.
// Go 1.22+ uses method patterns with named parameters, but we need a
// simple implementation for the mux.
func extractPathParam(r *http.Request, name string) (string, bool) {
	path := r.URL.Path
	rawParts := strings.Split(strings.Trim(path, "/"), "/")
	n := len(rawParts)

	switch n {
	case 1:
		// /status, /health
		switch path {
		case "status":
			if name == "id" {
				return rawParts[0], true
			}
		case "health":
			if name == "kind" {
				return rawParts[0], true
			}
		}

	case 2:
		// /tenants/{id}
		if rawParts[0] == "tenants" {
			switch name {
			case "id", "tenant_id":
				return rawParts[1], true
			}
		}

	case 3:
		// /tenants/{id}/status, /tenants/{id}/subscriptions, /tenants/{id}/agents, /tenants/{id}/resources
		if rawParts[0] == "tenants" {
			switch rawParts[2] {
			case "status":
				if name == "id" || name == "tenant_id" {
					return rawParts[1], true
				}
			case "subscriptions":
				if name == "id" || name == "tenant_id" {
					return rawParts[1], true
				}
			case "agents":
				if name == "id" || name == "tenant_id" {
					return rawParts[1], true
				}
			case "resources":
				if name == "id" || name == "tenant_id" {
					return rawParts[1], true
				}
			}
		}

	case 4:
		if rawParts[0] == "tenants" {
			switch rawParts[2] {
			case "status":
				// /tenants/{id}/status/transition
				if rawParts[3] == "transition" {
					if name == "id" || name == "tenant_id" {
						return rawParts[1], true
					}
				}
			case "agents":
				// /tenants/{id}/agents/{agent_id}
				switch name {
				case "id", "tenant_id":
					return rawParts[1], true
				case "agent_id":
					return rawParts[3], true
				}
			case "resources":
				// /tenants/{id}/resources/{resource_id}
				switch name {
				case "id", "tenant_id":
					return rawParts[1], true
				case "resource_id":
					return rawParts[3], true
				}
			case "subscriptions":
				// /tenants/{id}/subscriptions/cancel
				if rawParts[3] == "cancel" {
					if name == "id" || name == "tenant_id" {
						return rawParts[1], true
					}
				}
			case "secrets":
				// /tenants/{id}/secrets/{secret_id}
				switch name {
				case "id", "tenant_id":
					return rawParts[1], true
				case "secret_id":
					return rawParts[3], true
				}
			case "billing":
				// /tenants/{id}/billing/invoices
				if rawParts[3] == "invoices" {
					if name == "id" || name == "tenant_id" {
						return rawParts[1], true
					}
				}
			}
		}

	case 5:
		if rawParts[0] == "tenants" {
			switch rawParts[2] {
			case "agents":
				// /tenants/{id}/agents/{agent_id}/status
				if rawParts[4] == "status" {
					switch name {
					case "id", "tenant_id":
						return rawParts[1], true
					case "agent_id":
						return rawParts[3], true
					}
				}
			case "resources":
				// /tenants/{id}/resources/{resource_id}/status
				if rawParts[4] == "status" {
					switch name {
					case "id", "tenant_id":
						return rawParts[1], true
					case "resource_id":
						return rawParts[3], true
					}
				}
			case "secrets":
				// /tenants/{id}/secrets/{secret_id}/status or /keys
				if rawParts[4] == "status" || rawParts[4] == "keys" {
					switch name {
					case "id", "tenant_id":
						return rawParts[1], true
					case "secret_id":
						return rawParts[3], true
					}
				}
			}
		}
		// /tenants/{id}/billing/invoices/{invoice_id}
		if rawParts[2] == "billing" && rawParts[3] == "invoices" {
			switch name {
			case "id", "tenant_id":
				return rawParts[1], true
			case "invoice_id":
				return rawParts[4], true
			}
		}

	case 6:
		// /tenants/{id}/billing/invoices/{invoice_id}/lines
		if rawParts[0] == "tenants" && rawParts[2] == "billing" && rawParts[3] == "invoices" && rawParts[5] == "lines" {
			switch name {
			case "id", "tenant_id":
				return rawParts[1], true
			case "invoice_id":
				return rawParts[4], true
			}
		}
	}

	return "", false
}

// toInvoiceLineItems converts store line items to response type.
func toInvoiceLineItems(items []store.InvoiceLineItem) []InvoiceLineItemResponse {
	result := make([]InvoiceLineItemResponse, len(items))
	for i, item := range items {
		result[i] = InvoiceLineItemResponse{
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Amount:      item.Amount,
		}
	}
	return result
}

// parsePositiveInt parses a string as a positive integer.
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid positive integer: %s", s)
	}
	return n, nil
}
