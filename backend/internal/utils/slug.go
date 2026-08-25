package utils

import (
	"regexp"
	"strings"
)

// Turkce karakterleri ASCII karsiliklarina cevirir.
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
	nonSlugChars  = regexp.MustCompile(`[^a-z0-9-]+`)
	multipleDashs = regexp.MustCompile(`-{2,}`)
)

// Slugify, isletme adini subdomain'e uygun bir slug'a cevirir.
//
//	"Kahve Durağı"   -> "kahve-duragi"
//	"Çınar Restoran" -> "cinar-restoran"
//
// Bos ya da tamamen gecersiz girdide "isletme" dondurur.
func Slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = turkishReplacer.Replace(s)
	s = strings.ToLower(s) // replacer sonrasi kalan buyuk harfler icin
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = multipleDashs.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" {
		return "isletme"
	}
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

// Ayrilmis subdomain'ler — isletmelere verilemez.
var reservedSlugs = map[string]bool{
	"www": true, "api": true, "admin": true, "app": true, "panel": true,
	"mail": true, "ftp": true, "blog": true, "help": true, "destek": true,
	"karecik": true, "static": true, "cdn": true, "assets": true,
	"dashboard": true, "login": true, "register": true, "demo": true,
}

// IsReservedSlug, slug'in sistem tarafindan ayrilmis olup olmadigini soyler.
func IsReservedSlug(slug string) bool {
	return reservedSlugs[strings.ToLower(slug)]
}

// IsValidSlug, slug'in subdomain kurallarina uyup uymadigini denetler.
func IsValidSlug(slug string) bool {
	if len(slug) < 2 || len(slug) > 60 {
		return false
	}
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return false
	}
	return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(slug)
}
