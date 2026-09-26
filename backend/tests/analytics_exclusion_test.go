package tests

// The owner's own visits — and the platform operator's — are never stored in
// the analytics. Over HTTP, against a scratch database:
//
//   - each of the three mechanisms (the business's list, the platform's
//     ANALYTICS_EXCLUDED_IPS, the browser's opt-out cookie) answers the usual
//     204 and stores nothing, for a single IPv4 or IPv6 address, a range, and
//     every kind of view;
//   - a change to the list takes effect on the very next event, without
//     waiting for the cache's TTL;
//   - excluded traffic spends none of the event gate's budget: it is not
//     claimed as a repeat and not counted against the daily cap;
//   - adding an entry can delete the matching history, in one transaction with
//     its audit row, touching no other tenant's events and tolerating ip
//     values that are not addresses;
//   - duplicates, covered ranges, over-broad ranges and the 50-entry limit are
//     refused; the list is tenant-scoped in every direction;
//   - the opt-out cookie carries exactly the attributes the menus need, in
//     development and in production;
//   - the platform list never appears in any response;
//   - migration 015 can run twice, and its reverse script undoes it;
//   - karecik_try_inet reads only bare addresses, under both of its bodies,
//     and on PostgreSQL 16+ is inlined into the range queries.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/config"
	"karecik/backend/internal/eventgate"
	"karecik/backend/internal/ipexclude"
	"karecik/backend/internal/mailer"
	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
	"karecik/backend/migrations"
)

// ------------------------------------------------------------------ helpers

