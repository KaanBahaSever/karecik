package tests

// What bounds menu_events: the public endpoint is open to the whole internet
// and writes a row per request, so the suite pins down every guard that keeps
// the table from growing without bound on a small storage quota —
//
//   - a repeat of the same view within seconds is answered like any other and
//     stored once;
//   - past ANALYTICS_DAILY_EVENT_CAP a business's events are answered the same
//     and not stored, and a restart does not reset the count;
//   - the rate limiter holds a proven visitor to one visitor's budget and a
//     shared, unproven key to a crowd's;
//   - the retention purge removes exactly what expired, slice after slice.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"karecik/backend/internal/clientip"
	"karecik/backend/internal/config"
	"karecik/backend/internal/eventgate"
	"karecik/backend/internal/mailer"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/router"
	"karecik/backend/internal/utils"
)

// settableClock is a clock a test moves by hand.
type settableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *settableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *settableClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// separateVisits gives the harness an event gate whose clock moves a minute
// every time it is read, so that no two events of a fixture are ever repeats of
// each other: a fixture that sends the same view twice means two visits.
func separateVisits(h *harness) {
	clk := &settableClock{now: time.Now()}
	h.handler.Events = eventgate.New(eventgate.Config{Now: func() time.Time {
		clk.Advance(time.Minute)
		return clk.Now()
	}})
}

