package handlers

import (
	"net/http"

	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/seed"
	"github.com/operan/modules/05-department-template-engine/internal/validate"
)

// SeedTemplates handles POST /templates/seed — idempotently copies any
// missing built-in catalog templates into the caller's tenant.
func (h *TemplateHandlers) SeedTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	seeded := seed.EnsureTenant(h.TemplateStore, tenantID, middleware.UserIDFromContext(r.Context()))
	if seeded == nil {
		seeded = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"seeded":       seeded,
		"catalog_size": len(seed.Catalog()),
	})
}

// handleValidate handles POST /templates/{id}/validate — runs the same
// validation the deploy orchestrator runs at the configure stage.
func (h *TemplateHandlers) handleValidate(w http.ResponseWriter, r *http.Request, reqID, templateID string) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	tmpl, err := h.TemplateStore.GetByIDAndTenant(templateID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"Template not found", r.URL.Path, reqID)
		return
	}

	issues := validate.Template(tmpl)
	var errs, warns []validate.Issue
	for _, i := range issues {
		if i.Severity == "error" {
			errs = append(errs, i)
		} else {
			warns = append(warns, i)
		}
	}
	if errs == nil {
		errs = []validate.Issue{}
	}
	if warns == nil {
		warns = []validate.Issue{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":    len(errs) == 0,
		"errors":   errs,
		"warnings": warns,
	})
}