type excludedItem struct {
	ID             string    `json:"id"`
	CIDR           string    `json:"cidr"`
	Display        string    `json:"display"`
	Label          string    `json:"label"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedByEmail *string   `json:"created_by_email"`
}

type excludedList struct {
	Items             []excludedItem `json:"items"`
	CurrentIP         *string        `json:"current_ip"`
	CurrentIPSource   string         `json:"current_ip_source"`
	CurrentIPExcluded bool           `json:"current_ip_excluded"`
	OptOut            bool           `json:"optout"`
	Max               int            `json:"max"`
}

type addedExclusion struct {
	Item          excludedItem `json:"item"`
	DeletedEvents int          `json:"deleted_events"`
}

type matchCount struct {
	CIDR  string `json:"cidr"`
	Count int    `json:"count"`
}

// rawJSON is a request body sent exactly as written.
type rawJSON string

const excludedIPsPath = "/api/analytics/excluded-ips"

// call sends one request as a tenant (nil: no session), from the client the
// headers describe, with any extra cookies. doWith cannot do the last part: it
// sets extra headers after the session cookie, so a Cookie header would
// replace the session.
func call(t *testing.T, h *harness, method, path string, owner *tenant, body any,
	headers map[string]string, cookies ...*http.Cookie) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	switch value := body.(type) {
	case nil:
	case rawJSON:
		reader = strings.NewReader(string(value))
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("could not encode the body of %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if owner != nil {
		req.AddCookie(&http.Cookie{Name: utils.SessionCookieName, Value: owner.session})
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := h.app.Test(req, requestTimeoutMS)
	if err != nil {
		t.Fatalf("%s %s never completed: %v", method, path, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: could not read the body: %v", method, path, err)
	}
	return resp, payload
}

// fromEdge are the headers of a request whose address the platform edge
// reported.
func fromEdge(ip string) map[string]string {
	return map[string]string{"X-Real-IP": ip}
}

var optOutCookie = &http.Cookie{Name: middleware.OptOutCookieName, Value: middleware.OptOutCookieValue}

// addExclusion adds an entry and requires the 201.
func addExclusion(t *testing.T, h *harness, owner *tenant, body map[string]any) addedExclusion {
	t.Helper()
	resp, payload := call(t, h, http.MethodPost, excludedIPsPath, owner, body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("adding %v answered %d, want 201: %s", body, resp.StatusCode, payload)
	}
	var added addedExclusion
	decodeInto(t, "added exclusion", payload, &added)
	return added
}

// exclude adds an address without touching its history.
func exclude(t *testing.T, h *harness, owner *tenant, cidr string) excludedItem {
	t.Helper()
	return addExclusion(t, h, owner, map[string]any{"cidr": cidr, "delete_history": false}).Item
}

func removeExclusion(t *testing.T, h *harness, owner *tenant, id string) {
	t.Helper()
	resp, payload := call(t, h, http.MethodDelete, excludedIPsPath+"/"+id, owner, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("removing %s answered %d, want 204: %s", id, resp.StatusCode, payload)
	}
}

func listExclusions(t *testing.T, h *harness, owner *tenant, headers map[string]string,
	cookies ...*http.Cookie) (excludedList, []byte) {
	t.Helper()
	resp, payload := call(t, h, http.MethodGet, excludedIPsPath, owner, nil, headers, cookies...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s answered %d: %s", excludedIPsPath, resp.StatusCode, payload)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("GET %s answered Cache-Control %q, want no-store", excludedIPsPath, got)
	}
	var list excludedList
	decodeInto(t, "exclusion list", payload, &list)
	return list, payload
}

func matchCountOf(t *testing.T, h *harness, owner *tenant, cidr string) matchCount {
	t.Helper()
	path := excludedIPsPath + "/match-count?cidr=" + strings.ReplaceAll(cidr, "/", "%2F")
	resp, payload := call(t, h, http.MethodGet, path, owner, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("match-count of %s answered %d: %s", cidr, resp.StatusCode, payload)
	}
	var count matchCount
	decodeInto(t, "match count", payload, &count)
	return count
}

// trackFrom sends one view of the fixture's menu from the given client and
// requires the 204 every accepted event gets, stored or not.
func (f *analyticsFixture) trackFrom(t *testing.T, kind, visitor string, headers map[string]string) {
	t.Helper()
	extra := map[string]any{"visitor_id": visitor}
	switch kind {
	case "category_view":
		extra["category_id"] = f.catA.ID
	case "product_view":
		extra["product_id"] = f.prodA.ID
	}
	resp, payload := sendEvent(t, f.h, f.event(kind, extra), "text/plain;charset=UTF-8", headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("a %s of %s answered %d, want 204: %s", kind, visitor, resp.StatusCode, payload)
	}
}

// storedOf counts the stored events of one visitor id.
func storedOf(t *testing.T, h *harness, visitor string) int {
	t.Helper()
	return countWhere(t, h, `visitor_id = $1`, visitor)
}

// withCookie adds a Cookie header to a set of client headers. sendEvent sets
// headers, not cookies, and an event carries no session to collide with.
func withCookie(headers map[string]string, cookie string) map[string]string {
	merged := map[string]string{"Cookie": cookie}
	for key, value := range headers {
		merged[key] = value
	}
	return merged
}

// insertRawEvent stores an event row directly, with any ip value at all — the
// only way to put a value the endpoint would never write ('unknown', the empty
// string) into the table.
func insertRawEvent(t *testing.T, h *harness, businessID, menuID string, ip *string) {
	t.Helper()
	if _, err := h.pool.Exec(context.Background(), `
		INSERT INTO menu_events (business_id, menu_id, type, visitor_key, ip)
		VALUES ($1::text::uuid, $2::text::uuid, 'menu_view', 'h:raw', $3)`,
		businessID, menuID, ip); err != nil {
		t.Fatalf("could not insert a raw event with ip %v: %v", ip, err)
	}
}

func textPtr(s string) *string { return &s }

// ------------------------------------------------------- the three mechanisms

func TestExcludedVisitsAreNeverStored(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h

	exclude(t, h, f.owner, "198.18.139.87")
	exclude(t, h, f.owner, "203.0.113.0/24")
	exclude(t, h, f.owner, "2001:db8:1::/48")
	exclude(t, h, f.owner, "2001:db8:2::1")

	for _, tc := range []struct {
		name    string
		headers map[string]string
		stored  bool
	}{
		{"single_ipv4", fromEdge("198.18.139.87"), false},
		{"single_ipv4_mapped", fromEdge("::ffff:198.18.139.87"), false},
		{"single_ipv4_through_cloudflare", fromCloudflare("198.18.139.87", "40001"), false},
		{"ipv4_range_low", fromEdge("203.0.113.0"), false},
		{"ipv4_range_high", fromEdge("203.0.113.255"), false},
		{"ipv6_range", fromEdge("2001:db8:1:abcd::9"), false},
		{"ipv6_single", fromEdge("2001:db8:2::1"), false},
		{"optout_cookie", withCookie(fromEdge("198.51.100.50"), "karecik_analytics_optout=1"), false},
		{"optout_cookie_among_others", withCookie(fromEdge("198.51.100.50"), "a=b; karecik_analytics_optout=1; c=d"), false},

		// The neighbours of every entry are real visitors.
		{"ipv4_neighbour_below", fromEdge("198.18.139.86"), true},
		{"ipv4_neighbour_above", fromEdge("198.18.139.88"), true},
		{"ipv4_outside_range", fromEdge("203.0.114.1"), true},
		{"ipv6_outside_range", fromEdge("2001:db8:3::1"), true},
		{"ipv6_neighbour", fromEdge("2001:db8:2::2"), true},
		{"optout_cookie_other_value", withCookie(fromEdge("198.51.100.50"), "karecik_analytics_optout=0"), true},
		// An address that could not be established never matches.
		{"unknown_address", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, kind := range []string{"menu_view", "category_view", "product_view"} {
				visitor := tc.name + "-" + kind
				f.trackFrom(t, kind, visitor, tc.headers)
				want := 0
				if tc.stored {
					want = 1
				}
				if n := storedOf(t, h, visitor); n != want {
					t.Errorf("a %s stored %d rows, want %d", kind, n, want)
				}
			}
		})
	}

	// The list is the business's own: the same address visiting another
	// tenant's menu is that tenant's customer.
	t.Run("another_tenants_menu", func(t *testing.T) {
		body := eventBody(map[string]any{"business_slug": f.otherSlug, "menu_slug": f.otherMenu.Slug,
			"type": "menu_view", "visitor_id": "elsewhere"})
		if resp, payload := sendEvent(t, h, body, "text/plain", fromEdge("198.18.139.87")); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("answered %d: %s", resp.StatusCode, payload)
		}
		if n := storedOf(t, h, "elsewhere"); n != 1 {
			t.Errorf("the owner's list kept a visit to another tenant's menu out: %d rows, want 1", n)
		}
	})

	// Validation still comes first: an excluded address sending garbage gets
	// the same 422 as anybody else, not a 204 that would hide the bug.
	t.Run("invalid_events_are_still_refused", func(t *testing.T) {
		resp, payload := sendEvent(t, h, f.event("menu_explode", nil), "text/plain", fromEdge("198.18.139.87"))
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("an invalid event from an excluded address answered %d: %s", resp.StatusCode, payload)
		}
		body := eventBody(map[string]any{"business_slug": f.businessSlug, "menu_slug": "no-such-menu", "type": "menu_view"})
		if resp, payload := sendEvent(t, h, body, "text/plain", fromEdge("198.18.139.87")); resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("an event of an unknown menu from an excluded address answered %d: %s", resp.StatusCode, payload)
		}
	})
}

// ANALYTICS_EXCLUDED_IPS applies to every tenant, and is never shown to any of
// them — not in a list, not in a flag, not in a conflict.
func TestPlatformExclusionsApplyEverywhereAndAreNeverShown(t *testing.T) {
	platform, warnings := config.ExcludedIPs("192.0.2.77, 2001:db8:77::/48")
	if len(platform) != 2 || len(warnings) != 0 {
		t.Fatalf("the fixture list parsed to %v / %q", platform, warnings)
	}
	h := newHarnessConfigured(t, mailer.Disabled{}, func(cfg *config.Config) {
		cfg.AnalyticsExcludedIPs = platform
	})
	owner := h.register("owner", "Platform Kafe", "platform-owner@example.test")
	other := h.register("other", "Diger Kafe", "platform-other@example.test")
	ownerSlug, otherSlug := readBusinessSlug(t, h, owner), readBusinessSlug(t, h, other)
	ownerMenu, otherMenu := h.createMenu(owner, "Ana Menu"), h.createMenu(other, "Ana Menu")

	track := func(slug, menuSlug, visitor, ip string) {
		t.Helper()
		body := eventBody(map[string]any{"business_slug": slug, "menu_slug": menuSlug,
			"type": "menu_view", "visitor_id": visitor})
		if resp, payload := sendEvent(t, h, body, "text/plain", fromEdge(ip)); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("event of %s answered %d: %s", visitor, resp.StatusCode, payload)
		}
	}
	track(ownerSlug, ownerMenu.Slug, "operator-owner", "192.0.2.77")
	track(otherSlug, otherMenu.Slug, "operator-other", "192.0.2.77")
	track(ownerSlug, ownerMenu.Slug, "operator-v6", "2001:db8:77:5::9")
	track(ownerSlug, ownerMenu.Slug, "customer", "192.0.2.78")
	for visitor, want := range map[string]int{"operator-owner": 0, "operator-other": 0, "operator-v6": 0, "customer": 1} {
		if n := storedOf(t, h, visitor); n != want {
			t.Errorf("%s stored %d rows, want %d", visitor, n, want)
		}
	}

	// Every response a tenant can ask for, from an address of its own. None
	// may carry anything of the platform list.
	exclude(t, h, owner, "198.51.100.9")
	var responses []string
	record := func(what string, resp *http.Response, payload []byte) {
		t.Helper()
		if resp.StatusCode >= 500 {
			t.Fatalf("%s answered %d: %s", what, resp.StatusCode, payload)
		}
		var headers strings.Builder
		for key, values := range resp.Header {
			fmt.Fprintf(&headers, "%s: %s\n", key, strings.Join(values, ", "))
		}
		responses = append(responses, what+"\n"+headers.String()+string(payload))
	}
	client := fromEdge("198.51.100.20")
	for _, path := range []string{
		excludedIPsPath,
		excludedIPsPath + "/match-count?cidr=198.51.100.0%2F24",
		"/api/analytics/summary",
		"/api/analytics/events",
		"/api/audit-logs",
		"/api/business",
		"/api/auth/me",
		"/api/menus",
		"/api/meta",
		"/api/health",
		"/api/public/menu/" + ownerSlug + "/" + ownerMenu.Slug,
	} {
		resp, payload := call(t, h, http.MethodGet, path, owner, nil, client)
		record("GET "+path, resp, payload)
	}
	resp, payload := call(t, h, http.MethodPost, excludedIPsPath, owner,
		map[string]any{"cidr": "198.51.100.10", "delete_history": true}, client)
	record("POST "+excludedIPsPath, resp, payload)
	var added addedExclusion
	decodeInto(t, "added", payload, &added)
	resp, payload = call(t, h, http.MethodDelete, excludedIPsPath+"/"+added.Item.ID, owner, nil, client)
	record("DELETE "+excludedIPsPath, resp, payload)
	resp, payload = call(t, h, http.MethodPost, "/api/analytics/optout", owner, nil, client)
	record("POST /api/analytics/optout", resp, payload)

	for _, response := range responses {
		for _, secret := range []string{"192.0.2.77", "2001:db8:77", "ANALYTICS_EXCLUDED_IPS"} {
			if strings.Contains(strings.ToLower(response), strings.ToLower(secret)) {
				t.Errorf("a response carries %q:\n%s", secret, response)
			}
		}
	}

	// Asked from the operator's own address, the panel reports that address —
	// it is the caller's own — but not that the platform excludes it.
	list, _ := listExclusions(t, h, owner, fromEdge("192.0.2.77"))
	if list.CurrentIP == nil || *list.CurrentIP != "192.0.2.77" || list.CurrentIPExcluded {
		t.Errorf("from a platform-excluded address the panel reported ip %v excluded=%v, want the address and false",
			list.CurrentIP, list.CurrentIPExcluded)
	}
	// Nor does it refuse the address as a duplicate: that 409 would say the
	// platform lists it.
	exclude(t, h, owner, "192.0.2.77")
	if n := matchCountOf(t, h, owner, "2001:db8:77::/48"); n.Count != 0 {
		t.Errorf("the platform-excluded range matched %d stored events, want 0", n.Count)
	}
}

// The list the events endpoint reads is cached, and a change through the panel
// is seen by the very next event anyway. The Set's clock is frozen, so nothing
// but invalidation can explain a change.
func TestExclusionChangesTakeEffectOnTheNextEvent(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	frozen := time.Now()
	h.handler.Exclusions = ipexclude.New(ipexclude.Config{
		Load: func(ctx context.Context, businessID uuid.UUID) ([]netip.Prefix, error) {
			return repository.ExcludedPrefixes(ctx, h.pool, businessID)
		},
		Now: func() time.Time { return frozen },
	})
	owner := fromEdge("198.18.139.87")

	f.trackFrom(t, "menu_view", "before-any-entry", owner)
	if n := storedOf(t, h, "before-any-entry"); n != 1 {
		t.Fatalf("before any entry the owner's view stored %d rows, want 1", n)
	}

	// An entry written behind the endpoints' back is not seen while the cached
	// list is fresh — which is what makes the next step prove something.
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `INSERT INTO analytics_excluded_ips (business_id, cidr)
		VALUES ($1::text::uuid, '198.18.139.87/32')`, f.owner.businessID); err != nil {
		t.Fatal(err)
	}
	f.trackFrom(t, "menu_view", "behind-the-back", owner)
	if n := storedOf(t, h, "behind-the-back"); n != 1 {
		t.Fatalf("the cached list was not used: a view after a direct INSERT stored %d rows, want 1", n)
	}
	if _, err := h.pool.Exec(ctx, `DELETE FROM analytics_excluded_ips`); err != nil {
		t.Fatal(err)
	}

	item := exclude(t, h, f.owner, "198.18.139.87")
	f.trackFrom(t, "menu_view", "after-add", owner)
	if n := storedOf(t, h, "after-add"); n != 0 {
		t.Errorf("the view right after the add stored %d rows, want 0", n)
	}

	removeExclusion(t, h, f.owner, item.ID)
	f.trackFrom(t, "menu_view", "after-remove", owner)
	if n := storedOf(t, h, "after-remove"); n != 1 {
		t.Errorf("the view right after the removal stored %d rows, want 1", n)
	}
}

// An excluded view is decided before the event gate: it is not remembered as
// a repeat, and not counted against the daily cap.
func TestExcludedTrafficSpendsNoEventBudget(t *testing.T) {
	const eventCap = 3
	h := newHarnessConfigured(t, mailer.Disabled{}, func(cfg *config.Config) {
		cfg.AnalyticsDailyEventCap = eventCap
	})
	frozen := time.Now()
	h.handler.Events = eventgate.New(eventgate.Config{
		DailyCap: eventCap, Zone: utils.Istanbul, Now: func() time.Time { return frozen },
	})
	owner := h.register("owner", "Butce Kafe", "budget-owner@example.test")
	slug := readBusinessSlug(t, h, owner)
	menu := h.createMenu(owner, "Ana Menu")
	item := exclude(t, h, owner, "198.18.139.87")

	track := func(visitor string, headers map[string]string) {
		t.Helper()
		body := eventBody(map[string]any{"business_slug": slug, "menu_slug": menu.Slug,
			"type": "menu_view", "visitor_id": visitor})
		if resp, payload := sendEvent(t, h, body, "text/plain", headers); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("event of %s answered %d: %s", visitor, resp.StatusCode, payload)
		}
	}
	stored := func() int { return countWhere(t, h, `business_id = $1::text::uuid`, owner.businessID) }

	for i := 0; i < 5; i++ {
		track("the-owner", fromEdge("198.18.139.87")) // the same view, over and over
		track(fmt.Sprintf("owner-tab-%d", i), fromEdge("198.18.139.87"))
		track(fmt.Sprintf("owner-phone-%d", i), withCookie(fromEdge("198.51.100.7"), "karecik_analytics_optout=1"))
	}
	if n := stored(); n != 0 {
		t.Fatalf("fifteen excluded views stored %d rows, want 0", n)
	}

	// Not a repeat: the exact view sent five times while excluded is stored
	// the moment the address stops being excluded — with the gate's clock
	// frozen, a claim from any of those five would still be in force.
	removeExclusion(t, h, owner, item.ID)
	track("the-owner", fromEdge("198.18.139.87"))
	if n := stored(); n != 1 {
		t.Fatalf("the owner's view after the removal stored %d rows in all, want 1", n)
	}

	// Not counted: the cap of three still has room for two customers.
	track("customer-1", fromEdge("198.51.100.20"))
	track("customer-2", fromEdge("198.51.100.21"))
	if n := stored(); n != eventCap {
		t.Fatalf("two customers after fifteen excluded views stored %d rows in all, want %d", n, eventCap)
	}
	// And the cap is live, so the numbers above prove something.
	track("customer-3", fromEdge("198.51.100.22"))
	if n := stored(); n != eventCap {
		t.Errorf("a view past the cap was stored: %d rows, want %d", n, eventCap)
	}
}

// ------------------------------------------------------------ adding entries

func TestAddingAnExclusionWithAndWithoutItsHistory(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h

	ownerIPs := []*string{
		textPtr("198.18.139.87"), textPtr("198.18.139.87"), textPtr("198.18.139.87"),
		textPtr("198.18.139.88"),
		textPtr("203.0.113.5"), textPtr("203.0.113.5"),
		textPtr("2001:db8:1::5"),
		// Values that are not addresses: none may match, none may fail a
		// statement.
		textPtr("unknown"), textPtr("-"), textPtr(""), textPtr("garbage::zz"), textPtr("1.2.3.999"), nil,
	}
	for _, ip := range ownerIPs {
		insertRawEvent(t, h, f.owner.businessID, f.menu.ID, ip)
	}
	for _, ip := range []string{"198.18.139.87", "198.18.139.87", "203.0.113.5"} {
		insertRawEvent(t, h, f.other.businessID, f.otherMenu.ID, textPtr(ip))
	}
	ownerRows := func() int { return countWhere(t, h, `business_id = $1::text::uuid`, f.owner.businessID) }
	otherRows := func() int { return countWhere(t, h, `business_id = $1::text::uuid`, f.other.businessID) }
	nonAddresses := func() int {
		return countWhere(t, h, `business_id = $1::text::uuid AND (ip IS NULL OR ip IN ('unknown', '-', '', 'garbage::zz', '1.2.3.999'))`,
			f.owner.businessID)
	}

	t.Run("match_count", func(t *testing.T) {
		for cidr, want := range map[string]matchCount{
			"198.18.139.87":    {"198.18.139.87/32", 3},
			"198.18.139.99/24": {"198.18.139.0/24", 4},
			"203.0.113.0/24":   {"203.0.113.0/24", 2},
			"2001:db8:1::/48":  {"2001:db8:1::/48", 1},
			"198.51.100.1":     {"198.51.100.1/32", 0},
		} {
			if got := matchCountOf(t, h, f.owner, cidr); got != want {
				t.Errorf("match-count of %s = %+v, want %+v", cidr, got, want)
			}
		}
		// The other tenant counts its own events only.
		if got := matchCountOf(t, h, f.other, "198.18.139.87"); got.Count != 2 {
			t.Errorf("the other tenant's match-count = %d, want 2", got.Count)
		}
	})

	var keep, office, v6, wide addedExclusion
	t.Run("without_history", func(t *testing.T) {
		before := ownerRows()
		keep = addExclusion(t, h, f.owner, map[string]any{"cidr": "198.18.139.87", "delete_history": false})
		if keep.DeletedEvents != 0 || ownerRows() != before {
			t.Errorf("an add without history deleted %d (rows %d -> %d)", keep.DeletedEvents, before, ownerRows())
		}
		item := keep.Item
		if _, err := uuid.Parse(item.ID); err != nil || item.CIDR != "198.18.139.87/32" ||
			item.Display != "198.18.139.87" || item.Label != "" || item.CreatedAt.IsZero() ||
			item.CreatedByEmail == nil || *item.CreatedByEmail != "analytics-owner@example.test" {
			t.Errorf("the new entry reads %+v", item)
		}
	})

	t.Run("with_history", func(t *testing.T) {
		office = addExclusion(t, h, f.owner, map[string]any{"cidr": "203.0.113.0/24", "label": "  Ofis  ", "delete_history": true})
		if office.DeletedEvents != 2 {
			t.Errorf("the /24 deleted %d events, want 2", office.DeletedEvents)
		}
		if office.Item.Label != "Ofis" || office.Item.Display != "203.0.113.0/24" {
			t.Errorf("the entry reads %+v", office.Item)
		}
		if n := countWhere(t, h, `business_id = $1::text::uuid AND ip = '203.0.113.5'`, f.owner.businessID); n != 0 {
			t.Errorf("%d of the owner's events in the range survived", n)
		}

		v6 = addExclusion(t, h, f.owner, map[string]any{"cidr": "2001:db8:1::/48", "delete_history": true})
		if v6.DeletedEvents != 1 {
			t.Errorf("the IPv6 range deleted %d events, want 1", v6.DeletedEvents)
		}

		// A broader range over an existing narrower entry is accepted; its
		// history is everything inside it, the narrower entry's included.
		wide = addExclusion(t, h, f.owner, map[string]any{"cidr": "198.18.139.0/24", "delete_history": true})
		if wide.DeletedEvents != 4 {
			t.Errorf("the /24 over the /32 deleted %d events, want 4", wide.DeletedEvents)
		}

		if n := nonAddresses(); n != 6 {
			t.Errorf("%d of the six rows without an address survived, want all six", n)
		}
		if n := ownerRows(); n != 6 {
			t.Errorf("the owner has %d events left, want the 6 without an address", n)
		}
		if n := otherRows(); n != 3 {
			t.Errorf("the other tenant has %d events left, want its 3 untouched", n)
		}
	})

	t.Run("listed_in_order", func(t *testing.T) {
		list, _ := listExclusions(t, h, f.owner, nil)
		want := []string{keep.Item.ID, office.Item.ID, v6.Item.ID, wide.Item.ID}
		if len(list.Items) != len(want) {
			t.Fatalf("the list has %d entries, want %d: %+v", len(list.Items), len(want), list.Items)
		}
		for i, item := range list.Items {
			if item.ID != want[i] {
				t.Errorf("entry %d is %s, want %s", i, item.ID, want[i])
			}
		}
		listed := list.Items[1]
		if listed.CIDR != office.Item.CIDR || listed.Display != office.Item.Display ||
			listed.Label != office.Item.Label || !listed.CreatedAt.Equal(office.Item.CreatedAt) ||
			listed.CreatedByEmail == nil || office.Item.CreatedByEmail == nil ||
			*listed.CreatedByEmail != *office.Item.CreatedByEmail {
			t.Errorf("the listed entry %+v differs from the added %+v", listed, office.Item)
		}
	})

	t.Run("audited", func(t *testing.T) {
		suite := &auditSuite{h: h, owner: f.owner}
		rows := suite.only(t, audit.ActionAnalyticsExcludeIPAdd)
		if len(rows) != 4 {
			t.Fatalf("%d add rows, want 4", len(rows))
		}
		byID := map[string]auditRow{}
		for _, row := range rows {
			if row.EntityType != audit.EntityAnalyticsExclusion || row.EntityID == nil {
				t.Errorf("an add row reads %+v", row)
				continue
			}
			byID[*row.EntityID] = row
		}

		first := byID[keep.Item.ID]
		if first.EntityLabel != "198.18.139.87" ||
			rawString(t, first.Changes["cidr"].New) != "198.18.139.87/32" ||
			string(first.Changes["cidr"].Old) != "null" ||
			string(first.Changes["deleted_events"].New) != "0" {
			t.Errorf("the add without history was recorded as %+v", first)
		}
		if _, ok := first.Changes["label"]; ok {
			t.Error("an add without a label recorded one")
		}

		second := byID[office.Item.ID]
		if second.EntityLabel != "203.0.113.0/24" ||
			rawString(t, second.Changes["cidr"].New) != "203.0.113.0/24" ||
			rawString(t, second.Changes["label"].New) != "Ofis" ||
			string(second.Changes["label"].Old) != "null" ||
			string(second.Changes["deleted_events"].New) != "2" ||
			string(second.Changes["deleted_events"].Old) != "null" {
			t.Errorf("the add with history was recorded as %+v", second)
		}
		if second.UserEmail != "analytics-owner@example.test" {
			t.Errorf("the add was recorded for %q", second.UserEmail)
		}
	})
}

// The add, its history delete and its audit row are one transaction: when the
// audit row cannot be written, neither the entry nor the deletion survive.
func TestAnExclusionAddIsAllOrNothing(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	insertRawEvent(t, h, f.owner.businessID, f.menu.ID, textPtr("198.18.139.87"))

	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `ALTER TABLE audit_logs ADD CONSTRAINT refuse_exclusions
		CHECK (entity_type <> 'analytics_exclusion')`); err != nil {
		t.Fatal(err)
	}
	resp, payload := call(t, h, http.MethodPost, excludedIPsPath, f.owner,
		map[string]any{"cidr": "198.18.139.87", "delete_history": true}, nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("an add whose audit row failed answered %d: %s", resp.StatusCode, payload)
	}
	if n := countWhere(t, h, `ip = '198.18.139.87'`); n != 1 {
		t.Errorf("the failed add deleted the history anyway: %d rows left, want 1", n)
	}
	var entries int
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM analytics_excluded_ips`).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if entries != 0 {
		t.Errorf("the failed add left %d entries", entries)
	}
	if _, err := h.pool.Exec(ctx, `ALTER TABLE audit_logs DROP CONSTRAINT refuse_exclusions`); err != nil {
		t.Fatal(err)
	}
	// And the next event from the address is still stored: the failed add
	// did not leave a stale "excluded" answer in the cache either.
	f.trackFrom(t, "menu_view", "after-failed-add", fromEdge("198.18.139.87"))
	if n := storedOf(t, h, "after-failed-add"); n != 1 {
		t.Errorf("after a failed add the address's view stored %d rows, want 1", n)
	}
}

