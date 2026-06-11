package handlers

import "net/http"

// RegisterRoutes registers all Module 09 endpoints on the given ServeMux
// (Go 1.22+ pattern syntax with typed method + path wildcards).
func RegisterRoutes(mux *http.ServeMux, h *SupervisionHandlers) {
	// Approvals
	mux.HandleFunc("POST /approvals", h.CreateApproval)
	mux.HandleFunc("GET /approvals/{id}", h.GetApproval)
	mux.HandleFunc("PATCH /approvals/{id}", h.UpdateApproval)
	mux.HandleFunc("DELETE /approvals/{id}", h.DeleteApproval)
	mux.HandleFunc("POST /approvals/{id}/approve", h.ApproveApproval)
	mux.HandleFunc("POST /approvals/{id}/reject", h.RejectApproval)
	mux.HandleFunc("POST /approvals/{id}/delegate", h.DelegateApproval)

	// Escalations
	mux.HandleFunc("POST /escalations", h.CreateEscalation)
	mux.HandleFunc("GET /escalations/{id}", h.GetEscalation)
	mux.HandleFunc("PATCH /escalations/{id}", h.UpdateEscalation)
	mux.HandleFunc("DELETE /escalations/{id}", h.DeleteEscalation)
	mux.HandleFunc("POST /escalations/{id}/resolve", h.ResolveEscalation)

	// Interventions
	mux.HandleFunc("POST /interventions", h.CreateIntervention)
	mux.HandleFunc("GET /interventions/{id}", h.GetIntervention)
	mux.HandleFunc("PATCH /interventions/{id}", h.UpdateIntervention)
	mux.HandleFunc("DELETE /interventions/{id}", h.DeleteIntervention)
	mux.HandleFunc("POST /interventions/{id}/revoke", h.RevokeIntervention)

	// Queue, dashboard, HITL
	mux.HandleFunc("GET /queue", h.GetHumanQueue)
	mux.HandleFunc("GET /risk-dashboard", h.GetRiskDashboard)
	mux.HandleFunc("POST /hitl/{request_id}/answer", h.SubmitHitlAnswer)
}
