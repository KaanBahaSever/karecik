package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
)

// Visitor analytics of the customer menu — the menu_events table of migration
// 013. The public page writes one row per menu, category or product view
// (InsertMenuEvent); the dashboard reads them back aggregated (SummarizeEvents)
// and row by row (ListEvents). Every read is scoped to one business by its
// first predicate, and every join repeats the business, so a menu or a record
// of another tenant can never contribute a row or a name.

// The event types, exactly the menu_events.type CHECK of migration 013.
const (
	EventMenuView     = "menu_view"
	EventCategoryView = "category_view"
	EventProductView  = "product_view"
)

// IsEventType reports whether a string is one of the three event types.
func IsEventType(value string) bool {
	return value == EventMenuView || value == EventCategoryView || value == EventProductView
}

// AnalyticsZone is the calendar the dashboard counts days in. It is the zone of
// utils.Istanbul, spelled as PostgreSQL's AT TIME ZONE reads it; the handler
// builds the range boundaries in the same zone, so a day is the same 24 hours
// in the WHERE clause and in the GROUP BY.
const AnalyticsZone = "Europe/Istanbul"

// MenuEvent is one row to insert.
type MenuEvent struct {
	BusinessID uuid.UUID
	MenuID     uuid.UUID
	Type       string
	CategoryID *uuid.UUID
	ProductID  *uuid.UUID
	VisitorID  *string
	VisitorKey string
	IP         string // the resolver's string; "-" and "" are stored as NULL
	Port       string // likewise
	IPSource   string
	UserAgent  string
	Language   *string
}

// ErrEventTarget is InsertMenuEvent's answer to a category or product that is
// not part of the event's menu. Nothing was stored.
var ErrEventTarget = fmt.Errorf("the category or product is not part of this menu")

// ResolvePublishedMenu turns the two slugs of a customer address into the ids
// of a PUBLISHED menu and its business, in one query — the lookup the public
// events endpoint makes on every event. A tenant that does not exist, a menu it
// does not have and a menu it has not published are all ErrNotFound.
func ResolvePublishedMenu(ctx context.Context, db DB, businessSlug, menuSlug string) (uuid.UUID, uuid.UUID, error) {
	if UnstorableText(businessSlug) || UnstorableText(menuSlug) {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	var businessID, menuID uuid.UUID
	err := db.QueryRow(ctx, `
		SELECT b.id, m.id
		FROM businesses b
		JOIN menus m ON m.business_id = b.id
		WHERE b.slug = lower($1) AND m.slug = lower($2) AND m.is_active = true`,
		strings.TrimSpace(businessSlug), strings.TrimSpace(menuSlug)).Scan(&businessID, &menuID)
	if err != nil {
		if isNoRows(err) {
			return uuid.Nil, uuid.Nil, ErrNotFound
		}
		return uuid.Nil, uuid.Nil, err
	}
	return businessID, menuID, nil
}

// InsertMenuEvent stores one event, and proves in the same statement that its
// category and product belong to its menu. It returns ErrEventTarget, having
// stored nothing, when they do not:
//
//   - a category_id has to be a category of this business on this menu;
//   - a product_id has to be a product of this business in a category on this
//     menu — and in the given category, when both are sent;
//   - an event with a product but no category is stored with the product's
//     category filled in, so the event list can name it.
//
// One INSERT ... SELECT rather than a lookup and a write: the check and the
// row cannot drift apart, and a busy menu costs one round trip per view. Every
// parameter carries an explicit cast, because a parameter that appears only in
// a SELECT list has no type PostgreSQL could infer from it.
func InsertMenuEvent(ctx context.Context, db DB, event MenuEvent) error {
	ip, port, source := AddrColumns(event.IP, event.Port, event.IPSource)
	tag, err := db.Exec(ctx, `
		INSERT INTO menu_events (business_id, menu_id, type, category_id, product_id,
		                         visitor_id, visitor_key, ip, port, ip_source,
		                         user_agent, language)
		SELECT $1::uuid, $2::uuid, $3::text, target.category_id, $5::uuid, $6::text, $7::text,
		       $8::text, $9::int, $10::text, $11::text, $12::text
		FROM (
			SELECT CASE
				WHEN $5::uuid IS NOT NULL THEN (
					SELECT p.category_id
					FROM products p
					JOIN categories c ON c.id = p.category_id AND c.business_id = $1
					WHERE p.id = $5::uuid AND p.business_id = $1 AND c.menu_id = $2
					  AND ($4::uuid IS NULL OR p.category_id = $4::uuid))
				WHEN $4::uuid IS NOT NULL THEN (
					SELECT c.id FROM categories c
					WHERE c.id = $4::uuid AND c.business_id = $1 AND c.menu_id = $2)
			END AS category_id
		) AS target
		WHERE ($4::uuid IS NULL AND $5::uuid IS NULL) OR target.category_id IS NOT NULL`,
		event.BusinessID, event.MenuID, event.Type, event.CategoryID, event.ProductID,
		event.VisitorID, event.VisitorKey, ip, port, source, event.UserAgent, event.Language)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEventTarget
	}
	return nil
}

