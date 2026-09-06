package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// menuColumns is the menu scan order and the single source of truth for it:
// every SELECT, INSERT ... RETURNING and UPDATE ... RETURNING below lists
// exactly these columns and menuScanTargets returns the destinations in exactly
// the same order. The order follows the physical column order of the menus
// table after migration 006, which is why logo_fade_in, text_color,
// show_yerli_uretim and yerli_uretim_logo_url sit at the very end rather than
// next to header_display and primary_color: 006 appended all four to the table.
//
// NOTE: models.Menu declares its fields in a different order (Currency and
// CurrencySymbol sit before Theme, the timestamps sit at the end), so the scan
// targets follow this list and never the struct. A mismatch between the two is
// silent field corruption, not a compile error.
const menuColumns = `id, business_id, name, slug, description, is_active,
	position, created_at, updated_at, phone, address, instagram, wifi_ssid,
	wifi_password, logo_url, cover_url, theme, font_family, primary_color,
	background_type, background_color, background_image_url,
	background_overlay_opacity, splash_enabled, splash_logo_url, splash_headline,
	splash_text, splash_bg_color, splash_duration, splash_exit_animation,
	splash_exit_duration, splash_exit_easing, splash_display, splash_slide_fade,
	currency, currency_symbol, show_vat_note, vat_note_text, show_price_date,
	price_updated_at, header_display, default_language, languages, logo_fade_in,
	text_color, show_yerli_uretim, yerli_uretim_logo_url`

// menuColumnsM is menuColumns qualified with the m alias, needed wherever menus
// is joined against categories: id, business_id, position, is_active,
// created_at and updated_at exist on both tables. It is derived from the one
// list so the two can never drift apart.
var menuColumnsM = qualifyColumns(menuColumns, "m")

// qualifyColumns prefixes every column of a list with a table alias.
func qualifyColumns(columns, alias string) string {
	parts := strings.Split(columns, ",")
	for i, part := range parts {
		parts[i] = alias + "." + strings.TrimSpace(part)
	}
	return strings.Join(parts, ", ")
}

// menuScanTargets returns the scan destinations of menuColumns in exactly that
// order. Every menu read goes through it — scanMenu for the single-row queries
// and ListMenus for the list, which only appends its computed category_count —
// so the order lives in one place instead of three.
func menuScanTargets(menu *models.Menu) []any {
	return []any{
		&menu.ID, &menu.BusinessID, &menu.Name, &menu.Slug, &menu.Description,
		&menu.IsActive, &menu.Position,
		&menu.CreatedAt, &menu.UpdatedAt,

		&menu.Phone, &menu.Address, &menu.Instagram,
		&menu.WifiSSID, &menu.WifiPassword,
		&menu.LogoURL, &menu.CoverURL,

		&menu.Theme, &menu.FontFamily, &menu.PrimaryColor,
		&menu.BackgroundType, &menu.BackgroundColor, &menu.BackgroundImageURL,
		&menu.BackgroundOverlayOpacity,

		&menu.SplashEnabled, &menu.SplashLogoURL, &menu.SplashHeadline,
		&menu.SplashText, &menu.SplashBgColor, &menu.SplashDuration,
		&menu.SplashExitAnimation, &menu.SplashExitDuration, &menu.SplashExitEasing,
		&menu.SplashDisplay, &menu.SplashSlideFade,

		&menu.Currency, &menu.CurrencySymbol,
		&menu.ShowVatNote, &menu.VatNoteText,
		&menu.ShowPriceDate, &menu.PriceUpdatedAt,
		&menu.HeaderDisplay, &menu.DefaultLanguage, &menu.Languages,

		// Appended by migration 006, so they scan last — see menuColumns.
		// TextColor is NOT NULL with a '#111827' default and ShowYerliUretim is
		// NOT NULL too, so both are plain values; only the badge artwork is
		// nullable and therefore a pointer.
		&menu.LogoFadeIn,
		&menu.TextColor, &menu.ShowYerliUretim, &menu.YerliUretimLogoURL,
	}
}

