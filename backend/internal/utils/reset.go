package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// PasswordResetTTL is how long an e-mailed reset link stays usable.
//
// One hour, not the thirty days a session gets. The two are not the same kind
// of secret: a session cookie is HttpOnly and lives in one browser, while a
// reset token travels through mail servers and sits in an inbox in plain text.
// The window it stays live in is the window in which a compromised or shared
// inbox converts into a stolen account, so it is kept to about as long as
// someone realistically takes to go and read their mail.
const PasswordResetTTL = time.Hour

// resetTokenBytes is the entropy behind one reset link: 32 bytes, 256 bits.
//
// The same size as a session token, and for a stronger reason. This value
// arrives in a URL query string, where it is guessed against an endpoint that
// anyone can call. It has to be far beyond enumeration on its own, without
// relying on the rate limiter in front of it.
const resetTokenBytes = 32

// NewPasswordResetToken returns a fresh, unguessable reset token.
//
// crypto/rand, never math/rand: a predictable reset token hands over any
// account whose address is known. The error is returned rather than swallowed
// because issuing a weak token would be worse than failing the request.
func NewPasswordResetToken() (string, error) {
	buf := make([]byte, resetTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate a password reset token: %w", err)
	}
	// URL-safe and unpadded, so it survives a query string with no escaping.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashPasswordResetToken maps a token to what the database stores.
//
// Plain SHA-256, and bcrypt would be wrong here — the opposite of the rule for
// passwords. bcrypt is deliberately slow to survive a dictionary attack on a
// low-entropy human secret. This token has 256 bits of entropy and no
// dictionary to attack, so slowness buys nothing and would instead be paid on
// every reset attempt, including the ones an attacker generates.
func HashPasswordResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
