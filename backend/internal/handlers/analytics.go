package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// Visitor analytics: the public endpoint the customer menu reports views to,
// and the two dashboard reads over what it stored.
//
// The public half is the one exposed to the whole internet, so it is written
// to refuse everything it does not fully understand, cheaply and without ever
// answering 500 to garbage: the body is capped, decoded strictly, every field
// is checked against a fixed shape, and the menu, category and product are
// proven to belong together before a row is written. What it stores about the
// visitor — the address and source port, how the address was established, a
// truncated user agent — comes from the same resolver the request log line
// uses (middleware.ClientAddrOf), so the dashboard's drill-down and the
// platform log agree on who a visitor was. Those are personal data; see the
// retention sweep in cmd/api.
//
// What bounds the table is layered: the per-client limiter in router.Setup
// bounds the rate, and an event that passes every check is still not stored
// when it repeats one the same visitor sent moments ago or when its business
// has reached ANALYTICS_DAILY_EVENT_CAP for the day (package eventgate).

// MaxEventBodyBytes caps the body of one event. A real event is well under
// 400 bytes; the cap is what stops the endpoint from parsing a megabyte of
// JSON for somebody who is only trying to cost us CPU.
const MaxEventBodyBytes = 2048

// maxUserAgentRunes bounds the stored user agent. Real ones stay under 300;
// the header itself is client input of up to the edge's header limit.
const maxUserAgentRunes = 300

// maxEventSlugBytes bounds a slug in an event. Real slugs are at most 60; the
// bound keeps a junk value from reaching the database at all.
const maxEventSlugBytes = 100

// visitorIDPattern is the shape of the id the customer page keeps in the
// browser; the menu_events CHECK accepts exactly the same.
var visitorIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// eventRequest is the body of POST /api/public/events. Every field is a
// string: a number or an object where a string belongs fails the decode, and
// null reads as "" — absent.
type eventRequest struct {
	BusinessSlug string `json:"business_slug"`
	MenuSlug     string `json:"menu_slug"`
	Type         string `json:"type"`
	CategoryID   string `json:"category_id"`
	ProductID    string `json:"product_id"`
	VisitorID    string `json:"visitor_id"`
	Language     string `json:"language"`
}

