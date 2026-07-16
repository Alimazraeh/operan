package ctxkeys

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTenantID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-42")
	assert.Equal(t, "tenant-42", GetTenantID(ctx))
}

func TestGetTenantID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetTenantID(ctx))
}

func TestGetUserID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "user-99")
	assert.Equal(t, "user-99", GetUserID(ctx))
}

func TestGetUserID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetUserID(ctx))
}

func TestGetRoles_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), RolesKey, []string{"admin", "user"})
	assert.Equal(t, []string{"admin", "user"}, GetRoles(ctx))
}

func TestGetRoles_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, GetRoles(ctx))
}

func TestGetTraceID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-123")
	assert.Equal(t, "trace-123", GetTraceID(ctx))
}

func TestGetTraceID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetTraceID(ctx))
}

func TestGetRequestID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, "req-456")
	assert.Equal(t, "req-456", GetRequestID(ctx))
}

func TestGetRequestID_Missing(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetRequestID(ctx))
}