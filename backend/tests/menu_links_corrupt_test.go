package tests

// A menus.links row the API would never write must not take a menu down, and
// must not reach a customer.
//
// Every menu read scans links, so a scan that failed on one element of the
// wrong type, written straight into the table, would take down the menu list,
// the menu itself, the public menu and the preview with a 500. Each case below
// writes its row through h.pool, past every check the API makes, and reads it
// back four ways. The owner's two reads return what the row holds (skipping
// only what cannot be read at all), so the owner can see a bad entry and fix
// it; the public menu and the preview carry only the entries that pass the
// link rules, with ids that are unique.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// storeLinksPastTheAPI writes a value straight into menus.links.
func storeLinksPastTheAPI(t *testing.T, h *harness, menuID string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("could not encode %#v: %v", value, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := h.pool.Exec(ctx,
		`UPDATE menus SET links = $1::text::jsonb WHERE id = $2::text::uuid`, string(raw), menuID); err != nil {
		t.Fatalf("could not store links %s: %v", raw, err)
	}
}

// linksFromMenuList reads one menu's links through GET /api/menus.
func (s *contactSuite) linksFromMenuList(t *testing.T, what, menuID string) []contactLink {
	t.Helper()
	resp, payload := s.h.do(http.MethodGet, "/api/menus", s.owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: GET /api/menus answered %d, want 200: %s", what, resp.StatusCode, payload)
	}
	var list []contactMenu
	decodeInto(t, what, payload, &list)
	for _, menu := range list {
		if menu.ID == menuID {
			if menu.Links == nil {
				t.Fatalf("%s: GET /api/menus carries links null for menu %s: %s", what, menuID, payload)
			}
			return menu.Links
		}
	}
	t.Fatalf("%s: GET /api/menus does not list menu %s: %s", what, menuID, payload)
	return nil
}

// linksOfPayload decodes business.links of a public or preview payload, which
// has to be an array.
func linksOfPayload(t *testing.T, what string, business map[string]json.RawMessage) []contactLink {
	t.Helper()
	raw := string(business["links"])
	if !strings.HasPrefix(raw, "[") {
		t.Fatalf("%s: business.links is %s, want an array", what, raw)
	}
	var links []contactLink
	if err := json.Unmarshal(business["links"], &links); err != nil {
		t.Fatalf("%s: could not decode business.links %s: %v", what, raw, err)
	}
	return links
}

func TestCorruptStoredLinksNeverTakeAMenuDown(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Bozuk Link Kafe", "corrupt-links-owner@example.test")
	s := &contactSuite{h: h, owner: owner, businessSlug: readBusinessSlug(t, h, owner)}

	menu := h.createMenu(owner, "Bozuk Menü")
	category := h.createCategory(owner, menu.ID, "Kahveler")
	h.createProduct(owner, category.ID, "Latte", 120)

	good := map[string]any{"id": "rez", "label": "Rezervasyon", "url": "https://rezervasyon.example/masa"}
	goodLink := contactLink{ID: "rez", Label: "Rezervasyon", URL: "https://rezervasyon.example/masa"}

	zeroWidthSpace := string(rune(0x200b))
	bell := string(rune(0x07))
	rightToLeftOverride := string(rune(0x202e))

	cases := []struct {
		name   string
		stored any
		owner  []contactLink
		public []contactLink
	}{
		{
			name:   "a_numeric_label",
			stored: []any{map[string]any{"id": "num", "label": 42, "url": "https://a.example"}, good},
			owner:  []contactLink{goodLink},
			public: []contactLink{goodLink},
		},
		{
			name:   "a_string_element",
			stored: []any{"x", good},
			owner:  []contactLink{goodLink},
			public: []contactLink{goodLink},
		},
		{
			name:   "an_array_element",
			stored: []any{[]any{1, 2}, good},
			owner:  []contactLink{goodLink},
			public: []contactLink{goodLink},
		},
		{
			name:   "an_object_url",
			stored: []any{map[string]any{"id": "obj", "label": "Nesne", "url": map[string]any{}}, good},
			owner:  []contactLink{goodLink},
			public: []contactLink{goodLink},
		},
		{
			name:   "a_null_element",
			stored: []any{nil, good},
			owner:  []contactLink{goodLink},
			public: []contactLink{goodLink},
		},
		{
			name:   "nothing_but_numbers",
			stored: []any{1, 2},
			owner:  []contactLink{},
			public: []contactLink{},
		},
		{
			name: "every_bad_shape_at_once",
			stored: []any{
				map[string]any{"label": 42}, "x", []any{1, 2}, map[string]any{"url": map[string]any{}},
				nil, true, good, 7.5,
			},
			owner:  []contactLink{goodLink},
			public: []contactLink{goodLink},
		},
		{
			name:   "a_numeric_id",
			stored: []any{map[string]any{"id": 7, "label": "Yedi", "url": "https://yedi.example"}},
			owner:  []contactLink{{ID: "", Label: "Yedi", URL: "https://yedi.example"}},
			public: []contactLink{{ID: "link-0", Label: "Yedi", URL: "https://yedi.example"}},
		},
		{
			name: "entries_that_break_a_link_rule",
			stored: []any{
				map[string]any{"id": "svg", "label": "XSS", "url": "https://<svg/onload=alert(1)>"},
				map[string]any{"id": "rlo", "label": "Ters", "url": "https://exa" + rightToLeftOverride + "mple.com"},
				map[string]any{"id": "port", "label": "Port", "url": "https://example.com:99999"},
				map[string]any{"id": "doubled", "label": "Çift", "url": "https://https//ornek.com"},
				map[string]any{"id": "zwsp", "label": zeroWidthSpace, "url": "https://example.com"},
				map[string]any{"id": "bel", "label": bell, "url": "https://example.com"},
				map[string]any{"id": "long", "label": strings.Repeat("ş", 41), "url": "https://example.com"},
				good,
			},
			owner: []contactLink{
				{ID: "svg", Label: "XSS", URL: "https://<svg/onload=alert(1)>"},
				{ID: "rlo", Label: "Ters", URL: "https://exa" + rightToLeftOverride + "mple.com"},
				{ID: "port", Label: "Port", URL: "https://example.com:99999"},
				{ID: "doubled", Label: "Çift", URL: "https://https//ornek.com"},
				{ID: "zwsp", Label: zeroWidthSpace, URL: "https://example.com"},
				{ID: "bel", Label: bell, URL: "https://example.com"},
				{ID: "long", Label: strings.Repeat("ş", 41), URL: "https://example.com"},
				goodLink,
			},
			public: []contactLink{goodLink},
		},
		{
			name: "blank_and_repeated_ids",
			stored: []any{
				map[string]any{"label": "A", "url": "https://a.example"},
				map[string]any{"id": "x", "label": "B", "url": "https://b.example"},
				map[string]any{"id": "x", "label": "C", "url": "https://c.example"},
			},
			owner: []contactLink{
				{ID: "", Label: "A", URL: "https://a.example"},
				{ID: "x", Label: "B", URL: "https://b.example"},
				{ID: "x", Label: "C", URL: "https://c.example"},
			},
			public: []contactLink{
				{ID: "link-0", Label: "A", URL: "https://a.example"},
				{ID: "x", Label: "B", URL: "https://b.example"},
				{ID: "link-2", Label: "C", URL: "https://c.example"},
			},
		},
		{
			name:   "an_untrimmed_entry_is_served_clean",
			stored: []any{map[string]any{"id": "pad", "label": "  Paket  ", "url": "  HTTPS://Paket.example/x  "}},
			owner:  []contactLink{{ID: "pad", Label: "  Paket  ", URL: "  HTTPS://Paket.example/x  "}},
			public: []contactLink{{ID: "pad", Label: "Paket", URL: "https://Paket.example/x"}},
		},
	}

	check := func(t *testing.T, what string, wantOwner, wantPublic []contactLink) {
		t.Helper()
		if got := s.linksFromMenuList(t, what+" GET /api/menus", menu.ID); fmt.Sprint(got) != fmt.Sprint(wantOwner) {
			t.Errorf("%s: GET /api/menus carries links %+v, want %+v", what, got, wantOwner)
		}
		owned, _ := s.readMenu(t, what+" GET /api/menus/:id", menu.ID)
		if owned.Links == nil || fmt.Sprint(owned.Links) != fmt.Sprint(wantOwner) {
			t.Errorf("%s: GET /api/menus/:id carries links %+v, want %+v", what, owned.Links, wantOwner)
		}
		business, _, _ := s.publicMenu(t, what+" public menu", menu.Slug)
		if got := linksOfPayload(t, what+" public menu", business); fmt.Sprint(got) != fmt.Sprint(wantPublic) {
			t.Errorf("%s: the public menu carries links %+v, want only %+v", what, got, wantPublic)
		}
		preview, _ := s.previewMenu(t, what+" preview", menu.Slug)
		if got := linksOfPayload(t, what+" preview", preview); fmt.Sprint(got) != fmt.Sprint(wantPublic) {
			t.Errorf("%s: the preview carries links %+v, want only %+v", what, got, wantPublic)
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storeLinksPastTheAPI(t, h, menu.ID, tc.stored)
			check(t, tc.name, tc.owner, tc.public)
		})
	}

	// menus_links_check keeps the column an array, so these values cannot be
	// stored today. The constraint is dropped in this scratch database only, to
	// show that the reads would survive them too.
	t.Run("values_that_are_not_an_array", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := h.pool.Exec(ctx, `ALTER TABLE menus DROP CONSTRAINT menus_links_check`); err != nil {
			t.Fatalf("could not drop menus_links_check in the scratch database: %v", err)
		}
		for _, value := range []any{"x", 5, true, nil, good} {
			what := fmt.Sprintf("links %#v", value)
			storeLinksPastTheAPI(t, h, menu.ID, value)
			check(t, what, []contactLink{}, []contactLink{})
		}
	})
}

