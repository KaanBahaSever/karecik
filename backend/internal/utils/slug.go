package utils

import (
	"regexp"
	"strings"
)

// Maps Turkish characters to their ASCII equivalents.
var turkishReplacer = strings.NewReplacer(
	"ç", "c", "Ç", "c",
	"ğ", "g", "Ğ", "g",
	"ı", "i", "I", "i", "İ", "i",
	"ö", "o", "Ö", "o",
	"ş", "s", "Ş", "s",
	"ü", "u", "Ü", "u",
	"â", "a", "Â", "a",
	"î", "i", "Î", "i",
	"û", "u", "Û", "u",
	"&", "-ve-",
)

var (
	nonSlugChars   = regexp.MustCompile(`[^a-z0-9-]+`)
	repeatedDashes = regexp.MustCompile(`-{2,}`)
)

// SlugifyWithFallback turns free text into a URL-safe slug, returning
// `fallback` when nothing usable survives the conversion.
//
//	SlugifyWithFallback("Kahve Durağı", "menu") -> "kahve-duragi"
//	SlugifyWithFallback("!!!", "menu")          -> "menu"
//
// The two kinds of slug want different fallbacks — an unnameable business
// becomes "isletme" (the subdomain), an unnameable menu becomes "menu" (the
// path segment) — so the caller picks. `fallback` is echoed verbatim: it is
// always a constant chosen by the caller, never user input.
func SlugifyWithFallback(input, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = turkishReplacer.Replace(s)
	s = strings.ToLower(s) // catch any uppercase left by the replacer
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = repeatedDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" {
		return fallback
	}
	if len(s) > 60 {
		// Everything left is ASCII [a-z0-9-], so the cut cannot land inside a
		// rune, and s never starts with a dash, so the Trim cannot empty it.
		s = strings.Trim(s[:60], "-")
	}
	return s
}

// Slugify turns a business name into a subdomain-safe slug.
//
//	"Kahve Durağı"   -> "kahve-duragi"
//	"Çınar Restoran" -> "cinar-restoran"
//
// Empty or fully invalid input yields "isletme".
func Slugify(input string) string {
	return SlugifyWithFallback(input, "isletme")
}

// Subdomains reserved by the system — they are never handed out to businesses.
var reservedSlugs = map[string]bool{
	"www": true, "api": true, "admin": true, "app": true, "panel": true,
	"mail": true, "ftp": true, "blog": true, "help": true, "destek": true,
	"karecik": true, "static": true, "cdn": true, "assets": true,
	"dashboard": true, "login": true, "register": true, "demo": true,
}

// IsReservedSlug reports whether a slug is reserved by the system.
func IsReservedSlug(slug string) bool {
	return reservedSlugs[strings.ToLower(slug)]
}

// IsValidSlug checks a slug against the shared slug rules: 2 to 60 characters
// of [a-z0-9-] with no leading or trailing dash. Business slugs (subdomains)
// and menu slugs (path segments) both have to satisfy it; only the business
// slug is additionally reserved-checked.
func IsValidSlug(slug string) bool {
	if len(slug) < 2 || len(slug) > 60 {
		return false
	}
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return false
	}
	return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(slug)
}