func TestExclusionInputIsValidated(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h

	refuse := func(t *testing.T, body any, status int, message string) {
		t.Helper()
		resp, payload := call(t, h, http.MethodPost, excludedIPsPath, f.owner, body, nil)
		if resp.StatusCode != status {
			t.Errorf("%v answered %d, want %d: %s", body, resp.StatusCode, status, payload)
			return
		}
		if message != "" && !strings.Contains(string(payload), message) {
			t.Errorf("%v answered %s, want a message containing %q", body, payload, message)
		}
	}

	t.Run("refused", func(t *testing.T) {
		refuse(t, map[string]any{"cidr": "198.18.139.87"}, http.StatusUnprocessableEntity, "delete_history")
		refuse(t, map[string]any{"cidr": "198.18.139.87", "delete_history": nil}, http.StatusUnprocessableEntity, "delete_history")
		refuse(t, map[string]any{"cidr": "198.18.139.87", "delete_history": "yes"}, http.StatusBadRequest, "")
		refuse(t, map[string]any{"cidr": 176, "delete_history": false}, http.StatusBadRequest, "")
		refuse(t, rawJSON(`{"cidr": `), http.StatusBadRequest, "")
		refuse(t, map[string]any{"delete_history": false}, http.StatusUnprocessableEntity, "girilmelidir")
		refuse(t, map[string]any{"cidr": "   ", "delete_history": false}, http.StatusUnprocessableEntity, "girilmelidir")
		for _, bad := range []string{"abc", "198.18.139", "198.18.139.87/33", "fe80::1%eth0", "198.18.139.87:80"} {
			refuse(t, map[string]any{"cidr": bad, "delete_history": false}, http.StatusUnprocessableEntity, "Geçerli bir IP")
		}
		for _, broad := range []string{"198.18.0.0/15", "0.0.0.0/0", "2001:db8::/47", "::/0"} {
			refuse(t, map[string]any{"cidr": broad, "delete_history": false}, http.StatusUnprocessableEntity, "çok geniş")
		}
		refuse(t, map[string]any{"cidr": "198.18.139.87", "label": strings.Repeat("a", 61), "delete_history": false},
			http.StatusUnprocessableEntity, "60")
		refuse(t, map[string]any{"cidr": "198.18.139.87", "label": "a\u0000b", "delete_history": false},
			http.StatusUnprocessableEntity, "geçersiz karakter")
		refuse(t, map[string]any{"cidr": "198.18.139.87", "label": "a\nb", "delete_history": false},
			http.StatusUnprocessableEntity, "geçersiz karakter")

		var entries int
		if err := h.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM analytics_excluded_ips`).Scan(&entries); err != nil {
			t.Fatal(err)
		}
		if entries != 0 {
			t.Errorf("refused requests left %d entries", entries)
		}
	})

	t.Run("normalised", func(t *testing.T) {
		for _, tc := range []struct{ raw, cidr, display string }{
			{"198.18.139.99/24", "198.18.139.0/24", "198.18.139.0/24"},
			{"::ffff:198.51.100.9", "198.51.100.9/32", "198.51.100.9"},
			{"2001:DB8:0:0::1", "2001:db8::1/128", "2001:db8::1"},
			{" 10.20.0.0/16 ", "10.20.0.0/16", "10.20.0.0/16"},
		} {
			item := exclude(t, h, f.owner, tc.raw)
			if item.CIDR != tc.cidr || item.Display != tc.display {
				t.Errorf("%q was stored as %s / %s, want %s / %s", tc.raw, item.CIDR, item.Display, tc.cidr, tc.display)
			}
		}
		// A label of exactly 60 characters — of two bytes each — fits.
		addExclusion(t, h, f.owner, map[string]any{"cidr": "198.51.100.200", "label": strings.Repeat("ş", 60), "delete_history": false})
	})

	t.Run("duplicates_and_covered_ranges", func(t *testing.T) {
		for _, dup := range []string{
			"198.51.100.9", "198.51.100.9/32", "::ffff:198.51.100.9", // listed, however written
			"198.18.139.0/24", "198.18.139.200", "198.18.139.128/25", // inside a listed /24
			"10.20.30.40", "10.20.30.0/24", // inside a listed /16
			"2001:db8::1",
		} {
			refuse(t, map[string]any{"cidr": dup, "delete_history": true}, http.StatusConflict, "Bu IP zaten listede.")
		}
	})

	t.Run("match_count", func(t *testing.T) {
		for _, tc := range []struct{ query, message string }{
			{"", "girilmelidir"}, {"?cidr=", "girilmelidir"}, {"?cidr=abc", "Geçerli bir IP"},
			{"?cidr=176.0.0.0%2F8", "çok geniş"}, {"?cidr=%00", "Geçerli bir IP"}, {"?cidr=%FF", "Geçerli bir IP"},
		} {
			resp, payload := call(t, h, http.MethodGet, excludedIPsPath+"/match-count"+tc.query, f.owner, nil, nil)
			if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(payload), tc.message) {
				t.Errorf("match-count%s answered %d %s, want 422 containing %q", tc.query, resp.StatusCode, payload, tc.message)
			}
		}
	})
}

// A business keeps at most ipexclude.MaxPerBusiness entries — and two adds
// racing for the last places cannot overshoot it.
func TestTheExclusionListIsCapped(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	ctx := context.Background()

	if _, err := h.pool.Exec(ctx, `
		INSERT INTO analytics_excluded_ips (business_id, cidr)
		SELECT $1::text::uuid, ('10.99.0.' || n)::cidr FROM generate_series(1, $2::int) AS n`,
		f.owner.businessID, ipexclude.MaxPerBusiness-1); err != nil {
		t.Fatal(err)
	}
	last := exclude(t, h, f.owner, "198.51.100.50")
	resp, payload := call(t, h, http.MethodPost, excludedIPsPath, f.owner,
		map[string]any{"cidr": "198.51.100.51", "delete_history": false}, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(payload), "En fazla 50") {
		t.Fatalf("the 51st entry answered %d: %s", resp.StatusCode, payload)
	}
	list, _ := listExclusions(t, h, f.owner, nil)
	if len(list.Items) != ipexclude.MaxPerBusiness || list.Max != ipexclude.MaxPerBusiness {
		t.Errorf("the full list has %d entries and max %d", len(list.Items), list.Max)
	}
	// The limit is per business.
	exclude(t, h, f.other, "198.51.100.51")

	// Room for three, six adds at once: exactly three get in.
	removeExclusion(t, h, f.owner, last.ID)
	if _, err := h.pool.Exec(ctx, `DELETE FROM analytics_excluded_ips
		WHERE business_id = $1::text::uuid AND cidr IN ('10.99.0.1', '10.99.0.2')`, f.owner.businessID); err != nil {
		t.Fatal(err)
	}
	var pending []*pendingRequest
	for i := 0; i < 6; i++ {
		pending = append(pending, h.startRequest(http.MethodPost, excludedIPsPath, f.owner.session,
			map[string]any{"cidr": fmt.Sprintf("198.51.100.%d", 60+i), "delete_history": false}))
	}
	created, refused := 0, 0
	for i, p := range pending {
		status, body := p.wait(t, fmt.Sprintf("concurrent add %d", i))
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusUnprocessableEntity:
			refused++
		default:
			t.Errorf("concurrent add %d answered %d: %s", i, status, body)
		}
	}
	var entries int
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM analytics_excluded_ips WHERE business_id = $1::text::uuid`,
		f.owner.businessID).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if created != 3 || refused != 3 || entries != ipexclude.MaxPerBusiness {
		t.Errorf("six racing adds with room for three: %d created, %d refused, %d entries — want 3, 3, %d",
			created, refused, entries, ipexclude.MaxPerBusiness)
	}
}

// ------------------------------------------------------ tenant scope, removal

func TestTheExclusionListIsTenantScoped(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h

	mine := addExclusion(t, h, f.owner, map[string]any{"cidr": "198.18.139.87", "label": "Kasa", "delete_history": false}).Item
	theirs := exclude(t, h, f.other, "198.51.100.7")

	list, payload := listExclusions(t, h, f.other, nil)
	if len(list.Items) != 1 || list.Items[0].ID != theirs.ID || strings.Contains(string(payload), "198.18.139.87") {
		t.Errorf("the other tenant's list reads %s", payload)
	}

	if resp, payload := call(t, h, http.MethodDelete, excludedIPsPath+"/"+mine.ID, f.other, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("another tenant's DELETE of the owner's entry answered %d: %s", resp.StatusCode, payload)
	}
	if list, _ := listExclusions(t, h, f.owner, nil); len(list.Items) != 1 || list.Items[0].ID != mine.ID {
		t.Errorf("after another tenant's DELETE the owner's list reads %+v", list.Items)
	}

	// Another tenant's entry excludes nothing on this tenant's menu.
	f.trackFrom(t, "menu_view", "their-address-my-menu", fromEdge("198.51.100.7"))
	if n := storedOf(t, h, "their-address-my-menu"); n != 1 {
		t.Errorf("another tenant's entry kept a visit to this tenant's menu out: %d rows", n)
	}

	for _, tc := range []struct {
		path   string
		status int
	}{
		{excludedIPsPath + "/not-a-uuid", http.StatusBadRequest},
		{excludedIPsPath + "/" + uuid.NewString(), http.StatusNotFound},
	} {
		if resp, payload := call(t, h, http.MethodDelete, tc.path, f.owner, nil, nil); resp.StatusCode != tc.status {
			t.Errorf("DELETE %s answered %d, want %d: %s", tc.path, resp.StatusCode, tc.status, payload)
		}
	}

	// Without a session nothing is readable or writable.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, excludedIPsPath},
		{http.MethodGet, excludedIPsPath + "/match-count?cidr=198.18.139.87"},
		{http.MethodPost, excludedIPsPath},
		{http.MethodDelete, excludedIPsPath + "/" + mine.ID},
		{http.MethodPost, "/api/analytics/optout"},
		{http.MethodDelete, "/api/analytics/optout"},
	} {
		if resp, payload := call(t, h, tc.method, tc.path, nil,
			map[string]any{"cidr": "198.18.139.1", "delete_history": false}, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without a session answered %d: %s", tc.method, tc.path, resp.StatusCode, payload)
		}
	}

	removeExclusion(t, h, f.owner, mine.ID)
	if resp, _ := call(t, h, http.MethodDelete, excludedIPsPath+"/"+mine.ID, f.owner, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("a second DELETE answered %d, want 404", resp.StatusCode)
	}

	suite := &auditSuite{h: h, owner: f.owner}
	rows := suite.only(t, audit.ActionAnalyticsExcludeIPRemove)
	if len(rows) != 1 {
		t.Fatalf("%d remove rows, want 1 — a refused DELETE records nothing", len(rows))
	}
	row := rows[0]
	if row.EntityType != audit.EntityAnalyticsExclusion || row.EntityID == nil || *row.EntityID != mine.ID ||
		row.EntityLabel != "198.18.139.87" ||
		rawString(t, row.Changes["cidr"].Old) != "198.18.139.87/32" || string(row.Changes["cidr"].New) != "null" ||
		rawString(t, row.Changes["label"].Old) != "Kasa" || string(row.Changes["label"].New) != "null" {
		t.Errorf("the removal was recorded as %+v", row)
	}
	// The other tenant's trail holds its own add and nothing of the owner's.
	otherSuite := &auditSuite{h: h, owner: f.other}
	if rows := otherSuite.logs(t, "?limit=200&entity_type="+audit.EntityAnalyticsExclusion).Items; len(rows) != 1 ||
		rows[0].EntityID == nil || *rows[0].EntityID != theirs.ID {
		t.Errorf("the other tenant's exclusion trail reads %+v", rows)
	}
}