// countWhere counts the stored events matching a condition on menu_events.
func countWhere(t *testing.T, h *harness, condition string, args ...any) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM menu_events WHERE `+condition, args...).Scan(&n); err != nil {
		t.Fatalf("could not count menu_events where %s: %v", condition, err)
	}
	return n
}

// The same view sent again within seconds — a double tap, a dialog opened
// twice — is answered like any other and stored once. Anything that differs,
// or the same view once the window has passed, is stored.
func TestRepeatedEventsAreStoredOnce(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	clk := &settableClock{now: time.Now()}
	h.handler.Events = eventgate.New(eventgate.Config{Now: clk.Now})

	view := f.event("product_view", map[string]any{"visitor_id": "tapper", "product_id": f.prodA.ID})
	for i := 0; i < 5; i++ {
		f.mustTrack(t, view, nil)
	}
	if n := countWhere(t, h, `visitor_id = 'tapper'`); n != 1 {
		t.Fatalf("five identical views within the window stored %d rows, want 1", n)
	}

	f.mustTrack(t, f.event("product_view", map[string]any{"visitor_id": "tapper", "product_id": f.prodB.ID}), nil)
	f.mustTrack(t, f.event("menu_view", map[string]any{"visitor_id": "tapper"}), nil)
	f.mustTrack(t, f.event("product_view", map[string]any{"visitor_id": "someone-else", "product_id": f.prodA.ID}), nil)
	if n := countWhere(t, h, `visitor_id IN ('tapper', 'someone-else')`); n != 4 {
		t.Errorf("another product, another type and another visitor stored %d rows in all, want 4", n)
	}

	// A visitor without an id is recognised by address and browser.
	anonymous := map[string]string{clientip.HeaderXRealIP: "203.0.113.71", "User-Agent": "Anon/7"}
	f.mustTrack(t, f.event("menu_view", nil), anonymous)
	f.mustTrack(t, f.event("menu_view", nil), anonymous)
	if n := countWhere(t, h, `ip = '203.0.113.71'`); n != 1 {
		t.Errorf("an anonymous visitor's repeated view stored %d rows, want 1", n)
	}

	// Once the window has passed it is a new look at the product.
	clk.Advance(eventgate.DefaultRepeatWindow)
	f.mustTrack(t, view, nil)
	if n := countWhere(t, h, `visitor_id = 'tapper' AND product_id = $1::text::uuid`, f.prodA.ID); n != 2 {
		t.Errorf("the same view after the window stored %d rows in all, want 2", n)
	}

	// A refused event is not remembered as a stored one: sent again at once, it
	// is refused again rather than waved through as a repeat.
	mismatch := f.event("product_view", map[string]any{"visitor_id": "tapper",
		"product_id": f.prodA.ID, "category_id": f.catB.ID})
	for i := 0; i < 2; i++ {
		if resp, payload := sendEvent(t, h, mismatch, "text/plain", nil); resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("attempt %d of an event naming a product outside its category answered %d: %s",
				i+1, resp.StatusCode, payload)
		}
	}
}

// Past ANALYTICS_DAILY_EVENT_CAP a business's events are answered with the
// same 204 and not stored, the operator hears about it once, other tenants are
// unaffected, and a restart does not hand out a fresh allowance.
func TestTheDailyEventCap(t *testing.T) {
	const eventCap = 3
	h := newHarnessConfigured(t, mailer.Disabled{}, func(cfg *config.Config) {
		cfg.AnalyticsDailyEventCap = eventCap
	})
	owner := h.register("owner", "Sinir Kafe", "cap-owner@example.test")
	other := h.register("other", "Komsu Kafe", "cap-other@example.test")
	ownerSlug, otherSlug := readBusinessSlug(t, h, owner), readBusinessSlug(t, h, other)
	menu, otherMenu := h.createMenu(owner, "Ana Menu"), h.createMenu(other, "Ana Menu")
	logged := captureStandardLog(t)

	track := func(t *testing.T, businessSlug, menuSlug, visitor string) {
		t.Helper()
		body := eventBody(map[string]any{"business_slug": businessSlug, "menu_slug": menuSlug,
			"type": "menu_view", "visitor_id": visitor})
		if resp, payload := sendEvent(t, h, body, "text/plain", nil); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("event of %s answered %d, want 204 whether or not it is stored: %s", visitor, resp.StatusCode, payload)
		}
	}
	stored := func(businessID string) int {
		return countWhere(t, h, `business_id = $1::text::uuid`, businessID)
	}

	for i := 1; i <= eventCap+4; i++ {
		track(t, ownerSlug, menu.Slug, fmt.Sprintf("visitor%d", i))
	}
	if n := stored(owner.businessID); n != eventCap {
		t.Errorf("%d events under a cap of %d stored %d rows", eventCap+4, eventCap, n)
	}
	if lines := logged.linesContaining("ANALYTICS_DAILY_EVENT_CAP"); len(lines) != 1 ||
		!strings.Contains(lines[0], owner.businessID) {
		t.Errorf("the cap was reported %d times (%q), want once, naming the business", len(lines), lines)
	}

	track(t, otherSlug, otherMenu.Slug, "neighbour")
	if n := stored(other.businessID); n != 1 {
		t.Errorf("another tenant stored %d events after the first reached its cap, want 1", n)
	}

	// A restarted process: a fresh gate learns today's count from the table.
	h.handler.Events = eventgate.New(eventgate.Config{DailyCap: eventCap, Zone: utils.Istanbul})
	track(t, ownerSlug, menu.Slug, "after-restart")
	if n := stored(owner.businessID); n != eventCap {
		t.Errorf("after a restart the capped business stored %d rows, want still %d", n, eventCap)
	}
}

// A request whose address is proven gets the per-visitor budget; anything
// else keeps the budget sized for a crowd behind one shared address.
func TestTheEventLimiterBudgets(t *testing.T) {
	h := newAnalyticsHarness(t)

	// Garbage bodies: the limiter counts requests before the handler reads
	// them, and garbage never touches the database.
	firstRefusal := func(headers map[string]string, limit int) int {
		t.Helper()
		for i := 1; i <= limit; i++ {
			resp, _ := sendEvent(t, h, "x", "text/plain", headers)
			if resp.StatusCode == http.StatusTooManyRequests {
				return i
			}
		}
		return 0
	}

	proven := fromCloudflare("198.51.100.90", "40000")
	if at := firstRefusal(proven, router.ProvenVisitorEventsPerMinute+1); at != router.ProvenVisitorEventsPerMinute+1 {
		t.Errorf("a proven visitor was refused at request %d, want exactly the %dth",
			at, router.ProvenVisitorEventsPerMinute+1)
	}
	if at := firstRefusal(fromCloudflare("198.51.100.91", "40001"), 1); at != 0 {
		t.Error("another proven visitor was refused after the first used up its budget")
	}
	// An unproven key is shared by many visitors, so it is not held to one
	// visitor's budget.
	shared := map[string]string{clientip.HeaderXRealIP: "192.0.2.90"}
	if at := firstRefusal(shared, router.ProvenVisitorEventsPerMinute+1); at != 0 {
		t.Errorf("a shared, unproven key was refused at request %d, inside its own budget of %d",
			at, router.SharedKeyEventsPerMinute)
	}
}

// PurgeMenuEvents removes exactly the events older than the cutoff, across
// more than one of its slices, and nothing when nothing has expired.
func TestThePurgeRemovesExactlyTheExpiredEvents(t *testing.T) {
	f := newAnalyticsFixture(t)
	h := f.h
	ctx := context.Background()

	// 25,000 events straight into the table: 24,000 expired ones — more than
	// two purge slices — and 1,000 recent ones that must survive.
	if _, err := h.pool.Exec(ctx, `
		INSERT INTO menu_events (business_id, menu_id, type, visitor_key, created_at)
		SELECT m.business_id, m.id, 'menu_view', 'h:purge' || g,
		       CASE WHEN g <= 24000 THEN now() - interval '100 days' - g * interval '1 second'
		            ELSE now() - g * interval '1 millisecond' END
		FROM menus m, generate_series(1, 25000) AS g
		WHERE m.id = $1::text::uuid`, f.menu.ID); err != nil {
		t.Fatalf("could not insert the purge fixture: %v", err)
	}
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	removed, err := repository.PurgeMenuEvents(ctx, h.pool, cutoff)
	if err != nil {
		t.Fatalf("PurgeMenuEvents failed: %v", err)
	}
	if removed != 24000 {
		t.Errorf("the purge removed %d events, want the 24000 expired ones", removed)
	}
	if n := countWhere(t, h, `visitor_key LIKE 'h:purge%'`); n != 1000 {
		t.Errorf("%d events survived the purge, want the 1000 recent ones", n)
	}
	if again, err := repository.PurgeMenuEvents(ctx, h.pool, cutoff); err != nil || again != 0 {
		t.Errorf("a second purge removed %d (%v), want 0", again, err)
	}
}
