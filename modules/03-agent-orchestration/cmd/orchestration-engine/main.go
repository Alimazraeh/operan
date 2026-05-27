package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/operan/modules/03-agent-orchestration/internal/handler"
	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

func main() {
	// Initialize stores
	wfStore := store.NewWorkflowStore()
	scStore := store.NewScheduleStore()
	agStore := store.NewAgentStore()

	// Initialize handlers
	wfHandler := handler.NewWorkflowHandler(wfStore, scStore, agStore)
	scHandler := handler.NewScheduleHandler(scStore, wfStore, agStore)
	schedHandler := handler.NewSchedulingHandler(agStore, wfStore)

	// ─── Router ────────────────────────────────────────────────────────────

	const base = "/api/v1/orchestration"
	mux := http.NewServeMux()

	// Workflows - list
	mux.HandleFunc("GET "+base+"/workflows", wfHandler.ListWorkflows)
	// Workflows - create
	mux.HandleFunc("POST "+base+"/workflows", wfHandler.CreateWorkflow)
	// Schedules - create
	mux.HandleFunc("POST "+base+"/schedules", scHandler.ScheduleWorkflow)
	// Agents
	mux.HandleFunc("POST "+base+"/agents/assign", schedHandler.AssignAgent)
	mux.HandleFunc("GET "+base+"/agents/availability", schedHandler.GetAgentAvailability)

	// Dynamic route handlers
	mux.HandleFunc(base+"/workflows/", func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/workflows/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "workflow id is required")
			return
		}
		// Inject workflow ID into context for handlers to use
		ctx := r.Context()
		r = r.WithContext(context.WithValue(ctx, "workflow_id", id))

		// /workflows/{id}/state
		if strings.HasSuffix(r.URL.Path, "/state") {
			wfHandler.GetWorkflowState(w, r)
			return
		}
		// /workflows/{id}/checkpoint
		if strings.HasSuffix(r.URL.Path, "/checkpoint") && r.Method == "POST" {
			wfHandler.CreateCheckpoint(w, r)
			return
		}
		// /workflows/{id}/replay
		if strings.HasSuffix(r.URL.Path, "/replay") && r.Method == "POST" {
			wfHandler.ReplayWorkflow(w, r)
			return
		}
		// /workflows/{id}/variables
		if strings.HasSuffix(r.URL.Path, "/variables") {
			if r.Method == "GET" {
				wfHandler.GetWorkflowVariables(w, r)
			} else if r.Method == "PATCH" {
				wfHandler.UpdateWorkflowVariables(w, r)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		// /workflows/{id}/pause
		if strings.HasSuffix(r.URL.Path, "/pause") && r.Method == "POST" {
			wfHandler.PauseWorkflow(w, r)
			return
		}
		// /workflows/{id}/resume
		if strings.HasSuffix(r.URL.Path, "/resume") && r.Method == "POST" {
			wfHandler.ResumeWorkflow(w, r)
			return
		}
		// /workflows/{id} (GET)
		if r.Method == "GET" {
			wfHandler.GetWorkflow(w, r)
			return
		}
		// /workflows/{id} (DELETE)
		if r.Method == "DELETE" {
			wfHandler.CancelWorkflow(w, r)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	mux.HandleFunc(base+"/schedules/", func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/schedules/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "schedule id is required")
			return
		}
		// /schedules/{id}/trigger
		if strings.HasSuffix(r.URL.Path, "/trigger") && r.Method == "POST" {
			scHandler.TriggerSchedule(w, r)
			return
		}
		// /schedules/{id} (GET)
		if r.Method == "GET" {
			scHandler.GetSchedule(w, r)
			return
		}
		// /schedules/{id} (PATCH)
		if r.Method == "PATCH" {
			scHandler.UpdateSchedule(w, r)
			return
		}
		// /schedules/{id} (DELETE)
		if r.Method == "DELETE" {
			scHandler.DeleteSchedule(w, r)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	// ─── Middleware chain ──────────────────────────────────────────────────

	var chain http.Handler = mux
	chain = middleware.Logger(chain)
	chain = middleware.TenantContext(chain)
	chain = middleware.TraceID(chain)
	chain = middleware.RequestID(chain)

	// ─── Start server ──────────────────────────────────────────────────────

	port := 8003
	log.Printf("Orchestration Engine starting on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), chain))
}

func extractIDFromPath(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	idx := strings.Index(s, "/")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

func writeError(w http.ResponseWriter, status, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%d,"message":"%s"}}`, code, message)
}
