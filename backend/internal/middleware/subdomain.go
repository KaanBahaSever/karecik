package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/config"
)

// ExtractSubdomain resolves the business slug from a Host header.
//
//	kahve-duragi.karecik.com       -> "kahve-duragi"
//	kahve-duragi.localhost:5173    -> "kahve-duragi"
//	www.karecik.com                -> ""   (reserved)
//	karecik.com / localhost        -> ""   (no subdomain)
//	127.0.0.1:8080                 -> ""   (IP address)
func ExtractSubdomain(host, appDomain, devDomain string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}

	// Strip the port if present (including the bracketed IPv6 form)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")

	// A bare IP address has no subdomain
	if net.ParseIP(host) != nil {
		return ""
	}

	var sub string
	for _, root := range []string{appDomain, devDomain} {
		root = strings.ToLower(strings.TrimSpace(root))
		if root == "" {
			continue
		}
		if strings.HasSuffix(host, "."+root) {
			sub = strings.TrimSuffix(host, "."+root)
			break
		}
	}

	if sub == "" {
		return ""
	}

	// For multi-level hosts such as "a.b.karecik.com" take the leftmost label
	if i := strings.Index(sub, "."); i >= 0 {
		sub = sub[:i]
	}

	if sub == "www" || sub == "api" {
		return ""
	}
	return sub
}

// SubdomainOf resolves the slug from the Host header of the current request.
func SubdomainOf(c *fiber.Ctx, cfg *config.Config) string {
	host := c.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Hostname()
	}
	return ExtractSubdomain(host, cfg.AppDomain, cfg.DevDomain)
}
