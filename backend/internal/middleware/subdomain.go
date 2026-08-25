package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/config"
)

// ExtractSubdomain, Host basligindan isletme slug'ini cozer.
//
//	kahve-duragi.karecik.com       -> "kahve-duragi"
//	kahve-duragi.localhost:5173    -> "kahve-duragi"
//	www.karecik.com                -> ""   (ayrilmis)
//	karecik.com / localhost        -> ""   (subdomain yok)
//	127.0.0.1:8080                 -> ""   (IP adresi)
func ExtractSubdomain(host, appDomain, devDomain string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}

	// Port varsa ayikla (IPv6 koseli parantezli bicimi de dahil)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")

	// Ham IP adresinde subdomain kavrami yoktur
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

	// "a.b.karecik.com" gibi cok seviyeli adreslerde en soldaki etiketi al
	if i := strings.Index(sub, "."); i >= 0 {
		sub = sub[:i]
	}

	if sub == "www" || sub == "api" {
		return ""
	}
	return sub
}

// SubdomainOf, gecerli istegin Host basligindan slug cozer.
func SubdomainOf(c *fiber.Ctx, cfg *config.Config) string {
	host := c.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Hostname()
	}
	return ExtractSubdomain(host, cfg.AppDomain, cfg.DevDomain)
}
