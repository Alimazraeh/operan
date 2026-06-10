package ctxkeys

import (
	"context"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "t1")
	ctx = WithUserID(ctx, "u1")
	ctx = WithTraceID(ctx, "trace")
	ctx = WithRequestID(ctx, "req")
	ctx = context.WithValue(ctx, UserRoles, []string{"admin", "ops"})

	if TenantIDFrom(ctx) != "t1" {
		t.Errorf("tenant = %q", TenantIDFrom(ctx))
	}
	if UserIDFrom(ctx) != "u1" {
		t.Errorf("user = %q", UserIDFrom(ctx))
	}
	if TraceIDFrom(ctx) != "trace" {
		t.Errorf("trace = %q", TraceIDFrom(ctx))
	}
	if RequestIDFrom(ctx) != "req" {
		t.Errorf("request = %q", RequestIDFrom(ctx))
	}
	if roles := UserRolesFrom(ctx); len(roles) != 2 || roles[0] != "admin" {
		t.Errorf("roles = %v", roles)
	}
}

func TestEmptyContext(t *testing.T) {
	ctx := context.Background()
	if TenantIDFrom(ctx) != "" || UserIDFrom(ctx) != "" || TraceIDFrom(ctx) != "" ||
		RequestIDFrom(ctx) != "" || UserRolesFrom(ctx) != nil {
		t.Error("empty context should yield zero values")
	}
}
