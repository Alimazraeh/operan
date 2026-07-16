package ctxkeys

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTenantID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-1")
	assert.Equal(t, "tenant-1", GetTenantID(ctx))
}

func TestGetTenantID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetTenantID(ctx))
}

func TestGetUserID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "user-1")
	assert.Equal(t, "user-1", GetUserID(ctx))
}

func TestGetUserID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetUserID(ctx))
}

func TestGetRoles_Present(t *testing.T) {
	roles := []string{"admin", "editor"}
	ctx := context.WithValue(context.Background(), RolesKey, roles)
	assert.Equal(t, roles, GetRoles(ctx))
}

func TestGetRoles_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, GetRoles(ctx))
}

func TestGetTraceID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-1")
	assert.Equal(t, "trace-1", GetTraceID(ctx))
}

func TestGetTraceID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetTraceID(ctx))
}

func TestGetRequestID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, "req-1")
	assert.Equal(t, "req-1", GetRequestID(ctx))
}

func TestGetRequestID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetRequestID(ctx))
}

func TestGetAgentID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), AgentIDKey, "agent-1")
	assert.Equal(t, "agent-1", GetAgentID(ctx))
}

func TestGetAgentID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetAgentID(ctx))
}

func TestGetDepartmentID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), DepartmentIDKey, "dept-1")
	assert.Equal(t, "dept-1", GetDepartmentID(ctx))
}

func TestGetDepartmentID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetDepartmentID(ctx))
}

func TestGetTenantID_TypeMismatch(t *testing.T) {
	ctx := context.WithValue(context.Background(), TenantIDKey, 123) // int, not string
	assert.Equal(t, "", GetTenantID(ctx)) // type assertion fails, returns empty
}