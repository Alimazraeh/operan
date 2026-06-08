package authentik

import (
	"context"
	"testing"
)

func TestTenantManager_SetupAndRemove(t *testing.T) {
	c, _ := newTestClient(t, universalHandler)
	tm := NewTenantManager(c)
	t.Cleanup(tm.Stop)

	ctx := context.Background()
	state, err := tm.SetupTenant(ctx, "tenant-1", "tslug", "Tenant One")
	if err != nil {
		t.Fatalf("SetupTenant() error = %v", err)
	}
	if state == nil || state.TenantUUID == "" {
		t.Fatalf("SetupTenant() returned incomplete state: %+v", state)
	}
	if state.OIDCConfig == nil || state.SAMLConfig == nil {
		t.Error("SetupTenant() should populate OIDC and SAML config")
	}

	// Second call should hit the cache and return the same state.
	cached, err := tm.SetupTenant(ctx, "tenant-1", "tslug", "Tenant One")
	if err != nil {
		t.Fatalf("SetupTenant() cached call error = %v", err)
	}
	if cached.TenantUUID != state.TenantUUID {
		t.Error("SetupTenant() second call should return cached state")
	}

	// GetTenantState should resolve from cache.
	if _, err := tm.GetTenantState(ctx, "tenant-1"); err != nil {
		t.Errorf("GetTenantState() error = %v", err)
	}

	// RemoveTenant tears down resources and clears the cache.
	if err := tm.RemoveTenant(ctx, "tenant-1"); err != nil {
		t.Fatalf("RemoveTenant() error = %v", err)
	}
	if _, err := tm.GetTenantState(ctx, "tenant-1"); err == nil {
		t.Error("GetTenantState() should fail after RemoveTenant")
	}
}
