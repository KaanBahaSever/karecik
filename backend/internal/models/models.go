package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ------------------------------------------------------------- translations

// Translation holds the texts of a category or product in a single language.
type Translation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Ingredients string `json:"ingredients,omitempty"`
}

// Translations maps a language code (tr, en, de...) to its texts.
// It is stored as JSONB in the database.
type Translations map[string]Translation

// Resolve returns the texts for the requested language.
// If that language is missing it falls back to `fallback`, and finally to the
// first non-empty entry in the map.
func (t Translations) Resolve(lang, fallback string) Translation {
	if tr, ok := t[lang]; ok && tr.Name != "" {
		return tr
	}
	if tr, ok := t[fallback]; ok && tr.Name != "" {
		return tr
	}
	for _, tr := range t {
		if tr.Name != "" {
			return tr
		}
	}
	return Translation{}
}

// HasName reports whether a non-empty name exists for the given language.
func (t Translations) HasName(lang string) bool {
	tr, ok := t[lang]
	return ok && tr.Name != ""
}

// -------------------------------------------------------------------- users

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	BusinessName string    `json:"business_name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// --------------------------------------------------------------- businesses

type Business struct {
	ID     uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"-"`
	Name   string    `json:"name"`
	Slug   string    `json:"slug"`

	LogoURL  *string `json:"logo_url"`
	CoverURL *string `json:"cover_url"`

	Currency string `json:"currency"`

	Theme        string `json:"theme"`
	FontFamily   string `json:"font_family"`
	PrimaryColor string `json:"primary_color"`

	DefaultLanguage string   `json:"default_language"`
	Languages       []string `json:"languages"`

	SplashEnabled  bool   `json:"splash_enabled"`
	SplashDuration int    `json:"splash_duration"`
	SplashBgColor  string `json:"splash_bg_color"`
	SplashText     string `json:"splash_text"`

	// SplashText is the tagline under the headline; SplashHeadline is the
	// larger line and SplashDuration the hold time before the exit animation.
	// SplashExitEasing is the timing function of that exit animation and
	// SplashDisplay picks what the screen shows — logo, text or both.
	SplashLogoURL       *string `json:"splash_logo_url"`
	SplashHeadline      string  `json:"splash_headline"`
	SplashExitAnimation string  `json:"splash_exit_animation"`
	SplashExitDuration  int     `json:"splash_exit_duration"`
	SplashExitEasing    string  `json:"splash_exit_easing"`
	SplashDisplay       string  `json:"splash_display"`

	// Menu background — a flat colour or an image behind a darkening overlay.
	// BackgroundColor nil means "inherit the theme background".
	BackgroundType           string  `json:"background_type"`
	BackgroundColor          *string `json:"background_color"`
	BackgroundImageURL       *string `json:"background_image_url"`
	BackgroundOverlayOpacity float64 `json:"background_overlay_opacity"`

	// HeaderDisplay picks what the customer menu header shows — logo, name or
	// both.
	HeaderDisplay string `json:"header_display"`

	ShowVatNote    bool      `json:"show_vat_note"`
	VatNoteText    string    `json:"vat_note_text"`
	ShowPriceDate  bool      `json:"show_price_date"`
	PriceUpdatedAt time.Time `json:"price_updated_at"`

	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	Instagram    *string `json:"instagram"`
	WifiSSID     *string `json:"wifi_ssid"`
	WifiPassword *string `json:"wifi_password"`

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Computed field — it has no column in the database
	MenuURL string `json:"menu_url,omitempty"`
}

// ------------------------------------------------------- branches and menus

// Menu is one published menu of a business (kahvaltı, akşam, bar...). Every
// category belongs to a menu; exactly one menu per business is the default.
type Menu struct {
	ID          uuid.UUID `json:"id"`
	BusinessID  uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"is_default"`
	IsActive    bool      `json:"is_active"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Computed field — it has no column in the database
	CategoryCount int `json:"category_count"`
}

