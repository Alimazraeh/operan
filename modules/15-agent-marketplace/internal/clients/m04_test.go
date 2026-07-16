package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestM04Client_RegisterAgent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "tenant-1", r.Header.Get("X-Tenant-ID"))

		body := M04Agent{
			Name:    "Test Agent",
			Role:    "Analyst",
			TenantID: "tenant-1",
		}
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":     "agent-uuid-123",
				"name":   body.Name,
				"role":   body.Role,
				"tenant": body.TenantID,
			},
		})
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	result, err := client.RegisterAgent(context.Background(), "tenant-1", "test-token", M04Agent{
		Name:    "Test Agent",
		Role:    "Analyst",
		TenantID: "tenant-1",
	})

	assert.NoError(t, err)
	assert.Equal(t, "agent-uuid-123", result.ID)
	assert.Equal(t, "Test Agent", result.Name)
	assert.Equal(t, "Analyst", result.Role)
}

func TestM04Client_RegisterAgent_NoDataResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	result, err := client.RegisterAgent(context.Background(), "tenant-1", "test-token", M04Agent{
		Name:    "Fallback Agent",
		Role:    "Default",
		TenantID: "tenant-1",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.ID)
	assert.Equal(t, "Fallback Agent", result.Name)
}

func TestM04Client_RegisterAgent_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "agent already exists",
		})
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	_, err := client.RegisterAgent(context.Background(), "tenant-1", "test-token", M04Agent{
		Name:    "Duplicate Agent",
		Role:    "Analyst",
		TenantID: "tenant-1",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent already exists")
}

func TestM04Client_RegisterAgent_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	_, err := client.RegisterAgent(context.Background(), "tenant-1", "test-token", M04Agent{
		Name:    "Test Agent",
		Role:    "Analyst",
		TenantID: "tenant-1",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "M04 returned status 502")
}

func TestM04Client_RegisterAgent_ConnectionRefused(t *testing.T) {
	client := NewM04Client("http://localhost:1") // Unlikely to have a server
	_, err := client.RegisterAgent(context.Background(), "tenant-1", "test-token", M04Agent{
		Name: "Test Agent",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "M04 request failed")
}

func TestM04Client_HealthCheck_Success(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	err := client.HealthCheck(context.Background())

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestM04Client_HealthCheck_Failure(t *testing.T) {
	client := NewM04Client("http://localhost:1")
	err := client.HealthCheck(context.Background())

	assert.Error(t, err)
}