package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// Seats and who holds them.
//
// Position.human_ref has existed since the operating model shipped but was
// display-only. Making it a real Module 02 user id turns the org chart into
// the authorization graph: user → position → department → authority. Approval
// rights then come from Position.decision_rights and approval_gate_refs, which
// the templates already declare, instead of a parallel access-control list
// that would drift from the org model it is meant to describe.

type setHolderBody struct {
	HolderType string `json:"holder_type"` // human | ai_agent | vacant
	HumanRef   string `json:"human_ref"`   // M02 user id, when holder_type is human
	AgentID    string `json:"agent_id"`    // M04 agent id, when holder_type is ai_agent
}

// setPositionHolder handles PUT /departments/{id}/org-chart/{positionId}/holder.
func (h *TemplateHandlers) setPositionHolder(w http.ResponseWriter, r *http.Request, dept *store.Department, positionID string) {
	reqID := middleware.RequestIDFromContext(r.Context())

	var body setHolderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
			"invalid JSON body", r.URL.Path, reqID)
		return
	}
	switch body.HolderType {
	case "human", "ai_agent", "vacant":
	default:
		writeError(w, http.StatusUnprocessableEntity, "about:blank", "Validation failed",
			"holder_type must be human, ai_agent or vacant", r.URL.Path, reqID)
		return
	}

	idx := -1
	for i := range dept.OrgChart {
		if dept.OrgChart[i].ID == positionID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeError(w, http.StatusNotFound, "about:blank", "Not Found",
			"no such position in this department", r.URL.Path, reqID)
		return
	}

	if body.HolderType == "human" {
		if strings.TrimSpace(body.HumanRef) == "" {
			writeError(w, http.StatusUnprocessableEntity, "about:blank", "Validation failed",
				"human_ref is required when holder_type is human", r.URL.Path, reqID)
			return
		}
		// A seat bound to an id nobody can produce is worse than a vacant one:
		// it looks staffed and grants nothing.
		if h.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "about:blank", "Unavailable",
				"identity service not configured — cannot verify the user", r.URL.Path, reqID)
			return
		}
		caller := clients.Caller{
			Authorization: r.Header.Get("Authorization"),
			TenantID:      dept.TenantID,
		}
		if _, err := h.Identity.GetUser(r.Context(), caller, body.HumanRef); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "about:blank", "Validation failed",
				"no such user: "+body.HumanRef, r.URL.Path, reqID)
			return
		}
	}

	updated := dept.OrgChart
	switch body.HolderType {
	case "human":
		updated[idx].HolderType = "human"
		updated[idx].HumanRef = body.HumanRef
		updated[idx].AgentID = ""
	case "ai_agent":
		updated[idx].HolderType = "ai_agent"
		updated[idx].AgentID = body.AgentID
		updated[idx].HumanRef = ""
	case "vacant":
		updated[idx].HolderType = "vacant"
		updated[idx].HumanRef = ""
		updated[idx].AgentID = ""
	}

	saved, err := h.DepartmentStore.UpdateByTenant(dept.ID, dept.TenantID,
		map[string]interface{}{"org_chart": updated})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "about:blank", "Internal Error",
			err.Error(), r.URL.Path, reqID)
		return
	}
	writeJSON(w, http.StatusOK, orgChartResponse(saved))
}

// Assignment is one seat the caller holds, with the authority it carries.
type Assignment struct {
	DepartmentID     string                `json:"department_id"`
	DepartmentName   string                `json:"department_name"`
	DepartmentStatus string                `json:"department_status"`
	PositionID       string                `json:"position_id"`
	Title            string                `json:"title"`
	RoleType         string                `json:"role_type,omitempty"`
	Unit             string                `json:"unit,omitempty"`
	AutonomyTier     string                `json:"autonomy_tier,omitempty"`
	ReportsTo        string                `json:"reports_to,omitempty"`
	IsDepartmentRoot bool                  `json:"is_department_root"`
	DecisionRights   []store.DecisionRight `json:"decision_rights,omitempty"`
	ApprovalGateRefs []string              `json:"approval_gate_refs,omitempty"`
}

// MeAssignments handles GET /me/assignments — every seat the authenticated
// caller holds across the tenant's departments. This is the single query the
// portal needs to decide what somebody may see and do: department scope comes
// from the seats they hold, and approval authority from those seats' declared
// decision rights.
func (h *TemplateHandlers) MeAssignments(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	tenantID := middleware.TenantIDFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "about:blank", "Unauthorized",
			"no authenticated user", r.URL.Path, reqID)
		return
	}

	out := []Assignment{}
	for _, d := range h.DepartmentStore.All() {
		if d.TenantID != tenantID {
			continue
		}
		for _, p := range d.OrgChart {
			if p.HolderType != "human" || p.HumanRef != userID {
				continue
			}
			out = append(out, Assignment{
				DepartmentID: d.ID, DepartmentName: d.Name, DepartmentStatus: d.Status,
				PositionID: p.ID, Title: p.Title, RoleType: p.RoleType, Unit: p.Unit,
				AutonomyTier: p.AutonomyTier, ReportsTo: p.ReportsTo,
				IsDepartmentRoot: p.ReportsTo == "",
				DecisionRights:   p.DecisionRights,
				ApprovalGateRefs: p.ApprovalGateRefs,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"roles":   middleware.UserRolesFromContext(r.Context()),
		"data":    out,
		"meta":    map[string]interface{}{"total": len(out)},
	})
}
