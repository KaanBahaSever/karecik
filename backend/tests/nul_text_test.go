package tests

// U+0000 in user text. PostgreSQL accepts it neither in a text column nor in a
// jsonb value, so a write that carried one would fail inside the database and
// the API would answer 500. Every such field has to refuse it with the 422 it
// already answers an invalid value with — and write nothing.
//
// The character is built with string(rune(0)); encoding/json sends it as the
// escape sequence a browser would send.
//
// TestTextThatIsNotUTF8NeverFails covers the other text PostgreSQL refuses:
// bytes that are not valid UTF-8, which only a query string, a header or a form
// body can carry. Those bytes are built from byte values, or percent-encoded,
// so the source holds none of them.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"karecik/backend/internal/utils"
)

func TestNULInUserTextIsRefusedWith422(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Boş Karakter Kafe", "nul-text-owner@example.test")
	menu := h.createMenu(owner, "Boş Karakter Menü")
	category := h.createCategory(owner, menu.ID, "Tatlılar")
	product := h.createProduct(owner, category.ID, "Sütlaç", 90)

	nul := string(rune(0))
	name := func(text string) map[string]any {
		return map[string]any{"tr": map[string]any{"name": text}}
	}
	option := func(group, item string) []map[string]any {
		return []map[string]any{{"name": group, "type": "single",
			"items": []map[string]any{{"name": item, "price": 0}}}}
	}

	menuPath := "/api/menus/" + menu.ID
	categoryPath := "/api/categories/" + category.ID
	productPath := "/api/products/" + product.ID

	cases := []struct {
		name    string
		method  string
		path    string
		body    map[string]any
		message string
	}{
		// --- menu text fields
		{"menu_slogan", http.MethodPut, menuPath, map[string]any{"slogan": "Tatlı" + nul + "lar"}, "slogan alanı metin olmalıdır."},
		{"menu_name", http.MethodPut, menuPath, map[string]any{"name": "Menü" + nul}, "Menü adı metin olmalıdır."},
		{"menu_description", http.MethodPut, menuPath, map[string]any{"description": nul}, "Menü açıklaması metin olmalıdır."},
		{"menu_splash_text", http.MethodPut, menuPath, map[string]any{"splash_text": "Hoş" + nul}, "splash_text alanı metin olmalıdır."},
		{"menu_vat_note_text", http.MethodPut, menuPath, map[string]any{"vat_note_text": nul}, "vat_note_text alanı metin olmalıdır."},
		{"menu_phone", http.MethodPut, menuPath, map[string]any{"phone": "0555" + nul}, "phone alanı metin veya boş olmalıdır."},
		{"menu_wifi_password", http.MethodPut, menuPath, map[string]any{"wifi_password": nul}, "wifi_password alanı metin veya boş olmalıdır."},
		{"menu_create_name", http.MethodPost, "/api/menus", map[string]any{"name": "Yeni" + nul + "Menü"}, "Menü adı metin olmalıdır."},
		{"menu_create_slogan", http.MethodPost, "/api/menus", map[string]any{"name": "Yeni Menü", "slogan": nul}, "slogan alanı metin olmalıdır."},

		// --- link fields
		{"link_label", http.MethodPut, menuPath, map[string]any{"links": []map[string]any{
			{"label": "Rezervasyon" + nul, "url": "https://rezervasyon.example"}}}, "Link adı geçersiz karakter içeriyor."},
		{"link_url", http.MethodPut, menuPath, map[string]any{"links": []map[string]any{
			{"label": "Rezervasyon", "url": "https://rezervasyon.example/" + nul}}},
			"Link adresi http:// veya https:// ile başlayan geçerli bir adres olmalıdır."},

		// --- the account
		{"business_name", http.MethodPut, "/api/business", map[string]any{"name": "Kafe" + nul}, "İşletme adı metin olmalıdır."},

		// --- category
		{"category_name_on_create", http.MethodPost, "/api/categories",
			map[string]any{"menu_id": menu.ID, "translations": name("Pasta" + nul)}, "Çeviri alanı geçersiz."},
		{"category_name_on_update", http.MethodPut, categoryPath,
			map[string]any{"translations": name(nul + "Tatlılar")}, "Çeviri alanı geçersiz."},
		{"category_description_on_update", http.MethodPut, categoryPath,
			map[string]any{"translations": map[string]any{"tr": map[string]any{"name": "Tatlılar", "description": nul}}},
			"Çeviri alanı geçersiz."},
		{"category_icon_on_create", http.MethodPost, "/api/categories",
			map[string]any{"menu_id": menu.ID, "translations": name("Pasta"), "icon": nul}, "icon alanı metin veya boş olmalıdır."},
		{"category_image_on_create", http.MethodPost, "/api/categories",
			map[string]any{"menu_id": menu.ID, "translations": name("Pasta"), "image_url": "/uploads/a" + nul},
			"image_url alanı metin veya boş olmalıdır."},
		{"category_icon_on_update", http.MethodPut, categoryPath, map[string]any{"icon": "x" + nul}, "icon alanı metin veya boş olmalıdır."},

		// --- product
		{"product_name_on_create", http.MethodPost, "/api/products",
			map[string]any{"category_id": category.ID, "translations": name("Kazandibi" + nul), "price": 80}, "Çeviri alanı geçersiz."},
		{"product_name_on_update", http.MethodPut, productPath,
			map[string]any{"translations": name("Sütlaç" + nul)}, "Çeviri alanı geçersiz."},
		{"product_ingredients_on_update", http.MethodPut, productPath,
			map[string]any{"translations": map[string]any{"tr": map[string]any{"name": "Sütlaç", "ingredients": "pirinç" + nul}}},
			"Çeviri alanı geçersiz."},
		{"product_image_on_create", http.MethodPost, "/api/products",
			map[string]any{"category_id": category.ID, "translations": name("Kazandibi"), "price": 80, "image_url": nul},
			"Görsel adresi geçersiz."},
		{"product_image_on_update", http.MethodPut, productPath, map[string]any{"image_url": "/uploads/x" + nul}, "Görsel adresi geçersiz."},
		{"badge_text_on_create", http.MethodPost, "/api/products",
			map[string]any{"category_id": category.ID, "translations": name("Kazandibi"), "price": 80,
				"badges": []map[string]any{{"text": "Yeni" + nul}}}, "Rozet listesi geçersiz."},
		{"badge_text_on_update", http.MethodPut, productPath,
			map[string]any{"badges": []map[string]any{{"text": "Yeni" + nul}}}, "Rozet listesi geçersiz."},
		{"badge_id_on_update", http.MethodPut, productPath,
			map[string]any{"badges": []map[string]any{{"id": "b" + nul, "text": "Yeni"}}}, "Rozet listesi geçersiz."},
		{"option_group_name_on_update", http.MethodPut, productPath,
			map[string]any{"options": option("Boy"+nul, "Büyük")}, "Seçenek listesi geçersiz."},
		{"option_name_on_update", http.MethodPut, productPath,
			map[string]any{"options": option("Boy", "Büyük"+nul)}, "Seçenek listesi geçersiz."},
		{"option_name_on_create", http.MethodPost, "/api/products",
			map[string]any{"category_id": category.ID, "translations": name("Kazandibi"), "price": 80,
				"options": option("Boy", nul)}, "Seçenek listesi geçersiz."},
	}

	countOf := func(path string) int {
		t.Helper()
		resp, payload := h.do(http.MethodGet, path, owner.session, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s answered %d: %s", path, resp.StatusCode, payload)
		}
		var list []map[string]any
		decodeInto(t, "GET "+path, payload, &list)
		return len(list)
	}
	menusBefore, categoriesBefore, productsBefore := countOf("/api/menus"), countOf("/api/categories"), countOf("/api/products")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, payload := h.do(tc.method, tc.path, owner.session, tc.body)
			expectRefusal(t, tc.method+" "+tc.path, resp, payload, tc.message)
		})
	}

	// A refusal writes nothing: no new rows, and the rows that were targeted
	// still read exactly as they did.
	if got := countOf("/api/menus"); got != menusBefore {
		t.Errorf("the refused requests changed the number of menus from %d to %d", menusBefore, got)
	}
	if got := countOf("/api/categories"); got != categoriesBefore {
		t.Errorf("the refused requests changed the number of categories from %d to %d", categoriesBefore, got)
	}
	if got := countOf("/api/products"); got != productsBefore {
		t.Errorf("the refused requests changed the number of products from %d to %d", productsBefore, got)
	}
	s := &contactSuite{h: h, owner: owner}
	if stored, raw := s.readMenu(t, "the menu after the refusals", menu.ID); len(stored.Links) != 0 ||
		string(raw["slogan"]) != `""` || string(raw["name"]) != `"Boş Karakter Menü"` {
		t.Errorf("the refused requests changed the menu: name %s, slogan %s, links %+v", raw["name"], raw["slogan"], stored.Links)
	}
	products := h.listProductsAsOwner(t, "the product after the refusals", owner, category.ID)
	if len(products) != 1 || products[0].ID != product.ID {
		t.Errorf("the refused requests changed the products of the category: %+v", products)
	}

	// A link id is not refused: one that is not a plain identifier — U+0000
	// included — is replaced with a fresh UUID, like any other invalid id.
	t.Run("link_id_is_replaced", func(t *testing.T) {
		written, _ := s.put(t, "PUT a link with a NUL in its id", menu.ID, map[string]any{"links": []map[string]any{
			{"id": "rez" + nul, "label": "Rezervasyon", "url": "https://rezervasyon.example"}}})
		if len(written.Links) != 1 {
			t.Fatalf("PUT answered with links %+v, want one", written.Links)
		}
		if _, err := uuid.Parse(written.Links[0].ID); err != nil {
			t.Errorf("the link id is %q, want a fresh UUID in place of one holding U+0000", written.Links[0].ID)
		}
	})
}

