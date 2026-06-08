package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
)

func TestBoolPtrStr(t *testing.T) {
	if got := boolPtrStr(nil); got != "" {
		t.Errorf("boolPtrStr(nil) = %q, want empty", got)
	}
	s := "hello"
	if got := boolPtrStr(&s); got != "hello" {
		t.Errorf("boolPtrStr(&s) = %q, want hello", got)
	}
}

func TestParseAuthenticatorDevices(t *testing.T) {
	if got := parseAuthenticatorDevices(nil); len(got) != 0 {
		t.Errorf("nil devices = %v, want empty", got)
	}

	raw := []json.RawMessage{
		json.RawMessage(`{"uuid":"d1","name":"TOTP","type":"totp","created":"2024-01-01","enabled":true}`),
		json.RawMessage(`{"uuid":"d2","label":"Backup","created_at":"2024-02-02"}`),
		json.RawMessage(`{not valid json`),
		json.RawMessage(`{"uuid":"d3","properties":{"label":"Nested","type":"webauthn"}}`),
	}
	got := parseAuthenticatorDevices(raw)
	if len(got) != 3 {
		t.Fatalf("expected 3 valid devices (malformed skipped), got %d", len(got))
	}
	if got[0].UUID != "d1" || got[0].Label != "TOTP" || got[0].Type != "totp" || !got[0].IsDefault {
		t.Errorf("device 0 parsed wrong: %+v", got[0])
	}
	if got[1].Label != "Backup" || got[1].CreatedAt != "2024-02-02" {
		t.Errorf("device 1 parsed wrong: %+v", got[1])
	}
	if got[2].Label != "Nested" || got[2].Type != "webauthn" {
		t.Errorf("device 2 nested properties not applied: %+v", got[2])
	}
}

func TestSCIMHandler_applyPatchOps(t *testing.T) {
	h := &SCIMHandler{}
	ctx := context.Background()
	user := &authentik.User{UUID: "u1", IsActive: true}

	// Empty op defaults to Replace; a value with no recognized fields is a no-op
	// that returns the user unchanged without touching the Auth client.
	out, err := h.applyPatchOps(ctx, user, &SCIMPatchRequest{Value: map[string]interface{}{"unknown": "x"}})
	if err != nil {
		t.Fatalf("applyPatchOps(default op) error = %v", err)
	}
	if out.UUID != "u1" {
		t.Errorf("expected unchanged user, got %+v", out)
	}

	// Unsupported op returns an error.
	if _, err := h.applyPatchOps(ctx, user, &SCIMPatchRequest{Op: "Bogus"}); err == nil {
		t.Error("applyPatchOps(unsupported op) expected error")
	}
}

func TestSCIMHandler_applyReplaceAdd(t *testing.T) {
	h := &SCIMHandler{}
	ctx := context.Background()
	user := &authentik.User{UUID: "u1", IsActive: true}

	// nil value is an error
	if _, err := h.applyReplaceAdd(ctx, user, &SCIMPatchRequest{Op: "Replace"}); err == nil {
		t.Error("applyReplaceAdd(nil value) expected error")
	}

	// value with only unrecognized keys is a no-op returning the same user
	out, err := h.applyReplaceAdd(ctx, user, &SCIMPatchRequest{Op: "Replace", Value: map[string]interface{}{"foo": "bar"}})
	if err != nil {
		t.Fatalf("applyReplaceAdd(no-op) error = %v", err)
	}
	if out != user {
		t.Errorf("expected same user pointer for no-op, got %+v", out)
	}
}

func TestIdentityHandlerConstructors(t *testing.T) {
	if NewServiceIdentityHandler(nil, nil) == nil {
		t.Error("NewServiceIdentityHandler returned nil")
	}
	if NewAgentIdentityHandler(nil, nil) == nil {
		t.Error("NewAgentIdentityHandler returned nil")
	}
	if NewRoleHandler(nil, nil) == nil {
		t.Error("NewRoleHandler returned nil")
	}
	if NewSessionReplayHandler(nil, nil, nil) == nil {
		t.Error("NewSessionReplayHandler returned nil")
	}
}