// ------------------------------------------------------- what the panel sees

func TestTheExclusionListReportsTheCaller(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h

	list, payload := listExclusions(t, h, f.owner, fromEdge("198.51.100.44"))
	if !strings.Contains(string(payload), `"items":[]`) {
		t.Errorf("an empty list is not an empty array: %s", payload)
	}
	if list.CurrentIP == nil || *list.CurrentIP != "198.51.100.44" || list.CurrentIPSource != "edge" ||
		list.CurrentIPExcluded || list.OptOut || list.Max != ipexclude.MaxPerBusiness {
		t.Errorf("the caller was reported as %s", payload)
	}

	exclude(t, h, f.owner, "198.51.100.0/24")
	if list, payload := listExclusions(t, h, f.owner, fromEdge("198.51.100.44")); !list.CurrentIPExcluded {
		t.Errorf("an address inside a listed range was not reported excluded: %s", payload)
	}
	if list, payload := listExclusions(t, h, f.owner, fromCloudflare("2001:db8:5::1", "40002")); list.CurrentIP == nil ||
		*list.CurrentIP != "2001:db8:5::1" || list.CurrentIPSource != "cloudflare" || list.CurrentIPExcluded {
		t.Errorf("a proven IPv6 caller was reported as %s", payload)
	}
	if list, payload := listExclusions(t, h, f.owner, nil, optOutCookie); !list.OptOut {
		t.Errorf("a browser with the mark was reported as %s", payload)
	}
	if list, payload := listExclusions(t, h, f.owner, nil); list.CurrentIP != nil || list.CurrentIPSource != "unknown" ||
		!strings.Contains(string(payload), `"current_ip":null`) {
		t.Errorf("a caller with no established address was reported as %s", payload)
	}
}

