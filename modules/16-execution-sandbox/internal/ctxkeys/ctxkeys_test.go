package ctxkeys

import (
	"context"
	"testing"
)

func TestGetTenantID(t *testing.T) {
	ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-123")
	if v := GetTenantID(ctx); v != "tenant-123" {
		t.Errorf("expected 'tenant-123', got '%s'", v)
	}
}

func TestGetTenantID_Missing(t *testing.T) {
	ctx := context.Background()
	if v := GetTenantID(ctx); v != "" {
		t.Errorf("expected empty string, got '%s'", v)
	}
}

func TestGetUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "user-456")
	if v := GetUserID(ctx); v != "user-456" {
		t.Errorf("expected 'user-456', got '%s'", v)
	}
}

func TestGetRoles(t *testing.T) {
	ctx := context.WithValue(context.Background(), RolesKey, []string{"admin", "viewer"})
	roles := GetRoles(ctx)
	if len(roles) != 2 || roles[0] != "admin" {
		t.Errorf("expected [admin, viewer], got %v", roles)
	}
}

func TestGetRoles_Missing(t *testing.T) {
	ctx := context.Background()
	roles := GetRoles(ctx)
	if roles != nil && len(roles) != 0 {
		t.Errorf("expected nil/empty, got %v", roles)
	}
}

func TestGetTraceID(t *testing.T) {
	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-abc")
	if v := GetTraceID(ctx); v != "trace-abc" {
		t.Errorf("expected 'trace-abc', got '%s'", v)
	}
}

func TestGetRequestID(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, "req-xyz")
	if v := GetRequestID(ctx); v != "req-xyz" {
		t.Errorf("expected 'req-xyz', got '%s'", v)
	}
}