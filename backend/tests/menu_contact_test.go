package tests

// Black-box coverage of the contact block settings of a menu — contact_display
// and the owner's own links — and of the VAT note fallback in the public
// footer, driven over HTTP like every other suite in this package.
//
// Three properties carry the weight here, because none of them shows up on a
// happy-path click through the dashboard:
//
//   - the links validation is all-or-nothing: a refused list leaves the stored
//     one exactly as it was, and an accepted list is never shortened;
//   - "hidden" is enforced by the server and not by the page: the public
//     payload stops carrying the entries, while the owner still reads them;
//   - the footer's VAT note falls back to the default sentence instead of
//     printing nothing when the stored text is blank.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The Turkish messages of the contract, verbatim.
const (
	msgInvalidContactDisplay = "Geçersiz iletişim görünümü."
	msgTooManyLinks          = "En fazla 8 link ekleyebilirsiniz."
	msgLabelRequired         = "Link adı zorunludur."
	msgLabelInvalidChars     = "Link adı geçersiz karakter içeriyor."
	msgLabelTooLong          = "Link adı en fazla 40 karakter olabilir."
	msgURLTooLong            = "Link adresi en fazla 500 karakter olabilir."
	msgURLInvalid            = "Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır."
	msgLinksUnreadable       = "Link listesi geçersiz."
	defaultVatSentence       = "Fiyatlarımıza KDV dahildir."
)

// The contact details this suite stores. They are distinctive on purpose: the
// hidden-mode case searches the whole public payload for each of them, so none
// may occur there by coincidence.
const (
	contactPhone        = "+90 555 010 20 30"
	contactAddress      = "Moda Caddesi 7, Kadıköy"
	contactInstagram    = "iletisimkafe_test"
	contactWifiSSID     = "IletisimKafe-Misafir"
	contactWifiPassword = "gizli-sifre-2026"
	contactLinkURL      = "https://rezervasyon.example/iletisim-kafe"
)

// contactLink is one entry of menus.links as the API returns it.
type contactLink struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// contactMenu is a menu as the dashboard API returns it, reduced to the fields
// this suite reads.
type contactMenu struct {
	ID             string        `json:"id"`
	Slug           string        `json:"slug"`
	ContactDisplay string        `json:"contact_display"`
	Links          []contactLink `json:"links"`
	Phone          *string       `json:"phone"`
	Address        *string       `json:"address"`
	Instagram      *string       `json:"instagram"`
	WifiSSID       *string       `json:"wifi_ssid"`
	WifiPassword   *string       `json:"wifi_password"`
}

type contactSuite struct {
	h            *harness
	owner        *tenant
	businessSlug string
}

// validLink is a link body that passes every rule.
func validLink(label, address string) map[string]any {
	return map[string]any{"label": label, "url": address}
}

// urlOfBytes returns a valid https address exactly n bytes long.
func urlOfBytes(n int) string {
	const prefix = "https://example.com/"
	return prefix + strings.Repeat("a", n-len(prefix))
}

// rawObject decodes one JSON object into its raw members, so an assertion can
// tell null from a missing key and [] from null.
func rawObject(t *testing.T, what string, payload []byte) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	decodeInto(t, what, payload, &object)
	return object
}

// requireRaw asserts that key is present in object with exactly the raw JSON
// want — `null`, `[]`, `"hidden"`.
func requireRaw(t *testing.T, what string, object map[string]json.RawMessage, key, want string) {
	t.Helper()
	got, ok := object[key]
	if !ok {
		t.Errorf("%s: the payload has no %q key at all, want %s", what, key, want)
		return
	}
	if string(got) != want {
		t.Errorf("%s: %q is %s, want %s", what, key, got, want)
	}
}

func (s *contactSuite) readMenu(t *testing.T, what, id string) (contactMenu, map[string]json.RawMessage) {
	t.Helper()
	resp, payload := s.h.do(http.MethodGet, "/api/menus/"+id, s.owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: reading menu %s as its owner failed (got %d: %s)", what, id, resp.StatusCode, payload)
	}
	var menu contactMenu
	decodeInto(t, what, payload, &menu)
	return menu, rawObject(t, what, payload)
}

// put writes a partial menu update that has to succeed and returns the menu
// the response carries.
func (s *contactSuite) put(t *testing.T, what, id string, body any) (contactMenu, map[string]json.RawMessage) {
	t.Helper()
	resp, payload := s.h.do(http.MethodPut, "/api/menus/"+id, s.owner.session, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: PUT /api/menus/%s was refused (got %d: %s)", what, id, resp.StatusCode, payload)
	}
	var menu contactMenu
	decodeInto(t, what, payload, &menu)
	return menu, rawObject(t, what, payload)
}

// expectRefusal asserts a 422 carrying exactly message.
func expectRefusal(t *testing.T, what string, resp *http.Response, payload []byte, message string) {
	t.Helper()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("%s: got %d, want 422 %q: %s", what, resp.StatusCode, message, payload)
		return
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	decodeInto(t, what, payload, &body)
	if body.Error != message {
		t.Errorf("%s: refused with %q, want %q", what, body.Error, message)
	}
	if body.Code != "VALIDATION_ERROR" {
		t.Errorf("%s: refused with code %q, want VALIDATION_ERROR", what, body.Code)
	}
}

