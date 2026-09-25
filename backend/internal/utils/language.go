package utils

import (
	"sort"
	"strconv"
	"strings"
)

// Language negotiation of the customer menu.
//
// A customer who scans a QR code has never picked a language — the address on
// the table carries none — so the first answer has to come from what the phone
// already says: its Accept-Language header, which every browser fills from the
// system language (navigator.language is the first entry of the same list).
// The order of precedence is:
//
//  1. an explicit ?lang= the menu offers — the visitor chose it in the page's
//     language picker, and that choice always wins;
//  2. the first language of Accept-Language, in quality order, that the menu
//     offers — a German phone opens a menu that has German in German;
//  3. the menu's default language.
//
// An explicit ?lang= the menu does NOT offer (a stale link, a language the
// owner has since removed) is treated as absent rather than as an error, so it
// falls through to step 2 instead of pinning the visitor to the default.

// maxAcceptLanguageEntries bounds how much of a header is looked at. A real
// browser sends a handful of entries; the cap only keeps a hostile header from
// costing more than a real one.
const maxAcceptLanguageEntries = 32

// ParseAcceptLanguage returns the base language codes of an Accept-Language
// header, most preferred first.
//
//   - Region and script subtags are stripped and the code lowercased:
//     "de-DE", "DE_at" and "de" are all "de".
//   - Entries are ordered by their q value, highest first; entries with the
//     same q keep the order they were sent in. A missing q is 1.
//   - q=0 means "not acceptable", so such an entry is dropped. A base code
//     listed more than once keeps its highest q.
//   - The wildcard "*" is dropped: it names no language, and "anything else is
//     fine" is exactly what the fallback to the default language already does.
//   - Garbage is dropped entry by entry, never the whole header: a primary
//     subtag that is not 2 or 3 ASCII letters ("x-private", "12", "") and a q
//     that is not a number between 0 and 1 remove only their own entry.
//
// The result is never nil.
func ParseAcceptLanguage(header string) []string {
	type preference struct {
		code  string
		q     float64
		index int
	}

	byCode := make(map[string]*preference)
	entries := strings.Split(header, ",")
	if len(entries) > maxAcceptLanguageEntries {
		entries = entries[:maxAcceptLanguageEntries]
	}

	for index, entry := range entries {
		parts := strings.Split(entry, ";")
		code, ok := baseLanguageCode(strings.TrimSpace(parts[0]))
		if !ok {
			continue
		}

		q, ok := qualityOf(parts[1:])
		if !ok || q <= 0 {
			continue
		}

		if existing, seen := byCode[code]; seen {
			if q > existing.q {
				existing.q = q
			}
			continue
		}
		byCode[code] = &preference{code: code, q: q, index: index}
	}

	ordered := make([]*preference, 0, len(byCode))
	for _, pref := range byCode {
		ordered = append(ordered, pref)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].q != ordered[j].q {
			return ordered[i].q > ordered[j].q
		}
		return ordered[i].index < ordered[j].index
	})

	codes := make([]string, 0, len(ordered))
	for _, pref := range ordered {
		codes = append(codes, pref.code)
	}
	return codes
}

// baseLanguageCode reduces a language tag to its lowercased primary subtag and
// reports whether that subtag is a plausible language code: 2 or 3 ASCII
// letters. "*" and the private-use and grandfathered prefixes ("x-...",
// "i-...") are not.
func baseLanguageCode(tag string) (string, bool) {
	if cut := strings.IndexAny(tag, "-_"); cut >= 0 {
		tag = tag[:cut]
	}
	if len(tag) < 2 || len(tag) > 3 {
		return "", false
	}
	for i := 0; i < len(tag); i++ {
		c := tag[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return "", false
		}
	}
	return strings.ToLower(tag), true
}

// qualityOf reads the q parameter of one entry. A missing q is 1; a q that is
// present but not a number between 0 and 1 makes the entry unusable. Other
// parameters are ignored, as RFC 9110 allows.
func qualityOf(params []string) (float64, bool) {
	for _, param := range params {
		name, value, found := strings.Cut(param, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "q") {
			continue
		}
		q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || q < 0 || q > 1 {
			return 0, false
		}
		return q, true
	}
	return 1, true
}

// NegotiateLanguage picks the language a payload is written in, in the order
// described at the top of this file: the explicit choice when offered, then
// the first Accept-Language preference that is offered, then fallback.
//
// explicit is compared trimmed and lowercased, so "?lang=EN" means English.
// offered is the list the menu publishes; fallback is its default language,
// returned even when it is not in offered, because the default is always
// servable — Translations.Resolve falls back to it.
func NegotiateLanguage(explicit, acceptLanguage string, offered []string, fallback string) string {
	isOffered := func(code string) bool {
		for _, candidate := range offered {
			if candidate == code {
				return true
			}
		}
		return false
	}

	if code := strings.ToLower(strings.TrimSpace(explicit)); code != "" && isOffered(code) {
		return code
	}
	for _, code := range ParseAcceptLanguage(acceptLanguage) {
		if isOffered(code) {
			return code
		}
	}
	return fallback
}

// LanguageCodes lists the code of every supported language, in catalogue order
// — the "offered" list of a payload that has no menu to ask, the tenant
// directory.
func LanguageCodes() []string {
	codes := make([]string, 0, len(Languages))
	for _, language := range Languages {
		codes = append(codes, language.Code)
	}
	return codes
}
