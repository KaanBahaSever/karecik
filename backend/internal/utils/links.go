package utils

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The link rules of the contact block of a menu, in one place.
//
// Two callers need them and neither can own them for the other:
// handlers/menu.go refuses a request that breaks one of them, and
// repository/menu.go checks every stored entry again before it lets it into the
// customer payload, because a row written past the API must not reach a
// customer. The repository cannot import the handlers, so the rules live here
// and both call CheckMenuLink.
//
// The dashboard mirrors these rules in the browser, so every detail below is
// part of the contract and is spelled out in docs/API.md.
//
// NOTE: the messages are shown to the end user and are therefore Turkish.

// The messages of the link rules. The limits are spliced in from the constants
// in appearance.go, so a limit and the sentence that announces it cannot
// disagree.
var (
	MsgLinkLabelInvalidChars = "Link adı geçersiz karakter içeriyor."
	MsgLinkLabelRequired     = "Link adı zorunludur."
	MsgLinkLabelTooLong      = "Link adı en fazla " + strconv.Itoa(MaxMenuLinkLabelRunes) + " karakter olabilir."
	MsgLinkURLTooLong        = "Link adresi en fazla " + strconv.Itoa(MaxMenuLinkURLBytes) + " karakter olabilir."
	MsgLinkURLInvalid        = "Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır."
	MsgLinksTooMany          = "En fazla " + strconv.Itoa(MaxMenuLinks) + " link ekleyebilirsiniz."
	MsgLinksUnreadable       = "Link listesi geçersiz."
)

// Limits of a link host name, in characters (runes).
const (
	maxLinkHostRunes      = 253
	maxLinkHostLabelRunes = 63
)

// forbiddenLinkURLChars may not appear anywhere in a link address. Each is
// either unsafe to hand to a page unescaped (< > " `) or not valid unescaped in
// a URL at all, and net/url accepts several of them — "<" and ">" even inside a
// host name — so they are refused before the address is parsed.
//
// The square brackets are among them. In an address they only ever enclose an
// IP literal, which is not an allowed host anyway, and net/url's Hostname
// removes them from the host it returns, so "https://[ornek.com]" would reach
// validLinkHostname as ornek.com and pass although no browser opens it.
// Refusing the characters themselves, before parsing, keeps the answer from
// depending on what a Go version's net/url does with them.
const forbiddenLinkURLChars = "<>\"`{}|\\^[]"

// CheckMenuLink applies the label rules and then the address rules to one
// link. It returns the trimmed label, the trimmed address with its scheme
// lowercased, and the message of the first rule the link breaks — "" when it
// breaks none, in which case the two strings are what gets stored or served.
func CheckMenuLink(label, address string) (string, string, string) {
	cleanLabel, message := CheckMenuLinkLabel(label)
	if message != "" {
		return "", "", message
	}
	cleanURL, message := CheckMenuLinkURL(address)
	if message != "" {
		return "", "", message
	}
	return cleanLabel, cleanURL, ""
}

// CheckMenuLinkLabel applies the label rules in their order: trim with
// strings.TrimSpace, refuse a C0 or C1 control character, refuse a blank label,
// and refuse more than MaxMenuLinkLabelRunes runes. It returns the trimmed label
// and the message of the first rule broken, or "".
//
// A label is blank when nothing visible is left of it: the invisible format
// characters are removed and what remains is trimmed with strings.TrimSpace
// once more. The second trim is what makes a label of U+200B, a space and
// U+200B blank — the first trim cannot reach a space that sits between two
// invisible characters. The label that is returned, and stored, is still the
// one trimmed once, with any invisible characters inside it.
//
// The control check runs before the blank check, so a label made of BEL is
// "invalid characters" while a label made of newlines — which the trim removes
// — is "required".
func CheckMenuLinkLabel(label string) (string, string) {
	label = strings.TrimSpace(label)
	if strings.IndexFunc(label, isControlRune) >= 0 {
		return "", MsgLinkLabelInvalidChars
	}
	if strings.TrimSpace(strings.Map(dropInvisibleFormatRune, label)) == "" {
		return "", MsgLinkLabelRequired
	}
	if utf8.RuneCountInString(label) > MaxMenuLinkLabelRunes {
		return "", MsgLinkLabelTooLong
	}
	return label, ""
}

// CheckMenuLinkURL applies the address rules: trim with strings.TrimSpace,
// refuse more than MaxMenuLinkURLBytes bytes, then refuse anything that is not
// a plain http:// or https:// address (see linkURLAccepted). It returns the
// trimmed address with only its scheme lowercased — a path or a query may be
// case-sensitive — and the message of the first rule broken, or "".
func CheckMenuLinkURL(address string) (string, string) {
	address = strings.TrimSpace(address)
	if len(address) > MaxMenuLinkURLBytes {
		return "", MsgLinkURLTooLong
	}
	scheme, ok := linkURLAccepted(address)
	if !ok {
		return "", MsgLinkURLInvalid
	}
	// The scheme is the ASCII prefix just matched, so it spans the same bytes
	// in the address and in its lowercase spelling.
	return scheme + address[len(scheme):], ""
}

