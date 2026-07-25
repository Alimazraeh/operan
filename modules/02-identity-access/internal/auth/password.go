// Package auth holds credential handling for real per-user login.
//
// Scope, stated plainly: this is bootstrap-grade authentication — an admin
// sets a user's password and the user signs in with it. There is no
// self-service reset, no lockout, no password history and no MFA here. It
// exists so that actions in the platform can be attributed to a real person
// instead of the single shared "admin-001" identity. Authentik-backed SSO
// remains the intended production path.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength is deliberately modest: this guards an admin-provisioned
// bootstrap credential, and a length floor plus a character-class rule catches
// the failure mode that matters (someone setting "password").
const MinPasswordLength = 12

// ErrNoPassword is returned when an account has no credential set. Such an
// account exists but cannot log in — it is not the same as a wrong password,
// though callers must not leak the difference to the client.
var ErrNoPassword = errors.New("account has no password set")

// HashPassword validates and hashes a plaintext password.
func HashPassword(plain string) (string, error) {
	if err := ValidatePassword(plain); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// ValidatePassword enforces the credential policy.
func ValidatePassword(plain string) error {
	if len(plain) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	var hasLetter, hasOther bool
	for _, r := range plain {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		default:
			hasOther = true
		}
	}
	if !hasLetter || !hasOther {
		return errors.New("password must mix letters with digits or symbols")
	}
	if strings.EqualFold(strings.TrimSpace(plain), "password") {
		return errors.New("password is too predictable")
	}
	return nil
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
// An account with no hash returns ErrNoPassword; callers must answer the
// client identically for that and for a wrong password, so an attacker cannot
// enumerate which accounts exist.
func VerifyPassword(hash, plain string) error {
	if hash == "" {
		return ErrNoPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return errors.New("credentials do not match")
	}
	return nil
}
