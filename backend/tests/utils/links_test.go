package utils_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"karecik/backend/internal/utils"
)

// The link rules are a contract the dashboard mirrors in the browser, so every
// rule gets a case of its own here, and so do inputs that a check of the scheme
// alone would accept. No database is involved: CheckMenuLink is pure.

func TestLinkMessagesCarryTheLimits(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{utils.MsgLinkLabelTooLong, "Link adı en fazla 40 karakter olabilir."},
		{utils.MsgLinkURLTooLong, "Link adresi en fazla 500 karakter olabilir."},
		{utils.MsgLinksTooMany, "En fazla 8 link ekleyebilirsiniz."},
	} {
		if tc.got != tc.want {
			t.Errorf("message %q, want %q", tc.got, tc.want)
		}
	}
}

func TestCheckMenuLinkLabel(t *testing.T) {
	cases := []struct {
		name    string
		label   string
		want    string // the cleaned label when message is ""
		message string
	}{
		{"plain", "Rezervasyon", "Rezervasyon", ""},
		{"trimmed", "  Paket Servis \t", "Paket Servis", ""},
		{"inner_spaces_kept", "Yol  Tarifi", "Yol  Tarifi", ""},
		{"turkish_letters", "Şikâyet ve Öneri", "Şikâyet ve Öneri", ""},
		{"exactly_40_runes", strings.Repeat("ş", 40), strings.Repeat("ş", 40), ""},
		{"41_runes", strings.Repeat("ş", 41), "", utils.MsgLinkLabelTooLong},
		{"40_runes_after_the_trim", "  " + strings.Repeat("a", 40) + "  ", strings.Repeat("a", 40), ""},

		{"empty", "", "", utils.MsgLinkLabelRequired},
		{"spaces", "   ", "", utils.MsgLinkLabelRequired},
		{"newline_only", "\n", "", utils.MsgLinkLabelRequired},
		{"nel_only", "\u0085", "", utils.MsgLinkLabelRequired},
		{"nbsp_only", "\u00a0", "", utils.MsgLinkLabelRequired},
		{"ideographic_space_only", "\u3000", "", utils.MsgLinkLabelRequired},
		{"line_separator_only", "\u2028", "", utils.MsgLinkLabelRequired},
		{"zero_width_space_only", "\u200b", "", utils.MsgLinkLabelRequired},
		{"bom_only", "\ufeff", "", utils.MsgLinkLabelRequired},
		{"mongolian_vowel_separator_only", "\u180e", "", utils.MsgLinkLabelRequired},
		{"soft_hyphen_and_word_joiner", "\u00ad\u2060", "", utils.MsgLinkLabelRequired},
		{"bidi_override_only", "\u202e", "", utils.MsgLinkLabelRequired},
		{"bidi_isolates_only", "\u2066\u2069", "", utils.MsgLinkLabelRequired},
		{"arabic_letter_mark_only", "\u061c", "", utils.MsgLinkLabelRequired},
		{"zero_width_around_a_word", "\u200bMenü\u200b", "\u200bMenü\u200b", ""},

		{"nul", "a\u0000b", "", utils.MsgLinkLabelInvalidChars},
		{"nul_only", "\u0000", "", utils.MsgLinkLabelInvalidChars},
		{"bel", "\u0007", "", utils.MsgLinkLabelInvalidChars},
		{"inner_newline", "a\nb", "", utils.MsgLinkLabelInvalidChars},
		{"inner_tab", "a\tb", "", utils.MsgLinkLabelInvalidChars},
		{"delete", "a\u007fb", "", utils.MsgLinkLabelInvalidChars},
		{"c1_control", "a\u0090b", "", utils.MsgLinkLabelInvalidChars},
		{"inner_nel", "a\u0085b", "", utils.MsgLinkLabelInvalidChars},
		{"control_reported_before_the_length", strings.Repeat("a", 50) + "\u0007", "", utils.MsgLinkLabelInvalidChars},
		{"control_reported_before_required", "\u200b\u0007", "", utils.MsgLinkLabelInvalidChars},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, message := utils.CheckMenuLinkLabel(tc.label)
			if message != tc.message {
				t.Fatalf("CheckMenuLinkLabel(%q) message %q, want %q", tc.label, message, tc.message)
			}
			if message == "" && got != tc.want {
				t.Fatalf("CheckMenuLinkLabel(%q) = %q, want %q", tc.label, got, tc.want)
			}
		})
	}
}