// linkURLAccepted reports whether a trimmed address passes every address rule,
// and returns its scheme in lowercase:
//
//   - no whitespace (unicode.IsSpace), no C0 or C1 control character, no
//     invisible format character and none of forbiddenLinkURLChars — the
//     square brackets included — anywhere;
//   - it starts with "http://" or "https://", in any letter case;
//   - net/url parses it, and it carries no userinfo — any "@" in the authority
//     is refused, so "https://example.com@evil.example" never passes;
//   - its host name (net/url's Hostname, without the port) is a valid
//     linkHostname;
//   - a ":" after the host name is followed by a port of 1 to 5 digits whose
//     value is 1 to 65535. "https://example.com:" is refused, not read as
//     "no port".
func linkURLAccepted(address string) (string, bool) {
	for _, r := range address {
		if unicode.IsSpace(r) || isControlRune(r) || isInvisibleFormatRune(r) ||
			strings.ContainsRune(forbiddenLinkURLChars, r) {
			return "", false
		}
	}

	var scheme string
	switch {
	case hasASCIIPrefixFold(address, "https://"):
		scheme = "https"
	case hasASCIIPrefixFold(address, "http://"):
		scheme = "http"
	default:
		return "", false
	}

	parsed, err := url.Parse(address)
	if err != nil || parsed.User != nil {
		return "", false
	}
	if !validLinkHostname(parsed.Hostname()) || !validLinkPort(parsed.Host) {
		return "", false
	}
	return scheme, true
}

// validLinkHostname reports whether a host name is 1 to 253 characters of
// dot-separated labels, at least two of them, each 1 to 63 characters of
// letters (any Unicode letter), ASCII digits or "-", neither starting nor
// ending with "-", and the last one holding at least one letter.
//
// The last rule is what turns away IPv4 literals ("192.168.1.1") and the
// two-label rule single names like "localhost" or the "https" that
// "https://https//example.com" parses into. An IPv6 literal never gets here:
// its square brackets are refused before the address is parsed.
func validLinkHostname(host string) bool {
	if n := utf8.RuneCountInString(host); n < 1 || n > maxLinkHostRunes {
		return false
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if n := utf8.RuneCountInString(label); n < 1 || n > maxLinkHostLabelRunes {
			return false
		}
		// "-" is ASCII, and no byte of a multi-byte UTF-8 sequence can equal
		// it, so comparing the first and last bytes is exact.
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !unicode.IsLetter(r) && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return strings.IndexFunc(labels[len(labels)-1], unicode.IsLetter) >= 0
}

// validLinkPort checks the port of a host[:port] string. No ":" means no port,
// which is valid; otherwise what follows the last ":" has to be 1 to 5 ASCII
// digits with a value from 1 to 65535. validLinkHostname has already refused a
// ":" inside the host name, so the last ":" is the port separator.
func validLinkPort(hostPort string) bool {
	colon := strings.LastIndexByte(hostPort, ':')
	if colon < 0 {
		return true
	}
	port := hostPort[colon+1:]
	if len(port) < 1 || len(port) > 5 {
		return false
	}
	value := 0
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return false
		}
		value = value*10 + int(port[i]-'0')
	}
	return value >= 1 && value <= 65535
}

// hasASCIIPrefixFold reports whether s starts with prefix, comparing ASCII
// letters case-insensitively and nothing else. prefix must be lowercase ASCII.
// strings.EqualFold is not used because Unicode folding also matches letters
// such as U+017F (long s) to "s".
func hasASCIIPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// isControlRune reports a C0 (U+0000-U+001F) or C1 (U+007F-U+009F) control
// character.
func isControlRune(r rune) bool {
	return r <= 0x1F || (r >= 0x7F && r <= 0x9F)
}

// isInvisibleFormatRune reports the format characters that render as nothing:
// the soft hyphen, the Arabic letter mark, the Mongolian vowel separator, the
// zero-width characters and marks, the bidi embeddings, overrides and isolates,
// the invisible operators and the byte order mark. A label made only of them
// looks empty, and inside an address they hide what the address really is.
func isInvisibleFormatRune(r rune) bool {
	switch {
	case r == 0x00AD, r == 0x061C, r == 0x180E, r == 0xFEFF:
		return true
	case r >= 0x200B && r <= 0x200F,
		r >= 0x202A && r <= 0x202E,
		r >= 0x2060 && r <= 0x2064,
		r >= 0x2066 && r <= 0x206F:
		return true
	}
	return false
}

// dropInvisibleFormatRune is the strings.Map function that removes the
// invisible format characters: it maps each of them to -1, which strings.Map
// drops, and keeps every other rune as it is.
func dropInvisibleFormatRune(r rune) rune {
	if isInvisibleFormatRune(r) {
		return -1
	}
	return r
}
