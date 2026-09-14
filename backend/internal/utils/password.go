package utils

import "golang.org/x/crypto/bcrypt"

// MaxPasswordBytes is the longest password bcrypt works with. GenerateFromPassword
// refuses a longer one, and CompareHashAndPassword reads no further than this
// many bytes of the password it is given.
const MaxPasswordBytes = 72

// PasswordTooLong reports whether a password is longer than MaxPasswordBytes.
// The limit is in bytes, so a letter outside ASCII, such as "ş", counts twice.
func PasswordTooLong(plain string) bool {
	return len(plain) > MaxPasswordBytes
}

// HashPassword hashes a plain-text password with bcrypt. A password longer than
// MaxPasswordBytes fails with bcrypt.ErrPasswordTooLong; the handlers refuse one
// with a 422 before they get here.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether a plain-text password matches the stored hash.
//
// A password longer than MaxPasswordBytes never matches. No stored hash can come
// from one, because HashPassword refuses it, but CompareHashAndPassword only reads
// the first MaxPasswordBytes bytes — without this check a longer password that
// merely starts with the stored one would be accepted.
func CheckPassword(hash, plain string) bool {
	if PasswordTooLong(plain) {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