// The opt-out mark is written for every tenant host in production and for
// the panel's host alone in development; it is readable by script, lasts a
// year, and is cleared with exactly the attributes it was written with.
func TestTheOptOutCookie(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	suite := &auditSuite{h: h, owner: f.owner}
	trailBefore := suite.logs(t, "").Total

	optOutOf := func(t *testing.T, method string) (*http.Cookie, string) {
		t.Helper()
		resp, payload := call(t, h, method, "/api/analytics/optout", f.owner, nil, nil)
		if resp.StatusCode != http.StatusNoContent || len(payload) != 0 {
			t.Fatalf("%s /api/analytics/optout answered %d: %s", method, resp.StatusCode, payload)
		}
		var found *http.Cookie
		var raw string
		for _, cookie := range resp.Cookies() {
			if cookie.Name == middleware.OptOutCookieName {
				if found != nil {
					t.Fatalf("%s set the cookie twice", method)
				}
				found = cookie
			}
		}
		for _, line := range resp.Header.Values("Set-Cookie") {
			if strings.HasPrefix(line, middleware.OptOutCookieName+"=") {
				raw = strings.ToLower(line)
			}
		}
		if found == nil {
			t.Fatalf("%s set no %s cookie", method, middleware.OptOutCookieName)
		}
		return found, raw
	}

	check := func(t *testing.T, domain string, secure bool) {
		t.Helper()
		set, raw := optOutOf(t, http.MethodPost)
		if set.Value != "1" || set.Path != "/" || set.Domain != domain || set.MaxAge != 31536000 ||
			set.SameSite != http.SameSiteLaxMode || set.HttpOnly || set.Secure != secure {
			t.Errorf("the mark was set as %q, want value 1, path /, domain %q, max-age 31536000, Lax, not HttpOnly, secure=%v",
				raw, domain, secure)
		}
		if domain == "" && strings.Contains(raw, "domain=") {
			t.Errorf("a host-only mark carries a Domain: %q", raw)
		}

		cleared, raw := optOutOf(t, http.MethodDelete)
		if cleared.Value != "" || cleared.Path != "/" || cleared.Domain != domain ||
			cleared.SameSite != http.SameSiteLaxMode || cleared.HttpOnly || cleared.Secure != secure ||
			cleared.Expires.IsZero() || !cleared.Expires.Before(time.Now()) {
			t.Errorf("the mark was cleared as %q, want it expired with the attributes it was set with", raw)
		}
	}

	t.Run("development", func(t *testing.T) { check(t, "", false) })

	t.Run("production", func(t *testing.T) {
		h.cfg.Env, h.cfg.CookieSecure = "production", true
		t.Cleanup(func() { h.cfg.Env, h.cfg.CookieSecure = "development", false })
		check(t, "karecik.com", true)

		// Secure follows the session cookie's setting, not the environment
		// alone: a deployment that turned COOKIE_SECURE off gets neither.
		h.cfg.CookieSecure = false
		check(t, "karecik.com", false)
	})

	if total := suite.logs(t, "").Total; total != trailBefore {
		t.Errorf("the opt-out endpoints wrote %d audit rows, want none", total-trailBefore)
	}
}

