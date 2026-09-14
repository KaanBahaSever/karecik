// Package resetpw holds the input rules of the resetpw command (cmd/resetpw).
//
// The command checks what the operator typed before it connects to a database.
// The rules live here rather than in package main, which nothing can import, so
// that the tests in backend/tests can call them.
package resetpw

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"karecik/backend/internal/utils"
)

// MinPasswordLength is the shortest password the command accepts, the same
// minimum the API applies to every password it sets.
const MinPasswordLength = 8

// NormalizeEmail trims and lowercases the address the operator typed, the way
// the API's login does. Bytes that are not UTF-8 are refused before the
// lowering: strings.ToLower would turn each of them into U+FFFD and look up a
// different address.
func NormalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	switch {
	case email == "":
		return "", errors.New("-email is required")
	case !utf8.ValidString(email):
		return "", errors.New("-email is not valid UTF-8")
	}
	return strings.ToLower(email), nil
}

// CheckNewPassword applies the password rules of the API. The upper limit is
// counted in bytes, because that is what bcrypt counts: utils.HashPassword
// fails on a longer password, and the message names the limit instead.
func CheckNewPassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("the password must be at least %d characters", MinPasswordLength)
	}
	if utils.PasswordTooLong(password) {
		return fmt.Errorf("the password is %d bytes long; it must be at most %d bytes "+
			"(a Turkish letter such as ş takes 2)", len(password), utils.MaxPasswordBytes)
	}
	return nil
}