func TestCheckMenuLinkURL(t *testing.T) {
	const invalid = "invalid"
	longest := "https://example.com/" + strings.Repeat("a", 480)

	cases := []struct {
		name    string
		address string
		want    string // the stored address when accepted, "invalid" or "too long" otherwise
	}{
		// --- accepted
		{"https", "https://example.com", "https://example.com"},
		{"http", "http://example.com", "http://example.com"},
		{"scheme_is_lowercased_and_nothing_else", "HTTPS://Example.COM/Masa?No=4#Üst", "https://Example.COM/Masa?No=4#Üst"},
		{"mixed_case_scheme", "HtTp://ornek.com.tr/a", "http://ornek.com.tr/a"},
		{"trimmed", "  https://example.com/x \u00a0", "https://example.com/x"},
		{"exactly_500_bytes", longest, longest},
		{"unicode_host", "https://örnek.com", "https://örnek.com"},
		{"punycode_host", "https://xn--rnek-5qa.com", "https://xn--rnek-5qa.com"},
		{"hyphen_inside_labels", "https://a-b.c-d.com", "https://a-b.c-d.com"},
		{"digits_in_the_last_label_with_a_letter", "https://example.c0m", "https://example.c0m"},
		{"port_1", "https://example.com:1/", "https://example.com:1/"},
		{"port_65535", "https://example.com:65535", "https://example.com:65535"},
		{"port_with_leading_zeros_within_5_digits", "https://example.com:00080", "https://example.com:00080"},
		{"percent_escapes_in_the_path", "https://example.com/a%20b", "https://example.com/a%20b"},
		{"unicode_path", "https://example.com/menü", "https://example.com/menü"},
		{"63_character_label", "https://" + strings.Repeat("a", 63) + ".com", "https://" + strings.Repeat("a", 63) + ".com"},

		// --- too long, measured in bytes after the trim
		{"501_bytes", longest + "a", "too long"},
		{"502_bytes_of_261_runes", "https://example.com/" + strings.Repeat("ş", 241), "too long"},
		{"long_is_reported_before_the_scheme", "ftp://" + strings.Repeat("a", 600), "too long"},

		// --- inputs that a check of the scheme alone would accept
		{"markup_in_the_host", "https://<svg/onload=alert(1)>", invalid},
		{"rlo_in_the_host", "https://exa\u202emple.com", invalid},
		{"port_99999", "https://example.com:99999", invalid},
		{"dot_host", "https://.", invalid},
		{"hyphen_host", "https://-", invalid},
		{"https_host_of_a_doubled_scheme", "https://https//ornek.com", invalid},

		// --- characters
		{"space_in_the_path", "https://example.com/a b", invalid},
		{"tab_in_the_path", "https://example.com/a\tb", invalid},
		{"nbsp_in_the_path", "https://example.com/a\u00a0b", invalid},
		{"ideographic_space_in_the_path", "https://example.com/a\u3000b", invalid},
		{"nul", "https://example.com/\u0000", invalid},
		{"bel", "https://example.com/\u0007", invalid},
		{"c1_control_inside", "https://example.com/a" + string(rune(0x90)) + "b", invalid},
		{"nel_inside", "https://example.com/a" + string(rune(0x85)) + "b", invalid},
		{"trailing_nel_is_trimmed_first", "https://example.com/" + string(rune(0x85)), "https://example.com/"},
		{"zero_width_space", "https://example.com/\u200b", invalid},
		{"bom", "https://example.com/\ufeff", invalid},
		{"soft_hyphen_in_the_host", "https://exa\u00admple.com", invalid},
		{"less_than", "https://example.com/<", invalid},
		{"greater_than", "https://example.com/>", invalid},
		{"double_quote", "https://example.com/\"", invalid},
		{"backtick", "https://example.com/`", invalid},
		{"brace_open", "https://example.com/{", invalid},
		{"brace_close", "https://example.com/}", invalid},
		{"pipe", "https://example.com/|", invalid},
		{"backslash", "https://example.com\\evil.com", invalid},
		{"caret", "https://example.com/^", invalid},

		// --- scheme
		{"empty", "", invalid},
		{"blank", "   ", invalid},
		{"no_scheme", "example.com", invalid},
		{"scheme_relative", "//example.com", invalid},
		{"no_slashes", "https:example.com", invalid},
		{"one_slash", "https:/example.com", invalid},
		{"ftp", "ftp://example.com", invalid},
		{"javascript", "javascript:alert(1)", invalid},
		{"data", "data:text/html,<b>x</b>", invalid},
		{"long_s_is_not_s", "http\u017f://example.com", invalid},

		// --- parse and userinfo
		{"bad_percent_escape_in_the_path", "https://example.com/%zz", invalid},
		{"userinfo", "https://user@example.com", invalid},
		{"userinfo_with_password", "https://user:pass@example.com", invalid},
		{"empty_userinfo", "https://@example.com", invalid},
		{"host_disguised_by_userinfo", "https://example.com@evil.example", invalid},

		// --- host name
		{"no_host", "https://", invalid},
		{"port_without_host", "https://:80", invalid},
		{"empty_authority", "https:///example.com", invalid},
		{"single_label", "https://localhost", invalid},
		{"ipv4", "https://192.168.1.1", invalid},
		{"ipv6", "https://[::1]/", invalid},
		{"trailing_dot", "https://example.com.", invalid},
		{"leading_dot", "https://.example.com", invalid},
		{"empty_label", "https://example..com", invalid},
		{"label_starting_with_hyphen", "https://-example.com", invalid},
		{"label_ending_with_hyphen", "https://example-.com", invalid},
		{"last_label_starting_with_hyphen", "https://example.-com", invalid},
		{"underscore", "https://ex_ample.com", invalid},
		{"digits_only_last_label", "https://example.123", invalid},
		{"64_character_label", "https://" + strings.Repeat("a", 64) + ".com", invalid},
		{"254_character_host", "https://" + strings.Repeat(strings.Repeat("a", 62)+".", 4) + "com", invalid},

		// --- port
		{"port_0", "https://example.com:0", invalid},
		{"port_65536", "https://example.com:65536", invalid},
		{"port_of_6_digits", "https://example.com:000080", invalid},
		{"empty_port", "https://example.com:", invalid},
		{"two_ports", "https://example.com:80:90", invalid},
		{"letters_in_the_port", "https://example.com:8o", invalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, message := utils.CheckMenuLinkURL(tc.address)
			switch tc.want {
			case invalid:
				if message != utils.MsgLinkURLInvalid {
					t.Fatalf("CheckMenuLinkURL(%q) = (%q, %q), want the invalid-address message", tc.address, got, message)
				}
			case "too long":
				if message != utils.MsgLinkURLTooLong {
					t.Fatalf("CheckMenuLinkURL(%q) = (%q, %q), want the too-long message", tc.address, got, message)
				}
			default:
				if message != "" || got != tc.want {
					t.Fatalf("CheckMenuLinkURL(%q) = (%q, %q), want (%q, \"\")", tc.address, got, message, tc.want)
				}
			}
		})
	}

	// 253 characters exactly: four 62-letter labels, their dots and "c".
	host253 := strings.Repeat(strings.Repeat("a", 62)+".", 4) + "c"
	if len(host253) != 253 {
		t.Fatalf("fixture: host is %d characters, want 253", len(host253))
	}
	if _, message := utils.CheckMenuLinkURL("https://" + host253); message != "" {
		t.Errorf("a 253-character host name was refused with %q", message)
	}
}

