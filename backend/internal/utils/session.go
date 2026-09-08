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
//
// It was "karecik_session" until the deployment went back to a single container.
// The rename is not cosmetic: COOKIE_DOMAIN went from ".karecik.com" to empty at
// the same time, and a cookie is keyed by (name, domain, path). Reusing the name
// would leave browsers holding TWO cookies called the same thing — the old wide
// one and the new host-only one — and both would be sent. RFC 6265 orders equal
// paths by creation time, so the STALE one arrives first, which is the one the
// server reads; and ClearSessionCookie writes with the new empty Domain, so it
// cannot delete the wide one. The result is an owner who can never log in again
// without clearing cookies by hand. A new name cannot collide: the old cookie
// simply goes inert and expires on its own.
//
// Worth knowing for later: "__Host-karecik_sid" would additionally make the
// browser refuse any Set-Cookie for this name that carries a Domain attribute,
// which closes off a sibling subdomain shadowing it. The prefix requires the
// Secure attribute, though, and development runs over plain HTTP with
// COOKIE_SECURE=false, so it is not free.
const SessionCookieName = "karecik_sid"

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
