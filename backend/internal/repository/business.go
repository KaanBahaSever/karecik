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
	is_active, created_at, updated_at`

func scanBusiness(row pgx.Row) (*models.Business, error) {
	var b models.Business
	err := row.Scan(
		&b.ID, &b.UserID, &b.Name, &b.Slug, &b.LogoURL, &b.CoverURL, &b.Currency, &b.Theme,
		&b.FontFamily, &b.PrimaryColor, &b.DefaultLanguage, &b.Languages, &b.SplashEnabled,
		&b.SplashDuration, &b.SplashBgColor, &b.SplashText, &b.ShowVatNote, &b.VatNoteText,
		&b.ShowPriceDate, &b.PriceUpdatedAt, &b.Phone, &b.Address, &b.Instagram, &b.WifiPassword,
		&b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if b.Languages == nil {
		b.Languages = []string{b.DefaultLanguage}
	}
	return &b, nil
}

// insertBusiness, yeni kayitta varsayilan ayarlarla isletme olusturur.
func insertBusiness(ctx context.Context, db DB, userID uuid.UUID, name, slug string) (*models.Business, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO businesses (user_id, name, slug, languages)
		VALUES ($1, $2, $3, $4)
		RETURNING `+businessColumns,
		userID, name, slug, []string{"tr"})

	b, err := scanBusiness(row)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("isletme olusturulamadi: %w", err)
	}
	return b, nil
}

// GetBusinessByID, isletmeyi kimligine gore getirir.
func GetBusinessByID(ctx context.Context, db DB, id uuid.UUID) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE id = $1`, id))
}

// GetBusinessByUserID, kullaniciya ait isletmeyi getirir.
func GetBusinessByUserID(ctx context.Context, db DB, userID uuid.UUID) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE user_id = $1`, userID))
}

// GetBusinessBySlug, subdomain'den cozulen slug ile isletmeyi getirir.
// Yalnizca yayinda (is_active) olan isletmeler doner.
func GetBusinessBySlug(ctx context.Context, db DB, slug string) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE slug = lower($1) AND is_active = true`,
		strings.TrimSpace(slug)))
}

// SlugTaken, slug'in baska bir isletme tarafindan kullanilip kullanilmadigini soyler.
func SlugTaken(ctx context.Context, db DB, slug string, exceptID uuid.UUID) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM businesses WHERE slug = $1 AND id <> $2)`,
		strings.ToLower(strings.TrimSpace(slug)), exceptID).Scan(&exists)
	return exists, err
}

// businessUpdatableColumns, PUT /api/business ile guncellenebilen sutunlar.
// Sutun adlari yalnizca bu listeden gelir; SQL enjeksiyonu mumkun degildir.
var businessUpdatableColumns = map[string]bool{
	"name": true, "slug": true, "logo_url": true, "cover_url": true,
	"currency": true, "theme": true, "font_family": true, "primary_color": true,
	"default_language": true, "languages": true,
	"splash_enabled": true, "splash_duration": true, "splash_bg_color": true, "splash_text": true,
	"show_vat_note": true, "vat_note_text": true, "show_price_date": true,
	"phone": true, "address": true, "instagram": true, "wifi_password": true,
	"is_active": true,
}

// UpdateBusiness, verilen sutunlari gunceller ve guncel kaydi dondurur.
func UpdateBusiness(ctx context.Context, db DB, id uuid.UUID, fields map[string]any) (*models.Business, error) {
	if len(fields) == 0 {
		return GetBusinessByID(ctx, db, id)
	}

	// Sabit sira -> okunabilir SQL ve deterministik davranis
	columns := make([]string, 0, len(fields))
	for col := range fields {
		if businessUpdatableColumns[col] {
			columns = append(columns, col)
		}
	}
	if len(columns) == 0 {
		return GetBusinessByID(ctx, db, id)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+1)
	for i, col := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, i+1))
		args = append(args, fields[col])
	}
	args = append(args, id)

	query := `UPDATE businesses SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d RETURNING `, len(args)) + businessColumns

	b, err := scanBusiness(db.QueryRow(ctx, query, args...))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return b, nil
}

// TouchPriceUpdatedAt, toplu fiyat guncellemesinden sonra "fiyat gecerlilik tarihini"
// simdiye ceker. Musteri menusundeki footer bu degeri kullanir.
func TouchPriceUpdatedAt(ctx context.Context, db DB, businessID uuid.UUID) (time.Time, error) {
	var updatedAt time.Time
	err := db.QueryRow(ctx,
		`UPDATE businesses SET price_updated_at = now() WHERE id = $1 RETURNING price_updated_at`,
		businessID).Scan(&updatedAt)
	return updatedAt, err
}

// LogPriceUpdate, toplu fiyat degisiminin denetim kaydini yazar.
func LogPriceUpdate(ctx context.Context, db DB, businessID uuid.UUID,
	percentage float64, rounding string, affected int) error {
	_, err := db.Exec(ctx,
		`INSERT INTO price_update_logs (business_id, percentage, rounding, affected)
		 VALUES ($1, $2, $3, $4)`,
		businessID, percentage, rounding, affected)
	return err
}
