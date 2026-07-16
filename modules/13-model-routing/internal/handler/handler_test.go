package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/model-routing/internal/store"
	"github.com/stretchr/testify/assert"
)

// TestWriteJSON tests the WriteJSON helper function.
func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, rec.Body.String(), `"key"`)
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "bad input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error"`)
	assert.Contains(t, rec.Body.String(), "bad input")
}

func TestHealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestPaginatedResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	data := []map[string]string{{"id": "r1"}}
	WriteJSON(rec, http.StatusOK, PaginatedResponse{
		Data:     data,
		Page:     1,
		PageSize: 20,
		Total:    1,
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"data"`)
	assert.Contains(t, rec.Body.String(), `"page"`)
}

func TestPaginatedResponse_NilData(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, PaginatedResponse{
		Data:     nil,
		Page:     1,
		PageSize: 20,
		Total:    0,
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"data":null`)
}

func TestRoutingRule_ToJSON(t *testing.T) {
	rule := store.RoutingRule{
		ID:         "rule-1",
		TenantID:   "tenant-1",
		RuleName:   "test-rule",
		TaskType:   "chat",
		Priority:   50,
		MaxLatencyMs: 5000,
		IsActive:   true,
	}

	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, rule)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"id"`)
	assert.Contains(t, body, `"rule-1"`)
	assert.Contains(t, body, `"task_type"`)
	assert.Contains(t, body, `"chat"`)
}