// Square brackets are refused anywhere in an address, so the answer does not
// depend on what net/url's Hostname does with a bracketed host.
func TestCheckMenuLinkURLRefusesSquareBrackets(t *testing.T) {
	for _, address := range []string{
		"https://[ornek.com]",
		"https://[ornek.com]:8080/menu",
		"HTTPS://[a.b]/x",
		"https://[::1]/",
		"https://example.com/[menu]",
		"https://example.com/?masa=[4]",
		// One bracket on its own, in the parts net/url accepts it in, so each
		// of the two characters is refused by its own rule.
		"https://example.com/menu[",
		"https://example.com/menu]",
		"https://example.com/?masa=[",
		"https://example.com/?masa=]",
		"https://example.com/#[",
		"https://example.com/#]",
	} {
		if got, message := utils.CheckMenuLinkURL(address); message != utils.MsgLinkURLInvalid {
			t.Errorf("CheckMenuLinkURL(%q) = (%q, %q), want the invalid-address message", address, got, message)
		}
	}
}

// A label is blank when nothing visible is left once the invisible format
// characters are removed and the rest is trimmed again. U+200E and U+200F are
// the last two characters of the U+200B-U+200F range.
func TestCheckMenuLinkLabelIsBlankWithoutItsInvisibleCharacters(t *testing.T) {
	zeroWidthSpace := string(rune(0x200b))
	leftToRightMark := string(rune(0x200e))
	rightToLeftMark := string(rune(0x200f))
	noBreakSpace := string(rune(0x00a0))

	for _, tc := range []struct {
		name    string
		label   string
		message string
	}{
		{"space_between_two_zero_width_spaces", zeroWidthSpace + " " + zeroWidthSpace, utils.MsgLinkLabelRequired},
		{"spaces_between_invisible_characters", zeroWidthSpace + "  " + noBreakSpace + leftToRightMark, utils.MsgLinkLabelRequired},
		{"left_to_right_mark_only", leftToRightMark, utils.MsgLinkLabelRequired},
		{"right_to_left_mark_only", rightToLeftMark, utils.MsgLinkLabelRequired},
		{"space_between_the_two_marks", leftToRightMark + " " + rightToLeftMark, utils.MsgLinkLabelRequired},
		{"word_between_the_two_marks", leftToRightMark + "Menü" + rightToLeftMark, ""},
		{"word_after_an_invisible_character_and_a_space", zeroWidthSpace + " Menü", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, message := utils.CheckMenuLinkLabel(tc.label)
			if message != tc.message {
				t.Fatalf("CheckMenuLinkLabel(%q) message %q, want %q", tc.label, message, tc.message)
			}
			if message == "" && got != tc.label {
				t.Fatalf("CheckMenuLinkLabel(%q) = %q, want the label as it was, invisible characters included", tc.label, got)
			}
		})
	}
}

