package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mustHash returns a bcrypt hash of pw, failing the test on error.
func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(h)
}
