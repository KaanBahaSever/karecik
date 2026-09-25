package tests

// Visitor analytics over HTTP: the public events endpoint the customer menu
// reports views to, and the dashboard's summary and drill-down over what it
// stored.
//
// The properties that matter, none of which a click through the dashboard
// would reveal:
//
//   - the endpoint accepts what navigator.sendBeacon really sends (text/plain)
//     and answers 204;
//   - it stores the address, the source port and how the address was
//     established exactly as the request log line resolves them;
//   - garbage never costs a 500 and never stores a row, and an event naming a
//     category or product of another menu — or another tenant — is refused;
//   - its limiter is per client, not global (the guards that bound the table
//     — repeats, the daily cap, the budgets, the purge — are
//     analytics_limits_test.go's);
//   - the dashboard reads are tenant-scoped: another tenant's menu_id is a 404,
//     not an empty report;
//   - the counts, the daily series and the top lists add up.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karecik/backend/internal/clientip"
	"karecik/backend/internal/config"
	"karecik/backend/internal/mailer"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/router"
	"karecik/backend/internal/utils"
)

const (
	analyticsEdgeSecret = "analytics-edge-secret"
	analyticsPortHeader = "X-Client-Port"
)

// newAnalyticsHarness is a harness whose client-address resolver can prove a
// Cloudflare request and read its source port — the configuration under which
// a port is recorded at all.
func newAnalyticsHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessConfigured(t, mailer.Disabled{}, func(cfg *config.Config) {
		cfg.EdgeSecretHeader = "X-Edge-Secret"
		cfg.EdgeSecret = analyticsEdgeSecret
		cfg.ClientPortHeader = analyticsPortHeader
	})
}

// fromCloudflare are the headers of a proven Cloudflare request from ip:port.
func fromCloudflare(ip, port string) map[string]string {
	return map[string]string{
		clientip.HeaderCFConnectingIP: ip,
		"X-Edge-Secret":               analyticsEdgeSecret,
		analyticsPortHeader:           port,
	}
}

