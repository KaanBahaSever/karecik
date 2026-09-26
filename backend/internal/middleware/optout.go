package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/config"
)

// The analytics opt-out: a cookie that marks one browser as the owner's own, so
// that its visits to any customer menu on the platform are never stored.
//
// An address list cannot recognise a phone: on mobile data its address changes
// with every cell, and at home it is the household's. What stays put is the
// browser, so the panel can mark it (POST /api/analytics/optout) and unmark it
// (DELETE), and the events endpoint drops every view that arrives carrying the
// mark (handlers.TrackEvent). The customer page reads the same cookie and does
// not even send those views (frontend lib/analytics.js); the server still
// checks, because the page's check is a courtesy and this one is the rule.

// OptOutCookieName is the opt-out cookie. Its only meaningful value is
// OptOutCookieValue; anything else is no opt-out at all.
const (
	OptOutCookieName  = "karecik_analytics_optout"
	OptOutCookieValue = "1"
)

// OptOutMaxAge is how long the mark lasts: a year, renewed every time the owner
// marks the browser again. Long enough that the owner does not have to think
// about it, short enough that a browser handed on to somebody else forgets.
const OptOutMaxAge = 365 * 24 * 60 * 60

// AnalyticsOptedOut reports whether this request carries the opt-out mark.
func AnalyticsOptedOut(c *fiber.Ctx) bool {
	return c.Cookies(OptOutCookieName) == OptOutCookieValue
}

// OptOutCookieDomain is the Domain the opt-out cookie is written with.
//
// Unlike the session cookie, this one has to reach the customer menus, and they
// live on other hosts than the panel: {business}.karecik.com, not karecik.com.
// In production it is therefore written for the whole app domain, which every
// tenant's subdomain and the path-form menu on the apex both receive. That is
// safe precisely because of what the cookie is: a "1" that grants nothing and
// identifies nobody, readable by script on purpose (the page checks it before
// it sends a view). The session cookie stays host-only for the opposite
// reasons — see SetSessionCookie.
//
// In development it is host-only. The dev hosts are localhost and
// {slug}.localhost, and a Domain=localhost cookie is refused outright by some
// browsers, which would make the button appear to do nothing; the path-form
// menu on the same host as the panel still receives a host-only one.
func OptOutCookieDomain(cfg *config.Config) string {
	if cfg.IsProduction() {
		return strings.TrimSpace(cfg.AppDomain)
	}
	return ""
}

// SetAnalyticsOptOut writes the mark.
//
// NOT HttpOnly, deliberately: the customer page reads it to skip sending views
// at all, and there is nothing in it worth stealing. Secure follows the session
// cookie's setting — on wherever the site is served over HTTPS — and SameSite
// is Lax: the events a menu sends are same-site requests, which Lax lets the
// cookie ride along with.
func SetAnalyticsOptOut(c *fiber.Ctx, cfg *config.Config) {
	c.Cookie(&fiber.Cookie{
		Name:     OptOutCookieName,
		Value:    OptOutCookieValue,
		Path:     "/",
		Domain:   OptOutCookieDomain(cfg),
		MaxAge:   OptOutMaxAge,
		Secure:   cfg.CookieSecure,
		HTTPOnly: false,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// ClearAnalyticsOptOut removes the mark. Every attribute but the lifetime
// matches SetAnalyticsOptOut: a browser keys a cookie by name, Domain and Path,
// so a clear written with another Domain would leave the mark in place and the
// owner's visits uncounted with no way to tell why.
func ClearAnalyticsOptOut(c *fiber.Ctx, cfg *config.Config) {
	c.Cookie(&fiber.Cookie{
		Name:     OptOutCookieName,
		Value:    "",
		Path:     "/",
		Domain:   OptOutCookieDomain(cfg),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   cfg.CookieSecure,
		HTTPOnly: false,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