// ------------------------------------------------------------- migration 015

// Migration 015 can run again over itself, leaves exactly one CHECK per
// audit column, and its reverse script — run by hand in real life — undoes it
// and can itself run twice.
func TestTheExclusionMigrationIsIdempotentAndReversible(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.register("owner", "Migration Kafe", "migration@example.test")

	up, err := migrations.FS.ReadFile("015_analytics_excluded_ips.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../migrations/down/015_analytics_excluded_ips.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	checksOf := func() map[string]int {
		rows, err := h.pool.Query(ctx, `
			SELECT a.attname
			FROM pg_constraint c
			JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
			WHERE c.conrelid = 'audit_logs'::regclass AND c.contype = 'c'
			  AND a.attname IN ('action', 'entity_type')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		counts := map[string]int{}
		for rows.Next() {
			var column string
			if err := rows.Scan(&column); err != nil {
				t.Fatal(err)
			}
			counts[column]++
		}
		return counts
	}
	writeExclusionAudit := func() error {
		return repository.InsertAuditLog(ctx, h.pool, audit.Entry{
			BusinessID: uuid.MustParse(owner.businessID),
			Action:     audit.ActionAnalyticsExcludeIPAdd, EntityType: audit.EntityAnalyticsExclusion,
		})
	}

	if _, err := h.pool.Exec(ctx, string(up)); err != nil {
		t.Fatalf("migration 015 could not run a second time: %v", err)
	}
	if counts := checksOf(); counts["action"] != 1 || counts["entity_type"] != 1 {
		t.Errorf("after a re-run audit_logs has CHECKs %v, want one per column", counts)
	}
	if err := writeExclusionAudit(); err != nil {
		t.Errorf("the extended vocabulary was refused after a re-run: %v", err)
	}
	exclude(t, h, owner, "198.18.139.87")

	for run := 1; run <= 2; run++ {
		if _, err := h.pool.Exec(ctx, string(down)); err != nil {
			t.Fatalf("the reverse script failed on run %d: %v", run, err)
		}
	}
	var table, function *string
	if err := h.pool.QueryRow(ctx, `SELECT to_regclass('analytics_excluded_ips')::text,
		to_regproc('karecik_try_inet')::text`).Scan(&table, &function); err != nil {
		t.Fatal(err)
	}
	if table != nil || function != nil {
		t.Errorf("after the reverse script the table is %v and the function %v", table, function)
	}
	if err := writeExclusionAudit(); err == nil {
		t.Error("after the reverse script the audit vocabulary still admits the exclusion actions")
	}
	if counts := checksOf(); counts["action"] != 1 || counts["entity_type"] != 1 {
		t.Errorf("after the reverse script audit_logs has CHECKs %v, want one per column", counts)
	}

	if _, err := h.pool.Exec(ctx, string(up)); err != nil {
		t.Fatalf("migration 015 could not run again after its reverse: %v", err)
	}
	if err := writeExclusionAudit(); err != nil {
		t.Errorf("the vocabulary was refused after re-applying 015: %v", err)
	}
}

// karecik_try_inet reads exactly the bare addresses and nothing else, under
// both of the bodies migration 015 chooses between; and on PostgreSQL 16+ the
// planner inlines it into the range condition, so the match count and the
// history delete never pay a function call — let alone a subtransaction — per
// stored event. On a tenant at the cap for 90 days (~900k rows) the PL/pgSQL
// guard alone took seconds per count; inlined, the same count is a tenth of a
// second. The plan, not a stopwatch, is what this test holds it to.
func TestTryInetReadsOnlyBareAddressesAndStaysInlined(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// The body for servers older than 16 is exercised here too, whatever this
	// server's version: it is taken from the migration itself — the second of
	// the two statements the DO block EXECUTEs — and installed under another
	// name. A new layout of the file fails here rather than going untested.
	up, err := migrations.FS.ReadFile("015_analytics_excluded_ips.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(up), "$create$")
	if len(parts) != 5 {
		t.Fatalf("migration 015 has %d $create$ markers, want the 4 around its two function bodies", len(parts)-1)
	}
	modern, legacy := parts[1], parts[3]
	if !strings.Contains(modern, "pg_input_is_valid") || !strings.Contains(legacy, "LANGUAGE plpgsql") {
		t.Fatalf("the two bodies of karecik_try_inet are not where this test expects them:\n%s\n---\n%s", modern, legacy)
	}
	legacy = strings.Replace(legacy, "FUNCTION karecik_try_inet(", "FUNCTION karecik_try_inet_legacy(", 1)
	if _, err := h.pool.Exec(ctx, legacy); err != nil {
		t.Fatalf("the pre-16 body of karecik_try_inet does not install: %v", err)
	}

	// value -> host() of the address, or nil for "not an address".
	want := map[string]*string{
		"198.18.139.87":      textPtr("198.18.139.87"),
		"0.0.0.0":            textPtr("0.0.0.0"),
		"255.255.255.255":    textPtr("255.255.255.255"),
		"01.2.3.4":           textPtr("1.2.3.4"), // inet reads it; the IPv4 fast path leaves it to the guard
		"2001:db8::1":        textPtr("2001:db8::1"),
		"2001:DB8::1":        textPtr("2001:db8::1"),
		"::ffff:203.0.113.7": textPtr("::ffff:203.0.113.7"),
		"::":                 textPtr("::"),
		"unknown":            nil,
		"-":                  nil,
		"":                   nil,
		" 1.2.3.4":           nil,
		"1.2.3.4 ":           nil,
		"1.2.3.999":          nil,
		"256.1.1.1":          nil,
		"1.2.3":              nil,
		"1.2.3.4.5":          nil,
		"0x01020304":         nil,
		":::":                nil,
		"garbage::zz":        nil,
		"fe80::1%eth0":       nil,
		// A range is not an address a visit came from.
		"10.1.0.0/16":    nil,
		"203.0.113.7/32": nil,
	}
	for _, function := range []string{"karecik_try_inet", "karecik_try_inet_legacy"} {
		for value, expected := range want {
			var got *string
			if err := h.pool.QueryRow(ctx, `SELECT host(`+function+`($1::text))`, value).Scan(&got); err != nil {
				t.Errorf("%s(%q) failed instead of answering: %v", function, value, err)
				continue
			}
			if (got == nil) != (expected == nil) || (got != nil && *got != *expected) {
				t.Errorf("%s(%q) = %v, want %v", function, value, deref(got), deref(expected))
			}
		}
		var null *string
		if err := h.pool.QueryRow(ctx, `SELECT host(`+function+`(NULL::text))`).Scan(&null); err != nil || null != nil {
			t.Errorf("%s(NULL) = %v (err %v), want NULL", function, deref(null), err)
		}
	}

	var version int
	if err := h.pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version < 160000 {
		// Not a skip: the semantics above were checked; only the inlining is
		// a 16+ property.
		t.Logf("PostgreSQL %d has no pg_input_is_valid; the installed body is the PL/pgSQL one, not inlined", version)
		return
	}
	// The statements of repository.CountEventsInRange and AddExcludedIP's
	// history delete, with constants for their parameters.
	condition := `business_id = '00000000-0000-0000-0000-000000000000'::uuid
		AND karecik_try_inet(ip) <<= '203.0.113.0/24'::cidr`
	for _, statement := range []string{
		`SELECT COUNT(*) FROM menu_events WHERE ` + condition,
		`DELETE FROM menu_events WHERE ` + condition,
	} {
		plan := explainText(t, h, statement)
		if strings.Contains(plan, "karecik_try_inet") || !strings.Contains(plan, "pg_input_is_valid") {
			t.Errorf("karecik_try_inet is not inlined into\n  %s\nplan:\n%s", statement, plan)
		}
	}
}

// explainText is the EXPLAIN (VERBOSE) of a statement, one line per plan row.
// EXPLAIN plans a DELETE without running it.
func explainText(t *testing.T, h *harness, statement string) string {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `EXPLAIN (VERBOSE, COSTS OFF) `+statement)
	if err != nil {
		t.Fatalf("EXPLAIN %s: %v", statement, err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

// deref prints an optional string for a failure message.
func deref(s *string) string {
	if s == nil {
		return "NULL"
	}
	return fmt.Sprintf("%q", *s)
}
