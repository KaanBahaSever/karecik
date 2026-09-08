package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// SessionTTL is how long a session stays valid without being renewed.
const SessionTTL = 30 * 24 * time.Hour

// SessionCookieName is the cookie the browser carries. It is HttpOnly, so no
// script on the page — ours or anyone else's — can read it.
const SessionCookieName = "karecik_session"

// sessionTokenBytes is the entropy behind one session. 32 bytes is 256 bits:
// far beyond guessing, and it is the size the cookie carries rather than
// anything derived from the user, so two accounts can never collide.
const sessionTokenBytes = 32

// NewSessionToken returns a fresh, unguessable session token.
//
// crypto/rand, not math/rand: a predictable session token is an account
// takeover. The error is returned rather than swallowed for the same reason —
// if the system cannot produce randomness, issuing a weak token would be worse
// than refusing to log the user in.
func NewSessionToken() (string, error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate a session token: %w", err)
	}
	// URL-safe and unpadded, so it needs no escaping in a Set-Cookie header.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashSessionToken maps a token to what the database stores.
//
// A plain SHA-256 is right here and bcrypt would be wrong, which is the
// opposite of the rule for passwords. Password hashing is deliberately slow to
// survive a dictionary attack on a low-entropy human secret. A session token is
// 256 bits of randomness with no dictionary to attack, so slowness would buy
// nothing and would be paid on EVERY authenticated request.
//
// What the hash does buy is that a leaked database contains no usable
// credential: the server hashes whatever arrives in the cookie and compares.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