// Branch is one physical location of a business. Its Slug is globally unique
// because it is a subdomain: {branch}.karecik.com resolves without a business
// qualifier, and branch slugs are looked up before business slugs.
type Branch struct {
	ID           uuid.UUID `json:"id"`
	BusinessID   uuid.UUID `json:"-"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Phone        *string   `json:"phone"`
	Address      *string   `json:"address"`
	WifiSSID     *string   `json:"wifi_ssid"`
	WifiPassword *string   `json:"wifi_password"`
	IsDefault    bool      `json:"is_default"`
	IsActive     bool      `json:"is_active"`
	Position     int       `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Computed fields — they have no column in the database
	MenuIDs []uuid.UUID `json:"menu_ids"`
	MenuURL string      `json:"menu_url,omitempty"`
}

// BranchPrice overrides a product's price at one branch.
// A nil Price means "inherit the product's own price".
type BranchPrice struct {
	BranchID     uuid.UUID `json:"branch_id"`
	ProductID    uuid.UUID `json:"product_id"`
	Price        *float64  `json:"price"`
	ComparePrice *float64  `json:"compare_price"`
	IsAvailable  bool      `json:"is_available"`
}

// MenuRef is the lightweight menu descriptor the customer menu uses to render
// a switcher when a branch serves more than one menu.
type MenuRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// --------------------------------------------------------------- categories

// Category groups the products of one menu. MenuID is a pointer because the
// column is nullable: a category that was never assigned to a menu still
// belongs to the business and shows on its default menu.
type Category struct {
	ID           uuid.UUID    `json:"id"`
	BusinessID   uuid.UUID    `json:"-"`
	MenuID       *uuid.UUID   `json:"menu_id"`
	Translations Translations `json:"translations"`
	Icon         *string      `json:"icon"`
	ImageURL     *string      `json:"image_url"`
	Position     int          `json:"position"`
	IsActive     bool         `json:"is_active"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`

	ProductCount int `json:"product_count"`
}

// ------------------------------------------------------------------- badges

// Badge limits. The dashboard mirrors them in frontend/src/themes/badges.js.
const (
	MaxBadges         = 5
	MaxBadgeTextRunes = 24
)

// Badge is a free-form label the business designs itself, shown on the product
// card next to the fixed allergen icons.
type Badge struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Icon      string `json:"icon,omitempty"` // id from utils.BadgeIcons, may be empty
	BgColor   string `json:"bg_color"`
	TextColor string `json:"text_color"`
}

// Badges is stored as a JSONB array on the product.
type Badges []Badge

// Normalize drops badges without text, trims the fields, caps the list at
// MaxBadges and guarantees a non-nil slice.
//
// Colour validation lives in the handler, which owns the hex pattern; this
// only trims and truncates.
func (b Badges) Normalize() Badges {
	normalized := make(Badges, 0, len(b))

	for _, badge := range b {
		badge.ID = strings.TrimSpace(badge.ID)
		badge.Text = strings.TrimSpace(badge.Text)
		badge.Icon = strings.TrimSpace(badge.Icon)
		badge.BgColor = strings.TrimSpace(badge.BgColor)
		badge.TextColor = strings.TrimSpace(badge.TextColor)

		if badge.Text == "" {
			continue
		}
		if runes := []rune(badge.Text); len(runes) > MaxBadgeTextRunes {
			badge.Text = string(runes[:MaxBadgeTextRunes])
		}

		normalized = append(normalized, badge)
		if len(normalized) == MaxBadges {
			break
		}
	}

	return normalized
}

// ----------------------------------------------------------------- products