// TestStoredLinksNestedTooDeepNeverTakeAMenuDown: encoding/json refuses JSON
// nested more than 10000 levels deep, and it checks the whole value before any
// UnmarshalJSON of the target runs. A links value that deep is valid jsonb, so
// a scan through json.Unmarshal would fail every menu read with a 500. It has
// to read as [] on all four reads, with a line in the log that says so; a value
// exactly 10000 levels deep is still read as it is.
func TestStoredLinksNestedTooDeepNeverTakeAMenuDown(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Derin Link Kafe", "deep-links-owner@example.test")
	s := &contactSuite{h: h, owner: owner, businessSlug: readBusinessSlug(t, h, owner)}
	menu := h.createMenu(owner, "Derin Menü")

	good := `{"id":"rez","label":"Rezervasyon","url":"https://rezervasyon.example/masa"}`
	goodLink := contactLink{ID: "rez", Label: "Rezervasyon", URL: "https://rezervasyon.example/masa"}
	nested := func(levels int) string { return strings.Repeat("[", levels) + strings.Repeat("]", levels) }

	cases := []struct {
		name   string
		raw    string
		want   []contactLink
		logged bool
	}{
		// The outer array is the first level.
		{"an_element_10001_levels_deep", "[" + good + "," + nested(10000) + "]", []contactLink{}, true},
		// The outer array and the object are the first two levels.
		{"a_member_10001_levels_deep",
			`[{"id":"x","label":"a","url":"https://a.example","extra":` + nested(9999) + "}," + good + "]",
			[]contactLink{}, true},
		{"an_element_exactly_10000_levels_deep", "[" + good + "," + nested(9999) + "]",
			[]contactLink{goodLink}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.pool.Exec(ctx, `UPDATE menus SET links = $1::text::jsonb WHERE id = $2::text::uuid`,
				tc.raw, menu.ID); err != nil {
				t.Fatalf("could not store the links value: %v", err)
			}
			t.Cleanup(func() {
				if _, err := h.pool.Exec(context.Background(),
					`UPDATE menus SET links = '[]'::jsonb WHERE id = $1::text::uuid`, menu.ID); err != nil {
					t.Errorf("could not reset the links value: %v", err)
				}
			})
			logs.reset()

			reads := map[string][]contactLink{}
			reads["GET /api/menus"] = s.linksFromMenuList(t, tc.name, menu.ID)
			owned, _ := s.readMenu(t, tc.name+" GET /api/menus/:id", menu.ID)
			reads["GET /api/menus/:id"] = owned.Links
			business, _, _ := s.publicMenu(t, tc.name+" public menu", menu.Slug)
			reads["the public menu"] = linksOfPayload(t, tc.name+" public menu", business)
			preview, _ := s.previewMenu(t, tc.name+" preview", menu.Slug)
			reads["the preview"] = linksOfPayload(t, tc.name+" preview", preview)

			for read, got := range reads {
				if got == nil || fmt.Sprint(got) != fmt.Sprint(tc.want) {
					t.Errorf("%s carries links %+v, want %+v", read, got, tc.want)
				}
			}

			lines := logs.linesContaining("menus.links")
			switch {
			case tc.logged && len(lines) == 0:
				t.Errorf("the unreadable links value was read as an empty list without a log line")
			case !tc.logged && len(lines) > 0:
				t.Errorf("a readable links value wrote log lines: %q", lines)
			}
			for _, line := range lines {
				if strings.Contains(line, "[[[") {
					t.Errorf("the log line repeats the stored value: %.200s", line)
				}
			}
		})
	}
}
