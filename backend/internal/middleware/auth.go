package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/config"
	"karecik/backend/internal/session"
	"karecik/backend/internal/utils"
)

const (
	ctxUserID     = "karecik_user_id"
	ctxBusinessID = "karecik_business_id"
	ctxTokenHash  = "karecik_session_hash"
)

// Protected resolves the session cookie against the in-memory session store and
// stores the user and business identifiers on the request context.
//
// This was a JWT, then a database row, and is now an entry in a map owned by
// this process. The property that matters survived both moves: the session is
// looked up on EVERY request, so revoking it ends it immediately. A signed
// token was trusted on its own signature, which is why a logout or a revoked
// session could not stop it before it expired.
//
// What the move to memory changed is the cost — one map read behind a shared
// read lock, no database round trip at all. That is only true because
// session.Entry carries the business id as well as the user id; looking the
// business up here would put the round trip straight back.
//
// It also means a session cannot outlive the process. A restart or a deploy
// leaves every browser holding a cookie that matches nothing, which arrives
// here as an ordinary expiry and sends them to the login screen.
//
// NOTE: the error messages reach the end user and stay Turkish.
func Protected(store *session.Store, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Cookies(utils.SessionCookieName)
		if raw == "" {
			return utils.Unauthorized(c, "Bu işlem için oturum açmanız gerekiyor.")
		}

		// Hashed once and kept: ChangePassword needs this exact value to spare
		// the caller's own session while revoking the rest.
		hash := utils.HashSessionToken(raw)

		entry, ok := store.Lookup(hash)
		if !ok {
			// Expired, revoked, never ours, or issued before the last restart.
			// The cookie is cleared so the browser stops sending a value that
			// can never work again.
			ClearSessionCookie(c, cfg)
			return utils.Unauthorized(c, "Oturumunuz sona ermiş, lütfen tekrar giriş yapın.")
		}

		c.Locals(ctxUserID, entry.UserID)
		c.Locals(ctxBusinessID, entry.BusinessID)
		c.Locals(ctxTokenHash, hash)
		return c.Next()
	}
}

// UserID returns the user identifier stored by the Protected middleware.
func UserID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(ctxUserID).(uuid.UUID)
	return id
}

// BusinessID returns the business identifier stored by the Protected middleware.
// Every dashboard endpoint derives its tenant scope from here; the client never
// sends a business_id itself.
func BusinessID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(ctxBusinessID).(uuid.UUID)
	return id
}

// SessionHash returns the hash of the CURRENT request's session token.
//
// Logout deletes exactly this row, and a password change deletes every row of
// the user EXCEPT this one, so the person making the change is not thrown out
// of the page they are standing on.
func SessionHash(c *fiber.Ctx) string {
	hash, _ := c.Locals(ctxTokenHash).(string)
	return hash
}

// SetSessionCookie writes the session cookie.
//
// HttpOnly is the whole reason this migration was worth doing: unlike the token
// in localStorage it replaced, no script on the page can read this value, so an
// XSS bug can no longer walk off with a login.
//
// Domain, SameSite and Secure come from the configuration, but the deployed
// shape is a single container: one binary serves both the SPA and the API, so
// the page and the endpoint it calls always share an origin.
//
//	local (Vite proxy)   Domain "" · Lax · Secure off
//	production           Domain "" · Lax · Secure on
//
// Domain stays EMPTY, which makes the cookie host-only. Widening it to
// ".karecik.com" would also hand the session to every tenant's
// {slug}.karecik.com, and nothing there wants it — customer menus are entirely
// unauthenticated. SameSite=None is still supported by the configuration for a
// genuinely cross-site deployment, but nothing needs it in this topology.
func SetSessionCookie(c *fiber.Ctx, cfg *config.Config, token string, expiresAt time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     utils.SessionCookieName,
		Value:    token,
		Path:     "/",
		Domain:   cfg.CookieDomain,
		Expires:  expiresAt,
		Secure:   cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: cfg.CookieSameSite,
	})
}

// ClearSessionCookie removes the cookie from the browser.
//
// Domain, Path, Secure and SameSite must match what SetSessionCookie wrote:
// a browser treats a cookie with a different Domain or Path as a DIFFERENT
// cookie, so a mismatched clear leaves the original sitting there and the user
// appears not to have logged out at all.
func ClearSessionCookie(c *fiber.Ctx, cfg *config.Config) {
	c.Cookie(&fiber.Cookie{
		Name:     utils.SessionCookieName,
		Value:    "",
		Path:     "/",
		Domain:   cfg.CookieDomain,
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
		Secure:   cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: cfg.CookieSameSite,
	})
}
