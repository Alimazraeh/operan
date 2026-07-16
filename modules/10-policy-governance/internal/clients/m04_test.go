package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestM04Client_ValidateAgent_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent := AgentInfo{
			AgentID:      "agent-1",
			Name:         "Test Agent",
			Role:         "email-sender",
			IsActive:     true,
			DepartmentID: "dept-1",
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"agent": agent})
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "agent-1", agent.AgentID)
	assert.Equal(t, "Test Agent", agent.Name)
	assert.True(t, agent.IsActive)
}

func TestM04Client_ValidateAgent_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "not found")
}

func TestM04Client_ValidateAgent_InternalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "internal error")
}

func TestM04Client_ValidateAgent_ServiceUnavailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestM04Client_ValidateAgent_Unavailable(t *testing.T) {
	client := NewM04Client("http://localhost:65535")
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestM04Client_GetAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent := AgentInfo{
			AgentID: "agent-1",
			Name:    "Test Agent",
			Role:    "email-sender",
			IsActive: true,
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"agent": agent})
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "agent-1", agent.AgentID)
}

func TestM04Client_GetAgent_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	agent, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Nil(t, agent)
}

func TestM04Client_ValidateAgent_Status500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	_, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error")
}

func TestM04Client_ValidateAgent_Status503(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	client := NewM04Client(ts.URL)
	_, err := client.ValidateAgent(context.Background(), "agent-1", "tenant-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}