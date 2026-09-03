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
)

const businessColumns = `id, user_id, name, slug, logo_url, cover_url, currency, theme,
	font_family, primary_color, default_language, languages, splash_enabled,
	splash_duration, splash_bg_color, splash_text, show_vat_note, vat_note_text,
	show_price_date, price_updated_at, phone, address, instagram, wifi_password,
	wifi_ssid, splash_logo_url, splash_headline, splash_exit_animation,
	splash_exit_duration, background_type, background_color, background_image_url,
	background_overlay_opacity, splash_exit_easing, splash_display, header_display,
	is_active, created_at, updated_at`

func scanBusiness(row pgx.Row) (*models.Business, error) {
	var business models.Business
	err := row.Scan(
		&business.ID, &business.UserID, &business.Name, &business.Slug, &business.LogoURL,
		&business.CoverURL, &business.Currency, &business.Theme, &business.FontFamily,
		&business.PrimaryColor, &business.DefaultLanguage, &business.Languages,
		&business.SplashEnabled, &business.SplashDuration, &business.SplashBgColor,
		&business.SplashText, &business.ShowVatNote, &business.VatNoteText,
		&business.ShowPriceDate, &business.PriceUpdatedAt, &business.Phone, &business.Address,
		&business.Instagram, &business.WifiPassword, &business.WifiSSID,
		&business.SplashLogoURL, &business.SplashHeadline, &business.SplashExitAnimation,
		&business.SplashExitDuration, &business.BackgroundType, &business.BackgroundColor,
		&business.BackgroundImageURL, &business.BackgroundOverlayOpacity,
		&business.SplashExitEasing, &business.SplashDisplay, &business.HeaderDisplay,
		&business.IsActive, &business.CreatedAt, &business.UpdatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if business.Languages == nil {
		business.Languages = []string{business.DefaultLanguage}
	}
	return &business, nil
}

// insertBusiness creates a business with the default settings during sign-up.
func insertBusiness(ctx context.Context, db DB, userID uuid.UUID, name, slug string) (*models.Business, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO businesses (user_id, name, slug, languages)
		VALUES ($1, $2, $3, $4)
		RETURNING `+businessColumns,
		userID, name, slug, []string{"tr"})

	business, err := scanBusiness(row)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("could not create the business: %w", err)
	}

	if err := insertDefaultMenuAndBranch(ctx, db, business); err != nil {
		return nil, err
	}
	return business, nil
}

// insertDefaultMenuAndBranch gives a brand new business the same starting shape
// the 003 migration gave every existing one: a default menu, a default branch
// carrying the business slug, and the link between them.
//
// Without this a business created after 003 has no menu row at all, so
// ResolveTenant finds nothing and the customer menu 404s for every new tenant —
// the migration backfill runs once and never sees later sign-ups.
//
// The branch contact columns are left NULL so they keep inheriting from the
// business; see the note on backfill step 2 in the migration.
func insertDefaultMenuAndBranch(ctx context.Context, db DB, business *models.Business) error {
	var menuID uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO menus (business_id, name, slug, is_default, position)
		VALUES ($1, 'Ana Menü', 'ana-menu', true, 0)
		RETURNING id`, business.ID).Scan(&menuID)
	if err != nil {
		return fmt.Errorf("could not create the default menu: %w", err)
	}

	var branchID uuid.UUID
	err = db.QueryRow(ctx, `
		INSERT INTO branches (business_id, name, slug, is_default, position)
		VALUES ($1, 'Merkez', $2, true, 0)
		RETURNING id`, business.ID, business.Slug).Scan(&branchID)
	if err != nil {
		if IsUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("could not create the default branch: %w", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO branch_menus (branch_id, menu_id, is_default, position)
		VALUES ($1, $2, true, 0)`, branchID, menuID); err != nil {
		return fmt.Errorf("could not link the default menu to the branch: %w", err)
	}
	return nil
}

// GetBusinessByID fetches a business by identifier.
func GetBusinessByID(ctx context.Context, db DB, id uuid.UUID) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE id = $1`, id))
}

