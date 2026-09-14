package resetpw_test

import (
	"strconv"
	"strings"
	"testing"

	"karecik/backend/internal/resetpw"
	"karecik/backend/internal/utils"
)

// The input rules of the command run before it connects to a database, so they
// are tested on their own. Bytes that are not UTF-8 are built from byte values,
// so this file holds none of them.

func TestNormalizeEmail(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want, err string
	}{
		{"trimmed_and_lowercased", "  Owner@Example.COM \t", "owner@example.com", ""},
		{"turkish_letters_are_kept", "Şef@örnek.com", "şef@örnek.com", ""},
		{"empty", "", "", "-email is required"},
		{"blank", "   ", "", "-email is required"},
		{"byte_ff", "owner" + string([]byte{0xff}) + "@example.com", "", "-email is not valid UTF-8"},
		{"truncated_sequence", "owner@example.co" + string([]byte{0xc3}), "", "-email is not valid UTF-8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resetpw.NormalizeEmail(tc.raw)
			switch {
			case tc.err == "" && (err != nil || got != tc.want):
				t.Fatalf("NormalizeEmail(%q) = (%q, %v), want (%q, nil)", tc.raw, got, err, tc.want)
			case tc.err != "" && (err == nil || err.Error() != tc.err || got != ""):
				t.Fatalf("NormalizeEmail(%q) = (%q, %v), want the error %q", tc.raw, got, err, tc.err)
			}
		})
	}
}

func TestCheckNewPasswordAppliesTheLimitsOfTheAPI(t *testing.T) {
	if utils.MaxPasswordBytes != 72 {
		t.Fatalf("fixture: utils.MaxPasswordBytes is %d, want 72", utils.MaxPasswordBytes)
	}

	for _, tc := range []struct {
		name     string
		password string
		accepted bool
	}{
		{"7_characters", "1234567", false},
		{"8_characters", "12345678", true},
		{"72_bytes", strings.Repeat("a", 72), true},
		{"73_bytes", strings.Repeat("a", 73), false},
		{"36_turkish_letters_of_72_bytes", strings.Repeat("ş", 36), true},
		{"37_turkish_letters_of_74_bytes", strings.Repeat("ş", 37), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := resetpw.CheckNewPassword(tc.password)
			if tc.accepted {
				if err != nil {
					t.Fatalf("CheckNewPassword refused a password of %d bytes: %v", len(tc.password), err)
				}
				// A password the command accepts is one bcrypt can hash.
				if _, err := utils.HashPassword(tc.password); err != nil {
					t.Fatalf("an accepted password of %d bytes does not hash: %v", len(tc.password), err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckNewPassword accepted a password of %d bytes", len(tc.password))
			}
		})
	}

	// The refusal of a long password names the limit and the length, rather
	// than passing on bcrypt's own error.
	err := resetpw.CheckNewPassword(strings.Repeat("ş", 37))
	if err == nil || !strings.Contains(err.Error(), "at most "+strconv.Itoa(utils.MaxPasswordBytes)+" bytes") ||
		!strings.Contains(err.Error(), "74 bytes") {
		t.Fatalf("the refusal of a 74-byte password is %v, want one naming 74 bytes and the %d-byte limit",
			err, utils.MaxPasswordBytes)
	}
	if err := resetpw.CheckNewPassword("1234567"); err == nil || !strings.Contains(err.Error(), "at least 8 characters") {
		t.Fatalf("the refusal of a 7-character password is %v, want one naming the 8-character minimum", err)
	}
}