// TrackEvent — POST /api/public/events  (public, rate limited per client)
//
// The body is JSON whatever the Content-Type says: navigator.sendBeacon, which
// the menu uses so that a view is reported even while the page is closing,
// sends text/plain and cannot be told otherwise. Success is 204 with no body —
// a beacon never reads it.
func (h *Handler) TrackEvent(c *fiber.Ctx) error {
	body := c.Body()
	if len(body) > MaxEventBodyBytes {
		return utils.TooLarge(c, "İstek gövdesi çok büyük.")
	}

	var req eventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	req.Type = strings.TrimSpace(req.Type)
	if !repository.IsEventType(req.Type) {
		return utils.Unprocessable(c, "Geçersiz olay türü.")
	}

	businessSlug := strings.TrimSpace(req.BusinessSlug)
	menuSlug := strings.TrimSpace(req.MenuSlug)
	if businessSlug == "" || menuSlug == "" ||
		len(businessSlug) > maxEventSlugBytes || len(menuSlug) > maxEventSlugBytes {
		return utils.Unprocessable(c, "Menü bulunamadı.")
	}

	categoryID, ok := optionalUUID(req.CategoryID)
	if !ok {
		return utils.Unprocessable(c, "Geçersiz kategori kimliği.")
	}
	productID, ok := optionalUUID(req.ProductID)
	if !ok {
		return utils.Unprocessable(c, "Geçersiz ürün kimliği.")
	}
	switch {
	case req.Type == repository.EventCategoryView && categoryID == nil:
		return utils.Unprocessable(c, "Kategori görüntüleme olayı bir kategori kimliği içermelidir.")
	case req.Type == repository.EventProductView && productID == nil:
		return utils.Unprocessable(c, "Ürün görüntüleme olayı bir ürün kimliği içermelidir.")
	}

	var visitorID *string
	if id := strings.TrimSpace(req.VisitorID); id != "" {
		if !visitorIDPattern.MatchString(id) {
			return utils.Unprocessable(c, "Geçersiz ziyaretçi kimliği.")
		}
		visitorID = &id
	}

	var language *string
	if code := strings.ToLower(strings.TrimSpace(req.Language)); code != "" {
		if !utils.IsValidLanguage(code) {
			return utils.Unprocessable(c, "Desteklenmeyen dil kodu.")
		}
		language = &code
	}

	businessID, menuID, err := repository.ResolvePublishedMenu(c.Context(), h.DB, businessSlug, menuSlug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.Unprocessable(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	addr := middleware.ClientAddrOf(c)
	userAgent := CleanUserAgent(c.Get(fiber.HeaderUserAgent))
	idText := ""
	if visitorID != nil {
		idText = *visitorID
	}
	visitorKey := VisitorKey(idText, addr.IP, userAgent)

	// A repeat of an event this visitor sent moments ago, and any event past
	// the business's daily cap, are answered exactly like a stored one — the
	// page has nothing to do differently — and not stored (package eventgate).
	// Both are decided only now, for an event that has passed every check, so
	// a refused event is never mistaken for a stored one; and each is handed
	// back when the insert below fails after all.
	repeatKey := RepeatKey(visitorKey, req.Type, menuID, categoryID, productID)
	if !h.Events.Claim(repeatKey) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	admitted, firstRefusal, err := h.Events.Admit(c.Context(), businessID, h.storedEventsSince)
	if err != nil {
		h.Events.Release(repeatKey)
		return utils.Internal(c, err)
	}
	if !admitted {
		if firstRefusal {
			log.Printf("[karecik] analytics: business %s reached ANALYTICS_DAILY_EVENT_CAP (%d); "+
				"its further events today are accepted but not stored", businessID, h.Events.DailyCap())
		}
		return c.SendStatus(fiber.StatusNoContent)
	}

	err = repository.InsertMenuEvent(c.Context(), h.DB, repository.MenuEvent{
		BusinessID: businessID,
		MenuID:     menuID,
		Type:       req.Type,
		CategoryID: categoryID,
		ProductID:  productID,
		VisitorID:  visitorID,
		VisitorKey: visitorKey,
		IP:         addr.IP,
		Port:       addr.Port,
		IPSource:   string(addr.Source),
		UserAgent:  userAgent,
		Language:   language,
	})
	if err != nil {
		h.Events.Release(repeatKey)
		h.Events.Return(businessID)
		// A category or product of another menu, or the menu deleted between
		// the lookup and the insert: either way the event names something this
		// menu does not have, and nothing was stored.
		if errors.Is(err, repository.ErrEventTarget) || repository.IsForeignKeyViolation(err) {
			return utils.Unprocessable(c, "Kategori veya ürün bu menüye ait değil.")
		}
		return utils.Internal(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// storedEventsSince is the eventgate.Counter of the daily cap: what the
// business already stored today, asked once per business per day.
func (h *Handler) storedEventsSince(ctx context.Context, businessID uuid.UUID, since time.Time) (int, error) {
	return repository.CountEventsSince(ctx, h.DB, businessID, since)
}

// RepeatKey identifies an event for the repeat check: the visitor, the type,
// and every id the event names. Two events with the same key describe the same
// look at the same thing.
func RepeatKey(visitorKey, eventType string, menuID uuid.UUID, categoryID, productID *uuid.UUID) string {
	idOf := func(id *uuid.UUID) string {
		if id == nil {
			return "-"
		}
		return id.String()
	}
	return strings.Join([]string{visitorKey, eventType, menuID.String(), idOf(categoryID), idOf(productID)}, "|")
}

// optionalUUID reads an optional id: "" is absent, anything else has to parse.
func optionalUUID(raw string) (*uuid.UUID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return nil, false
	}
	return &id, true
}

// VisitorKey is what unique-visitor counts group on. A browser that sent its
// own visitor id is that id; one that did not is a hash of its address and user
// agent — coarser (two phones on one café Wi-Fi with the same browser count as
// one) but the best available, and never the address itself, so the key column
// adds no personal data the ip column does not already hold.
func VisitorKey(visitorID, ip, userAgent string) string {
	if visitorID != "" {
		return "v:" + visitorID
	}
	sum := sha256.Sum256([]byte(ip + "\n" + userAgent))
	return "h:" + hex.EncodeToString(sum[:16])
}

// CleanUserAgent makes a User-Agent header storable: invalid UTF-8 and control
// characters removed, trimmed, and cut to maxUserAgentRunes.
func CleanUserAgent(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(raw, ""))
	cleaned = strings.TrimSpace(cleaned)
	if runes := []rune(cleaned); len(runes) > maxUserAgentRunes {
		cleaned = string(runes[:maxUserAgentRunes])
	}
	return cleaned
}

// -------------------------------------------------------------- dashboard

// maxAnalyticsDays bounds the summary window. A year of daily rows is what a
// chart can still show; beyond it the request is a mistake, not a report.
const maxAnalyticsDays = 366

// defaultAnalyticsDays is the window when the request names none: the last
// 30 days, today included.
const defaultAnalyticsDays = 30

// AnalyticsSummary — GET /api/analytics/summary?menu_id=&from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Days are Europe/Istanbul calendar days and both ends are inclusive. With no
// dates the window is the last 30 days; with only one, the other is filled in
// around it the same way. A menu_id of another tenant is a 404, never a
// summary of nothing.
func (h *Handler) AnalyticsSummary(c *fiber.Ctx) error {
	businessID := middleware.BusinessID(c)

	menuID, ok, err := h.analyticsMenu(c, businessID)
	if !ok {
		return err
	}

	window, message := summaryWindow(c.Query("from"), c.Query("to"), time.Now())
	if message != "" {
		return utils.Unprocessable(c, message)
	}
	window.MenuID = menuID

	summary, err := repository.SummarizeEvents(c.Context(), h.DB, businessID, window)
	if err != nil {
		return utils.Internal(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return utils.OK(c, summary)
}

// AnalyticsEvents — GET /api/analytics/events?menu_id=&type=&from=&to=&ip=&limit=50&offset=0
//
// The drill-down: one row per event, newest first, with the visitor's address,
// source port and how the address was established. Every filter is optional;
// ip matches a prefix ("85.105." finds the whole block).
func (h *Handler) AnalyticsEvents(c *fiber.Ctx) error {
	businessID := middleware.BusinessID(c)

	menuID, ok, err := h.analyticsMenu(c, businessID)
	if !ok {
		return err
	}

	eventType := strings.TrimSpace(c.Query("type"))
	if eventType != "" && !repository.IsEventType(eventType) {
		return utils.Unprocessable(c, "Geçersiz olay türü.")
	}

	ipPrefix := strings.TrimSpace(c.Query("ip"))
	if !ipFilterPattern.MatchString(ipPrefix) {
		return utils.Unprocessable(c, "IP filtresi yalnızca rakam, a-f harfleri, nokta ve iki nokta içerebilir.")
	}

	window, message := eventWindow(c.Query("from"), c.Query("to"))
	if message != "" {
		return utils.Unprocessable(c, message)
	}
	window.MenuID = menuID

	limit, offset, ok := pageOf(c)
	if !ok {
		return utils.Unprocessable(c, "Sayfalama değerleri geçersiz.")
	}

	items, total, err := repository.ListEvents(c.Context(), h.DB, businessID, repository.EventFilter{
		Window:   window,
		Type:     eventType,
		IPPrefix: ipPrefix,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return utils.Internal(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return utils.OK(c, fiber.Map{"items": items, "total": total, "limit": limit, "offset": offset})
}

// ipFilterPattern is what an ip filter may hold: the alphabet of IPv4 and IPv6
// addresses, nothing else — which is also what keeps it free of LIKE
// wildcards. Empty means no filter.
var ipFilterPattern = regexp.MustCompile(`^[0-9A-Fa-f:.]{0,45}$`)

// analyticsMenu reads the optional menu_id filter and proves the menu belongs
// to the business. A refused request is reported through the false ok after
// the response has been written.
func (h *Handler) analyticsMenu(c *fiber.Ctx, businessID uuid.UUID) (*uuid.UUID, bool, error) {
	raw := strings.TrimSpace(c.Query("menu_id"))
	if raw == "" {
		return nil, true, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, false, utils.BadRequest(c, "Geçersiz menü kimliği.")
	}
	if _, err := repository.GetMenu(c.Context(), h.DB, id, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, false, utils.NotFound(c, "Menü bulunamadı.")
		}
		return nil, false, utils.Internal(c, err)
	}
	return &id, true, nil
}

// The years a report may name. The layout already stops at 9999; the lower
// bound is what matters. Go reads "0000" as a year, but PostgreSQL's calendar
// has no year 0, so such a day reached the daily series' ::date casts and
// failed there as a 500. No event predates this code, so a year before 2000
// is a typo either way, and it gets the same 422 as any other bad date.
const (
	minReportYear = 2000
	maxReportYear = 9999
)

// parseDay reads a YYYY-MM-DD day as midnight in Istanbul.
func parseDay(raw string) (time.Time, bool) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), utils.Istanbul)
	if err != nil || day.Year() < minReportYear || day.Year() > maxReportYear {
		return time.Time{}, false
	}
	return day, true
}

// dayRange turns two inclusive calendar days into the instants that bound
// them: from's midnight and the midnight after to. AddDate works on the
// calendar, so a day is always the whole day in Istanbul.
func dayRange(from, to time.Time) repository.AnalyticsRange {
	return repository.AnalyticsRange{
		Start:    from,
		End:      to.AddDate(0, 0, 1),
		FromDay:  from.Format("2006-01-02"),
		ToDay:    to.Format("2006-01-02"),
		HasStart: true,
		HasEnd:   true,
	}
}

// SummaryWindow is summaryWindow, exported for tests with a fixed "now".
func SummaryWindow(fromRaw, toRaw string, now time.Time) (repository.AnalyticsRange, string) {
	return summaryWindow(fromRaw, toRaw, now)
}

// summaryWindow resolves the summary's from/to into a range, or a Turkish
// message when they cannot be used.
func summaryWindow(fromRaw, toRaw string, now time.Time) (repository.AnalyticsRange, string) {
	today := now.In(utils.Istanbul)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, utils.Istanbul)

	var from, to time.Time
	var ok bool
	switch {
	case strings.TrimSpace(fromRaw) == "" && strings.TrimSpace(toRaw) == "":
		to = today
		from = to.AddDate(0, 0, -(defaultAnalyticsDays - 1))
	case strings.TrimSpace(toRaw) == "":
		if from, ok = parseDay(fromRaw); !ok {
			return repository.AnalyticsRange{}, "Başlangıç tarihi YYYY-AA-GG biçiminde olmalıdır."
		}
		to = today
		if to.Before(from) {
			to = from
		}
	case strings.TrimSpace(fromRaw) == "":
		if to, ok = parseDay(toRaw); !ok {
			return repository.AnalyticsRange{}, "Bitiş tarihi YYYY-AA-GG biçiminde olmalıdır."
		}
		from = to.AddDate(0, 0, -(defaultAnalyticsDays - 1))
	default:
		if from, ok = parseDay(fromRaw); !ok {
			return repository.AnalyticsRange{}, "Başlangıç tarihi YYYY-AA-GG biçiminde olmalıdır."
		}
		if to, ok = parseDay(toRaw); !ok {
			return repository.AnalyticsRange{}, "Bitiş tarihi YYYY-AA-GG biçiminde olmalıdır."
		}
	}

	if to.Before(from) {
		return repository.AnalyticsRange{}, "Başlangıç tarihi bitiş tarihinden sonra olamaz."
	}
	if from.AddDate(0, 0, maxAnalyticsDays).Before(to.AddDate(0, 0, 1)) {
		return repository.AnalyticsRange{}, "Tarih aralığı en fazla 366 gün olabilir."
	}
	return dayRange(from, to), ""
}

// eventWindow resolves the event list's optional from/to. Either bound may be
// left out, and then that side is open.
func eventWindow(fromRaw, toRaw string) (repository.AnalyticsRange, string) {
	var window repository.AnalyticsRange
	if strings.TrimSpace(fromRaw) != "" {
		from, ok := parseDay(fromRaw)
		if !ok {
			return window, "Başlangıç tarihi YYYY-AA-GG biçiminde olmalıdır."
		}
		window.Start, window.HasStart = from, true
		window.FromDay = from.Format("2006-01-02")
	}
	if strings.TrimSpace(toRaw) != "" {
		to, ok := parseDay(toRaw)
		if !ok {
			return window, "Bitiş tarihi YYYY-AA-GG biçiminde olmalıdır."
		}
		window.End, window.HasEnd = to.AddDate(0, 0, 1), true
		window.ToDay = to.Format("2006-01-02")
	}
	if window.HasStart && window.HasEnd && !window.Start.Before(window.End) {
		return window, "Başlangıç tarihi bitiş tarihinden sonra olamaz."
	}
	return window, ""
}