// GetBusinessByUserID fetches the business that belongs to a user.
func GetBusinessByUserID(ctx context.Context, db DB, userID uuid.UUID) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE user_id = $1`, userID))
}

// GetBusinessBySlug fetches a business by the slug resolved from the subdomain.
// Only published (is_active) businesses are returned.
func GetBusinessBySlug(ctx context.Context, db DB, slug string) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE slug = lower($1) AND is_active = true`,
		strings.TrimSpace(slug)))
}

// SlugTaken reports whether a slug is already used by another business.
// Business slugs and branch slugs share one subdomain namespace and the public
// lookup resolves a branch first, so a slug owned by a foreign branch would
// swallow this business' address — the branches table is checked as well. The
// branches of the business itself are excluded: the 003 backfill gave its
// default branch exactly this slug.
func SlugTaken(ctx context.Context, db DB, slug string, exceptID uuid.UUID) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM businesses WHERE slug = $1 AND id <> $2)
		    OR EXISTS (SELECT 1 FROM branches WHERE slug = $1 AND business_id <> $2)`,
		strings.ToLower(strings.TrimSpace(slug)), exceptID).Scan(&exists)
	return exists, err
}

// businessUpdatableColumns lists the columns PUT /api/business may change.
// Column names only ever come from this allowlist, so SQL injection is impossible.
var businessUpdatableColumns = map[string]bool{
	"name": true, "slug": true, "logo_url": true, "cover_url": true,
	"currency": true, "theme": true, "font_family": true, "primary_color": true,
	"default_language": true, "languages": true,
	"splash_enabled": true, "splash_duration": true, "splash_bg_color": true, "splash_text": true,
	"show_vat_note": true, "vat_note_text": true, "show_price_date": true,
	"phone": true, "address": true, "instagram": true, "wifi_password": true,
	"wifi_ssid": true, "splash_logo_url": true, "splash_headline": true,
	"splash_exit_animation": true, "splash_exit_duration": true,
	"splash_exit_easing": true, "splash_display": true,
	"background_type": true, "background_color": true,
	"background_image_url": true, "background_overlay_opacity": true,
	"header_display": true, "is_active": true,
}

// UpdateBusiness updates the given columns and returns the fresh record.
func UpdateBusiness(ctx context.Context, db DB, id uuid.UUID, fields map[string]any) (*models.Business, error) {
	if len(fields) == 0 {
		return GetBusinessByID(ctx, db, id)
	}

	// A stable order keeps the SQL readable and the behaviour deterministic.
	columns := make([]string, 0, len(fields))
	for column := range fields {
		if businessUpdatableColumns[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return GetBusinessByID(ctx, db, id)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+1)
	for i, column := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
		args = append(args, fields[column])
	}
	args = append(args, id)

	query := `UPDATE businesses SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d RETURNING `, len(args)) + businessColumns

	business, err := scanBusiness(db.QueryRow(ctx, query, args...))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return business, nil
}

// TouchPriceUpdatedAt moves the "prices valid from" date to now after a bulk
// price update. The customer menu footer reads this value.
func TouchPriceUpdatedAt(ctx context.Context, db DB, businessID uuid.UUID) (time.Time, error) {
	var updatedAt time.Time
	err := db.QueryRow(ctx,
		`UPDATE businesses SET price_updated_at = now() WHERE id = $1 RETURNING price_updated_at`,
		businessID).Scan(&updatedAt)
	return updatedAt, err
}

// LogPriceUpdate writes the audit record of a bulk price change.
func LogPriceUpdate(ctx context.Context, db DB, businessID uuid.UUID,
	percentage float64, rounding string, affected int) error {
	_, err := db.Exec(ctx,
		`INSERT INTO price_update_logs (business_id, percentage, rounding, affected)
		 VALUES ($1, $2, $3, $4)`,
		businessID, percentage, rounding, affected)
	return err
}