// TestNULInSignUpAndLookupsNeverFails covers the text a stranger types that is
// used to look something up — an e-mail address, a search term, a menu slug —
// and the sign-up form. No stored text can hold U+0000 and PostgreSQL refuses
// it as a parameter, so these would end in a 500 as well. Sign-up refuses it with
// the 422 of the field; a lookup answers the way it answers any value that
// matches nothing, so it tells a stranger nothing new.
func TestNULInSignUpAndLookupsNeverFails(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	owner := h.register("owner", "Arama Kafe", "nul-lookup-owner@example.test")
	businessSlug := readBusinessSlug(t, h, owner)
	menu := h.createMenu(owner, "Arama Menü")
	category := h.createCategory(owner, menu.ID, "Tatlılar")
	h.createProduct(owner, category.ID, "Sütlaç", 90)

	nul := string(rune(0))

	errorOf := func(t *testing.T, what string, payload []byte) string {
		t.Helper()
		var body struct {
			Error string `json:"error"`
		}
		decodeInto(t, what, payload, &body)
		return body.Error
	}

	t.Run("sign_up_business_name", func(t *testing.T) {
		resp, payload := h.do(http.MethodPost, "/api/auth/register", "", map[string]any{
			"business_name": "Yeni" + nul + "Kafe", "email": "nul-name@example.test", "password": "karecik-test-password"})
		expectRefusal(t, "POST /api/auth/register", resp, payload, "İşletme adı metin olmalıdır.")
	})

	t.Run("sign_up_email", func(t *testing.T) {
		resp, payload := h.do(http.MethodPost, "/api/auth/register", "", map[string]any{
			"business_name": "Yeni Kafe", "email": "nul" + nul + "@example.test", "password": "karecik-test-password"})
		expectRefusal(t, "POST /api/auth/register", resp, payload, "Geçerli bir e-posta adresi giriniz.")
	})

	t.Run("login_email", func(t *testing.T) {
		resp, payload := h.do(http.MethodPost, "/api/auth/login", "", map[string]any{
			"email": "nul-lookup-owner" + nul + "@example.test", "password": "karecik-test-password"})
		if resp.StatusCode != http.StatusUnauthorized || errorOf(t, "login", payload) != "E-posta veya şifre hatalı." {
			t.Errorf("POST /api/auth/login with U+0000 in the address answered %d %s, want the 401 of any "+
				"unknown address", resp.StatusCode, payload)
		}
	})

	t.Run("forgot_password_email", func(t *testing.T) {
		unknownResp, unknown := h.do(http.MethodPost, "/api/auth/forgot-password", "",
			map[string]any{"email": "nobody@example.test"})
		resp, payload := h.do(http.MethodPost, "/api/auth/forgot-password", "",
			map[string]any{"email": "nul-lookup-owner" + nul + "@example.test"})
		if unknownResp.StatusCode != http.StatusOK || resp.StatusCode != http.StatusOK || string(payload) != string(unknown) {
			t.Errorf("POST /api/auth/forgot-password with U+0000 answered %d %s, want the answer an unknown address "+
				"gets (%d %s)", resp.StatusCode, payload, unknownResp.StatusCode, unknown)
		}
		if sent := mail.count(); sent != 0 {
			t.Errorf("%d reset e-mail(s) went out, want none", sent)
		}
	})

	t.Run("product_search", func(t *testing.T) {
		for _, query := range []string{"%00", "S%C3%BCt%00la%C3%A7"} {
			resp, payload := h.do(http.MethodGet, "/api/products?search="+query, owner.session, nil)
			if resp.StatusCode != http.StatusOK || string(payload) != "[]" {
				t.Errorf("GET /api/products?search=%s answered %d %s, want 200 []", query, resp.StatusCode, payload)
			}
		}
	})

	t.Run("public_menu_slug_by_host", func(t *testing.T) {
		get := func(path string) (int, []byte) {
			t.Helper()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = businessSlug + ".karecik.com"
			resp, err := h.app.Test(req, requestTimeoutMS)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return resp.StatusCode, body
		}

		if status, body := get("/api/public/menu?menu=" + menu.Slug); status != http.StatusOK {
			t.Fatalf("fixture: the host form does not serve the menu (got %d: %s)", status, body)
		}
		status, body := get("/api/public/menu?menu=%00")
		if status != http.StatusNotFound || errorOf(t, "public menu", body) != "Böyle bir menü bulunamadı." {
			t.Errorf("GET /api/public/menu?menu=%%00 answered %d %s, want the 404 of an unknown menu", status, body)
		}
	})

	// The Host form reads the tenant from X-Forwarded-Host as well. A U+0000 in
	// it survives into the business slug and reaches the business lookup,
	// which has to answer it as the unknown tenant it is.
	t.Run("business_slug_by_forwarded_host", func(t *testing.T) {
		for _, host := range []string{"a" + nul + "b.karecik.com", businessSlug + nul + ".karecik.com"} {
			status, body := sendRawRequest(t, h, http.MethodGet, "/api/public/menu", "",
				map[string]string{"X-Forwarded-Host": host}, "", "")
			if status != http.StatusNotFound || errorOf(t, "public menu", body) != "Böyle bir menü bulunamadı." {
				t.Errorf("GET /api/public/menu with X-Forwarded-Host %q answered %d %s, want the 404 of an "+
					"unknown business", host, status, body)
			}
		}
	})
}

