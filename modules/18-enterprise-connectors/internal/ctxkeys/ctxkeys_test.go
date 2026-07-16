package ctxkeys

import (
	"context"
	"testing"
)

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int      { return &i }
func ptrTime(t interface{}) interface{} { return t }

func TestGetTenantID_WithKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-123")
	got := GetTenantID(ctx)
	if got != "tenant-123" {
		t.Errorf("expected 'tenant-123', got '%s'", got)
	}
}

func TestGetTenantID_MissingKey(t *testing.T) {
	ctx := context.Background()
	got := GetTenantID(ctx)
	if got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

func TestGetUserID_WithKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "user-456")
	got := GetUserID(ctx)
	if got != "user-456" {
		t.Errorf("expected 'user-456', got '%s'", got)
	}
}

func TestGetUserID_MissingKey(t *testing.T) {
	ctx := context.Background()
	got := GetUserID(ctx)
	if got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

func TestGetRoles_WithKey(t *testing.T) {
	roles := []string{"admin", "editor"}
	ctx := context.WithValue(context.Background(), RolesKey, roles)
	got := GetRoles(ctx)
	if len(got) != 2 || got[0] != "admin" {
		t.Errorf("expected [admin, editor], got %v", got)
	}
}

func TestGetRoles_MissingKey(t *testing.T) {
	ctx := context.Background()
	got := GetRoles(ctx)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestGetTraceID_WithKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-abc")
	got := GetTraceID(ctx)
	if got != "trace-abc" {
		t.Errorf("expected 'trace-abc', got '%s'", got)
	}
}

func TestGetTraceID_MissingKey(t *testing.T) {
	ctx := context.Background()
	got := GetTraceID(ctx)
	if got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

func TestGetRequestID_WithKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, "req-xyz")
	got := GetRequestID(ctx)
	if got != "req-xyz" {
		t.Errorf("expected 'req-xyz', got '%s'", got)
	}
}

func TestGetRequestID_MissingKey(t *testing.T) {
	ctx := context.Background()
	got := GetRequestID(ctx)
	if got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}