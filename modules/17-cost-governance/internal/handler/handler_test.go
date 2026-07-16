package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/operan/cost-governance/internal/config"
	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/middleware"
	"github.com/operan/cost-governance/internal/store"

	"github.com/golang-jwt/jwt/v5"
)

func setupTestRouter(t *testing.T) (http.Handler, *engine.ThrottleManager, func()) {
	t.Helper()
	budgetStore := store.NewBudgetStore(nil)
	eventStore := store.NewCostEventStore(nil)
	alertStore := store.NewAlertStore(nil)
	throttleMgr := engine.NewThrottleManager()
	budgetEngine := engine.NewEngine(budgetStore, eventStore, alertStore, throttleMgr)

	cfg := &config.Config{JWTSecret: "test-secret"}
	authValidator := middleware.NewAuthValidator(cfg)

	router := SetupRouter(authValidator, budgetStore, eventStore, alertStore, budgetEngine, throttleMgr, nil)

	cleanup := func() {
		throttleMgr.SetState("test-tenant", "none")
	}
	return router, throttleMgr, cleanup
}

func createTestToken(secret, tenantID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": tenantID,
		"sub":       "user-123",
		"roles":     []any{"admin"},
		"iss":       "operan-auth",
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}