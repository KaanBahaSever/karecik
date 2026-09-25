package tests

// Language negotiation of the customer menu, over HTTP.
//
// A customer who scans a QR code never chose a language; the phone already
// says which one it reads, in Accept-Language. The contract is: an explicit
// ?lang= the menu offers wins, then the first Accept-Language entry the menu
// offers, then the menu's default — and the payload says which one it used, in
// "language". Because the same address now answers differently per header,
// every answer also carries Vary: Accept-Language, and the ETag of one language
// never revalidates another's copy.

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

type negotiatedMenu struct {
	Language     string `json:"language"`
	MenuResolved bool   `json:"menu_resolved"`
	Categories   []struct {
		Name     string `json:"name"`
		Products []struct {
			Name string `json:"name"`
		} `json:"products"`
	} `json:"categories"`
	Footer struct {
		PriceDate string `json:"price_date"`
		PriceNote string `json:"price_note"`
	} `json:"footer"`
}

func TestPublicMenuNegotiatesTheLanguage(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Dil Kafe", "language-owner@example.test")
	businessSlug := readBusinessSlug(t, h, owner)

	menu := h.createMenu(owner, "Ana Menü")
	resp, payload := h.do(http.MethodPut, "/api/menus/"+menu.ID, owner.session, map[string]any{
		"default_language": "tr",
		"languages":        []string{"tr", "en", "de"},
	})
	h.requireSuccess("menu languages", resp, payload)

	resp, payload = h.do(http.MethodPost, "/api/categories", owner.session, map[string]any{
		"menu_id": menu.ID,
		"translations": map[string]any{
			"tr": map[string]any{"name": "Sıcak İçecekler"},
			"en": map[string]any{"name": "Hot Drinks"},
			"de": map[string]any{"name": "Heißgetränke"},
		},
	})
	h.requireSuccess("category", resp, payload)
	var category categoryPayload
	decodeInto(t, "category", payload, &category)

	resp, payload = h.do(http.MethodPost, "/api/products", owner.session, map[string]any{
		"category_id": category.ID,
		"translations": map[string]any{
			"tr": map[string]any{"name": "Latte"},
			"en": map[string]any{"name": "Latte"},
			"de": map[string]any{"name": "Latte"},
		},
		"price": 90,
	})
	h.requireSuccess("product", resp, payload)

	path := "/api/public/menu/" + businessSlug + "/" + menu.Slug
	read := func(t *testing.T, what, query, acceptLanguage string) (negotiatedMenu, *http.Response) {
		t.Helper()
		headers := map[string]string{}
		if acceptLanguage != "" {
			headers["Accept-Language"] = acceptLanguage
		}
		resp, payload := h.doWith(http.MethodGet, path+query, "", nil, headers)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: GET %s%s answered %d: %s", what, path, query, resp.StatusCode, payload)
		}
		var body negotiatedMenu
		decodeInto(t, what, payload, &body)
		return body, resp
	}

	for _, tc := range []struct {
		name, query, header, want, category string
	}{
		{"no_hint_is_the_default", "", "", "tr", "Sıcak İçecekler"},
		{"the_header_beats_the_default", "", "de-DE,de;q=0.9,en;q=0.8", "de", "Heißgetränke"},
		{"the_first_offered_header_entry", "", "fr-FR, ja;q=0.9, en;q=0.5", "en", "Hot Drinks"},
		{"an_unoffered_header_is_the_default", "", "ja, fr;q=0.5", "tr", "Sıcak İçecekler"},
		{"lang_beats_the_header", "?lang=en", "de", "en", "Hot Drinks"},
		{"an_empty_lang_is_absent", "?lang=", "de", "de", "Heißgetränke"},
		{"an_unoffered_lang_falls_through_to_the_header", "?lang=fr", "de", "de", "Heißgetränke"},
		{"garbage_header", "", ";;q=x,*,", "tr", "Sıcak İçecekler"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, resp := read(t, tc.name, tc.query, tc.header)
			if body.Language != tc.want {
				t.Errorf("language %q, want %q", body.Language, tc.want)
			}
			if len(body.Categories) != 1 || body.Categories[0].Name != tc.category {
				t.Errorf("categories %+v, want one named %q", body.Categories, tc.category)
			}
			if vary := resp.Header.Get("Vary"); !strings.Contains(vary, "Accept-Language") {
				t.Errorf("Vary is %q, want it to name Accept-Language", vary)
			}
		})
	}

	t.Run("an_etag_never_revalidates_another_language", func(t *testing.T) {
		_, german := read(t, "german", "", "de")
		etag := german.Header.Get("ETag")
		if etag == "" {
			t.Fatal("the public menu carries no ETag")
		}
		resp, _ := h.doWith(http.MethodGet, path, "", nil,
			map[string]string{"Accept-Language": "de", "If-None-Match": etag})
		if resp.StatusCode != http.StatusNotModified {
			t.Errorf("the same language revalidated with %d, want 304", resp.StatusCode)
		}
		resp, payload := h.doWith(http.MethodGet, path, "", nil,
			map[string]string{"Accept-Language": "en", "If-None-Match": etag})
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(payload), "Hot Drinks") {
			t.Errorf("an English visitor holding the German ETag got %d: %s", resp.StatusCode, payload)
		}
	})

	t.Run("the_footer_carries_the_price_date", func(t *testing.T) {
		body, _ := read(t, "footer", "", "")
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(body.Footer.PriceDate) {
			t.Fatalf("footer.price_date is %q, want YYYY-MM-DD", body.Footer.PriceDate)
		}
		if body.Footer.PriceNote == "" {
			t.Error("footer.price_note disappeared; it stays for older clients")
		}

		resp, payload := h.do(http.MethodPut, "/api/menus/"+menu.ID, owner.session,
			map[string]any{"show_price_date": false})
		h.requireSuccess("hide the price date", resp, payload)
		body, _ = read(t, "footer hidden", "", "")
		if body.Footer.PriceDate != "" || body.Footer.PriceNote != "" {
			t.Errorf("a hidden price date still sends %q / %q", body.Footer.PriceDate, body.Footer.PriceNote)
		}
		resp, payload = h.do(http.MethodPut, "/api/menus/"+menu.ID, owner.session,
			map[string]any{"show_price_date": true})
		h.requireSuccess("show the price date again", resp, payload)
	})

	t.Run("the_preview_reports_its_language_but_ignores_the_owners_browser", func(t *testing.T) {
		for _, tc := range []struct{ query, header, want string }{
			{"", "de", "tr"},
			{"&lang=en", "de", "en"},
		} {
			resp, payload := h.doWith(http.MethodGet, "/api/preview/menu?menu="+menu.Slug+tc.query,
				owner.session, nil, map[string]string{"Accept-Language": tc.header})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("preview answered %d: %s", resp.StatusCode, payload)
			}
			var body negotiatedMenu
			decodeInto(t, "preview", payload, &body)
			if body.Language != tc.want {
				t.Errorf("preview%s with Accept-Language %s: language %q, want %q",
					tc.query, tc.header, body.Language, tc.want)
			}
		}
	})

	t.Run("the_directory_negotiates_against_every_supported_language", func(t *testing.T) {
		h.createMenu(owner, "Akşam Menüsü") // two menus: the bare address is the directory
		directory := "/api/public/menu/" + businessSlug
		for _, tc := range []struct{ query, header, want string }{
			{"", "", "tr"},
			{"", "ar-SA,ar;q=0.9", "ar"},
			{"", "ja", "tr"},
			{"?lang=ru", "de", "ru"},
		} {
			resp, payload := h.doWith(http.MethodGet, directory+tc.query, "", nil,
				map[string]string{"Accept-Language": tc.header})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("directory answered %d: %s", resp.StatusCode, payload)
			}
			var body negotiatedMenu
			decodeInto(t, "directory", payload, &body)
			if body.MenuResolved {
				t.Fatalf("fixture: the bare address resolved a menu: %s", payload)
			}
			if body.Language != tc.want {
				t.Errorf("directory%s with %q: language %q, want %q", tc.query, tc.header, body.Language, tc.want)
			}
			if vary := resp.Header.Get("Vary"); !strings.Contains(vary, "Accept-Language") {
				t.Errorf("the directory's Vary is %q", vary)
			}
		}

		// The field is always present, never omitted.
		_, payload := h.do(http.MethodGet, directory, "", nil)
		var raw map[string]json.RawMessage
		decodeInto(t, "directory raw", payload, &raw)
		if _, ok := raw["language"]; !ok {
			t.Errorf("the directory payload has no language key: %s", payload)
		}
	})

	t.Run("a_missing_tenant_still_varies", func(t *testing.T) {
		resp, _ := h.do(http.MethodGet, "/api/public/menu/no-such-tenant-here", "", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("an unknown tenant answered %d", resp.StatusCode)
		}
		if vary := resp.Header.Get("Vary"); !strings.Contains(vary, "Accept-Language") {
			t.Errorf("the 404's Vary is %q", vary)
		}
	})
}