// normalizeMenu fills in what the customer menu may not receive as null.
func normalizeMenu(menu *models.Menu) {
	if menu.Languages == nil {
		menu.Languages = []string{menu.DefaultLanguage}
	}
}

func scanMenu(row pgx.Row) (*models.Menu, error) {
	var menu models.Menu
	if err := row.Scan(menuScanTargets(&menu)...); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	normalizeMenu(&menu)
	return &menu, nil
}

// menuUpdatableColumns lists the columns a menu payload may change. Column
// names only ever come from this allowlist, so SQL injection is impossible.
//
// Deliberately missing: id, business_id, created_at and updated_at, which
// nobody may rewrite; price_updated_at, which only TouchPriceUpdatedAt moves;
// and currency_symbol, which the repository derives itself (see
// applyCurrencySymbol).
var menuUpdatableColumns = map[string]bool{
	"name": true, "slug": true, "description": true,
	"is_active": true, "position": true,

	"phone": true, "address": true, "instagram": true,
	"wifi_ssid": true, "wifi_password": true,
	"logo_url": true, "cover_url": true,

	"theme": true, "font_family": true, "primary_color": true,
	"background_type": true, "background_color": true,
	"background_image_url": true, "background_overlay_opacity": true,

	"splash_enabled": true, "splash_logo_url": true, "splash_headline": true,
	"splash_text": true, "splash_bg_color": true, "splash_duration": true,
	"splash_exit_animation": true, "splash_exit_duration": true,
	"splash_exit_easing": true, "splash_display": true, "splash_slide_fade": true,

	"currency": true, "show_vat_note": true, "vat_note_text": true,
	"show_price_date": true, "header_display": true, "logo_fade_in": true,
	"default_language": true, "languages": true,

	"text_color": true, "show_yerli_uretim": true,
	"yerli_uretim_logo_url": true,
}

// applyCurrencySymbol derives currency_symbol from the currency code whenever
// the currency is written. The repository layer is the column's only writer, so
// a payload carrying currency_symbol is ignored — never rejected — and a stored
// symbol can never drift away from its code.
func applyCurrencySymbol(values map[string]any) {
	code, ok := values["currency"].(string)
	if !ok {
		return
	}
	values["currency_symbol"] = utils.CurrencySymbol(code)
}

// updatableFields copies the allowed columns of a payload and derives the
// values the repository owns itself.
func updatableFields(fields map[string]any) map[string]any {
	values := make(map[string]any, len(fields)+4)
	for column, value := range fields {
		if menuUpdatableColumns[column] {
			values[column] = value
		}
	}
	applyCurrencySymbol(values)
	return values
}

