package ctxkeys

import (
	"context"
	"testing"
)

func TestTenantIDFrom(t *testing.T) {
	ctx := context.Background()

	// Empty context returns empty string
	if got := TenantIDFrom(ctx); got != "" {
		t.Errorf("TenantIDFrom(empty) = %q, want empty", got)
	}

	// Context with value
	ctx = WithTenantID(ctx, "tenant-123")
	if got := TenantIDFrom(ctx); got != "tenant-123" {
		t.Errorf("TenantIDFrom(ctx) = %q, want %q", got, "tenant-123")
	}

	// Wrong key type returns empty
	ctx = context.WithValue(ctx, Key("wrong_key"), "wrong-value")
	if got := TenantIDFrom(ctx); got != "tenant-123" {
		t.Errorf("TenantIDFrom with wrong key = %q, want original value", got)
	}
}

func TestUserIDFrom(t *testing.T) {
	ctx := context.Background()

	if got := UserIDFrom(ctx); got != "" {
		t.Errorf("UserIDFrom(empty) = %q, want empty", got)
	}

	ctx = WithUserID(ctx, "user-456")
	if got := UserIDFrom(ctx); got != "user-456" {
		t.Errorf("UserIDFrom(ctx) = %q, want %q", got, "user-456")
	}
}

func TestUserRolesFrom(t *testing.T) {
	ctx := context.Background()

	if got := UserRolesFrom(ctx); got != nil {
		t.Errorf("UserRolesFrom(empty) = %v, want nil", got)
	}

	ctx = WithUserRoles(ctx, []string{"admin", "editor"})
	if got := UserRolesFrom(ctx); len(got) != 2 || got[0] != "admin" {
		t.Errorf("UserRolesFrom(ctx) = %v, want [admin editor]", got)
	}

	// Wrong type returns nil
	ctx = context.WithValue(ctx, UserRoles, "not-a-slice")
	if got := UserRolesFrom(ctx); got != nil {
		t.Errorf("UserRolesFrom(wrong type) = %v, want nil", got)
	}
}

func TestTraceIDFrom(t *testing.T) {
	ctx := context.Background()

	if got := TraceIDFrom(ctx); got != "" {
		t.Errorf("TraceIDFrom(empty) = %q, want empty", got)
	}

	ctx = WithTraceID(ctx, "trace-789")
	if got := TraceIDFrom(ctx); got != "trace-789" {
		t.Errorf("TraceIDFrom(ctx) = %q, want %q", got, "trace-789")
	}
}

func TestRequestIDFrom(t *testing.T) {
	ctx := context.Background()

	if got := RequestIDFrom(ctx); got != "" {
		t.Errorf("RequestIDFrom(empty) = %q, want empty", got)
	}

	ctx = WithRequestID(ctx, "req-001")
	if got := RequestIDFrom(ctx); got != "req-001" {
		t.Errorf("RequestIDFrom(ctx) = %q, want %q", got, "req-001")
	}
}

func TestWithTenantID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithTenantID(ctx, "t1")
	v := newCtx.Value(TenantID)
	if v != "t1" {
		t.Errorf("WithTenantID value = %v, want %v", v, "t1")
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithUserID(ctx, "u1")
	v := newCtx.Value(UserID)
	if v != "u1" {
		t.Errorf("WithUserID value = %v, want %v", v, "u1")
	}
}

func TestWithUserRoles(t *testing.T) {
	ctx := context.Background()
	newCtx := WithUserRoles(ctx, []string{"a", "b"})
	v := newCtx.Value(UserRoles)
	roles, ok := v.([]string)
	if !ok || len(roles) != 2 {
		t.Errorf("WithUserRoles value = %v, want [a b]", v)
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithTraceID(ctx, "tr1")
	v := newCtx.Value(TraceID)
	if v != "tr1" {
		t.Errorf("WithTraceID value = %v, want %v", v, "tr1")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithRequestID(ctx, "rq1")
	v := newCtx.Value(RequestID)
	if v != "rq1" {
		t.Errorf("WithRequestID value = %v, want %v", v, "rq1")
	}
}

func TestContextChaining(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tenant-1")
	ctx = WithUserID(ctx, "user-2")
	ctx = WithTraceID(ctx, "trace-3")
	ctx = WithRequestID(ctx, "req-4")

	if TenantIDFrom(ctx) != "tenant-1" {
		t.Error("TenantIDFrom failed after chaining")
	}
	if UserIDFrom(ctx) != "user-2" {
		t.Error("UserIDFrom failed after chaining")
	}
	if TraceIDFrom(ctx) != "trace-3" {
		t.Error("TraceIDFrom failed after chaining")
	}
	if RequestIDFrom(ctx) != "req-4" {
		t.Error("RequestIDFrom failed after chaining")
	}
}
