package repository_test

import (
	"fmt"
	"strings"
	"testing"

	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// PublicLinks is the last gate between a stored links row and a customer. The
// API refuses a bad list, so these cases are rows written past it.

func idsOf(links models.MenuLinks) []string {
	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.ID)
	}
	return ids
}

func TestPublicLinksDropsEveryEntryThatBreaksALinkRule(t *testing.T) {
	stored := models.MenuLinks{
		{ID: "rez", Label: "  Rezervasyon ", URL: " HTTPS://Rezervasyon.example/Masa "},
		{ID: "svg", Label: "XSS", URL: "https://<svg/onload=alert(1)>"},
		{ID: "rlo", Label: "Ters", URL: "https://exa\u202emple.com"},
		{ID: "port", Label: "Port", URL: "https://example.com:99999"},
		{ID: "dot", Label: "Nokta", URL: "https://."},
		{ID: "dash", Label: "Tire", URL: "https://-"},
		{ID: "doubled", Label: "Çift", URL: "https://https//ornek.com"},
		{ID: "js", Label: "Betik", URL: "javascript:alert(1)"},
		{ID: "zwsp", Label: "\u200b", URL: "https://example.com"},
		{ID: "newline", Label: "\n", URL: "https://example.com"},
		{ID: "bel", Label: "\u0007", URL: "https://example.com"},
		{ID: "nul", Label: "a\u0000b", URL: "https://example.com"},
		{ID: "long", Label: strings.Repeat("ş", 41), URL: "https://example.com"},
		{ID: "longurl", Label: "Uzun", URL: "https://example.com/" + strings.Repeat("a", 481)},
		{ID: "paket", Label: "Paket Servis", URL: "http://paket.example/siparis"},
	}

	got := repository.PublicLinks(stored)
	want := models.MenuLinks{
		{ID: "rez", Label: "Rezervasyon", URL: "https://Rezervasyon.example/Masa"},
		{ID: "paket", Label: "Paket Servis", URL: "http://paket.example/siparis"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("PublicLinks kept\n%+v\nwant only the two valid entries, cleaned, in their stored order:\n%+v", got, want)
	}
}

func TestPublicLinksGivesEveryEntryAUniqueID(t *testing.T) {
	link := func(id string) models.MenuLink {
		return models.MenuLink{ID: id, Label: "Link", URL: "https://example.com"}
	}

	cases := []struct {
		name   string
		stored models.MenuLinks
		want   []string
	}{
		{"ids_kept_as_stored", models.MenuLinks{link("a"), link("b")}, []string{"a", "b"}},
		{"any_non_blank_id_is_kept_without_rewriting", models.MenuLinks{link("bad id!"), link(" x ")}, []string{"bad id!", " x "}},
		{"blank_ids", models.MenuLinks{link(""), link("   ")}, []string{"link-0", "link-1"}},
		{"repeated_id", models.MenuLinks{link("x"), link("x"), link("x")}, []string{"x", "link-1", "link-2"}},
		{"index_is_the_position_in_the_payload",
			models.MenuLinks{{ID: "bad", Label: "", URL: "https://example.com"}, link("")}, []string{"link-0"}},
		{"generated_id_skips_one_an_earlier_entry_carries", models.MenuLinks{link("link-1"), link("")}, []string{"link-1", "link-2"}},
		{"generated_id_skips_one_a_later_entry_carries", models.MenuLinks{link(""), link("link-0")}, []string{"link-1", "link-0"}},
		{"every_generated_id_is_free", models.MenuLinks{link("link-2"), link(""), link("link-1"), link("")},
			[]string{"link-2", "link-3", "link-1", "link-4"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := idsOf(repository.PublicLinks(tc.stored))
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("PublicLinks ids %q, want %q", got, tc.want)
			}
			seen := make(map[string]bool)
			for _, id := range got {
				if seen[id] {
					t.Fatalf("PublicLinks ids %q repeat %q", got, id)
				}
				seen[id] = true
			}
		})
	}
}

func TestPublicLinksKeepsAtMostTheLimit(t *testing.T) {
	stored := make(models.MenuLinks, 0, 12)
	stored = append(stored, models.MenuLink{ID: "bad", Label: "", URL: "https://example.com"})
	for i := 0; i < 11; i++ {
		stored = append(stored, models.MenuLink{ID: fmt.Sprintf("l%d", i), Label: "Link", URL: "https://example.com"})
	}

	got := idsOf(repository.PublicLinks(stored))
	want := []string{"l0", "l1", "l2", "l3", "l4", "l5", "l6", "l7"}
	if len(want) != utils.MaxMenuLinks {
		t.Fatalf("fixture: want %d ids, the limit is %d", len(want), utils.MaxMenuLinks)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("PublicLinks of 11 valid entries kept %q, want the first %d that pass: %q",
			got, utils.MaxMenuLinks, want)
	}
}

func TestPublicLinksIsNeverNilAndAlwaysTheSame(t *testing.T) {
	for _, stored := range []models.MenuLinks{nil, {}, {{Label: "", URL: ""}}} {
		if got := repository.PublicLinks(stored); got == nil || len(got) != 0 {
			t.Fatalf("PublicLinks(%#v) = %#v, want an empty, non-nil list", stored, got)
		}
	}

	stored := models.MenuLinks{{ID: "", Label: "A", URL: "https://a.example"}, {ID: "", Label: "B", URL: "https://b.example"}}
	first, second := repository.PublicLinks(stored), repository.PublicLinks(stored)
	if fmt.Sprint(first) != fmt.Sprint(second) {
		t.Fatalf("two payloads built from the same row differ: %+v and %+v", first, second)
	}
	if stored[0].ID != "" || stored[1].ID != "" {
		t.Fatalf("PublicLinks rewrote the stored list itself: %+v", stored)
	}
}

func TestToPublicBusinessFiltersLinksAndHidesThemInHiddenMode(t *testing.T) {
	business := &models.Business{Name: "Kafe", Slug: "kafe"}
	menu := &models.Menu{
		Name: "Menü", Slug: "menu", ContactDisplay: utils.ContactDisplayInline,
		Links: models.MenuLinks{
			{ID: "ok", Label: "Rezervasyon", URL: "https://rezervasyon.example"},
			{ID: "svg", Label: "XSS", URL: "https://<svg/onload=alert(1)>"},
		},
	}

	public := repository.ToPublicBusiness(business, menu)
	if got := idsOf(public.Links); strings.Join(got, "|") != "ok" {
		t.Fatalf("inline mode: public links %q, want only the valid entry", got)
	}
	if len(menu.Links) != 2 {
		t.Fatalf("ToPublicBusiness changed the menu's own links to %+v", menu.Links)
	}

	menu.ContactDisplay = utils.ContactDisplayHidden
	public = repository.ToPublicBusiness(business, menu)
	if public.Links == nil || len(public.Links) != 0 {
		t.Fatalf("hidden mode: public links %#v, want an empty, non-nil list", public.Links)
	}
}
