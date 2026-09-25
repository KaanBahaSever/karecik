package utils_test

import (
	"reflect"
	"strings"
	"testing"

	"karecik/backend/internal/utils"
)

// The Accept-Language parser decides which language a customer who never
// touched the language picker reads the menu in, so every shape a browser —
// or something pretending to be one — can send is pinned down here.
func TestParseAcceptLanguage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   []string
	}{
		{"empty", "", []string{}},
		{"one_bare_language", "de", []string{"de"}},
		{"region_is_stripped", "de-DE", []string{"de"}},
		{"underscore_region_and_case", "EN_gb", []string{"en"}},
		{"script_subtag_is_stripped", "zh-Hant-TW", []string{"zh"}},
		{"browser_order_without_q", "tr-TR,tr,en-US,en", []string{"tr", "en"}},
		{"q_values_reorder", "en;q=0.5, de;q=0.9, fr", []string{"fr", "de", "en"}},
		{"equal_q_keeps_the_sent_order", "fr;q=0.7, ru;q=0.7, ar;q=0.7", []string{"fr", "ru", "ar"}},
		{"a_repeated_base_keeps_its_highest_q", "en-US;q=0.2, de;q=0.5, en-GB;q=0.9", []string{"en", "de"}},
		{"q_zero_means_not_acceptable", "de;q=0, en", []string{"en"}},
		{"wildcard_is_dropped", "*, de;q=0.5", []string{"de"}},
		{"wildcard_alone", "*", []string{}},
		{"spaces_everywhere", "  de-AT ; q = 0.8 ,  en ", []string{"en", "de"}},
		{"uppercase_q_parameter", "de;Q=0.1, fr;q=0.2", []string{"fr", "de"}},
		{"other_parameters_are_ignored", "de;level=1;q=0.4, en;q=0.3", []string{"de", "en"}},
		{"garbage_q_drops_only_its_entry", "de;q=abc, en;q=0.3", []string{"en"}},
		{"q_above_one_drops_its_entry", "de;q=1.5, en", []string{"en"}},
		{"negative_q_drops_its_entry", "de;q=-0.1, en", []string{"en"}},
		{"private_use_is_dropped", "x-klingon, i-enochian, en", []string{"en"}},
		{"digits_are_not_a_language", "12, 1a, en", []string{"en"}},
		{"too_long_a_primary_subtag", "english, en", []string{"en"}},
		{"empty_entries", ",,;q=1, ,en,", []string{"en"}},
		{"non_ascii_letters", "tü, en", []string{"en"}},
		{"three_letter_code", "fil-PH", []string{"fil"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.ParseAcceptLanguage(tc.header)
			if got == nil {
				t.Fatalf("ParseAcceptLanguage(%q) = nil, want a non-nil list", tc.header)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseAcceptLanguage(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

// A hostile header costs no more than a real one: only the first entries are
// looked at.
func TestParseAcceptLanguageBoundsTheWork(t *testing.T) {
	header := strings.Repeat("xx,", 10000) + "de"
	if got := utils.ParseAcceptLanguage(header); len(got) > 32 {
		t.Fatalf("ParseAcceptLanguage returned %d entries for a 10000-entry header", len(got))
	}
}

// The precedence the public menu relies on: an explicit offered ?lang= wins,
// then the first offered Accept-Language entry, then the default.
func TestNegotiateLanguage(t *testing.T) {
	offered := []string{"tr", "en", "de"}
	for _, tc := range []struct {
		name     string
		explicit string
		header   string
		want     string
	}{
		{"nothing_gives_the_default", "", "", "tr"},
		{"the_header_beats_the_default", "", "de-DE,de;q=0.9,en;q=0.8", "de"},
		{"the_first_offered_header_entry", "", "fr-FR, ru;q=0.9, en;q=0.5", "en"},
		{"nothing_offered_gives_the_default", "", "fr, ru", "tr"},
		{"explicit_beats_the_header", "en", "de", "en"},
		{"explicit_is_case_insensitive", " EN ", "de", "en"},
		{"an_explicit_language_not_offered_falls_through", "fr", "de", "de"},
		{"an_unsupported_explicit_value_falls_through", "xx", "", "tr"},
		{"garbage_header", "", ";;;,,q=5", "tr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := utils.NegotiateLanguage(tc.explicit, tc.header, offered, "tr"); got != tc.want {
				t.Errorf("NegotiateLanguage(%q, %q) = %q, want %q", tc.explicit, tc.header, got, tc.want)
			}
		})
	}

	// The fallback is returned even when the menu's list does not name it.
	if got := utils.NegotiateLanguage("", "ar", []string{"en"}, "tr"); got != "tr" {
		t.Errorf("fallback outside the offered list: got %q, want tr", got)
	}
}

func TestLanguageCodesMatchesTheCatalogue(t *testing.T) {
	codes := utils.LanguageCodes()
	if len(codes) != len(utils.Languages) {
		t.Fatalf("LanguageCodes has %d entries, the catalogue %d", len(codes), len(utils.Languages))
	}
	for i, language := range utils.Languages {
		if codes[i] != language.Code {
			t.Errorf("LanguageCodes[%d] = %q, want %q", i, codes[i], language.Code)
		}
	}
}
