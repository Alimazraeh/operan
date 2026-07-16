package handler

import (
	"net/http"

	"github.com/operan/cost-governance/internal/consumers"
	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/middleware"
	"github.com/operan/cost-governance/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// SetupRouter creates the HTTP router with all endpoints.
func SetupRouter(
	authValidator *middleware.AuthValidator,
	budgetStore *store.BudgetStore,
	eventStore *store.CostEventStore,
	alertStore *store.AlertStore,
	budgetEngine *engine.Engine,
	throttleMgr *engine.ThrottleManager,
	costConsumer *consumers.CostEventConsumer,
) http.Handler {
	r := chi.NewRouter()

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health — no auth required
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())

		// Budgets
		budgetHandler := NewBudgetHandler(budgetStore, eventStore, budgetEngine)
		r.Post("/v1/budgets", budgetHandler.CreateBudget)
		r.Get("/v1/budgets", budgetHandler.ListBudgets)
		r.Get("/v1/budgets/{id}", budgetHandler.GetBudget)
		r.Patch("/v1/budgets/{id}", budgetHandler.UpdateBudget)
		r.Delete("/v1/budgets/{id}", budgetHandler.DeleteBudget)

		// Cost Events
		eventsHandler := NewEventsHandler(eventStore, budgetEngine)
		r.Post("/v1/cost-events", eventsHandler.IngestCostEvent)
		r.Get("/v1/cost-events", eventsHandler.ListCostEvents)

		// Summary
		summaryHandler := NewSummaryHandler(eventStore, budgetStore, alertStore, throttleMgr)
		r.Get("/v1/summary", summaryHandler.CostSummary)

		// Alerts
		alertsHandler := NewAlertsHandler(alertStore)
		r.Get("/v1/alerts", alertsHandler.ListAlerts)

		// Throttle
		throttleHandler := NewThrottleHandler(throttleMgr)
		r.Get("/v1/throttle", throttleHandler.GetThrottle)
		r.Patch("/v1/throttle/{status}", throttleHandler.SetThrottle)
	})

	// Register Kafka event consumer (skip if nil, e.g. in tests)
	if costConsumer != nil {
		costConsumer.Register()
	}

	return r
}