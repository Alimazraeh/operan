// Package main implements the Agent Orchestration Engine entrypoint.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/operan/modules/03-agent-orchestration/internal/config"
	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/gates"
	"github.com/operan/modules/03-agent-orchestration/internal/execution"
	"github.com/operan/modules/03-agent-orchestration/internal/handler"
	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
)

const base = "/api/v1/orchestration"

func main() {
	// ─── Load and validate config ──────────────────────────────────────────────
	cfg := config.ParseConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	log.Printf("Orchestration Engine v%s starting (listen: %s, log: %s)",
		cfg.Version, cfg.ListenAddr, cfg.LogLevel)

	// ─── Initialize stores ────────────────────────────────────────────────────
	storeMode := repository.NewModeFromConfig(cfg)
	store, err := repository.NewStore(storeMode, cfg)
	if err != nil {
		log.Fatalf("store initialization error: %v", err)
	}

	// ─── Initialize event publisher ───────────────────────────────────────────
	// Build broker config from runtime config
	brokerCfg := events.BrokerConfig{
		BrokerAddress: cfg.EventBusHost + ":" + cfg.EventBusPort,
		Username:      cfg.EventBusUser,
		Password:      cfg.EventBusPass,
		TopicPrefix:   "operan.orchestration",
		EnableTLS:     cfg.EventBusTLS,
		RetryEnabled:  cfg.EventBusSASL, // enable retry when SASL is used (often cloud Kafka)
		RetryConfig:   events.DefaultRetryConfig(),
	}

	// Create broker based on configured protocol, fall back to log-only if unavailable
	var pub *events.Publisher
	brokerFactory := events.NewBrokerFactory()
	brokerType := events.BrokerType(cfg.EventBusProto)
	broker, err := brokerFactory.CreateBroker(brokerType, brokerCfg)
	if err != nil {
		log.Printf("[WARN] event broker unavailable (%s): %v — falling back to log-only", cfg.EventBusProto, err)
		pub = events.NewPublisher()
	} else {
		pub = events.NewPublisherWithBroker(broker)

		// ─── Supervision gate enforcement (US-402) ───────────────────────────
		// A dedicated subscriber broker with a pass-through prefix ("operan")
		// so Module 09's topics are not re-prefixed with operan.orchestration.
		subCfg := brokerCfg
		subCfg.TopicPrefix = "operan"
		if subBroker, subErr := brokerFactory.CreateBroker(brokerType, subCfg); subErr == nil {
			enforcer := gates.NewEnforcer(store.HumanTaskStore())
			if startErr := enforcer.Start(context.Background(), gateSubscriber{subBroker}, "module03-orchestration"); startErr != nil {
				log.Printf("[WARN] gate enforcement unavailable: %v", startErr)
			}
		} else {
			log.Printf("[WARN] gate enforcement subscriber unavailable: %v", subErr)
		}
	}

	// ─── Initialize DAG engine (LangGraph stack) ──────────────────────────────
	dagEngine := execution.NewEngine(store.WorkflowStore(), pub, nil, events.StackLangGraph)

	// ─── Initialize handlers ──────────────────────────────────────────────────
	wfHandler := handler.NewWorkflowHandler(store.WorkflowStore(), store.ScheduleStore(), store.AgentStore())
	wfHandler.DAGEngine = dagEngine
	wfHandler.Events = pub
	scHandler := handler.NewScheduleHandler(store.ScheduleStore(), store.WorkflowStore(), store.AgentStore())
	scHandler.Events = pub
	schedHandler := handler.NewSchedulingHandler(store.AgentStore(), store.WorkflowStore())
	plHandler := handler.NewPipelineHandler(store.PipelineStore(), store.ExecutionStore(), store.HumanTaskStore())
	plHandler.WithEvents(pub)
	exHandler := handler.NewExecutionHandler(store.ExecutionStore(), store.PipelineStore())
	exHandler.WithEvents(pub)
	htHandler := handler.NewHumanTaskHandler(store.HumanTaskStore(), store.ExecutionStore())
	htHandler.WithEvents(pub)
	escHandler := handler.NewEscalationHandler(store.EscalationStore(), store.WorkflowStore())
	escHandler.Events = pub
	retryHdlr := handler.NewRetryHandler(store.RetryRecordStore(), store.WorkflowStore(), store.ExecutionStore())
	retryHdlr.WithEvents(pub)
	nodesHdlr := handler.NewNodesResultsHandler(store.WorkflowStore())
	agentWorkersHdlr := handler.NewAgentWorkersHandler(store.AgentStore())
	stackHealthHdlr := handler.NewStackHealthHandler(store.StackHealthStore())
	delegateHdlr := handler.NewDelegationHandler(store.DelegationStore(), store.WorkflowStore(), store.AgentStore())
	delegateHdlr.WithEvents(pub)

	// ─── Router ───────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// ═══ Workflows ═══
	mux.HandleFunc("GET "+base+"/workflows", wfHandler.ListWorkflows)
	mux.HandleFunc("POST "+base+"/workflows", wfHandler.CreateWorkflow)
	mux.HandleFunc(base+"/workflows/", handleWorkflowDetail(wfHandler, delegateHdlr))

	// ═══ Schedules ═══
	mux.HandleFunc("GET "+base+"/schedules", scHandler.ListSchedules)
	mux.HandleFunc("POST "+base+"/schedules", scHandler.ScheduleWorkflow)
	mux.HandleFunc(base+"/schedules/", handleScheduleDetail(scHandler))

	// ═══ Agents ═══
	mux.HandleFunc("GET "+base+"/agents", handler.ListAgents(store.AgentStore()))
	mux.HandleFunc("GET "+base+"/agents/"+agentIDPath+"/availability", schedHandler.GetAgentAvailability)
	mux.HandleFunc("POST "+base+"/agents/assign", schedHandler.AssignAgent)
	mux.HandleFunc("GET "+base+"/agents/"+agentIDPath+"/workers", agentWorkersHdlr.GetAgentWorkers)

	// ═══ Pipelines ═══
	mux.HandleFunc("GET "+base+"/pipeline", plHandler.ListPipelines)
	mux.HandleFunc("POST "+base+"/pipeline", plHandler.CreatePipeline)
	mux.HandleFunc(base+"/pipeline/", handlePipelineDetail(plHandler, exHandler, htHandler))

	// ═══ Executions ═══
	mux.HandleFunc("GET "+base+"/executions", exHandler.ListExecutions)
	mux.HandleFunc("POST "+base+"/executions", exHandler.CreateExecution)
	mux.HandleFunc("GET "+base+"/executions/analytics", exHandler.GetExecutionAnalytics)
	mux.HandleFunc(base+"/executions/", handleExecutionDetail(exHandler))

	// ═══ Human Tasks ═══
	mux.HandleFunc("GET "+base+"/human-tasks", htHandler.ListHumanTasks)
	mux.HandleFunc("POST "+base+"/human-tasks", htHandler.CreateHumanTask)
	mux.HandleFunc("GET "+base+"/human-tasks/pending", htHandler.GetPendingTasks)
	mux.HandleFunc(base+"/human-tasks/", handleHumanTaskDetail(htHandler))

	// ═══ Escalations ═══
	mux.HandleFunc(base+"/workflows/"+workflowIDPath+"/escalations/", func(w http.ResponseWriter, r *http.Request) {
		workflowID := r.PathValue("workflowId")
		switch r.Method {
		case "GET":
			escHandler.ListWorkflowEscalations(w, r, workflowID)
		case "POST":
			escHandler.CreateEscalation(w, r, workflowID)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	})

	// ═══ Retry Records ═══
	mux.HandleFunc(base+"/workflows/"+workflowIDPath+"/retry-records", func(w http.ResponseWriter, r *http.Request) {
		workflowID := r.PathValue("workflowId")
		retryHdlr.ListWorkflowRetryRecords(w, r, workflowID)
	})
	mux.HandleFunc(base+"/workflows/"+workflowIDPath+"/nodes/"+nodeIDPath+"/retry", func(w http.ResponseWriter, r *http.Request) {
		workflowID := r.PathValue("workflowId")
		nodeID := r.PathValue("nodeId")
		retryHdlr.RetryNode(w, r, workflowID, nodeID)
	})

	// ═══ Nodes & Results ═══
	mux.HandleFunc(base+"/workflows/"+workflowIDPath+"/nodes", func(w http.ResponseWriter, r *http.Request) {
		workflowID := r.PathValue("workflowId")
		nodesHdlr.ListWorkflowNodes(w, r, workflowID)
	})
	mux.HandleFunc(base+"/workflows/"+workflowIDPath+"/results", func(w http.ResponseWriter, r *http.Request) {
		workflowID := r.PathValue("workflowId")
		nodesHdlr.ListWorkflowResults(w, r, workflowID)
	})

	// ═══ Agent Workers ═══
	mux.HandleFunc(base+"/agents/"+agentIDPath+"/workers", func(w http.ResponseWriter, r *http.Request) {
		agentWorkersHdlr.GetAgentWorkers(w, r)
	})

	// ═══ Stack Health ═══
	mux.HandleFunc("GET "+base+"/stack/health", stackHealthHdlr.GetStackHealth)

	// ═══ Multi-stack: LangGraph ═══
	mux.HandleFunc(base+"/stack/langgraph/graphs", stackHealthHdlr.ListLangGraphs)
	mux.HandleFunc("POST "+base+"/stack/langgraph/graphs", stackHealthHdlr.CreateLangGraph)
	mux.HandleFunc(base+"/stack/langgraph/graphs/"+idPath, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		switch r.Method {
		case "GET":
			stackHealthHdlr.GetLangGraph(w, r, id)
		case "PUT":
			stackHealthHdlr.UpdateLangGraph(w, r, id)
		case "DELETE":
			stackHealthHdlr.DeleteLangGraph(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	})

	// ═══ Multi-stack: Temporal ═══
	mux.HandleFunc(base+"/stack/temporal/workflows", stackHealthHdlr.ListTemporalWorkflows)
	mux.HandleFunc("POST "+base+"/stack/temporal/workflows", stackHealthHdlr.CreateTemporalWorkflow)
	mux.HandleFunc(base+"/stack/temporal/workflows/"+idPath, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		switch r.Method {
		case "GET":
			stackHealthHdlr.GetTemporalWorkflow(w, r, id)
		case "PUT":
			stackHealthHdlr.UpdateTemporalWorkflow(w, r, id)
		case "DELETE":
			stackHealthHdlr.DeleteTemporalWorkflow(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	})
	mux.HandleFunc(base+"/stack/temporal/checkpoints", func(w http.ResponseWriter, r *http.Request) {
		stackHealthHdlr.ListTemporalCheckpoints(w, r, "")
	})

	// ═══ Multi-stack: Ray ═══
	mux.HandleFunc(base+"/stack/ray/pools", stackHealthHdlr.ListRayPools)
	mux.HandleFunc("POST "+base+"/stack/ray/pools", stackHealthHdlr.CreateRayPool)
	mux.HandleFunc(base+"/stack/ray/pools/"+idPath, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		switch r.Method {
		case "GET":
			stackHealthHdlr.GetRayPool(w, r, id)
		case "POST":
			stackHealthHdlr.ScaleRayPool(w, r, id)
		case "DELETE":
			stackHealthHdlr.DeleteRayPool(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	})

	// ═══ Multi-stack: Celery ═══
	mux.HandleFunc(base+"/stack/celery/queues", stackHealthHdlr.ListCeleryQueues)
	mux.HandleFunc("POST "+base+"/stack/celery/queues", stackHealthHdlr.CreateCeleryQueue)
	mux.HandleFunc(base+"/stack/celery/queues/"+idPath, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		switch r.Method {
		case "GET":
			stackHealthHdlr.GetCeleryQueue(w, r, id)
		case "PUT":
			stackHealthHdlr.UpdateCeleryQueue(w, r, id)
		case "DELETE":
			stackHealthHdlr.DeleteCeleryQueue(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	})
	mux.HandleFunc(base+"/stack/celery/queues/"+idPath+"/consumers", func(w http.ResponseWriter, r *http.Request) {
		// Contract uses {name} param but implementation uses {id} — note the path param name mismatch
		stackHealthHdlr.ListCeleryConsumers(w, r, r.PathValue("id"))
	})

	// ═══ Escalation management ═══
	mux.HandleFunc(base+"/escalations/"+idPath+"/acknowledge", func(w http.ResponseWriter, r *http.Request) {
		escHandler.AcknowledgeEscalation(w, r, r.PathValue("id"))
	})
	mux.HandleFunc(base+"/escalations/"+idPath+"/resolve", func(w http.ResponseWriter, r *http.Request) {
		escHandler.ResolveEscalation(w, r, r.PathValue("id"))
	})

	// ─── Middleware chain (outer to inner) ─────────────────────────────────────
	var chain http.Handler = mux
	chain = middleware.Logger(chain)
	chain = middleware.TenantContext(chain)
	chain = middleware.TraceID(chain)
	chain = middleware.RequestID(chain)
	chain = middleware.JWTAuth(cfg.JWTSecret, chain)

	// ─── Root mux: liveness probe bypasses auth ────────────────────────────────
	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"agent-orchestration","version":"1.0.0"}`))
	})
	root.Handle("/", chain)

	// ─── Start server ──────────────────────────────────────────────────────────
	log.Printf("Listening on %s (PID %d)", cfg.ListenAddr, os.Getpid())
	if err := http.ListenAndServe(cfg.ListenAddr, root); err != nil {
		log.Fatalf("server error: %v", err)
	}

	// ─── Cleanup ──────────────────────────────────────────────────────────────
	_ = pub.Close()
}

// ─── Route path parameters ──────────────────────────────────────────────────

const (
	idPath       = "{id}"
	workflowIDPath = "{workflowId}"
	nodeIDPath   = "{nodeId}"
	agentIDPath  = "{agentId}"
)

// ─── Dynamic route handlers ──────────────────────────────────────────────────

func handleWorkflowDetail(wf *handler.WorkflowHandler, del *handler.DelegationHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/workflows/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "workflow id required")
			return
		}
		ctx := context.WithValue(r.Context(), "workflow_id", id)
		r = r.WithContext(ctx)

		switch {
		case strings.HasSuffix(r.URL.Path, "/delegate") && r.Method == "POST":
			del.DelegateNodeTask(w, r, id)
		case strings.HasSuffix(r.URL.Path, "/execute") && r.Method == "POST":
			wf.ExecuteWorkflow(w, r)
		case strings.HasSuffix(r.URL.Path, "/state") && r.Method == "GET":
			wf.GetWorkflowState(w, r)
		case strings.HasSuffix(r.URL.Path, "/checkpoint") && r.Method == "POST":
			wf.CreateCheckpoint(w, r)
		case strings.HasSuffix(r.URL.Path, "/replay") && r.Method == "POST":
			wf.ReplayWorkflow(w, r)
		case strings.HasSuffix(r.URL.Path, "/variables") && r.Method == "GET":
			wf.GetWorkflowVariables(w, r)
		case strings.HasSuffix(r.URL.Path, "/variables") && r.Method == "PATCH":
			wf.UpdateWorkflowVariables(w, r)
		case strings.HasSuffix(r.URL.Path, "/pause") && r.Method == "POST":
			wf.PauseWorkflow(w, r)
		case strings.HasSuffix(r.URL.Path, "/resume") && r.Method == "POST":
			wf.ResumeWorkflow(w, r)
		case r.Method == "GET":
			wf.GetWorkflow(w, r)
		case r.Method == "DELETE":
			wf.CancelWorkflow(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	}
}

func handleScheduleDetail(sc *handler.ScheduleHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/schedules/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "schedule id required")
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/trigger") && r.Method == "POST":
			sc.TriggerSchedule(w, r)
		case strings.HasSuffix(r.URL.Path, "/pause") && r.Method == "POST":
			sc.PauseSchedule(w, r)
		case strings.HasSuffix(r.URL.Path, "/resume") && r.Method == "POST":
			sc.ResumeSchedule(w, r)
		case r.Method == "GET":
			sc.GetSchedule(w, r)
		case r.Method == "PATCH":
			sc.UpdateSchedule(w, r)
		case r.Method == "DELETE":
			sc.DeleteSchedule(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	}
}

func handlePipelineDetail(pl *handler.PipelineHandler, ex *handler.ExecutionHandler, ht *handler.HumanTaskHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/pipeline/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "pipeline id required")
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/start") && r.Method == "POST":
			pl.StartPipeline(w, r)
		case strings.HasSuffix(r.URL.Path, "/stop") && r.Method == "POST":
			pl.StopPipeline(w, r)
		case strings.HasSuffix(r.URL.Path, "/analytics") && r.Method == "GET":
			pl.GetPipelineAnalytics(w, r)
		case strings.HasSuffix(r.URL.Path, "/history") && r.Method == "GET":
			pl.GetPipelineHistory(w, r)
		case r.Method == "GET":
			pl.GetPipeline(w, r)
		case r.Method == "PATCH":
			pl.UpdatePipeline(w, r)
		case r.Method == "DELETE":
			pl.DeletePipeline(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	}
}

func handleExecutionDetail(ex *handler.ExecutionHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/executions/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "execution id required")
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/start") && r.Method == "POST":
			ex.StartExecution(w, r)
		case strings.HasSuffix(r.URL.Path, "/stop") && r.Method == "POST":
			ex.StopExecution(w, r)
		case strings.HasSuffix(r.URL.Path, "/retry") && r.Method == "POST":
			ex.RetryExecution(w, r)
		case strings.HasSuffix(r.URL.Path, "/steps") && r.Method == "GET":
			ex.GetExecutionSteps(w, r)
		case r.Method == "GET":
			ex.GetExecution(w, r)
		case r.Method == "DELETE":
			ex.DeleteExecution(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	}
}

func handleHumanTaskDetail(ht *handler.HumanTaskHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractIDFromPath(r.URL.Path, base+"/human-tasks/")
		if id == "" {
			writeError(w, http.StatusBadRequest, 400, "human task id required")
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/respond") && r.Method == "POST":
			ht.RespondToHumanTask(w, r)
		case strings.HasSuffix(r.URL.Path, "/cancel") && r.Method == "POST":
			ht.CancelHumanTask(w, r)
		case r.Method == "GET":
			ht.GetHumanTask(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

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

// gateSubscriber adapts events.Broker to the gates.Subscriber interface.
type gateSubscriber struct{ b events.Broker }

func (g gateSubscriber) Subscribe(ctx context.Context, topic, group string, on func(context.Context, gates.Message)) error {
	return g.b.Subscribe(ctx, topic, group, func(ctx context.Context, m events.Message) {
		on(ctx, gates.Message{Topic: m.Topic, Value: m.Value})
	})
}