// refuse sends a menu update that must be refused with message and then checks
// that the stored contact settings did not move at all — a refusal that still
// writes part of the list is exactly the failure all-or-nothing rules out.
func (s *contactSuite) refuse(t *testing.T, what, id string, body any, message string) {
	t.Helper()
	_, before := s.readMenu(t, what+" (before)", id)

	resp, payload := s.h.do(http.MethodPut, "/api/menus/"+id, s.owner.session, body)
	expectRefusal(t, what, resp, payload, message)

	_, after := s.readMenu(t, what+" (after)", id)
	for _, key := range []string{"contact_display", "links"} {
		if string(before[key]) != string(after[key]) {
			t.Errorf("%s: the refused request still changed %s from %s to %s",
				what, key, before[key], after[key])
		}
	}
}

// publicMenu reads the customer payload of one menu.
func (s *contactSuite) publicMenu(t *testing.T, what, menuSlug string) (map[string]json.RawMessage, []byte, publicFooter) {
	t.Helper()
	path := "/api/public/menu/" + s.businessSlug + "/" + menuSlug
	resp, payload := s.h.do(http.MethodGet, path, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: GET %s failed (got %d: %s)", what, path, resp.StatusCode, payload)
	}
	var menu struct {
		Business json.RawMessage `json:"business"`
		Footer   publicFooter    `json:"footer"`
	}
	decodeInto(t, what, payload, &menu)
	return rawObject(t, what+" business", menu.Business), payload, menu.Footer
}

// previewMenu reads the owner's live preview of one menu — the same payload
// builder as the public endpoint, reached with the owner's session.
func (s *contactSuite) previewMenu(t *testing.T, what, menuSlug string) (map[string]json.RawMessage, []byte) {
	t.Helper()
	path := "/api/preview/menu?menu=" + menuSlug
	resp, payload := s.h.do(http.MethodGet, path, s.owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: GET %s failed (got %d: %s)", what, path, resp.StatusCode, payload)
	}
	var menu struct {
		Business json.RawMessage `json:"business"`
	}
	decodeInto(t, what, payload, &menu)
	return rawObject(t, what+" business", menu.Business), payload
}

type publicFooter struct {
	VatNote *string `json:"vat_note"`
}

// readBusinessSlug reads the tenant's subdomain slug, which the public address
// needs and register does not keep.
func readBusinessSlug(t *testing.T, h *harness, owner *tenant) string {
	t.Helper()
	resp, payload := h.do(http.MethodGet, "/api/business", owner.session, nil)
	h.requireSuccess("read the business slug of "+owner.label, resp, payload)
	var business struct {
		Slug string `json:"slug"`
	}
	decodeInto(t, "fixture business", payload, &business)
	if business.Slug == "" {
		t.Fatalf("fixture: %s's business carried no slug: %s", owner.label, payload)
	}
	return business.Slug
}

// storedLinksColumn reads menus.links straight from the table: its jsonb type
// and its length. The API could normalise a bad row on the way out; the column
// cannot hide one.
func storedLinksColumn(t *testing.T, h *harness, what, menuID string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var kind string
	var length int
	err := h.pool.QueryRow(ctx, `
		SELECT jsonb_typeof(links),
		       CASE WHEN jsonb_typeof(links) = 'array' THEN jsonb_array_length(links) ELSE -1 END
		FROM menus WHERE id = $1::text::uuid`, menuID).Scan(&kind, &length)
	if err != nil {
		t.Fatalf("%s: could not read menus.links of %s: %v", what, menuID, err)
	}
	return kind, length
}