// sendEvent posts a raw body the way a beacon does. contentType "" sends none
// at all.
func sendEvent(t *testing.T, h *harness, body, contentType string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/public/events", bytes.NewReader([]byte(body)))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := h.app.Test(req, requestTimeoutMS)
	if err != nil {
		t.Fatalf("POST /api/public/events never completed: %v", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	return resp, payload
}

// eventBody renders one event as JSON.
func eventBody(fields map[string]any) string {
	return mustJSONString(fields)
}

func mustJSONString(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func countEvents(t *testing.T, h *harness) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM menu_events`).Scan(&n); err != nil {
		t.Fatalf("could not count menu_events: %v", err)
	}
	return n
}

type analyticsFixture struct {
	h            *harness
	owner, other *tenant
	businessSlug string
	menu         menuPayload // published, languages tr only
	second       menuPayload // published, another menu of the same tenant
	draft        menuPayload // unpublished
	catA, catB   categoryPayload
	catSecond    categoryPayload
	prodA, prodB productPayload
	otherSlug    string
	otherMenu    menuPayload
	otherCat     categoryPayload
	otherProd    productPayload
}

func newAnalyticsFixture(t *testing.T) *analyticsFixture {
	h := newAnalyticsHarness(t)
	f := &analyticsFixture{h: h}
	f.owner = h.register("owner", "Melly Coffee Co", "analytics-owner@example.test")
	f.other = h.register("other", "Başka Kafe", "analytics-other@example.test")
	f.businessSlug = readBusinessSlug(t, h, f.owner)
	f.otherSlug = readBusinessSlug(t, h, f.other)

	f.menu = h.createMenu(f.owner, "Ana Menü")
	f.second = h.createMenu(f.owner, "Tatlı Menüsü")
	f.draft = h.createMenu(f.owner, "Taslak Menü")
	resp, payload := h.do(http.MethodPut, "/api/menus/"+f.draft.ID, f.owner.session, map[string]any{"is_active": false})
	h.requireSuccess("unpublish the draft", resp, payload)

	f.catA = h.createCategory(f.owner, f.menu.ID, "Kahveler")
	f.catB = h.createCategory(f.owner, f.menu.ID, "Sandviçler")
	f.catSecond = h.createCategory(f.owner, f.second.ID, "Tatlılar")
	f.prodA = h.createProduct(f.owner, f.catA.ID, "Latte", 90)
	f.prodB = h.createProduct(f.owner, f.catB.ID, "Ciabatta Sandwich", 180)

	f.otherMenu = h.createMenu(f.other, "Diğer Menü")
	f.otherCat = h.createCategory(f.other, f.otherMenu.ID, "Diğer Kategori")
	f.otherProd = h.createProduct(f.other, f.otherCat.ID, "Diğer Ürün", 10)
	return f
}

// event is one event of the fixture's own menu.
func (f *analyticsFixture) event(kind string, extra map[string]any) string {
	fields := map[string]any{"business_slug": f.businessSlug, "menu_slug": f.menu.Slug, "type": kind}
	for key, value := range extra {
		fields[key] = value
	}
	return eventBody(fields)
}

func (f *analyticsFixture) mustTrack(t *testing.T, body string, headers map[string]string) {
	t.Helper()
	resp, payload := sendEvent(t, f.h, body, "text/plain;charset=UTF-8", headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("event %s answered %d, want 204: %s", body, resp.StatusCode, payload)
	}
}

func TestPublicEventsAreStoredWithTheResolvedAddress(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h

	t.Run("a_beacon_is_stored_with_ip_port_and_source", func(t *testing.T) {
		headers := fromCloudflare("198.51.100.7", "51234")
		headers["User-Agent"] = "Mozilla/5.0 (iPhone; Test)"
		resp, payload := sendEvent(t, h,
			f.event("menu_view", map[string]any{"visitor_id": "visitor_one", "language": "EN"}),
			"text/plain;charset=UTF-8", headers)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("answered %d, want 204: %s", resp.StatusCode, payload)
		}
		if len(payload) != 0 {
			t.Errorf("a 204 carried a body: %s", payload)
		}

		var (
			ip, source, ua, visitorKey string
			port                       *int
			language, visitorID        *string
			menuID                     string
		)
		err := h.pool.QueryRow(context.Background(), `
			SELECT ip, port, ip_source, user_agent, language, visitor_id, visitor_key, menu_id::text
			FROM menu_events ORDER BY created_at DESC LIMIT 1`).
			Scan(&ip, &port, &source, &ua, &language, &visitorID, &visitorKey, &menuID)
		if err != nil {
			t.Fatalf("the event was not stored: %v", err)
		}
		if ip != "198.51.100.7" || port == nil || *port != 51234 || source != "cloudflare" {
			t.Errorf("stored address %s:%v via %s, want 198.51.100.7:51234 via cloudflare", ip, port, source)
		}
		if ua != "Mozilla/5.0 (iPhone; Test)" {
			t.Errorf("stored user agent %q", ua)
		}
		if language == nil || *language != "en" {
			t.Errorf("stored language %v, want en", language)
		}
		if visitorID == nil || *visitorID != "visitor_one" || visitorKey != "v:visitor_one" {
			t.Errorf("stored visitor %v / %q", visitorID, visitorKey)
		}
		if menuID != f.menu.ID {
			t.Errorf("stored menu %s, want %s", menuID, f.menu.ID)
		}
	})

	t.Run("no_content_type_and_an_unproven_address", func(t *testing.T) {
		resp, payload := sendEvent(t, h, f.event("menu_view", nil), "",
			map[string]string{clientip.HeaderXRealIP: "203.0.113.50", "User-Agent": "Anon/1"})
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("answered %d: %s", resp.StatusCode, payload)
		}
		var source string
		var port *int
		var visitorKey string
		if err := h.pool.QueryRow(context.Background(), `
			SELECT ip_source, port, visitor_key FROM menu_events WHERE ip = '203.0.113.50'`).
			Scan(&source, &port, &visitorKey); err != nil {
			t.Fatalf("the event was not stored: %v", err)
		}
		if source != "edge" || port != nil {
			t.Errorf("an X-Real-IP request stored source %q and port %v, want edge and null", source, port)
		}
		if !strings.HasPrefix(visitorKey, "h:") {
			t.Errorf("an anonymous visitor keyed as %q, want a hash", visitorKey)
		}
	})

	t.Run("a_product_view_is_stored_with_its_category", func(t *testing.T) {
		f.mustTrack(t, f.event("product_view", map[string]any{"product_id": f.prodA.ID}), nil)
		var category string
		if err := h.pool.QueryRow(context.Background(), `
			SELECT category_id::text FROM menu_events WHERE product_id = $1::text::uuid`, f.prodA.ID).
			Scan(&category); err != nil {
			t.Fatalf("the product view was not stored: %v", err)
		}
		if category != f.catA.ID {
			t.Errorf("stored category %s, want the product's %s", category, f.catA.ID)
		}
	})

	t.Run("refusals_store_nothing_and_never_answer_500", func(t *testing.T) {
		before := countEvents(t, h)
		long65 := strings.Repeat("a", 65)
		for _, tc := range []struct {
			name   string
			body   string
			status int
		}{
			{"not_json", "hello", http.StatusBadRequest},
			{"a_json_array", `[1,2,3]`, http.StatusBadRequest},
			{"a_number_where_a_string_belongs", `{"type": 5}`, http.StatusBadRequest},
			{"an_object_slug", `{"business_slug": {}, "menu_slug": "x", "type": "menu_view"}`, http.StatusBadRequest},
			{"json_null", `null`, http.StatusUnprocessableEntity},
			{"empty_object", `{}`, http.StatusUnprocessableEntity},
			{"an_unknown_type", f.event("page_view", nil), http.StatusUnprocessableEntity},
			{"a_category_view_without_a_category", f.event("category_view", nil), http.StatusUnprocessableEntity},
			{"a_product_view_without_a_product", f.event("product_view", nil), http.StatusUnprocessableEntity},
			{"a_malformed_category_id", f.event("category_view", map[string]any{"category_id": "1; DROP TABLE"}), http.StatusUnprocessableEntity},
			{"the_nil_uuid", f.event("category_view", map[string]any{"category_id": "00000000-0000-0000-0000-000000000000"}), http.StatusUnprocessableEntity},
			{"a_category_of_another_menu", f.event("category_view", map[string]any{"category_id": f.catSecond.ID}), http.StatusUnprocessableEntity},
			{"a_category_of_another_tenant", f.event("category_view", map[string]any{"category_id": f.otherCat.ID}), http.StatusUnprocessableEntity},
			{"a_product_of_another_tenant", f.event("product_view", map[string]any{"product_id": f.otherProd.ID}), http.StatusUnprocessableEntity},
			{"a_product_in_another_category", f.event("product_view", map[string]any{"product_id": f.prodA.ID, "category_id": f.catB.ID}), http.StatusUnprocessableEntity},
			{"a_menu_of_another_tenant", eventBody(map[string]any{"business_slug": f.businessSlug, "menu_slug": f.otherMenu.Slug, "type": "menu_view"}), http.StatusUnprocessableEntity},
			{"an_unpublished_menu", eventBody(map[string]any{"business_slug": f.businessSlug, "menu_slug": f.draft.Slug, "type": "menu_view"}), http.StatusUnprocessableEntity},
			{"an_unknown_tenant", eventBody(map[string]any{"business_slug": "yok-boyle-bir-yer", "menu_slug": "menu", "type": "menu_view"}), http.StatusUnprocessableEntity},
			{"a_nul_in_the_slug", eventBody(map[string]any{"business_slug": "a\u0000b", "menu_slug": "x", "type": "menu_view"}), http.StatusUnprocessableEntity},
			{"a_huge_slug", eventBody(map[string]any{"business_slug": strings.Repeat("a", 500), "menu_slug": "x", "type": "menu_view"}), http.StatusUnprocessableEntity},
			{"a_visitor_id_with_bad_characters", f.event("menu_view", map[string]any{"visitor_id": "a b<script>"}), http.StatusUnprocessableEntity},
			{"a_visitor_id_too_long", f.event("menu_view", map[string]any{"visitor_id": long65}), http.StatusUnprocessableEntity},
			{"an_unsupported_language", f.event("menu_view", map[string]any{"language": "xx"}), http.StatusUnprocessableEntity},
			{"a_body_over_the_cap", f.event("menu_view", map[string]any{"padding": strings.Repeat("x", 3000)}), http.StatusRequestEntityTooLarge},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp, payload := sendEvent(t, h, tc.body, "text/plain", nil)
				if resp.StatusCode != tc.status {
					t.Errorf("answered %d, want %d: %s", resp.StatusCode, tc.status, payload)
				}
			})
		}
		if after := countEvents(t, h); after != before {
			t.Errorf("refused events still stored %d row(s)", after-before)
		}
	})

	t.Run("the_limiter_is_per_client", func(t *testing.T) {
		// Garbage bodies: the limiter counts requests before the handler reads
		// them, and garbage never touches the database. An X-Real-IP request
		// is not proven, so it has the shared key's budget; the proven one is
		// TestTheEventLimiterBudgets'.
		refusedAt := 0
		for i := 1; i <= router.SharedKeyEventsPerMinute+1; i++ {
			resp, _ := sendEvent(t, h, "x", "text/plain",
				map[string]string{clientip.HeaderXRealIP: "192.0.2.77"})
			if resp.StatusCode == http.StatusTooManyRequests {
				refusedAt = i
				break
			}
		}
		if refusedAt != router.SharedKeyEventsPerMinute+1 {
			t.Fatalf("the busy client was refused at request %d, want exactly the %dth",
				refusedAt, router.SharedKeyEventsPerMinute+1)
		}
		resp, payload := sendEvent(t, h, f.event("menu_view", nil), "text/plain",
			map[string]string{clientip.HeaderXRealIP: "192.0.2.78"})
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("another client was refused after the first used up its budget: %d %s",
				resp.StatusCode, payload)
		}
	})
}

// summaryPayload mirrors GET /api/analytics/summary.
type summaryPayload struct {
	From           string `json:"from"`
	To             string `json:"to"`
	TotalVisits    int    `json:"total_visits"`
	UniqueVisitors int    `json:"unique_visitors"`
	MenuViews      int    `json:"menu_views"`
	CategoryViews  int    `json:"category_views"`
	ProductViews   int    `json:"product_views"`
	Daily          []struct {
		Date           string `json:"date"`
		Visits         int    `json:"visits"`
		UniqueVisitors int    `json:"unique_visitors"`
		CategoryViews  int    `json:"category_views"`
		ProductViews   int    `json:"product_views"`
	} `json:"daily"`
	TopCategories []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Views int    `json:"views"`
	} `json:"top_categories"`
	TopProducts []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		CategoryName string `json:"category_name"`
		Views        int    `json:"views"`
	} `json:"top_products"`
}

type eventsPayload struct {
	Items []struct {
		ID           string  `json:"id"`
		CreatedAt    string  `json:"created_at"`
		Type         string  `json:"type"`
		MenuID       string  `json:"menu_id"`
		MenuName     string  `json:"menu_name"`
		CategoryID   *string `json:"category_id"`
		CategoryName *string `json:"category_name"`
		ProductID    *string `json:"product_id"`
		ProductName  *string `json:"product_name"`
		IP           *string `json:"ip"`
		Port         *int    `json:"port"`
		IPSource     string  `json:"ip_source"`
		VisitorID    *string `json:"visitor_id"`
		Language     *string `json:"language"`
		UserAgent    string  `json:"user_agent"`
	} `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func TestAnalyticsSummaryAndEvents(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	separateVisits(h)
	cf := func(port string) map[string]string { return fromCloudflare("198.51.100.20", port) }

	// Visitor one: the menu, a category and the same product twice.
	f.mustTrack(t, f.event("menu_view", map[string]any{"visitor_id": "v1"}), cf("40001"))
	f.mustTrack(t, f.event("category_view", map[string]any{"visitor_id": "v1", "category_id": f.catA.ID}), cf("40001"))
	f.mustTrack(t, f.event("product_view", map[string]any{"visitor_id": "v1", "product_id": f.prodA.ID}), cf("40001"))
	f.mustTrack(t, f.event("product_view", map[string]any{"visitor_id": "v1", "product_id": f.prodA.ID}), cf("40001"))
	// Visitor two: the menu, both categories, the other product.
	f.mustTrack(t, f.event("menu_view", map[string]any{"visitor_id": "v2"}), cf("40002"))
	f.mustTrack(t, f.event("category_view", map[string]any{"visitor_id": "v2", "category_id": f.catA.ID}), cf("40002"))
	f.mustTrack(t, f.event("category_view", map[string]any{"visitor_id": "v2", "category_id": f.catB.ID}), cf("40002"))
	f.mustTrack(t, f.event("product_view", map[string]any{"visitor_id": "v2", "product_id": f.prodB.ID}), cf("40002"))
	// An anonymous visitor twice: one key.
	anon := map[string]string{clientip.HeaderXRealIP: "203.0.113.99", "User-Agent": "Anon/2"}
	f.mustTrack(t, f.event("menu_view", nil), anon)
	f.mustTrack(t, f.event("menu_view", nil), anon)
	// Visitor three, on the tenant's second menu.
	f.mustTrack(t, eventBody(map[string]any{"business_slug": f.businessSlug, "menu_slug": f.second.Slug,
		"type": "menu_view", "visitor_id": "v3"}), nil)
	f.mustTrack(t, eventBody(map[string]any{"business_slug": f.businessSlug, "menu_slug": f.second.Slug,
		"type": "category_view", "visitor_id": "v3", "category_id": f.catSecond.ID}), nil)
	// The other tenant's traffic, which must never show up.
	for i := 0; i < 5; i++ {
		f.mustTrack(t, eventBody(map[string]any{"business_slug": f.otherSlug, "menu_slug": f.otherMenu.Slug,
			"type": "product_view", "product_id": f.otherProd.ID, "visitor_id": fmt.Sprintf("o%d", i)}), nil)
	}

	// Visitor two's visit happened three days ago.
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE menu_events SET created_at = now() - interval '3 days' WHERE visitor_id = 'v2'`); err != nil {
		t.Fatalf("could not age visitor two's events: %v", err)
	}
	today := time.Now().In(utils.Istanbul).Format("2006-01-02")
	threeDaysAgo := time.Now().Add(-72 * time.Hour).In(utils.Istanbul).Format("2006-01-02")

	summary := func(t *testing.T, who *tenant, query string) (summaryPayload, int, []byte) {
		t.Helper()
		resp, payload := h.do(http.MethodGet, "/api/analytics/summary"+query, who.session, nil)
		var body summaryPayload
		if resp.StatusCode == http.StatusOK {
			decodeInto(t, "summary", payload, &body)
		}
		return body, resp.StatusCode, payload
	}

	t.Run("the_whole_business_over_the_default_window", func(t *testing.T) {
		body, status, payload := summary(t, f.owner, "")
		if status != http.StatusOK {
			t.Fatalf("answered %d: %s", status, payload)
		}
		if body.To != today || len(body.Daily) != 30 || body.Daily[29].Date != today {
			t.Errorf("window %s..%s with %d days, want 30 days ending today %s", body.From, body.To, len(body.Daily), today)
		}
		if body.TotalVisits != 5 || body.MenuViews != 5 || body.CategoryViews != 4 ||
			body.ProductViews != 3 || body.UniqueVisitors != 4 {
			t.Errorf("totals %+v, want visits 5, category 4, product 3, unique 4", body)
		}
		var sawToday, sawPast bool
		for _, day := range body.Daily {
			switch day.Date {
			case today:
				sawToday = true
				if day.Visits != 4 || day.UniqueVisitors != 3 || day.CategoryViews != 2 || day.ProductViews != 2 {
					t.Errorf("today %+v, want visits 4, unique 3, category 2, product 2", day)
				}
			case threeDaysAgo:
				sawPast = true
				if day.Visits != 1 || day.UniqueVisitors != 1 || day.CategoryViews != 2 || day.ProductViews != 1 {
					t.Errorf("three days ago %+v, want visits 1, unique 1, category 2, product 1", day)
				}
			default:
				if day.Visits+day.UniqueVisitors+day.CategoryViews+day.ProductViews != 0 {
					t.Errorf("an empty day %s is %+v", day.Date, day)
				}
			}
		}
		if !sawToday || !sawPast {
			t.Errorf("the series lacks today (%v) or three days ago (%v)", sawToday, sawPast)
		}
	})

	t.Run("one_menu_and_its_top_lists", func(t *testing.T) {
		body, status, payload := summary(t, f.owner, "?menu_id="+f.menu.ID)
		if status != http.StatusOK {
			t.Fatalf("answered %d: %s", status, payload)
		}
		if body.TotalVisits != 4 || body.CategoryViews != 3 || body.ProductViews != 3 || body.UniqueVisitors != 3 {
			t.Errorf("menu totals %+v, want visits 4, category 3, product 3, unique 3", body)
		}
		if len(body.TopCategories) != 2 || body.TopCategories[0].ID != f.catA.ID ||
			body.TopCategories[0].Name != "Kahveler" || body.TopCategories[0].Views != 2 ||
			body.TopCategories[1].Name != "Sandviçler" || body.TopCategories[1].Views != 1 {
			t.Errorf("top categories %+v, want Kahveler 2, Sandviçler 1", body.TopCategories)
		}
		if len(body.TopProducts) != 2 || body.TopProducts[0].Name != "Latte" ||
			body.TopProducts[0].CategoryName != "Kahveler" || body.TopProducts[0].Views != 2 ||
			body.TopProducts[1].Name != "Ciabatta Sandwich" || body.TopProducts[1].Views != 1 {
			t.Errorf("top products %+v, want Latte (Kahveler) 2, Ciabatta Sandwich 1", body.TopProducts)
		}
	})

	t.Run("an_explicit_window", func(t *testing.T) {
		body, status, payload := summary(t, f.owner, "?from="+today+"&to="+today)
		if status != http.StatusOK {
			t.Fatalf("answered %d: %s", status, payload)
		}
		if len(body.Daily) != 1 || body.TotalVisits != 4 || body.UniqueVisitors != 3 {
			t.Errorf("today only: %+v, want one day, visits 4, unique 3", body)
		}
	})

	t.Run("tenant_isolation", func(t *testing.T) {
		if _, status, payload := summary(t, f.other, "?menu_id="+f.menu.ID); status != http.StatusNotFound {
			t.Errorf("another tenant's menu_id answered %d, want 404: %s", status, payload)
		}
		body, status, payload := summary(t, f.other, "")
		if status != http.StatusOK {
			t.Fatalf("the other tenant's own summary answered %d: %s", status, payload)
		}
		if body.ProductViews != 5 || body.MenuViews != 0 || body.UniqueVisitors != 5 {
			t.Errorf("the other tenant sees %+v, want only its own 5 product views", body)
		}
		resp, payload := h.do(http.MethodGet, "/api/analytics/events?menu_id="+f.menu.ID, f.other.session, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("another tenant's events answered %d, want 404: %s", resp.StatusCode, payload)
		}
		resp, payload = h.do(http.MethodGet, "/api/analytics/events", f.other.session, nil)
		var events eventsPayload
		decodeInto(t, "other events", payload, &events)
		if resp.StatusCode != http.StatusOK || events.Total != 5 {
			t.Errorf("the other tenant's event list: %d, total %d, want 5", resp.StatusCode, events.Total)
		}
		for _, item := range events.Items {
			if item.MenuID != f.otherMenu.ID {
				t.Errorf("the other tenant sees an event of menu %s", item.MenuID)
			}
		}
		if resp, _ := h.do(http.MethodGet, "/api/analytics/summary", "", nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("an anonymous summary answered %d, want 401", resp.StatusCode)
		}
	})

	t.Run("summary_refusals", func(t *testing.T) {
		for _, tc := range []struct {
			query  string
			status int
		}{
			{"?menu_id=not-a-uuid", http.StatusBadRequest},
			{"?from=2026-09-10&to=2026-09-01", http.StatusUnprocessableEntity},
			{"?from=2020-01-01&to=2026-01-01", http.StatusUnprocessableEntity},
			{"?from=yesterday", http.StatusUnprocessableEntity},
			// Go parses year 0000, which PostgreSQL's calendar does not have:
			// a 422 like any other bad date, never a 500.
			{"?from=0000-01-01&to=0000-01-31", http.StatusUnprocessableEntity},
			{"?to=0000-12-31", http.StatusUnprocessableEntity},
			{"?from=1999-12-31&to=2000-01-02", http.StatusUnprocessableEntity},
		} {
			if _, status, payload := summary(t, f.owner, tc.query); status != tc.status {
				t.Errorf("%s answered %d, want %d: %s", tc.query, status, tc.status, payload)
			}
		}
	})

	events := func(t *testing.T, query string) (eventsPayload, int, []byte) {
		t.Helper()
		resp, payload := h.do(http.MethodGet, "/api/analytics/events"+query, f.owner.session, nil)
		var body eventsPayload
		if resp.StatusCode == http.StatusOK {
			decodeInto(t, "events", payload, &body)
		}
		return body, resp.StatusCode, payload
	}

	t.Run("the_event_list", func(t *testing.T) {
		body, status, payload := events(t, "?menu_id="+f.menu.ID+"&limit=3")
		if status != http.StatusOK {
			t.Fatalf("answered %d: %s", status, payload)
		}
		if body.Total != 10 || len(body.Items) != 3 || body.Limit != 3 || body.Offset != 0 {
			t.Fatalf("total %d, %d items, limit %d — want 10 in all, 3 on the page", body.Total, len(body.Items), body.Limit)
		}
		for i := 1; i < len(body.Items); i++ {
			if body.Items[i-1].CreatedAt < body.Items[i].CreatedAt {
				t.Errorf("the list is not newest first: %s before %s", body.Items[i-1].CreatedAt, body.Items[i].CreatedAt)
			}
		}
		for _, item := range body.Items {
			if _, err := time.Parse(time.RFC3339, item.CreatedAt); err != nil {
				t.Errorf("created_at %q is not RFC 3339", item.CreatedAt)
			}
			if item.MenuName != "Ana Menü" {
				t.Errorf("menu_name %q", item.MenuName)
			}
		}

		products, _, _ := events(t, "?menu_id="+f.menu.ID+"&type=product_view")
		if products.Total != 3 {
			t.Fatalf("product views: %d, want 3", products.Total)
		}
		for _, item := range products.Items {
			if item.ProductName == nil || item.CategoryName == nil || item.IP == nil || item.Port == nil ||
				item.IPSource != "cloudflare" {
				t.Errorf("a product view lacks its names or address: %+v", item)
				continue
			}
			if *item.IP != "198.51.100.20" || (*item.Port != 40001 && *item.Port != 40002) {
				t.Errorf("address %s:%d", *item.IP, *item.Port)
			}
		}

		byIP, _, _ := events(t, "?ip=203.0.113.")
		if byIP.Total != 2 {
			t.Errorf("ip prefix 203.0.113.: %d events, want the anonymous visitor's 2", byIP.Total)
		}
		for _, item := range byIP.Items {
			if item.Port != nil || item.IPSource != "edge" {
				t.Errorf("an X-Real-IP event lists port %v via %s", item.Port, item.IPSource)
			}
		}

		old, _, _ := events(t, "?from="+threeDaysAgo+"&to="+threeDaysAgo)
		if old.Total != 4 {
			t.Errorf("three days ago: %d events, want visitor two's 4", old.Total)
		}

		capped, _, _ := events(t, "?limit=1000")
		if capped.Limit != 200 {
			t.Errorf("limit 1000 answered with limit %d, want 200", capped.Limit)
		}
		for _, bad := range []string{"?type=click", "?ip=1'%20OR%201=1", "?offset=-1", "?limit=abc",
			"?from=2026-99-01", "?from=2026-09-10&to=2026-09-01", "?from=0000-01-01", "?to=0000-01-01"} {
			if _, status, payload := events(t, bad); status != http.StatusUnprocessableEntity {
				t.Errorf("%s answered %d, want 422: %s", bad, status, payload)
			}
		}
	})

	t.Run("a_deleted_product_keeps_its_events_but_loses_its_name", func(t *testing.T) {
		resp, payload := h.do(http.MethodDelete, "/api/products/"+f.prodB.ID, f.owner.session, nil)
		h.requireSuccess("delete product B", resp, payload)

		body, _, _ := events(t, "?type=product_view&menu_id="+f.menu.ID)
		if body.Total != 3 {
			t.Fatalf("product views after the delete: %d, want 3 — history is not rewritten", body.Total)
		}
		for _, item := range body.Items {
			if item.ProductID != nil && *item.ProductID == f.prodB.ID && item.ProductName != nil {
				t.Errorf("the deleted product is still named %q", *item.ProductName)
			}
		}
		summary, _, _ := summary(t, f.owner, "?menu_id="+f.menu.ID)
		if summary.ProductViews != 3 || len(summary.TopProducts) != 1 {
			t.Errorf("after the delete: %d product views, top products %+v — want 3 and only Latte",
				summary.ProductViews, summary.TopProducts)
		}
	})

	t.Run("the_retention_purge", func(t *testing.T) {
		before := countEvents(t, h)
		removed, err := repository.PurgeMenuEvents(context.Background(), h.pool, time.Now().Add(-48*time.Hour))
		if err != nil {
			t.Fatalf("PurgeMenuEvents failed: %v", err)
		}
		if removed != 4 || countEvents(t, h) != before-4 {
			t.Errorf("the purge removed %d of %d events, want visitor two's 4", removed, before)
		}
	})

	t.Run("deleting_a_menu_deletes_its_events", func(t *testing.T) {
		resp, payload := h.do(http.MethodDelete, "/api/menus/"+f.second.ID, f.owner.session, nil)
		h.requireSuccess("delete the second menu", resp, payload)
		var n int
		if err := h.pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM menu_events WHERE menu_id = $1::text::uuid`, f.second.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%d events of the deleted menu survived it", n)
		}
	})
}
