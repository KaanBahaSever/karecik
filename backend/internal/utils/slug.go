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

// Slugify turns a business name into a subdomain-safe slug.
//
//	"Kahve Durağı"   -> "kahve-duragi"
//	"Çınar Restoran" -> "cinar-restoran"
//
// Empty or fully invalid input yields "isletme".
func Slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = turkishReplacer.Replace(s)
	s = strings.ToLower(s) // catch any uppercase left by the replacer
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = repeatedDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" {
		return "isletme"
	}
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
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

// IsValidSlug checks a slug against the subdomain rules.
func IsValidSlug(slug string) bool {
	if len(slug) < 2 || len(slug) > 60 {
		return false
	}
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return false
	}
	return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(slug)
}
