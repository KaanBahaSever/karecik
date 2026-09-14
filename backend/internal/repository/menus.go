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
// table after migration 011, which is why logo_fade_in, text_color,
// show_yerli_uretim and yerli_uretim_logo_url sit near the end rather than
// next to header_display and primary_color: 006 appended all four to the table.
// slogan and splash_entrance follow them for the same reason — 007 appends
// both after all four, slogan first and splash_entrance second — and
// contact_display and links close the list because 011 appends them after
// those, contact_display first. splash_entrance therefore scans right after
// slogan, NOT next to splash_exit_animation where models.Menu declares it: the
// list follows the table, never the struct.
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
	text_color, show_yerli_uretim, yerli_uretim_logo_url, slogan,
	splash_entrance, contact_display, links`

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

		// Appended by migration 007, after all four of those, so they scan
		// after them and in the order 007 adds them: slogan first, then
		// splash_entrance. Both are NOT NULL with a default — '' and 'fade' —
		// hence plain strings.
		&menu.Slogan,
		&menu.SplashEntrance,

		// Appended by migration 011, after splash_entrance, so they scan last
		// and in the order 011 adds them: contact_display first, then links.
		// Both are NOT NULL with a default — 'inline' and '[]' — so the mode is
		// a plain string and links scans from jsonb straight into the slice.
		&menu.ContactDisplay,
		&menu.Links,
	}
}

// normalizeMenu fills in what the customer menu may not receive as null.
//
// links is NOT NULL and CHECKed to be an array, and '[]' scans into an empty
// slice rather than nil, so the Links branch is a guarantee rather than a
// repair: the payload says [] in every case, exactly as it does for languages.
func normalizeMenu(menu *models.Menu) {
	if menu.Languages == nil {
		menu.Languages = []string{menu.DefaultLanguage}
	}
	if menu.Links == nil {
		menu.Links = models.MenuLinks{}
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
// nobody may rewrite; price_updated_at, which only the products trigger of
// migration 010 moves (see GetMenuPriceUpdatedAt); and currency_symbol, which
// the repository derives itself (see applyCurrencySymbol).
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
	"splash_entrance": true, "splash_exit_animation": true,
	"splash_exit_duration": true, "splash_exit_easing": true,
	"splash_display": true, "splash_slide_fade": true,

	"currency": true, "show_vat_note": true, "vat_note_text": true,
	"show_price_date": true, "header_display": true, "logo_fade_in": true,
	"slogan": true, "default_language": true, "languages": true,

	"text_color": true, "show_yerli_uretim": true,
	"yerli_uretim_logo_url": true,

	"contact_display": true, "links": true,
}

// applyLinksArray makes sure a links write stores a JSON array. The handler
// already hands over a non-nil list, but a nil one — a nil models.MenuLinks, a
// nil []models.MenuLink or a bare nil — would reach the jsonb column as SQL
// NULL, which the NOT NULL column refuses and the caller would answer with a
// 500. An absent list means no links, so an empty one is what gets written.
func applyLinksArray(values map[string]any) {
	value, ok := values["links"]
	if !ok {
		return
	}
	switch links := value.(type) {
	case nil:
		values["links"] = models.MenuLinks{}
	case models.MenuLinks:
		if links == nil {
			values["links"] = models.MenuLinks{}
		}
	case []models.MenuLink:
		if links == nil {
			values["links"] = models.MenuLinks{}
		}
	}
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
	applyLinksArray(values)
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

	// The slug comes from the address a customer opened; see UnstorableText.
	if UnstorableText(slug) {
		return nil, ErrNotFound
	}
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

// MaxMenuSlugSuffix caps the "-2, -3, ..." search of EnsureUniqueMenuSlug.
const MaxMenuSlugSuffix = 1000

// MenuSlugMaxLength mirrors the 60-character cap of utils.IsValidSlug. Whatever
// EnsureUniqueMenuSlug returns is stored by CreateMenu and re-validated by the
// app on the next edit, so a longer slug is a row the application itself
// rejects — the length has to be enforced on the way in, not only on input.
const MenuSlugMaxLength = 60

// TrimSlug cuts a slug to at most limit characters and drops any trailing
// hyphen the cut leaves behind, since utils.IsValidSlug rejects one. A slug is
// [a-z0-9-] only, so slicing by byte can never split a rune.
func TrimSlug(slug string, limit int) string {
	if limit < 0 {
		limit = 0
	}
	if len(slug) > limit {
		slug = slug[:limit]
	}
	return strings.TrimRight(slug, "-")
}

// MenuSlugFamily is the prefix every "-N" candidate is built on: the base cut
// short enough that base + "-" + the widest suffix the search can reach still
// fits MenuSlugMaxLength. Without it a 59-character base yields a 61-character
// "-2" slug that CreateMenu stores and utils.IsValidSlug then rejects.
//
// It is pure, which is what lets the length guarantee be tested without a
// database.
func MenuSlugFamily(base string) string {
	room := MenuSlugMaxLength - len(fmt.Sprintf("-%d", MaxMenuSlugSuffix))
	family := TrimSlug(base, room)
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
// MenuSlugMaxLength and the "-N" candidates are built on MenuSlugFamily(base),
// which reserves room for the suffix.
func EnsureUniqueMenuSlug(ctx context.Context, db DB, businessID uuid.UUID,
	baseSlug string, excludeID *uuid.UUID) (string, error) {

	base := TrimSlug(strings.ToLower(strings.TrimSpace(baseSlug)), MenuSlugMaxLength)
	if base == "" {
		base = "menu"
	}

	// The suffixed candidates hang off a shorter prefix so that base-N still
	// fits the 60-character cap, and that prefix decides which family the query
	// has to collect — so it is computed BEFORE the query, not after it. The
	// untrimmed base is still matched exactly, because a base that is free is
	// returned whole rather than needlessly shortened.
	family := MenuSlugFamily(base)

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
	for suffix := 2; suffix <= MaxMenuSlugSuffix; suffix++ {
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

	// The next position is computed as a bigint and capped at the largest
	// INTEGER. A request may set menus.position to that largest value itself,
	// and one more would fail the INSERT instead of appending the menu. A capped
	// position ties with the menu holding the largest one, and ListMenus orders
	// a tie by created_at, so the new menu still comes last.
	var nextPosition int
	err := db.QueryRow(ctx, `
		SELECT LEAST(COALESCE(MAX(position)::bigint + 1, 0), 2147483647)
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
//
// LOCK ORDER. A menu the business does not own is ErrNotFound before anything
// is locked. Otherwise DeleteMenu takes, inside one transaction and in this
// order:
//
//  1. the menu's exclusive advisory lock (see locks.go);
//  2. every product of the menu, in ascending id order;
//  3. every category of the menu, in ascending id order;
//  4. the menu row, through the DELETE.
//
// Every writer in this package that locks more than one product row takes the
// product rows in ascending id order, product rows before category rows, and a
// menu row — when it takes one — last. Every writer that takes an advisory lock
// takes it before its first row lock.
//
// The advisory lock first: a write into this menu — a category created in it or
// moved into it, a product created, moved or reordered into one of its
// categories — takes the same lock shared before it locks anything. While the
// delete runs, such a write waits at that lock holding nothing, and then finds
// its target gone; a delete that starts while such a write is running waits for
// it before the delete has locked anything. Without the lock, a write that
// landed between the steps above — and a price edit of its product after it —
// could each end up holding a row the other needed. It also means step 2 takes
// its snapshot once no write into the menu is in progress, so the DELETE's
// cascade finds every product of the menu locked.
//
// Step 2 reaches the products through a join to their categories. Under READ
// COMMITTED a product that another transaction changed while the delete waited
// for it is checked again against the category row the statement read first,
// so a product whose category_id changed in the meantime drops out of the lock.
// That is right for a product that left the menu, and leaving it is the only
// such change the delete can meet: a product moved or reordered into a category
// of this menu, the one it is in included, takes this menu's advisory lock and
// so never runs while the delete does. A product whose category moved to
// another menu keeps its category_id and stays locked until the delete ends;
// step 3 no longer finds that category in the menu, so the cascade removes
// neither.
//
// Products before the menu row: a single-product edit locks its one product
// and then, through the products_touch_menu_price_date trigger of migration
// 010, the menu. A delete that locked the menu row first and reached the
// products only through the cascade would take the two the other way round,
// and a price edit and a delete in the same menu at the same instant would wait
// on each other until PostgreSQL aborted one of them after deadlock_timeout.
// With the products locked first, a price edit that holds one of them
// finishes, menu row included, while the delete waits for it; a delete that
// already holds them makes the price edit wait before it has locked anything
// the delete still needs.
//
// Categories before the menu row: the cascade needs every category of the menu
// FOR UPDATE, which conflicts with the FOR NO KEY UPDATE lock a category reorder
// holds and with the KEY SHARE lock the foreign key check of a product write
// takes. Locked in step 3, in the id order ReorderCategories locks in, those
// writers make the delete wait before it has the menu row, or wait for the
// delete before they have locked anything it needs.
//
// The ownership check and both SELECTs are scoped to the business exactly like
// the DELETE, so a request naming another tenant's menu takes no lock at all,
// deletes nothing and comes back as ErrNotFound.
func DeleteMenu(ctx context.Context, db TxDB, id, businessID uuid.UUID) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		owned, err := menuOwned(ctx, tx, id, businessID)
		if err != nil {
			return err
		}
		if !owned {
			return ErrNotFound
		}
		if err := lockMenu(ctx, tx, id, true); err != nil {
			return err
		}

		// Exec discards the rows of both SELECTs. What they are for are the row
		// locks they leave behind, which the transaction holds until the DELETE
		// commits.
		if _, err := tx.Exec(ctx, `
			SELECT p.id
			FROM products p
			JOIN categories c ON c.id = p.category_id
			WHERE c.menu_id = $1 AND c.business_id = $2 AND p.business_id = $2
			ORDER BY p.id
			FOR UPDATE OF p`, id, businessID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			SELECT id
			FROM categories
			WHERE menu_id = $1 AND business_id = $2
			ORDER BY id
			FOR UPDATE`, id, businessID); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx,
			`DELETE FROM menus WHERE id = $1 AND business_id = $2`, id, businessID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
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

// GetMenuPriceUpdatedAt reads the "prices valid from" date of one menu — the
// value the customer menu footer prints.
//
// Nothing in the application writes that column. The
// products_touch_menu_price_date trigger of migration 010 moves it whenever a
// price on the menu really changes, so the bulk price endpoint reads the date
// back after its UPDATE instead of setting it, which is what keeps an apply
// that changed nothing from moving it.
//
// Like every other menu read in this package it runs on (id, business_id) and
// never on a bare id. A menu that belongs to somebody else simply matches no
// row, which surfaces as ErrNotFound instead of as a silent zero timestamp.
func GetMenuPriceUpdatedAt(ctx context.Context, db DB, menuID, businessID uuid.UUID) (time.Time, error) {
	var updatedAt time.Time
	err := db.QueryRow(ctx, `
		SELECT price_updated_at FROM menus
		WHERE id = $1 AND business_id = $2`,
		menuID, businessID).Scan(&updatedAt)
	if err != nil {
		if isNoRows(err) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, err
	}
	return updatedAt, nil
}
