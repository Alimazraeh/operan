package ctxkeys

import (
	"context"
	"testing"
)

func TestGetSetTenantID(t *testing.T) {
	ctx := context.Background()
	if got := GetTenantID(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = SetTenantID(ctx, "tenant-123")
	if got := GetTenantID(ctx); got != "tenant-123" {
		t.Errorf("expected tenant-123, got %q", got)
	}
}

func TestGetSetUserID(t *testing.T) {
	ctx := context.Background()
	if got := GetUserID(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = SetUserID(ctx, "user-456")
	if got := GetUserID(ctx); got != "user-456" {
		t.Errorf("expected user-456, got %q", got)
	}
}

func TestGetSetUserRole(t *testing.T) {
	ctx := context.Background()
	if got := GetUserRole(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = SetUserRole(ctx, "admin")
	if got := GetUserRole(ctx); got != "admin" {
		t.Errorf("expected admin, got %q", got)
	}
}

func TestGetSetTraceID(t *testing.T) {
	ctx := context.Background()
	if got := GetTraceID(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = SetTraceID(ctx, "trace-789")
	if got := GetTraceID(ctx); got != "trace-789" {
		t.Errorf("expected trace-789, got %q", got)
	}
}

func TestGetSetRequestID(t *testing.T) {
	ctx := context.Background()
	if got := GetRequestID(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = SetRequestID(ctx, "req-001")
	if got := GetRequestID(ctx); got != "req-001" {
		t.Errorf("expected req-001, got %q", got)
	}
}

func TestMultipleKeys(t *testing.T) {
	ctx := context.Background()
	ctx = SetTenantID(ctx, "t1")
	ctx = SetUserID(ctx, "u1")
	ctx = SetUserRole(ctx, "admin")
	ctx = SetTraceID(ctx, "tr1")
	ctx = SetRequestID(ctx, "r1")

	if got := GetTenantID(ctx); got != "t1" {
		t.Errorf("tenant: expected t1, got %q", got)
	}
	if got := GetUserID(ctx); got != "u1" {
		t.Errorf("user: expected u1, got %q", got)
	}
	if got := GetUserRole(ctx); got != "admin" {
		t.Errorf("role: expected admin, got %q", got)
	}
	if got := GetTraceID(ctx); got != "tr1" {
		t.Errorf("trace: expected tr1, got %q", got)
	}
	if got := GetRequestID(ctx); got != "r1" {
		t.Errorf("request: expected r1, got %q", got)
	}
}

func TestGetNonString(t *testing.T) {
	ctx := context.WithValue(context.Background(), TenantID, 123)
	if got := GetTenantID(ctx); got != "" {
		t.Errorf("expected empty for non-string value, got %q", got)
	}
}