func TestCheckMenuLinkURLRefusesTheDirectionMarks(t *testing.T) {
	for _, mark := range []rune{0x200e, 0x200f} {
		for _, address := range []string{
			"https://example.com/" + string(mark),
			"https://exa" + string(mark) + "mple.com",
			"https://example.com/?q=a" + string(mark) + "b",
		} {
			if got, message := utils.CheckMenuLinkURL(address); message != utils.MsgLinkURLInvalid {
				t.Errorf("CheckMenuLinkURL(%q) = (%q, %q), want the invalid-address message", address, got, message)
			}
		}
	}
}

// The host name limit counts characters, not bytes. Every label below is 61
// ASCII letters and one two-byte letter, so the host is longer in bytes than
// in characters, and the address stays well under the 500-byte limit.
func TestCheckMenuLinkURLCountsTheHostNameInCharacters(t *testing.T) {
	label := strings.Repeat("a", 61) + "ö"
	host253 := strings.Repeat(label+".", 4) + "c"
	host254 := strings.Repeat(label+".", 4) + "cc"

	if n := utf8.RuneCountInString(host253); n != 253 || len(host253) <= 253 {
		t.Fatalf("fixture: host is %d characters and %d bytes, want 253 characters in more bytes", n, len(host253))
	}
	if n := utf8.RuneCountInString(host254); n != 254 {
		t.Fatalf("fixture: host is %d characters, want 254", n)
	}

	if got, message := utils.CheckMenuLinkURL("https://" + host253); message != "" || got != "https://"+host253 {
		t.Errorf("a host name of 253 characters (%d bytes) was refused: (%q, %q)", len(host253), got, message)
	}
	if _, message := utils.CheckMenuLinkURL("https://" + host254); message != utils.MsgLinkURLInvalid {
		t.Errorf("a host name of 254 characters got %q, want the invalid-address message", message)
	}
}

func TestCheckMenuLinkChecksTheLabelFirst(t *testing.T) {
	for _, tc := range []struct {
		name, label, address, message string
	}{
		{"blank_label_and_bad_url", "", "ftp://x", utils.MsgLinkLabelRequired},
		{"long_label_and_long_url", strings.Repeat("a", 41), "https://example.com/" + strings.Repeat("a", 600), utils.MsgLinkLabelTooLong},
		{"control_label_and_bad_url", "\u0007", "javascript:alert(1)", utils.MsgLinkLabelInvalidChars},
		{"good_label_and_bad_url", "Menü", "https://localhost", utils.MsgLinkURLInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, message := utils.CheckMenuLink(tc.label, tc.address); message != tc.message {
				t.Fatalf("CheckMenuLink(%q, %q) message %q, want %q", tc.label, tc.address, message, tc.message)
			}
		})
	}

	label, address, message := utils.CheckMenuLink("  Rezervasyon ", " HTTPS://Rezervasyon.example/Masa ")
	if message != "" || label != "Rezervasyon" || address != "https://Rezervasyon.example/Masa" {
		t.Fatalf("CheckMenuLink of a valid link = (%q, %q, %q)", label, address, message)
	}
}
