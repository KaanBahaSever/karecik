package utils_test

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"karecik/backend/internal/utils"
)

func TestPasswordTooLongCountsBytes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
		tooLong  bool
	}{
		{"72_ascii_letters", strings.Repeat("a", 72), false},
		{"73_ascii_letters", strings.Repeat("a", 73), true},
		{"36_turkish_letters_are_72_bytes", strings.Repeat("ş", 36), false},
		{"37_turkish_letters_are_74_bytes", strings.Repeat("ş", 37), true},
	} {
		if got := utils.PasswordTooLong(tc.password); got != tc.tooLong {
			t.Errorf("PasswordTooLong(%s) = %t, want %t", tc.name, got, tc.tooLong)
		}
	}
}

func TestHashPasswordRefusesAPasswordLongerThan72Bytes(t *testing.T) {
	if _, err := utils.HashPassword(strings.Repeat("a", 73)); !errors.Is(err, bcrypt.ErrPasswordTooLong) {
		t.Fatalf("HashPassword of 73 bytes returned %v, want bcrypt.ErrPasswordTooLong", err)
	}
}

// bcrypt compares only the first 72 bytes, so a longer password that starts
// with the stored one matches the hash as far as bcrypt is concerned.
// CheckPassword must not let it in.
func TestCheckPasswordRefusesAPasswordLongerThan72Bytes(t *testing.T) {
	stored := strings.Repeat("a", utils.MaxPasswordBytes)
	hash, err := bcrypt.GenerateFromPassword([]byte(stored), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("fixture: could not hash the password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(stored+"a")); err != nil {
		t.Fatalf("fixture: bcrypt itself refused the 73-byte password (%v), so the check below proves nothing", err)
	}

	if !utils.CheckPassword(string(hash), stored) {
		t.Fatalf("CheckPassword refused the stored 72-byte password")
	}
	if utils.CheckPassword(string(hash), stored+"a") {
		t.Errorf("CheckPassword accepted a 73-byte password that starts with the stored one")
	}
}
