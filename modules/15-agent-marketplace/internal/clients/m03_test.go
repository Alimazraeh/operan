package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestM03Client_CreateWorkflow_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "tenant-1", r.Header.Get("X-Tenant-ID"))

		body := M03Workflow{}
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":   "workflow-uuid-456",
				"name": body.Name,
			},
		})
	}))
	defer server.Close()

	client := NewM03Client(server.URL)
	result, err := client.CreateWorkflow(context.Background(), "tenant-1", "test-token", M03Workflow{
		Name:     "Test Workflow",
		TenantID: "tenant-1",
		Steps:    []map[string]interface{}{{"action": "initialize"}},
	})

	assert.NoError(t, err)
	assert.Equal(t, "workflow-uuid-456", result.ID)
	assert.Equal(t, "Test Workflow", result.Name)
}

func TestM03Client_CreateWorkflow_NoDataResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer server.Close()

	client := NewM03Client(server.URL)
	result, err := client.CreateWorkflow(context.Background(), "tenant-1", "test-token", M03Workflow{
		Name:     "Test Workflow",
		TenantID: "tenant-1",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Test Workflow", result.Name)
	assert.Empty(t, result.ID)
}

func TestM03Client_CreateWorkflow_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "invalid workflow definition",
		})
	}))
	defer server.Close()

	client := NewM03Client(server.URL)
	_, err := client.CreateWorkflow(context.Background(), "tenant-1", "test-token", M03Workflow{
		Name:     "Invalid Workflow",
		TenantID: "tenant-1",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workflow definition")
}

func TestM03Client_CreateWorkflow_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	client := NewM03Client(server.URL)
	_, err := client.CreateWorkflow(context.Background(), "tenant-1", "test-token", M03Workflow{
		Name:     "Conflict Workflow",
		TenantID: "tenant-1",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "M03 returned status 409")
}

func TestM03Client_CreateWorkflow_ConnectionRefused(t *testing.T) {
	client := NewM03Client("http://localhost:1")
	_, err := client.CreateWorkflow(context.Background(), "tenant-1", "test-token", M03Workflow{
		Name: "Test Workflow",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "M03 request failed")
}

func TestM03Client_HealthCheck_Success(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewM03Client(server.URL)
	err := client.HealthCheck(context.Background())

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestM03Client_HealthCheck_Failure(t *testing.T) {
	client := NewM03Client("http://localhost:1")
	err := client.HealthCheck(context.Background())

	assert.Error(t, err)
}