type Product struct {
	ID           uuid.UUID    `json:"id"`
	BusinessID   uuid.UUID    `json:"-"`
	CategoryID   uuid.UUID    `json:"category_id"`
	Translations Translations `json:"translations"`
	Price        float64      `json:"price"`
	ComparePrice *float64     `json:"compare_price"`
	Calories     *int         `json:"calories"`
	ImageURL     *string      `json:"image_url"`
	Allergens    []string     `json:"allergens"`
	Badges       Badges       `json:"badges"`
	IsActive     bool         `json:"is_active"`
	IsFeatured   bool         `json:"is_featured"`
	Position     int          `json:"position"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ------------------------------------------------ customer-facing menu DTOs
// On the public endpoints the translations are already resolved: instead of the
// translations map the payload carries plain name / description fields.

type PublicMenu struct {
	Business   PublicBusiness   `json:"business"`
	Categories []PublicCategory `json:"categories"`
	Footer     PublicFooter     `json:"footer"`

	// Menus lists the other menus reachable from this context. Always non-nil;
	// empty when there is nothing to switch between.
	Menus []MenuRef `json:"menus"`
}

type PublicBusiness struct {
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	LogoURL        *string `json:"logo_url"`
	CoverURL       *string `json:"cover_url"`
	Currency       string  `json:"currency"`
	CurrencySymbol string  `json:"currency_symbol"`
	Theme          string  `json:"theme"`
	FontFamily     string  `json:"font_family"`
	PrimaryColor   string  `json:"primary_color"`

	DefaultLanguage string   `json:"default_language"`
	Languages       []string `json:"languages"`

	SplashEnabled  bool   `json:"splash_enabled"`
	SplashDuration int    `json:"splash_duration"`
	SplashBgColor  string `json:"splash_bg_color"`
	SplashText     string `json:"splash_text"`

	SplashLogoURL       *string `json:"splash_logo_url"`
	SplashHeadline      string  `json:"splash_headline"`
	SplashExitAnimation string  `json:"splash_exit_animation"`
	SplashExitDuration  int     `json:"splash_exit_duration"`
	SplashExitEasing    string  `json:"splash_exit_easing"`
	SplashDisplay       string  `json:"splash_display"`

	BackgroundType           string  `json:"background_type"`
	BackgroundColor          *string `json:"background_color"`
	BackgroundImageURL       *string `json:"background_image_url"`
	BackgroundOverlayOpacity float64 `json:"background_overlay_opacity"`

	HeaderDisplay string `json:"header_display"`

	ShowVatNote    bool      `json:"show_vat_note"`
	VatNoteText    string    `json:"vat_note_text"`
	ShowPriceDate  bool      `json:"show_price_date"`
	PriceUpdatedAt time.Time `json:"price_updated_at"`

	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	Instagram    *string `json:"instagram"`
	WifiSSID     *string `json:"wifi_ssid"`
	WifiPassword *string `json:"wifi_password"`

	// Set when the menu is served through a branch and/or a named menu.
	BranchName *string `json:"branch_name"`
	BranchSlug *string `json:"branch_slug"`
	MenuName   *string `json:"menu_name"`
	MenuSlug   *string `json:"menu_slug"`
}

type PublicCategory struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Icon        *string         `json:"icon"`
	ImageURL    *string         `json:"image_url"`
	IsActive    bool            `json:"is_active"`
	Products    []PublicProduct `json:"products"`
}

type PublicProduct struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Ingredients  string    `json:"ingredients,omitempty"`
	Price        float64   `json:"price"`
	ComparePrice *float64  `json:"compare_price"`
	Calories     *int      `json:"calories"`
	ImageURL     *string   `json:"image_url"`
	Allergens    []string  `json:"allergens"`
	Badges       Badges    `json:"badges"`
	IsFeatured   bool      `json:"is_featured"`
	IsActive     bool      `json:"is_active"`
}

type PublicFooter struct {
	PriceNote string `json:"price_note"`
	VatNote   string `json:"vat_note"`
	PoweredBy string `json:"powered_by"`
}

// --------------------------------------------------------- bulk price update

type PriceChangePreview struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	OldPrice float64   `json:"old_price"`
	NewPrice float64   `json:"new_price"`
}

type BulkPriceResult struct {
	Applied        bool                 `json:"applied"`
	Affected       int                  `json:"affected"`
	Preview        []PriceChangePreview `json:"preview"`
	PriceUpdatedAt *time.Time           `json:"price_updated_at,omitempty"`
}
