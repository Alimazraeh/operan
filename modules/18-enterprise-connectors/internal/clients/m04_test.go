package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/enterprise-connectors/internal/connectors"
	"github.com/stretchr/testify/require"
)

func TestM04Client_RegisterTools_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/agents/tools/batch", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NotEmpty(t, r.Header.Get("X-Tenant-ID"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewM04Client(server.URL)

	tools := []connectors.ToolDefinition{
		{Name: "test_tool", Description: "A test tool"},
	}

	err := client.RegisterTools(context.Background(), "tenant-1", tools)
	require.NoError(t, err)
}

func TestM04Client_RegisterTools_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := NewM04Client(server.URL)

	tools := []connectors.ToolDefinition{
		{Name: "test_tool", Description: "A test tool"},
	}

	err := client.RegisterTools(context.Background(), "tenant-1", tools)
	require.Error(t, err)
	require.Contains(t, err.Error(), "M04 registration failed")
}

func TestM04Client_CheckHealth_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	err := client.CheckHealth(context.Background())
	require.NoError(t, err)
}

func TestM04Client_CheckHealth_Unavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewM04Client(server.URL)
	err := client.CheckHealth(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "M04 health check returned HTTP 503")
}

func TestM04Client_M04Client_BaseURLAndClient(t *testing.T) {
	client := NewM04Client("http://localhost:8004")
	require.NotNil(t, client)
	require.Equal(t, "http://localhost:8004", client.baseURL)
	require.NotNil(t, client.client)
}

func TestM04Client_RegisterTools_MarshalError(t *testing.T) {
	// Create a tool with an unmarshallable parameters type
	tools := []connectors.ToolDefinition{
		{
			Name:        "test",
			Description: "test",
			Parameters:  map[string]interface{}{"key": make(chan int)}, // channel is not JSON serializable
		},
	}

	client := NewM04Client("http://localhost:9999")
	err := client.RegisterTools(context.Background(), "tenant-1", tools)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal tools")
}

func TestM04Client_CheckHealth_ConnectionRefused(t *testing.T) {
	client := NewM04Client("http://localhost:99999")
	ctx, cancel := context.WithTimeout(context.Background(), 100)
	defer cancel()
	err := client.CheckHealth(ctx)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "M04 health check failed"))
}