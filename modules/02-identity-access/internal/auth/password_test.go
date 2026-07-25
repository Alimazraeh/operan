package auth

import (
	"errors"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	const good = "correct-horse-9-battery"
	h, err := HashPassword(good)
	if err != nil {
		t.Fatal(err)
	}
	if h == good {
		t.Fatal("stored value must be a hash, not the password")
	}
	if err := VerifyPassword(h, good); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := VerifyPassword(h, "correct-horse-9-batterX"); err == nil {
		t.Error("wrong password accepted")
	}
	// Same password hashed twice must differ (bcrypt salts).
	h2, _ := HashPassword(good)
	if h == h2 {
		t.Error("hashes are unsalted")
	}
}

func TestNoPasswordIsDistinctFromWrongPassword(t *testing.T) {
	// The distinction exists for the server's benefit (logging), and must not
	// be surfaced to a client — otherwise accounts can be enumerated.
	if err := VerifyPassword("", "anything"); !errors.Is(err, ErrNoPassword) {
		t.Errorf("empty hash should report ErrNoPassword, got %v", err)
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	for _, bad := range []string{"", "short1!", "aaaaaaaaaaaaaaa", "123456789012", "password"} {
		if err := ValidatePassword(bad); err == nil {
			t.Errorf("policy accepted %q", bad)
		}
	}
	for _, ok := range []string{"correct-horse-9", "Tr0ubad0ur-basics"} {
		if err := ValidatePassword(ok); err != nil {
			t.Errorf("policy rejected %q: %v", ok, err)
		}
	}
}