// sortedColumns keeps the generated SQL readable and the behaviour deterministic.
func sortedColumns(values map[string]any) []string {
	columns := make([]string, 0, len(values))
	for column := range values {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return columns
}

// ListMenus returns the menus of a business ordered by position, each with the
// number of categories it holds. categories.menu_id is NOT NULL since migration
// 005, so every category is counted on exactly one menu.
//
// The list may legitimately be empty: a business is allowed to own no menus at
// all, permanently.
func ListMenus(ctx context.Context, db DB, businessID uuid.UUID) ([]models.Menu, error) {
	rows, err := db.Query(ctx, `
		SELECT `+menuColumnsM+`, COUNT(c.id) AS category_count
		FROM menus m
		LEFT JOIN categories c ON c.menu_id = m.id
		WHERE m.business_id = $1
		GROUP BY m.id
		ORDER BY m.position ASC, m.created_at ASC`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := make([]models.Menu, 0)
	for rows.Next() {
		var menu models.Menu
		if err := rows.Scan(append(menuScanTargets(&menu), &menu.CategoryCount)...); err != nil {
			return nil, err
		}
		normalizeMenu(&menu)
		menus = append(menus, menu)
	}
	return menus, rows.Err()
}

// ListActiveMenus returns the published menus of a business in position order.
// It is the public list: the tenant directory renders it when no single menu
// resolved, and the in-menu switcher renders it when one did.
func ListActiveMenus(ctx context.Context, db DB, businessID uuid.UUID) ([]models.Menu, error) {
	rows, err := db.Query(ctx, `
		SELECT `+menuColumns+` FROM menus
		WHERE business_id = $1 AND is_active = true
		ORDER BY position ASC, created_at ASC`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := make([]models.Menu, 0)
	for rows.Next() {
		var menu models.Menu
		if err := rows.Scan(menuScanTargets(&menu)...); err != nil {
			return nil, err
		}
		normalizeMenu(&menu)
		menus = append(menus, menu)
	}
	return menus, rows.Err()
}

// GetMenu fetches a menu and verifies that it belongs to the business.
func GetMenu(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Menu, error) {
	return scanMenu(db.QueryRow(ctx,
		`SELECT `+menuColumns+` FROM menus WHERE id = $1 AND business_id = $2`,
		id, businessID))
}

// GetMenuBySlug fetches one published menu of a business by its slug — the path
// segment of {business-slug}.karecik.com/{menu-slug}. The lookup is scoped to
// the business because that is the whole namespace of a menu slug: two tenants
// may both publish "kahvalti".
//
// This is the public lookup, so an unpublished menu stays invisible; the
// owner's own preview reaches it through ListMenus instead.
func GetMenuBySlug(ctx context.Context, db DB, businessID uuid.UUID,
	slug string) (*models.Menu, error) {

	return scanMenu(db.QueryRow(ctx, `
		SELECT `+menuColumns+` FROM menus
		WHERE business_id = $1 AND slug = lower($2) AND is_active = true`,
		businessID, strings.TrimSpace(slug)))
}

// menuWriteAttempts bounds the slug re-resolution retry of CreateMenu and
// UpdateMenu. Five is far more than a real race needs: the only writer that can
// still collide is a concurrent create in the SAME business, and every attempt
// re-reads the taken slugs before it writes.
const menuWriteAttempts = 5

// maxMenuSlugSuffix caps the "-2, -3, ..." search of EnsureUniqueMenuSlug.
const maxMenuSlugSuffix = 1000

// menuSlugMaxLength mirrors the 60-character cap of utils.IsValidSlug. Whatever
// EnsureUniqueMenuSlug returns is stored by CreateMenu and re-validated by the
// app on the next edit, so a longer slug is a row the application itself
// rejects — the length has to be enforced on the way in, not only on input.
const menuSlugMaxLength = 60

// trimSlug cuts a slug to at most limit characters and drops any trailing
// hyphen the cut leaves behind, since utils.IsValidSlug rejects one. A slug is
// [a-z0-9-] only, so slicing by byte can never split a rune.
func trimSlug(slug string, limit int) string {
	if limit < 0 {
		limit = 0
	}
	if len(slug) > limit {
		slug = slug[:limit]
	}
	return strings.TrimRight(slug, "-")
}

// menuSlugFamily is the prefix every "-N" candidate is built on: the base cut
// short enough that base + "-" + the widest suffix the search can reach still
// fits menuSlugMaxLength. Without it a 59-character base yields a 61-character
// "-2" slug that CreateMenu stores and utils.IsValidSlug then rejects.
//
// It is pure, which is what lets the length guarantee be tested without a
// database.
func menuSlugFamily(base string) string {
	room := menuSlugMaxLength - len(fmt.Sprintf("-%d", maxMenuSlugSuffix))
	family := trimSlug(base, room)
	if family == "" {
		// Only reachable if the base were all hyphens, which Slugify cannot
		// produce; the fallback keeps the result a valid slug regardless.
		family = "menu"
	}
	return family
}

// EnsureUniqueMenuSlug resolves baseSlug into a slug that is free WITHIN this
// business: baseSlug itself when nothing holds it, otherwise the smallest free
// baseSlug-N with N >= 2.
//
// A menu slug is a path segment under the tenant's own subdomain, so its
// namespace is one business — two tenants both getting "kahvalti" is the
// expected outcome, not a collision. excludeID keeps a menu from colliding with
// itself while it is being updated.
//
// It is a single round trip: one query collects every slug in the family and
// the search then runs in memory. baseSlug always comes out of utils.Slugify /
// utils.SlugifyWithFallback, so it is [a-z0-9-] only and cannot carry a LIKE
// metacharacter (% or _) — no ESCAPE clause is needed.
//
// Every slug it returns satisfies utils.IsValidSlug: the base is capped at
// menuSlugMaxLength and the "-N" candidates are built on menuSlugFamily(base),
// which reserves room for the suffix.
func EnsureUniqueMenuSlug(ctx context.Context, db DB, businessID uuid.UUID,
	baseSlug string, excludeID *uuid.UUID) (string, error) {

	base := trimSlug(strings.ToLower(strings.TrimSpace(baseSlug)), menuSlugMaxLength)
	if base == "" {
		base = "menu"
	}

	// The suffixed candidates hang off a shorter prefix so that base-N still
	// fits the 60-character cap, and that prefix decides which family the query
	// has to collect — so it is computed BEFORE the query, not after it. The
	// untrimmed base is still matched exactly, because a base that is free is
	// returned whole rather than needlessly shortened.
	family := menuSlugFamily(base)

	rows, err := db.Query(ctx, `
		SELECT slug FROM menus
		WHERE business_id = $1
		  AND (slug = $2 OR slug LIKE $3 || '-%')
		  AND ($4::uuid IS NULL OR id <> $4)`, businessID, base, family, excludeID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	taken := make(map[string]bool)
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return "", err
		}
		taken[slug] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	if !taken[base] {
		return base, nil
	}
	for suffix := 2; suffix <= maxMenuSlugSuffix; suffix++ {
		candidate := fmt.Sprintf("%s-%d", family, suffix)
		if !taken[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not generate a free menu address for %q", base)
}

// insertMenu writes one menu row from the prepared column map.
func insertMenu(ctx context.Context, db DB, values map[string]any) (*models.Menu, error) {
	columns := sortedColumns(values)
	placeholders := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	for i, column := range columns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, values[column])
	}

	return scanMenu(db.QueryRow(ctx,
		`INSERT INTO menus (`+strings.Join(columns, ", ")+`)
		 VALUES (`+strings.Join(placeholders, ", ")+`)
		 RETURNING `+menuColumns, args...))
}

// CreateMenu appends a menu to the end of the list. Only the columns present in
// fields are written, everything else keeps its schema default, so a new menu
// starts from the same values migration 005 gave the existing ones.
//
// The slug is resolved against the menus of this business before every attempt
// and the write is retried when it still trips menus_business_id_slug_key, so
// the composite index stays a safety net for a concurrent create instead of
// being the thing that reports the failure. A collision is never an error: the
// caller reads the slug that was actually taken off the returned menu.
//
// NOTE: the retry re-resolves from the slug it was handed, so a race against an
// already-suffixed candidate can produce "kahvalti-2-2". That needs two writers
// inside the same business in the same instant and is preferred over silently
// handing the loser a slug from a family it never asked for.
func CreateMenu(ctx context.Context, db DB, businessID uuid.UUID,
	fields map[string]any) (*models.Menu, error) {

	var nextPosition int
	err := db.QueryRow(ctx, `
		SELECT COALESCE(MAX(position) + 1, 0)
		FROM menus WHERE business_id = $1`, businessID).Scan(&nextPosition)
	if err != nil {
		return nil, err
	}

	values := updatableFields(fields)
	values["business_id"] = businessID
	if _, ok := values["position"]; !ok {
		values["position"] = nextPosition
	}

	// menus.slug is NOT NULL and a menu without an address cannot be served, so
	// a caller that named none still gets one.
	baseSlug, _ := values["slug"].(string)

	for attempt := 0; attempt < menuWriteAttempts; attempt++ {
		slug, err := EnsureUniqueMenuSlug(ctx, db, businessID, baseSlug, nil)
		if err != nil {
			return nil, err
		}
		values["slug"] = slug

		menu, err := insertMenu(ctx, db, values)
		if err == nil {
			return menu, nil
		}
		if !IsUniqueViolation(err) {
			return nil, fmt.Errorf("could not create the menu: %w", err)
		}
	}
	return nil, ErrDuplicate
}

// UpdateMenu applies a partial update to the given columns. When the slug is
// among them it is resolved inside this business first — excluding the menu
// itself, so rewriting a menu with its own slug is a no-op rather than a bump
// to -2 — and the write is retried on a unique violation exactly like
// CreateMenu's.
func UpdateMenu(ctx context.Context, db DB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Menu, error) {

	values := updatableFields(fields)
	if len(values) == 0 {
		return GetMenu(ctx, db, id, businessID)
	}

	baseSlug, hasSlug := values["slug"].(string)

	for attempt := 0; attempt < menuWriteAttempts; attempt++ {
		if hasSlug {
			slug, err := EnsureUniqueMenuSlug(ctx, db, businessID, baseSlug, &id)
			if err != nil {
				return nil, err
			}
			values["slug"] = slug
		}

		columns := sortedColumns(values)
		setParts := make([]string, 0, len(columns))
		args := make([]any, 0, len(columns)+2)
		for i, column := range columns {
			setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
			args = append(args, values[column])
		}
		args = append(args, id, businessID)

		query := `UPDATE menus SET ` + strings.Join(setParts, ", ") +
			fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
			menuColumns

		menu, err := scanMenu(db.QueryRow(ctx, query, args...))
		if err == nil {
			return menu, nil
		}
		if !hasSlug || !IsUniqueViolation(err) {
			return nil, err
		}
	}
	return nil, ErrDuplicate
}

// DeleteMenu removes a menu. It never refuses: a business is allowed to own
// zero menus, so deleting the last one is an ordinary success, and no other
// menu is promoted in its place because there is nothing to promote it to.
//
// NOTE: categories.menu_id cascades, so deleting a menu also deletes its
// categories and their products. The handler warns the user before calling it.
func DeleteMenu(ctx context.Context, db DB, id, businessID uuid.UUID) error {
	tag, err := db.Exec(ctx,
		`DELETE FROM menus WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MenuSlugTaken reports whether another menu OF THE SAME BUSINESS already uses
// the slug. The scope is the whole point: a menu slug is a path segment under
// the tenant's subdomain, so "kahvalti" belongs to as many businesses as want
// it. Compare BusinessSlugTaken, which is global because a business slug is a
// hostname label.
func MenuSlugTaken(ctx context.Context, db DB, businessID uuid.UUID, slug string,
	exceptID uuid.UUID) (bool, error) {

	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM menus
			WHERE business_id = $1 AND slug = $2 AND id <> $3
		)`,
		businessID, strings.ToLower(strings.TrimSpace(slug)), exceptID).Scan(&exists)
	return exists, err
}

// TouchPriceUpdatedAt moves the "prices valid from" date of one menu to now
// after a bulk price update. The customer menu footer reads this value.
//
// Like every other write in this package it runs on (id, business_id) and never
// on a bare id. Its only caller checks ownership first, so the predicate is
// belt-and-braces today — but a mutation that trusts an id alone is one
// careless future call site away from being a real IDOR, so the rule holds
// here too. A menu that belongs to somebody else simply matches no row, which
// surfaces as ErrNotFound instead of as a silent zero timestamp.
func TouchPriceUpdatedAt(ctx context.Context, db DB, menuID, businessID uuid.UUID) (time.Time, error) {
	var updatedAt time.Time
	err := db.QueryRow(ctx, `
		UPDATE menus SET price_updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING price_updated_at`,
		menuID, businessID).Scan(&updatedAt)
	if err != nil {
		if isNoRows(err) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, err
	}
	return updatedAt, nil
}