// CountEventsSince counts the events a business has stored since the given
// instant — how the daily cap learns, once per business per day, what an
// earlier run of the process already stored (handlers.TrackEvent). It reads
// menu_events_business_created_idx, and the cap bounds how many entries it can
// find.
func CountEventsSince(ctx context.Context, db DB, businessID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM menu_events WHERE business_id = $1 AND created_at >= $2`,
		businessID, since).Scan(&n)
	return n, err
}

// AnalyticsRange is the window of a summary or an event list: [Start, End) as
// instants, and the calendar days they span for the daily series. MenuID
// narrows it to one menu, already proven to belong to the business.
type AnalyticsRange struct {
	Start    time.Time
	End      time.Time
	FromDay  string // YYYY-MM-DD, inclusive
	ToDay    string // YYYY-MM-DD, inclusive
	MenuID   *uuid.UUID
	HasStart bool // false: no lower bound (event list only)
	HasEnd   bool // false: no upper bound (event list only)
}

// DailyStats is one day of the summary's series.
type DailyStats struct {
	Date           string `json:"date"`
	Visits         int    `json:"visits"`
	UniqueVisitors int    `json:"unique_visitors"`
	CategoryViews  int    `json:"category_views"`
	ProductViews   int    `json:"product_views"`
}

// TopCategory is one entry of the summary's most-viewed categories.
type TopCategory struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Views int       `json:"views"`
}

// TopProduct is one entry of the summary's most-opened products.
type TopProduct struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	CategoryName string    `json:"category_name"`
	Views        int       `json:"views"`
}

// AnalyticsSummary is the body of GET /api/analytics/summary.
//
// TotalVisits and MenuViews are the same number — every menu_view is a visit —
// and both are sent because the contract names both. UniqueVisitors counts
// distinct visitor keys over EVERY event in the range, so a visitor who opened
// a product from a shared link without passing the menu page still counts.
type AnalyticsSummary struct {
	From           string        `json:"from"`
	To             string        `json:"to"`
	TotalVisits    int           `json:"total_visits"`
	UniqueVisitors int           `json:"unique_visitors"`
	MenuViews      int           `json:"menu_views"`
	CategoryViews  int           `json:"category_views"`
	ProductViews   int           `json:"product_views"`
	Daily          []DailyStats  `json:"daily"`
	TopCategories  []TopCategory `json:"top_categories"`
	TopProducts    []TopProduct  `json:"top_products"`
}

// topListLimit is how many entries each top list carries.
const topListLimit = 10

// eventScope is the WHERE clause every read shares: the business, the window
// and the optional menu, as conditions on menu_events aliased e, with their
// arguments. Further conditions are appended by the caller with the next
// placeholder numbers.
func eventScope(businessID uuid.UUID, window AnalyticsRange) ([]string, []any) {
	conditions := []string{"e.business_id = $1"}
	args := []any{businessID}
	if window.HasStart {
		args = append(args, window.Start)
		conditions = append(conditions, "e.created_at >= $"+strconv.Itoa(len(args)))
	}
	if window.HasEnd {
		args = append(args, window.End)
		conditions = append(conditions, "e.created_at < $"+strconv.Itoa(len(args)))
	}
	if window.MenuID != nil {
		args = append(args, *window.MenuID)
		conditions = append(conditions, "e.menu_id = $"+strconv.Itoa(len(args)))
	}
	return conditions, args
}

// SummarizeEvents builds the dashboard summary of one business over a window
// that has both bounds.
func SummarizeEvents(ctx context.Context, db DB, businessID uuid.UUID,
	window AnalyticsRange) (*AnalyticsSummary, error) {

	conditions, args := eventScope(businessID, window)
	where := strings.Join(conditions, " AND ")

	summary := &AnalyticsSummary{
		From:          window.FromDay,
		To:            window.ToDay,
		Daily:         make([]DailyStats, 0),
		TopCategories: make([]TopCategory, 0),
		TopProducts:   make([]TopProduct, 0),
	}

	// --- totals
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE e.type = 'menu_view'),
		       COUNT(*) FILTER (WHERE e.type = 'category_view'),
		       COUNT(*) FILTER (WHERE e.type = 'product_view'),
		       COUNT(DISTINCT e.visitor_key)
		FROM menu_events e
		WHERE `+where, args...).Scan(&summary.MenuViews, &summary.CategoryViews,
		&summary.ProductViews, &summary.UniqueVisitors); err != nil {
		return nil, fmt.Errorf("could not count the events: %w", err)
	}
	summary.TotalVisits = summary.MenuViews

	// --- daily series: every day of the window, the empty ones included.
	// generate_series supplies the days and the events are LEFT JOINed onto
	// them by their Istanbul calendar day, so a day nobody visited is a row of
	// zeros rather than a gap the chart has to invent.
	n := len(args)
	dailyArgs := append(append([]any{}, args...), window.FromDay, window.ToDay, AnalyticsZone)
	rows, err := db.Query(ctx, fmt.Sprintf(`
		WITH days AS (
			SELECT generate_series($%[1]d::date, $%[2]d::date, interval '1 day')::date AS day
		), events AS (
			SELECT (e.created_at AT TIME ZONE $%[3]d)::date AS day, e.type, e.visitor_key
			FROM menu_events e
			WHERE %[4]s
		)
		SELECT to_char(days.day, 'YYYY-MM-DD'),
		       COUNT(events.type) FILTER (WHERE events.type = 'menu_view'),
		       COUNT(DISTINCT events.visitor_key),
		       COUNT(events.type) FILTER (WHERE events.type = 'category_view'),
		       COUNT(events.type) FILTER (WHERE events.type = 'product_view')
		FROM days
		LEFT JOIN events ON events.day = days.day
		GROUP BY days.day
		ORDER BY days.day`, n+1, n+2, n+3, where), dailyArgs...)
	if err != nil {
		return nil, fmt.Errorf("could not build the daily series: %w", err)
	}
	for rows.Next() {
		var day DailyStats
		if err := rows.Scan(&day.Date, &day.Visits, &day.UniqueVisitors,
			&day.CategoryViews, &day.ProductViews); err != nil {
			rows.Close()
			return nil, err
		}
		summary.Daily = append(summary.Daily, day)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// --- top categories: counted per id first, then joined to the categories
	// that still exist in this business. A deleted category's views still
	// count in the totals above; it simply cannot be named in a top list.
	rows, err = db.Query(ctx, `
		SELECT t.category_id, c.translations, m.default_language, t.views
		FROM (
			SELECT e.category_id, COUNT(*) AS views
			FROM menu_events e
			WHERE `+where+` AND e.type = 'category_view' AND e.category_id IS NOT NULL
			GROUP BY e.category_id
		) t
		JOIN categories c ON c.id = t.category_id AND c.business_id = $1
		JOIN menus m ON m.id = c.menu_id AND m.business_id = $1
		ORDER BY t.views DESC, t.category_id
		LIMIT `+strconv.Itoa(topListLimit), args...)
	if err != nil {
		return nil, fmt.Errorf("could not rank the categories: %w", err)
	}
	for rows.Next() {
		var (
			top          TopCategory
			translations models.Translations
			language     string
		)
		if err := rows.Scan(&top.ID, &translations, &language, &top.Views); err != nil {
			rows.Close()
			return nil, err
		}
		top.Name = translations.Resolve(language, language).Name
		summary.TopCategories = append(summary.TopCategories, top)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// --- top products, the same way, with the category each one is in now.
	rows, err = db.Query(ctx, `
		SELECT t.product_id, p.translations, c.translations, m.default_language, t.views
		FROM (
			SELECT e.product_id, COUNT(*) AS views
			FROM menu_events e
			WHERE `+where+` AND e.type = 'product_view' AND e.product_id IS NOT NULL
			GROUP BY e.product_id
		) t
		JOIN products p ON p.id = t.product_id AND p.business_id = $1
		JOIN categories c ON c.id = p.category_id AND c.business_id = $1
		JOIN menus m ON m.id = c.menu_id AND m.business_id = $1
		ORDER BY t.views DESC, t.product_id
		LIMIT `+strconv.Itoa(topListLimit), args...)
	if err != nil {
		return nil, fmt.Errorf("could not rank the products: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			top                  TopProduct
			product, categoryTrs models.Translations
			language             string
		)
		if err := rows.Scan(&top.ID, &product, &categoryTrs, &language, &top.Views); err != nil {
			return nil, err
		}
		top.Name = product.Resolve(language, language).Name
		top.CategoryName = categoryTrs.Resolve(language, language).Name
		summary.TopProducts = append(summary.TopProducts, top)
	}
	return summary, rows.Err()
}

// EventFilter narrows the event list. Empty values do not filter; the handler
// has already validated every non-empty one.
type EventFilter struct {
	Window   AnalyticsRange
	Type     string
	IPPrefix string // [0-9A-Fa-f:.] only, so it cannot carry a LIKE wildcard
	Limit    int
	Offset   int
}

// EventRow is one row of GET /api/analytics/events. The names are resolved in
// the default language of the event's menu and are null when the category or
// product has been deleted since.
type EventRow struct {
	ID           uuid.UUID  `json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	Type         string     `json:"type"`
	MenuID       uuid.UUID  `json:"menu_id"`
	MenuName     string     `json:"menu_name"`
	CategoryID   *uuid.UUID `json:"category_id"`
	CategoryName *string    `json:"category_name"`
	ProductID    *uuid.UUID `json:"product_id"`
	ProductName  *string    `json:"product_name"`
	IP           *string    `json:"ip"`
	Port         *int       `json:"port"`
	IPSource     string     `json:"ip_source"`
	VisitorID    *string    `json:"visitor_id"`
	Language     *string    `json:"language"`
	UserAgent    string     `json:"user_agent"`
}

// ListEvents returns one page of a business's events, newest first, and the
// number the filter matches in all.
func ListEvents(ctx context.Context, db DB, businessID uuid.UUID,
	filter EventFilter) ([]EventRow, int, error) {

	conditions, args := eventScope(businessID, filter.Window)
	if filter.Type != "" {
		args = append(args, filter.Type)
		conditions = append(conditions, "e.type = $"+strconv.Itoa(len(args)))
	}
	if filter.IPPrefix != "" {
		// A prefix, so "85.105." finds a whole block. The filter holds only
		// hex digits, colons and dots — no % or _ — so it needs no escaping.
		args = append(args, strings.ToLower(filter.IPPrefix)+"%")
		conditions = append(conditions, "lower(e.ip) LIKE $"+strconv.Itoa(len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM menu_events e WHERE `+where, args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("could not count the events: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := db.Query(ctx, `
		SELECT e.id, e.created_at, e.type, e.menu_id, m.name, m.default_language,
		       e.category_id, c.translations, e.product_id, p.translations,
		       e.ip, e.port, e.ip_source, e.visitor_id, e.language, e.user_agent
		FROM menu_events e
		JOIN menus m ON m.id = e.menu_id AND m.business_id = e.business_id
		LEFT JOIN categories c ON c.id = e.category_id AND c.business_id = e.business_id
		LEFT JOIN products p ON p.id = e.product_id AND p.business_id = e.business_id
		WHERE `+where+`
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("could not list the events: %w", err)
	}
	defer rows.Close()

	events := make([]EventRow, 0)
	for rows.Next() {
		var (
			row                  EventRow
			language             string
			category, productTrs *models.Translations
		)
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.Type, &row.MenuID, &row.MenuName,
			&language, &row.CategoryID, &category, &row.ProductID, &productTrs,
			&row.IP, &row.Port, &row.IPSource, &row.VisitorID, &row.Language,
			&row.UserAgent); err != nil {
			return nil, 0, err
		}
		if category != nil {
			name := category.Resolve(language, language).Name
			row.CategoryName = &name
		}
		if productTrs != nil {
			name := productTrs.Resolve(language, language).Name
			row.ProductName = &name
		}
		events = append(events, row)
	}
	return events, total, rows.Err()
}