func TestMenuContactSettings(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "İletişim Kafe", "contact-owner@example.test")
	s := &contactSuite{h: h, owner: owner, businessSlug: readBusinessSlug(t, h, owner)}

	menu := h.createMenu(owner, "Ana Menü")

	// The contact details every public-payload case below reads back.
	s.put(t, "fixture contact details", menu.ID, map[string]any{
		"phone":         contactPhone,
		"address":       contactAddress,
		"instagram":     contactInstagram,
		"wifi_ssid":     contactWifiSSID,
		"wifi_password": contactWifiPassword,
	})

	t.Run("1_a_new_menu_starts_inline_with_no_links", func(t *testing.T) {
		resp, payload := h.do(http.MethodPost, "/api/menus", owner.session, map[string]any{"name": "Yepyeni Menü"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("creating a menu failed (got %d: %s)", resp.StatusCode, payload)
		}
		created := rawObject(t, "POST /api/menus", payload)
		requireRaw(t, "POST /api/menus response", created, "contact_display", `"inline"`)
		requireRaw(t, "POST /api/menus response", created, "links", `[]`)

		var createdID string
		if err := json.Unmarshal(created["id"], &createdID); err != nil || createdID == "" {
			t.Fatalf("the created menu carried no id: %s", payload)
		}
		_, read := s.readMenu(t, "GET /api/menus/:id", createdID)
		requireRaw(t, "GET /api/menus/:id", read, "contact_display", `"inline"`)
		requireRaw(t, "GET /api/menus/:id", read, "links", `[]`)

		if kind, length := storedLinksColumn(t, h, "a new menu", createdID); kind != "array" || length != 0 {
			t.Errorf("a new menu stores links as a jsonb %s of length %d, want an empty array", kind, length)
		}

		resp, payload = h.do(http.MethodGet, "/api/menus", owner.session, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/menus failed (got %d: %s)", resp.StatusCode, payload)
		}
		var list []map[string]json.RawMessage
		decodeInto(t, "GET /api/menus", payload, &list)
		if len(list) < 2 {
			t.Fatalf("GET /api/menus lists %d menus, want at least the fixture menu and the new one", len(list))
		}
		for i, item := range list {
			if _, ok := item["contact_display"]; !ok {
				t.Errorf("GET /api/menus: menu #%d carries no contact_display: %s", i, payload)
			}
			if links := string(item["links"]); !strings.HasPrefix(links, "[") {
				t.Errorf("GET /api/menus: menu #%d carries links %s, want an array", i, links)
			}
		}
	})

	t.Run("2_contact_display_accepts_the_four_modes_and_refuses_anything_else", func(t *testing.T) {
		for _, mode := range []string{"inline", "list", "footer", "hidden"} {
			written, _ := s.put(t, "PUT contact_display "+mode, menu.ID, map[string]any{"contact_display": mode})
			if written.ContactDisplay != mode {
				t.Errorf("PUT contact_display %q answered with %q", mode, written.ContactDisplay)
			}
			if read, _ := s.readMenu(t, "re-read "+mode, menu.ID); read.ContactDisplay != mode {
				t.Errorf("PUT contact_display %q stored %q", mode, read.ContactDisplay)
			}
		}

		s.put(t, "back to list", menu.ID, map[string]any{"contact_display": "list"})
		for _, value := range []any{"grid", "", "INLINE", " inline", "gizli", 5, true, nil} {
			s.refuse(t, fmt.Sprintf("contact_display %#v", value), menu.ID,
				map[string]any{"contact_display": value}, msgInvalidContactDisplay)
		}
	})

	t.Run("3_links_are_refused_all_or_nothing_with_the_first_applicable_message", func(t *testing.T) {
		// A known stored list, so every refusal can be shown to leave it alone.
		s.put(t, "fixture two links", menu.ID, map[string]any{"links": []map[string]any{
			validLink("Rezervasyon", contactLinkURL),
			validLink("Paket Servis", "https://paket.example/siparis"),
		}})

		nine := make([]map[string]any, 0, 9)
		nineBlank := make([]map[string]any, 0, 9)
		for i := 0; i < 9; i++ {
			nine = append(nine, validLink(fmt.Sprintf("Link %d", i+1), fmt.Sprintf("https://example.com/%d", i+1)))
			nineBlank = append(nineBlank, validLink("", "ftp://x"))
		}

		long41 := strings.Repeat("ş", 41)
		// Built from code points, so the source itself holds no raw control
		// or invisible character.
		zeroWidthSpace := string(rune(0x200b))
		newline := string(rune(0x0a))
		bell := string(rune(0x07))
		nul := string(rune(0x00))
		nextLine := string(rune(0x85))
		backslash := string(rune(0x5c))
		cases := []struct {
			name    string
			links   any
			message string
		}{
			{"nine_links", nine, msgTooManyLinks},
			{"nine_links_that_are_also_invalid_report_the_count", nineBlank, msgTooManyLinks},
			{"a_blank_label", []map[string]any{validLink("   ", "https://example.com")}, msgLabelRequired},
			{"a_missing_label", []map[string]any{{"url": "https://example.com"}}, msgLabelRequired},
			{"a_41_rune_label", []map[string]any{validLink(long41, "https://example.com")}, msgLabelTooLong},
			{"a_blank_label_is_reported_before_a_bad_url", []map[string]any{validLink("", "ftp://x")}, msgLabelRequired},
			{"a_long_label_is_reported_before_a_long_url", []map[string]any{validLink(long41, urlOfBytes(501))}, msgLabelTooLong},
			{"a_501_byte_url", []map[string]any{validLink("Uzun", urlOfBytes(501))}, msgURLTooLong},
			// 20 ASCII bytes and 241 two-byte runes: 502 bytes but only 261
			// runes, so a limit counted in runes would let it through.
			{"a_502_byte_url_of_261_runes", []map[string]any{
				validLink("Uzun", "https://example.com/"+strings.Repeat("ş", 241))}, msgURLTooLong},
			{"a_long_url_is_reported_for_its_length_before_its_scheme", []map[string]any{
				validLink("Uzun", "ftp://"+strings.Repeat("a", 600))}, msgURLTooLong},
			{"javascript_scheme", []map[string]any{validLink("Kötü", "javascript:alert(1)")}, msgURLInvalid},
			{"ftp_scheme", []map[string]any{validLink("Kötü", "ftp://x")}, msgURLInvalid},
			{"no_host", []map[string]any{validLink("Kötü", "https://")}, msgURLInvalid},
			{"a_port_but_no_host", []map[string]any{validLink("Kötü", "https://:80")}, msgURLInvalid},
			{"a_space_in_the_host", []map[string]any{validLink("Kötü", "https://exa mple.com")}, msgURLInvalid},
			{"a_space_in_the_path", []map[string]any{validLink("Kötü", "https://example.com/a b")}, msgURLInvalid},
			{"a_tab_in_the_path", []map[string]any{validLink("Kötü", "https://example.com/a\tb")}, msgURLInvalid},
			{"a_control_character", []map[string]any{validLink("Kötü", "https://example.com/\u0007")}, msgURLInvalid},
			{"no_scheme", []map[string]any{validLink("Kötü", "example.com")}, msgURLInvalid},
			{"scheme_relative", []map[string]any{validLink("Kötü", "//example.com")}, msgURLInvalid},
			{"no_slashes_after_the_scheme", []map[string]any{validLink("Kötü", "https:example.com")}, msgURLInvalid},
			{"a_data_uri", []map[string]any{validLink("Kötü", "data:text/html,<b>merhaba</b>")}, msgURLInvalid},
			{"an_empty_url", []map[string]any{validLink("Kötü", "   ")}, msgURLInvalid},
			{"a_valid_link_followed_by_an_invalid_one", []map[string]any{
				validLink("İyi", "https://iyi.example"), validLink("Kötü", "ftp://x")}, msgURLInvalid},
			{"links_is_a_string", "https://example.com", msgLinksUnreadable},
			{"links_is_an_object", validLink("Tek", "https://example.com"), msgLinksUnreadable},
			{"a_label_that_is_a_number", []map[string]any{{"label": 5, "url": "https://example.com"}}, msgLinksUnreadable},
			{"links_is_a_number", 5, msgLinksUnreadable},
			{"an_entry_that_is_a_number", []any{5}, msgLinksUnreadable},
			{"an_id_that_is_a_number", []map[string]any{{"id": 5, "label": "a", "url": "https://example.com"}}, msgLinksUnreadable},
			// A null entry decodes as an entry with no label and no url.
			{"a_null_entry", []any{nil}, msgLabelRequired},

			// The label rules: a control character anywhere is refused, and a
			// label with nothing visible left is required.
			{"a_label_with_a_nul", []map[string]any{validLink("a"+nul+"b", "https://example.com")}, msgLabelInvalidChars},
			{"a_label_of_bel", []map[string]any{validLink(bell, "https://example.com")}, msgLabelInvalidChars},
			{"a_label_with_a_newline_inside", []map[string]any{validLink("a"+newline+"b", "https://example.com")}, msgLabelInvalidChars},
			{"a_control_label_is_reported_before_a_bad_url", []map[string]any{validLink(bell, "ftp://x")}, msgLabelInvalidChars},
			{"a_label_that_is_only_a_newline", []map[string]any{validLink(newline, "https://example.com")}, msgLabelRequired},
			{"a_label_that_is_only_nel", []map[string]any{validLink(nextLine, "https://example.com")}, msgLabelRequired},
			{"a_label_of_zero_width_spaces", []map[string]any{validLink(zeroWidthSpace+zeroWidthSpace, "https://example.com")}, msgLabelRequired},

			// The address rules, with inputs that a check of the scheme alone
			// would accept.
			{"markup_in_the_host", []map[string]any{validLink("Kötü", "https://<svg/onload=alert(1)>")}, msgURLInvalid},
			{"a_right_to_left_override_in_the_host", []map[string]any{
				validLink("Kötü", "https://exa"+string(rune(0x202e))+"mple.com")}, msgURLInvalid},
			{"port_99999", []map[string]any{validLink("Kötü", "https://example.com:99999")}, msgURLInvalid},
			{"port_0", []map[string]any{validLink("Kötü", "https://example.com:0")}, msgURLInvalid},
			{"an_empty_port", []map[string]any{validLink("Kötü", "https://example.com:")}, msgURLInvalid},
			{"a_dot_host", []map[string]any{validLink("Kötü", "https://.")}, msgURLInvalid},
			{"a_hyphen_host", []map[string]any{validLink("Kötü", "https://-")}, msgURLInvalid},
			{"a_doubled_scheme_leaves_a_single_label_host", []map[string]any{validLink("Kötü", "https://https//ornek.com")}, msgURLInvalid},
			{"a_single_label_host", []map[string]any{validLink("Kötü", "https://localhost")}, msgURLInvalid},
			{"an_ipv4_host", []map[string]any{validLink("Kötü", "https://192.168.1.1")}, msgURLInvalid},
			{"userinfo", []map[string]any{validLink("Kötü", "https://kullanici@example.com")}, msgURLInvalid},
			{"a_backslash", []map[string]any{validLink("Kötü", "https://example.com"+backslash+"evil.example")}, msgURLInvalid},
			{"a_backtick", []map[string]any{validLink("Kötü", "https://example.com/`")}, msgURLInvalid},
			{"a_zero_width_space_in_the_path", []map[string]any{validLink("Kötü", "https://example.com/a"+zeroWidthSpace+"b")}, msgURLInvalid},
			{"a_nul_in_the_url", []map[string]any{validLink("Kötü", "https://example.com/"+nul)}, msgURLInvalid},

			// Square brackets anywhere in the address, and a label whose only
			// character besides two invisible ones is a space.
			{"a_bracketed_host", []map[string]any{validLink("Köşeli", "https://[ornek.com]")}, msgURLInvalid},
			{"a_bracketed_host_with_a_port", []map[string]any{
				validLink("Köşeli", "https://[ornek.com]:8080/menu")}, msgURLInvalid},
			{"a_bracketed_host_after_an_upper_case_scheme", []map[string]any{
				validLink("Köşeli", "HTTPS://[a.b]/x")}, msgURLInvalid},
			{"a_space_between_two_zero_width_spaces", []map[string]any{
				validLink(zeroWidthSpace+" "+zeroWidthSpace, "https://example.com")}, msgLabelRequired},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s.refuse(t, tc.name, menu.ID, map[string]any{"links": tc.links}, tc.message)
			})
		}

		// The create path runs the same validation, and a refused create writes
		// no menu at all.
		countMenus := func() int {
			resp, payload := h.do(http.MethodGet, "/api/menus", owner.session, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /api/menus failed (got %d: %s)", resp.StatusCode, payload)
			}
			var list []json.RawMessage
			decodeInto(t, "GET /api/menus", payload, &list)
			return len(list)
		}
		before := countMenus()
		for _, body := range []map[string]any{
			{"name": "Hatalı Menü", "links": nine},
			{"name": "Hatalı Menü", "links": []map[string]any{validLink("Kötü", "javascript:alert(1)")}},
			{"name": "Hatalı Menü", "contact_display": "grid"},
		} {
			resp, payload := h.do(http.MethodPost, "/api/menus", owner.session, body)
			want := msgTooManyLinks
			if _, ok := body["contact_display"]; ok {
				want = msgInvalidContactDisplay
			} else if links, _ := body["links"].([]map[string]any); len(links) == 1 {
				want = msgURLInvalid
			}
			expectRefusal(t, fmt.Sprintf("POST /api/menus %v", body), resp, payload, want)
		}
		if after := countMenus(); after != before {
			t.Errorf("refused creates still wrote menus: %d before, %d after", before, after)
		}
	})

	t.Run("4_links_are_trimmed_scheme_lowercased_and_given_ids_without_losing_any", func(t *testing.T) {
		longest := "https://example.com/" + strings.Repeat("a", 480) // exactly 500 bytes
		input := []map[string]any{
			{"id": "keep_me-1", "label": "  Rezervasyon  ", "url": "  HTTPS://Example.COM/Rezervasyon?Masa=4  "},
			{"label": "Paket Servis", "url": "Http://paket.example/Siparis"},
			{"id": "bad id!", "label": "Yorum Yap", "url": "https://yorum.example/r"},
			{"id": "", "label": "Boş Kimlik", "url": "https://bos.example"},
			{"id": strings.Repeat("a", 65), "label": "Uzun Kimlik", "url": "https://uzun.example"},
			{"id": strings.Repeat("b", 64), "label": "Tam Sınır", "url": "https://sinir.example"},
			{"id": "şifre", "label": "Türkçe Kimlik", "url": "https://tr.example"},
			{"id": nil, "label": strings.Repeat("ş", 40), "url": "  " + longest + "  "},
		}
		type expectation struct {
			keepID string // "" means a generated UUID
			label  string
			url    string
		}
		want := []expectation{
			{"keep_me-1", "Rezervasyon", "https://Example.COM/Rezervasyon?Masa=4"},
			{"", "Paket Servis", "http://paket.example/Siparis"},
			{"", "Yorum Yap", "https://yorum.example/r"},
			{"", "Boş Kimlik", "https://bos.example"},
			{"", "Uzun Kimlik", "https://uzun.example"},
			{strings.Repeat("b", 64), "Tam Sınır", "https://sinir.example"},
			{"", "Türkçe Kimlik", "https://tr.example"},
			{"", strings.Repeat("ş", 40), longest},
		}

		written, _ := s.put(t, "PUT eight links", menu.ID, map[string]any{"links": input})
		if len(written.Links) != len(want) {
			t.Fatalf("PUT eight links answered with %d links, want %d — an accepted list is never "+
				"shortened: %+v", len(written.Links), len(want), written.Links)
		}

		seen := make(map[string]bool)
		for i, link := range written.Links {
			w := want[i]
			if link.Label != w.label {
				t.Errorf("link #%d: label %q, want %q (trimmed, in the owner's order)", i, link.Label, w.label)
			}
			if link.URL != w.url {
				t.Errorf("link #%d: url %q, want %q (trimmed, only the scheme lowercased)", i, link.URL, w.url)
			}
			switch {
			case w.keepID != "" && link.ID != w.keepID:
				t.Errorf("link #%d: id %q, want the valid id %q kept as it was sent", i, link.ID, w.keepID)
			case w.keepID == "":
				if _, err := uuid.Parse(link.ID); err != nil {
					t.Errorf("link #%d: id %q, want a generated UUID in place of %#v", i, link.ID, input[i]["id"])
				}
			}
			if seen[link.ID] {
				t.Errorf("link #%d: id %q is used twice", i, link.ID)
			}
			seen[link.ID] = true
		}

		read, _ := s.readMenu(t, "re-read eight links", menu.ID)
		if fmt.Sprint(read.Links) != fmt.Sprint(written.Links) {
			t.Errorf("the stored links differ from the ones the PUT answered with:\nstored   %+v\nanswered %+v",
				read.Links, written.Links)
		}

		// Saving the list back exactly as it was read keeps every id, so the
		// dashboard can round-trip the list without renaming its entries.
		again, _ := s.put(t, "PUT the same eight links back", menu.ID, map[string]any{"links": read.Links})
		if fmt.Sprint(again.Links) != fmt.Sprint(read.Links) {
			t.Errorf("re-saving the stored links changed them:\nbefore %+v\nafter  %+v", read.Links, again.Links)
		}

		if kind, length := storedLinksColumn(t, h, "eight links", menu.ID); kind != "array" || length != 8 {
			t.Errorf("menus.links is a jsonb %s of length %d, want an array of 8", kind, length)
		}
	})

	t.Run("4b_a_repeated_id_is_replaced_and_the_first_one_kept", func(t *testing.T) {
		written, _ := s.put(t, "PUT links with a repeated id", menu.ID, map[string]any{"links": []map[string]any{
			{"id": "masa", "label": "Rezervasyon", "url": "https://rezervasyon.example"},
			{"id": "masa", "label": "Paket Servis", "url": "https://paket.example"},
			{"id": "yorum", "label": "Yorum", "url": "https://yorum.example"},
			{"id": "masa", "label": "Harita", "url": "https://harita.example"},
		}})
		if len(written.Links) != 4 {
			t.Fatalf("PUT answered with %d links, want 4: %+v", len(written.Links), written.Links)
		}
		if written.Links[0].ID != "masa" || written.Links[2].ID != "yorum" {
			t.Errorf("the first carriers of each id lost it: %+v", written.Links)
		}
		seen := make(map[string]bool)
		for i, link := range written.Links {
			if i == 1 || i == 3 {
				if _, err := uuid.Parse(link.ID); err != nil {
					t.Errorf("link #%d repeats an earlier id and kept %q, want a fresh UUID", i, link.ID)
				}
			}
			if seen[link.ID] {
				t.Errorf("link #%d id %q is used twice: %+v", i, link.ID, written.Links)
			}
			seen[link.ID] = true
		}

		// Now that the ids are unique, saving the list back as it was read
		// keeps every one of them.
		read, _ := s.readMenu(t, "re-read the de-duplicated links", menu.ID)
		again, _ := s.put(t, "PUT the de-duplicated links back", menu.ID, map[string]any{"links": read.Links})
		if fmt.Sprint(again.Links) != fmt.Sprint(read.Links) {
			t.Errorf("re-saving the stored links changed them:\nbefore %+v\nafter  %+v", read.Links, again.Links)
		}
	})

	t.Run("5_null_and_an_empty_array_both_clear_the_links", func(t *testing.T) {
		for _, clear := range []struct {
			name  string
			value any
		}{{"null", nil}, {"empty_array", []any{}}} {
			s.put(t, "fixture one link", menu.ID, map[string]any{"links": []map[string]any{
				validLink("Rezervasyon", contactLinkURL)}})

			_, raw := s.put(t, "PUT links "+clear.name, menu.ID, map[string]any{"links": clear.value})
			requireRaw(t, "PUT links "+clear.name, raw, "links", `[]`)
			if kind, length := storedLinksColumn(t, h, "links "+clear.name, menu.ID); kind != "array" || length != 0 {
				t.Errorf("PUT links %s stored a jsonb %s of length %d, want an empty array",
					clear.name, kind, length)
			}
		}

		// A partial update that does not name links leaves them alone.
		s.put(t, "fixture one link", menu.ID, map[string]any{"links": []map[string]any{
			validLink("Rezervasyon", contactLinkURL)}})
		written, _ := s.put(t, "PUT without links", menu.ID, map[string]any{"slogan": "Kahvenin en iyi hali"})
		if len(written.Links) != 1 {
			t.Errorf("a PUT that does not name links changed them to %+v", written.Links)
		}
	})

	t.Run("6_create_accepts_both_fields", func(t *testing.T) {
		resp, payload := h.do(http.MethodPost, "/api/menus", owner.session, map[string]any{
			"name":            "Bar Menüsü",
			"contact_display": "footer",
			"links":           []map[string]any{validLink(" Rezervasyon ", "HTTPS://bar.example")},
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("creating a menu with contact settings failed (got %d: %s)", resp.StatusCode, payload)
		}
		var created contactMenu
		decodeInto(t, "POST /api/menus with contact settings", payload, &created)
		if created.ContactDisplay != "footer" {
			t.Errorf("the created menu has contact_display %q, want footer", created.ContactDisplay)
		}
		if len(created.Links) != 1 || created.Links[0].Label != "Rezervasyon" ||
			created.Links[0].URL != "https://bar.example" {
			t.Errorf("the created menu has links %+v, want one trimmed link with a lowercased scheme", created.Links)
		} else if _, err := uuid.Parse(created.Links[0].ID); err != nil {
			t.Errorf("the created link has id %q, want a generated UUID", created.Links[0].ID)
		}
	})

	// From here on the menu carries one link and every contact detail.
	s.put(t, "fixture public payload", menu.ID, map[string]any{
		"links": []map[string]any{{"id": "rezervasyon", "label": "Rezervasyon", "url": contactLinkURL}},
	})

	t.Run("7_the_public_payload_carries_contact_display_and_links", func(t *testing.T) {
		s.put(t, "contact_display list", menu.ID, map[string]any{"contact_display": "list"})

		business, _, _ := s.publicMenu(t, "public payload in list mode", menu.Slug)
		requireRaw(t, "public payload in list mode", business, "contact_display", `"list"`)

		var links []contactLink
		if err := json.Unmarshal(business["links"], &links); err != nil {
			t.Fatalf("public payload: business.links is %s, want an array of links: %v", business["links"], err)
		}
		want := []contactLink{{ID: "rezervasyon", Label: "Rezervasyon", URL: contactLinkURL}}
		if fmt.Sprint(links) != fmt.Sprint(want) {
			t.Errorf("public payload: business.links is %+v, want %+v", links, want)
		}

		preview, _ := s.previewMenu(t, "preview in list mode", menu.Slug)
		requireRaw(t, "preview in list mode", preview, "contact_display", `"list"`)
		if string(preview["links"]) != string(business["links"]) {
			t.Errorf("preview: business.links is %s, the public payload says %s", preview["links"], business["links"])
		}

		// The bare tenant address of a business with several menus is the
		// directory. There is no menu to take a contact block from, but the
		// payload still says [] rather than null.
		resp, directoryPayload := h.do(http.MethodGet, "/api/public/menu/"+s.businessSlug, "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET the tenant directory failed (got %d: %s)", resp.StatusCode, directoryPayload)
		}
		var directory struct {
			MenuResolved bool            `json:"menu_resolved"`
			Business     json.RawMessage `json:"business"`
		}
		decodeInto(t, "tenant directory", directoryPayload, &directory)
		if directory.MenuResolved {
			t.Fatalf("fixture: the tenant address resolved a single menu, want the directory of several: %s",
				directoryPayload)
		}
		requireRaw(t, "tenant directory", rawObject(t, "tenant directory business", directory.Business), "links", `[]`)
	})

	t.Run("8_hidden_redacts_the_public_payload_but_never_the_owner", func(t *testing.T) {
		s.put(t, "contact_display hidden", menu.ID, map[string]any{"contact_display": "hidden"})

		secrets := map[string]string{
			"phone":         contactPhone,
			"instagram":     contactInstagram,
			"wifi_ssid":     contactWifiSSID,
			"wifi_password": contactWifiPassword,
			"a link url":    contactLinkURL,
		}

		business, payload, _ := s.publicMenu(t, "public payload in hidden mode", menu.Slug)
		requireRaw(t, "public payload in hidden mode", business, "contact_display", `"hidden"`)
		for _, key := range []string{"phone", "instagram", "wifi_ssid", "wifi_password"} {
			requireRaw(t, "public payload in hidden mode", business, key, "null")
		}
		requireRaw(t, "public payload in hidden mode", business, "links", `[]`)
		for what, secret := range secrets {
			if strings.Contains(string(payload), secret) {
				t.Errorf("public payload in hidden mode: %s (%q) still appears in the response", what, secret)
			}
		}
		// Address is not part of the contact block, so hidden leaves it alone.
		requireRaw(t, "public payload in hidden mode", business, "address", mustJSON(t, contactAddress))

		preview, previewPayload := s.previewMenu(t, "preview in hidden mode", menu.Slug)
		for _, key := range []string{"phone", "instagram", "wifi_ssid", "wifi_password"} {
			requireRaw(t, "preview in hidden mode", preview, key, "null")
		}
		requireRaw(t, "preview in hidden mode", preview, "links", `[]`)
		for what, secret := range secrets {
			if strings.Contains(string(previewPayload), secret) {
				t.Errorf("preview in hidden mode: %s (%q) still appears in the response", what, secret)
			}
		}

		// The owner reads the menu itself, not the payload built from it.
		owned, _ := s.readMenu(t, "the owner's view in hidden mode", menu.ID)
		for what, pair := range map[string][2]*string{
			"phone":         {owned.Phone, strPointer(contactPhone)},
			"address":       {owned.Address, strPointer(contactAddress)},
			"instagram":     {owned.Instagram, strPointer(contactInstagram)},
			"wifi_ssid":     {owned.WifiSSID, strPointer(contactWifiSSID)},
			"wifi_password": {owned.WifiPassword, strPointer(contactWifiPassword)},
		} {
			if pair[0] == nil || *pair[0] != *pair[1] {
				t.Errorf("the owner's view in hidden mode: %s is %v, want %q — hidden redacts the public "+
					"payload, never the owner's own settings", what, derefOrNil(pair[0]), *pair[1])
			}
		}
		if len(owned.Links) != 1 || owned.Links[0].URL != contactLinkURL {
			t.Errorf("the owner's view in hidden mode: links are %+v, want the stored link", owned.Links)
		}

		resp, listPayload := h.do(http.MethodGet, "/api/menus", owner.session, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/menus failed (got %d: %s)", resp.StatusCode, listPayload)
		}
		for what, secret := range secrets {
			if !strings.Contains(string(listPayload), secret) {
				t.Errorf("the owner's menu list in hidden mode no longer carries %s (%q)", what, secret)
			}
		}
	})

	t.Run("9_the_other_modes_are_not_redacted", func(t *testing.T) {
		for _, mode := range []string{"inline", "list", "footer"} {
			s.put(t, "contact_display "+mode, menu.ID, map[string]any{"contact_display": mode})

			what := "public payload in " + mode + " mode"
			business, _, _ := s.publicMenu(t, what, menu.Slug)
			requireRaw(t, what, business, "contact_display", mustJSON(t, mode))
			requireRaw(t, what, business, "phone", mustJSON(t, contactPhone))
			requireRaw(t, what, business, "instagram", mustJSON(t, contactInstagram))
			requireRaw(t, what, business, "wifi_ssid", mustJSON(t, contactWifiSSID))
			requireRaw(t, what, business, "wifi_password", mustJSON(t, contactWifiPassword))
			requireRaw(t, what, business, "address", mustJSON(t, contactAddress))
			if !strings.Contains(string(business["links"]), contactLinkURL) {
				t.Errorf("%s: business.links is %s, want the stored link", what, business["links"])
			}
		}
	})

	t.Run("10_the_vat_note_falls_back_to_the_default_sentence", func(t *testing.T) {
		// Through the API: the handler trims the text before storing it.
		for _, tc := range []struct {
			name string
			show bool
			text string
			want string
		}{
			{"shown_with_an_empty_text", true, "", defaultVatSentence},
			{"shown_with_a_blank_text", true, "   ", defaultVatSentence},
			{"shown_with_its_own_text", true, "  Tüm fiyatlara KDV dahildir.  ", "Tüm fiyatlara KDV dahildir."},
			{"not_shown_with_an_empty_text", false, "", ""},
			{"not_shown_with_its_own_text", false, "Özel KDV notu", ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s.put(t, tc.name, menu.ID, map[string]any{"show_vat_note": tc.show, "vat_note_text": tc.text})
				_, _, footer := s.publicMenu(t, tc.name, menu.Slug)
				if footer.VatNote == nil {
					t.Fatalf("%s: footer.vat_note is missing or null, want %q", tc.name, tc.want)
				}
				if *footer.VatNote != tc.want {
					t.Errorf("%s: footer.vat_note is %q, want %q", tc.name, *footer.VatNote, tc.want)
				}
			})
		}

		// Straight into the column, past the handler's trim, so it is the
		// payload builder itself that has to trim and fall back.
		for _, tc := range []struct {
			name   string
			stored string
			want   string
		}{
			{"a_whitespace_text_in_the_column", "  \t ", defaultVatSentence},
			{"a_padded_text_in_the_column", "  KDV dahil  ", "KDV dahil"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if _, err := h.pool.Exec(ctx,
					`UPDATE menus SET show_vat_note = true, vat_note_text = $1 WHERE id = $2::text::uuid`,
					tc.stored, menu.ID); err != nil {
					t.Fatalf("could not store the VAT text %q: %v", tc.stored, err)
				}
				_, _, footer := s.publicMenu(t, tc.name, menu.Slug)
				if footer.VatNote == nil || *footer.VatNote != tc.want {
					t.Errorf("%s: footer.vat_note is %v, want %q", tc.name, derefOrNil(footer.VatNote), tc.want)
				}
			})
		}
	})

	t.Run("11_meta_lists_the_contact_display_modes", func(t *testing.T) {
		resp, payload := h.do(http.MethodGet, "/api/meta", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/meta failed (got %d: %s)", resp.StatusCode, payload)
		}
		var meta struct {
			ContactDisplayModes []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"contact_display_modes"`
		}
		decodeInto(t, "GET /api/meta", payload, &meta)

		want := []struct{ id, label string }{
			{"inline", "Yan yana"},
			{"list", "Açık liste"},
			{"footer", "Sadece alt bilgi"},
			{"hidden", "Hiç gösterme"},
		}
		if len(meta.ContactDisplayModes) != len(want) {
			t.Fatalf("GET /api/meta lists %d contact display modes, want %d: %s",
				len(meta.ContactDisplayModes), len(want), payload)
		}
		for i, mode := range meta.ContactDisplayModes {
			if mode.ID != want[i].id || mode.Label != want[i].label {
				t.Errorf("contact_display_modes[%d] is {%q, %q}, want {%q, %q}",
					i, mode.ID, mode.Label, want[i].id, want[i].label)
			}
		}
	})
}

// mustJSON renders a value the way it appears in a payload.
func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("could not encode %#v: %v", value, err)
	}
	return string(raw)
}

func strPointer(s string) *string { return &s }

// derefOrNil prints a nullable string for a failure message.
func derefOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
