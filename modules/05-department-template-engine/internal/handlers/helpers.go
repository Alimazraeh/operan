package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// ─── Response helpers ────────────────────────────────────────────────────────

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an RFC 7807 Problem Details response.
func writeError(w http.ResponseWriter, status int, typ, title, detail, instance, reqID string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":       typ,
		"title":      title,
		"status":     status,
		"detail":     detail,
		"instance":   instance,
		"request_id": reqID,
	})
}

// ─── DTO-to-JSON response builders ──────────────────────────────────────────

func toTemplateResponse(tmpl *store.Template) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          tmpl.ID,
		"tenant_id":   tmpl.TenantID,
		"name":        tmpl.Name,
		"description": tmpl.Description,
		"category":    tmpl.Category,
		"version":     tmpl.Version,
		"status":      tmpl.Status,
		"created_at":  tmpl.CreatedAt,
		"updated_at":  tmpl.UpdatedAt,
		"created_by":  tmpl.CreatedBy,
	}
	if tmpl.Agents != nil {
		resp["agents"] = tmpl.Agents
	}
	if tmpl.Workflows != nil {
		resp["workflows"] = tmpl.Workflows
	}
	if tmpl.MemoryTopology != nil {
		resp["memory_topology"] = tmpl.MemoryTopology
	}
	if tmpl.GovernanceRules != nil {
		resp["governance_rules"] = tmpl.GovernanceRules
	}
	if tmpl.KPIS != nil {
		resp["kpis"] = tmpl.KPIS
	}
	if tmpl.Integrations != nil {
		resp["integrations"] = tmpl.Integrations
	}
	if tmpl.OperationalPolicies != nil {
		resp["operational_policies"] = tmpl.OperationalPolicies
	}
	if tmpl.Metadata != nil {
		resp["metadata"] = tmpl.Metadata
	}
	if tmpl.Tags != nil {
		resp["tags"] = tmpl.Tags
	}
	return resp
}

func toTemplateListResponse(templates []store.Template) []interface{} {
	result := make([]interface{}, 0, len(templates))
	for _, t := range templates {
		resp := map[string]interface{}{
			"id":              t.ID,
			"tenant_id":       t.TenantID,
			"name":            t.Name,
			"description":     t.Description,
			"category":        t.Category,
			"version":         t.Version,
			"status":          t.Status,
			"created_at":      t.CreatedAt,
			"updated_at":      t.UpdatedAt,
			"created_by":      t.CreatedBy,
			"agents_count":       len(t.Agents),
			"workflows_count":  len(t.Workflows),
			"integrations_count": len(t.Integrations),
		}
		if t.Tags != nil {
			resp["tags"] = t.Tags
		}
		result = append(result, resp)
	}
	return result
}

func toCustomTemplateResponse(ct *store.CustomTemplate) map[string]interface{} {
	return map[string]interface{}{
		"id":          ct.ID,
		"tenant_id":   ct.TenantID,
		"name":        ct.Name,
		"description": ct.Description,
		"category":    ct.Category,
		"owner_id":    ct.OwnerID,
		"shared_with": ct.SharedWith,
		"version":     ct.Version,
		"status":      ct.Status,
		"created_at":  ct.CreatedAt,
		"updated_at":  ct.UpdatedAt,
		"created_by":  ct.CreatedBy,
	}
}

func toCustomTemplateListResponse(templates []store.CustomTemplate) []interface{} {
	result := make([]interface{}, 0, len(templates))
	for _, ct := range templates {
		resp := map[string]interface{}{
			"id":          ct.ID,
			"tenant_id":   ct.TenantID,
			"name":        ct.Name,
			"description": ct.Description,
			"category":    ct.Category,
			"owner_id":    ct.OwnerID,
			"status":      ct.Status,
			"version":     ct.Version,
			"created_at":  ct.CreatedAt,
			"updated_at":  ct.UpdatedAt,
		}
		result = append(result, resp)
	}
	return result
}

func toDeploymentResponse(d *store.TemplateDeployment) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          d.ID,
		"tenant_id":   d.TenantID,
		"template_id": d.TemplateID,
		"version":     d.Version,
		"status":      d.Status,
		"environment": d.Environment,
		"created_at":  d.CreatedAt,
		"updated_at":  d.UpdatedAt,
		"deployed_by": d.DeployedBy,
	}
	if d.Configuration != nil {
		resp["configuration"] = d.Configuration
	}
	if d.ProvisionedEntities != nil {
		resp["provisioned_entities"] = d.ProvisionedEntities
	}
	if d.StartedAt != nil {
		resp["started_at"] = d.StartedAt
	}
	if d.CompletedAt != nil {
		resp["completed_at"] = d.CompletedAt
	}
	if d.ErrorMessage != "" {
		resp["error_message"] = d.ErrorMessage
	}
	return resp
}

func toDeploymentListResponse(deployments []store.TemplateDeployment) []interface{} {
	result := make([]interface{}, 0, len(deployments))
	for _, d := range deployments {
		resp := map[string]interface{}{
			"id":          d.ID,
			"tenant_id":   d.TenantID,
			"template_id": d.TemplateID,
			"version":     d.Version,
			"status":      d.Status,
			"environment": d.Environment,
			"created_at":  d.CreatedAt,
			"updated_at":  d.UpdatedAt,
			"deployed_by": d.DeployedBy,
		}
		result = append(result, resp)
	}
	return result
}

// ─── URL path helpers ───────────────────────────────────────────────────────

// extractIDFromPath extracts a resource ID from a URL path like "/templates/{id}".
// If the prefix doesn't match, returns empty string.
func extractIDFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	id := strings.TrimPrefix(path, prefix)
	// Handle sub-paths (e.g., /templates/{id}/deployments)
	id = strings.SplitN(id, "/", 2)[0]
	return id
}

// extractTemplateIDFromNestedPath extracts the template ID from nested paths.
// For paths like /templates/{id}/deployments, /templates/{id}/versions, /templates/{id}/clone,
// it extracts {id} by splitting on "/" and taking the second element.
// Example: "/templates/abc123/deployments" → "abc123"
func extractTemplateIDFromNestedPath(path string) string {
	parts := strings.Split(path, "/")
	// Expected format: "" "templates" "{id}" ...
	if len(parts) >= 3 && parts[1] == "templates" {
		return parts[2]
	}
	return ""
}

// parsePositiveInt parses a string as a positive integer, defaulting to 1 on error.
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 1, nil
	}
	return n, nil
}