// sendRawRequest sends one request with exactly the bytes given — a form body,
// a header value — and returns the status and the body. A "Host" entry of
// headers becomes the Host of the request; session, when not empty, is sent as
// the session cookie.
func sendRawRequest(t *testing.T, h *harness, method, target, session string, headers map[string]string,
	contentType, body string) (int, []byte) {

	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if session != "" {
		req.AddCookie(&http.Cookie{Name: utils.SessionCookieName, Value: session})
	}
	for key, value := range headers {
		if key == "Host" {
			req.Host = value
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := h.app.Test(req, requestTimeoutMS)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: could not read the body: %v", method, target, err)
	}
	return resp.StatusCode, payload
}

// TestTextThatIsNotUTF8NeverFails covers bytes that are not valid UTF-8 in the
// inputs that do not go through encoding/json — which would have replaced them
// with U+FFFD — but reach a query as they are: a query string, a header and a
// form body. PostgreSQL refuses such bytes in a text parameter with SQLSTATE
// 22021, so each of these would end in a 500 if it reached a query. A lookup
// answers the way it answers any value that matches nothing, and an address is
// checked before it is lowercased, which would turn such a byte into U+FFFD.
func TestTextThatIsNotUTF8NeverFails(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	owner := h.register("owner", "Bayt Kafe", "not-utf8-owner@example.test")
	businessSlug := readBusinessSlug(t, h, owner)
	menu := h.createMenu(owner, "Bayt Menü")
	category := h.createCategory(owner, menu.ID, "Tatlılar")
	h.createProduct(owner, category.ID, "Sütlaç", 90)

	const form = "application/x-www-form-urlencoded"
	errorOf := func(t *testing.T, what string, payload []byte) string {
		t.Helper()
		var body struct {
			Error string `json:"error"`
		}
		decodeInto(t, what, payload, &body)
		return body.Error
	}
	tenantHost := businessSlug + ".karecik.com"

	if status, body := sendRawRequest(t, h, http.MethodGet, "/api/public/menu?menu="+menu.Slug, "",
		map[string]string{"Host": tenantHost}, "", ""); status != http.StatusOK {
		t.Fatalf("fixture: the host form does not serve the menu (got %d: %s)", status, body)
	}

	t.Run("public_menu_slug", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			target  string
			headers map[string]string
		}{
			{"host_ff", "/api/public/menu?menu=%FF", map[string]string{"Host": tenantHost}},
			{"host_truncated_sequence", "/api/public/menu?menu=ka%C3", map[string]string{"Host": tenantHost}},
			{"forwarded_host_ff", "/api/public/menu?menu=%FF", map[string]string{"X-Forwarded-Host": tenantHost}},
		} {
			status, body := sendRawRequest(t, h, http.MethodGet, tc.target, "", tc.headers, "", "")
			if status != http.StatusNotFound || errorOf(t, tc.name, body) != "Böyle bir menü bulunamadı." {
				t.Errorf("%s: GET %s answered %d %s, want the 404 of an unknown menu", tc.name, tc.target, status, body)
			}
		}
	})

	t.Run("forwarded_host_of_bytes_that_are_not_utf8", func(t *testing.T) {
		host := string([]byte{'a', 0xff, 'b'}) + ".karecik.com"
		status, body := sendRawRequest(t, h, http.MethodGet, "/api/public/menu", "",
			map[string]string{"X-Forwarded-Host": host}, "", "")
		if status != http.StatusNotFound {
			t.Errorf("GET /api/public/menu with X-Forwarded-Host %q answered %d %s, want 404", host, status, body)
		}
	})

	t.Run("forgot_password_form", func(t *testing.T) {
		unknownStatus, unknown := sendRawRequest(t, h, http.MethodPost, "/api/auth/forgot-password", "", nil,
			form, "email=nobody%40example.test")
		status, body := sendRawRequest(t, h, http.MethodPost, "/api/auth/forgot-password", "", nil,
			form, "email=%FF%40example.test")
		if unknownStatus != http.StatusOK || status != http.StatusOK || string(body) != string(unknown) {
			t.Errorf("POST /api/auth/forgot-password with email=%%FF%%40example.test answered %d %s, want the "+
				"answer an unknown address gets (%d %s)", status, body, unknownStatus, unknown)
		}
		if sent := mail.count(); sent != 0 {
			t.Errorf("%d reset e-mail(s) went out, want none", sent)
		}
	})

	t.Run("login_form", func(t *testing.T) {
		status, body := sendRawRequest(t, h, http.MethodPost, "/api/auth/login", "", nil,
			form, "email=%FF%40example.test&password=karecik-test-password")
		if status != http.StatusUnauthorized || errorOf(t, "login", body) != "E-posta veya şifre hatalı." {
			t.Errorf("POST /api/auth/login with email=%%FF%%40example.test answered %d %s, want the 401 of any "+
				"unknown address", status, body)
		}
	})

	t.Run("product_search", func(t *testing.T) {
		for _, query := range []string{"%FF", "ka%C3", "S%C3%BCt%FFla%C3%A7"} {
			status, body := sendRawRequest(t, h, http.MethodGet, "/api/products?search="+query, owner.session, nil, "", "")
			if status != http.StatusOK || string(body) != "[]" {
				t.Errorf("GET /api/products?search=%s answered %d %s, want 200 []", query, status, body)
			}
		}
	})

	// strings.ToLower turns a byte that is not UTF-8 into U+FFFD, so an address
	// lowercased before it is checked would become a different, valid one. The
	// replacement character is built from its code point.
	replacement := string(rune(0xFFFD))
	countUsers := func(t *testing.T, email string) int {
		t.Helper()
		var n int
		if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = $1`, email).Scan(&n); err != nil {
			t.Fatalf("could not count the accounts of %q: %v", email, err)
		}
		return n
	}

	t.Run("register_form", func(t *testing.T) {
		// Fiber reads a form field by the name of the Go field, so the business
		// name travels as BusinessName.
		for _, address := range []string{"%FF%40utf8-register.example.test", "ALI%FE%40UTF8-REGISTER.EXAMPLE.TEST"} {
			status, body := sendRawRequest(t, h, http.MethodPost, "/api/auth/register", "", nil, form,
				"BusinessName=Bayt+Kafe&email="+address+"&password=karecik-test-password")
			if status != http.StatusUnprocessableEntity || errorOf(t, "register", body) != "Geçerli bir e-posta adresi giriniz." {
				t.Errorf("POST /api/auth/register with email=%s answered %d %s, want 422 %q", address, status, body,
					"Geçerli bir e-posta adresi giriniz.")
			}
		}
		for _, stored := range []string{replacement + "@utf8-register.example.test", "ali" + replacement + "@utf8-register.example.test"} {
			if n := countUsers(t, stored); n != 0 {
				t.Errorf("%d account(s) were created with the address %q", n, stored)
			}
		}
	})

}

// TestAnAddressHoldingTheReplacementCharacterIsNotReachedThroughBytes: an
// account whose address really holds U+FFFD — valid UTF-8, so a JSON sign-up
// accepts it — must not be reached by an address holding a byte that is not
// UTF-8, which strings.ToLower would turn into that very character. The test
// has a harness of its own because the forgot-password route answers only two
// requests an hour from one client, and the test above uses both.
func TestAnAddressHoldingTheReplacementCharacterIsNotReachedThroughBytes(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	const form = "application/x-www-form-urlencoded"
	errorOf := func(t *testing.T, what string, payload []byte) string {
		t.Helper()
		var body struct {
			Error string `json:"error"`
		}
		decodeInto(t, what, payload, &body)
		return body.Error
	}

	address := string(rune(0xFFFD)) + "@utf8-login.example.test"
	resp, payload := h.do(http.MethodPost, "/api/auth/register", "", map[string]any{
		"business_name": "Yer Tutucu Kafe", "email": address, "password": "karecik-test-password"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("fixture: registering %q answered %d: %s", address, resp.StatusCode, payload)
	}
	if resp, payload := h.do(http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": address, "password": "karecik-test-password"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("fixture: logging in as %q answered %d: %s", address, resp.StatusCode, payload)
	}

	status, body := sendRawRequest(t, h, http.MethodPost, "/api/auth/login", "", nil, form,
		"email=%FF%40utf8-login.example.test&password=karecik-test-password")
	if status != http.StatusUnauthorized || errorOf(t, "login", body) != "E-posta veya şifre hatalı." {
		t.Errorf("POST /api/auth/login with email=%%FF%%40utf8-login.example.test answered %d %s, want the 401 "+
			"of any unknown address — not a login to %q", status, shorten(body), address)
	}

	unknownStatus, unknown := sendRawRequest(t, h, http.MethodPost, "/api/auth/forgot-password", "", nil,
		form, "email=nobody%40utf8-login.example.test")
	status, body = sendRawRequest(t, h, http.MethodPost, "/api/auth/forgot-password", "", nil,
		form, "email=%FF%40utf8-login.example.test")
	if unknownStatus != http.StatusOK || status != http.StatusOK || string(body) != string(unknown) {
		t.Errorf("POST /api/auth/forgot-password with email=%%FF%%40utf8-login.example.test answered %d %s, want "+
			"the answer an unknown address gets (%d %s)", status, body, unknownStatus, unknown)
	}
	if sent := mail.count(); sent != 0 {
		t.Errorf("%d reset e-mail(s) went out, want none", sent)
	}
}
