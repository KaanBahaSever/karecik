package handlers_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"karecik/backend/internal/handlers"
	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// The rule for each single link is tested in utils (CheckMenuLink). These are
// the list rules around it: the count, the order the messages come in, and the
// ids.

func TestSanitizeMenuLinksReportsTheFirstProblemInOrder(t *testing.T) {
	link := func(label, address string) models.MenuLink {
		return models.MenuLink{Label: label, URL: address}
	}
	nine := make([]models.MenuLink, 9)
	for i := range nine {
		nine[i] = link("", "ftp://x") // every one of them invalid, too
	}

	cases := []struct {
		name    string
		in      []models.MenuLink
		message string
	}{
		{"nine_entries_report_the_count_first", nine, utils.MsgLinksTooMany},
		{"first_entry_wins", []models.MenuLink{link("İyi", "ftp://x"), link("", "https://example.com")}, utils.MsgLinkURLInvalid},
		{"a_later_entry_is_reached", []models.MenuLink{link("İyi", "https://iyi.example"), link("\u0007", "https://example.com")}, utils.MsgLinkLabelInvalidChars},
		{"label_before_url", []models.MenuLink{link(strings.Repeat("a", 41), "https://example.com/"+strings.Repeat("a", 600))}, utils.MsgLinkLabelTooLong},
		{"nul_label_is_a_refusal", []models.MenuLink{link("a\u0000b", "https://example.com")}, utils.MsgLinkLabelInvalidChars},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, message := handlers.SanitizeMenuLinks(tc.in)
			if message != tc.message {
				t.Fatalf("SanitizeMenuLinks message %q, want %q", message, tc.message)
			}
			if out != nil {
				t.Fatalf("a refused list came back with %+v, want nil", out)
			}
		})
	}
}

func TestSanitizeMenuLinksKeepsOnlyUniqueValidIDs(t *testing.T) {
	in := []models.MenuLink{
		{ID: "keep_me-1", Label: "Bir", URL: "https://bir.example"},
		{ID: "keep_me-1", Label: "İki", URL: "https://iki.example"}, // repeated: replaced
		{ID: "", Label: "Üç", URL: "https://uc.example"},            // missing: replaced
		{ID: "bad id!", Label: "Dört", URL: "https://dort.example"}, // invalid: replaced
		{ID: strings.Repeat("b", 64), Label: "Beş", URL: "https://bes.example"},
		{ID: strings.Repeat("b", 64), Label: "Altı", URL: "https://alti.example"}, // repeated: replaced
		{ID: "other", Label: "Yedi", URL: "https://yedi.example"},
		{ID: "keep_me-1", Label: "Sekiz", URL: "https://sekiz.example"}, // repeated again: replaced
	}
	kept := map[int]string{0: "keep_me-1", 4: strings.Repeat("b", 64), 6: "other"}

	out, message := handlers.SanitizeMenuLinks(in)
	if message != "" {
		t.Fatalf("SanitizeMenuLinks refused a valid list: %q", message)
	}
	if len(out) != len(in) {
		t.Fatalf("SanitizeMenuLinks returned %d links, want %d", len(out), len(in))
	}

	seen := make(map[string]bool)
	for i, link := range out {
		if link.Label != in[i].Label || link.URL != in[i].URL {
			t.Errorf("link #%d is %+v, want label %q and url %q in the owner's order", i, link, in[i].Label, in[i].URL)
		}
		if want, ok := kept[i]; ok {
			if link.ID != want {
				t.Errorf("link #%d id %q, want the first valid occurrence %q kept", i, link.ID, want)
			}
		} else if _, err := uuid.Parse(link.ID); err != nil {
			t.Errorf("link #%d id %q, want a fresh UUID in place of %q", i, link.ID, in[i].ID)
		}
		if seen[link.ID] {
			t.Errorf("link #%d id %q is used twice: %v", i, link.ID, out)
		}
		seen[link.ID] = true
	}
}

func TestSanitizeMenuLinksOfNothingIsAnEmptyList(t *testing.T) {
	for _, in := range [][]models.MenuLink{nil, {}} {
		out, message := handlers.SanitizeMenuLinks(in)
		if message != "" || out == nil || len(out) != 0 {
			t.Fatalf("SanitizeMenuLinks(%#v) = (%#v, %q), want an empty, non-nil list", in, out, message)
		}
	}

	// Exactly the limit is fine.
	eight := make([]models.MenuLink, utils.MaxMenuLinks)
	for i := range eight {
		eight[i] = models.MenuLink{Label: fmt.Sprintf("Link %d", i), URL: "https://example.com"}
	}
	if _, message := handlers.SanitizeMenuLinks(eight); message != "" {
		t.Fatalf("eight links were refused with %q", message)
	}
}