// purgeBatch bounds one DELETE of the retention sweep, so the first sweep
// after a long pause — or after the retention was shortened — removes a
// backlog in slices instead of one long statement.
const purgeBatch = 10000

// PurgeMenuEvents deletes every event recorded before the cutoff, across every
// tenant, and returns how many it removed. It is the KVKK half of the
// analytics: the rows hold visitor addresses, and addresses are kept only as
// long as ANALYTICS_RETENTION_DAYS says (cmd/api runs this at start-up and
// every 24 hours).
//
// Both halves of each slice are index-driven. The ids are collected into an
// array first — the oldest purgeBatch of them, read off menu_events_created_idx
// in created_at order — and the DELETE matches that array against the primary
// key. The obvious "WHERE id IN (SELECT ... LIMIT n)" is not that: PostgreSQL
// plans it as a semi-join whose outer side scans the whole table, so every
// slice, and every daily sweep with even one expired row, read all of
// menu_events, dead rows of the earlier slices included.
func PurgeMenuEvents(ctx context.Context, db DB, cutoff time.Time) (int64, error) {
	var removed int64
	for {
		tag, err := db.Exec(ctx, `
			DELETE FROM menu_events
			WHERE id = ANY (ARRAY(
				SELECT id FROM menu_events
				WHERE created_at < $1
				ORDER BY created_at
				LIMIT $2
			))`, cutoff, purgeBatch)
		if err != nil {
			return removed, err
		}
		removed += tag.RowsAffected()
		if tag.RowsAffected() < purgeBatch {
			return removed, nil
		}
		if err := ctx.Err(); err != nil {
			return removed, err
		}
	}
}
