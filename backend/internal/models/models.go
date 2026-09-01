package models

import (
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

	ShowVatNote    bool      `json:"show_vat_note"`
	VatNoteText    string    `json:"vat_note_text"`
	ShowPriceDate  bool      `json:"show_price_date"`
	PriceUpdatedAt time.Time `json:"price_updated_at"`

	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	Instagram    *string `json:"instagram"`
	WifiPassword *string `json:"wifi_password"`

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Computed field — it has no column in the database
	MenuURL string `json:"menu_url,omitempty"`
}

// --------------------------------------------------------------- categories

type Category struct {
	ID           uuid.UUID    `json:"id"`
	BusinessID   uuid.UUID    `json:"-"`
	Translations Translations `json:"translations"`
	Icon         *string      `json:"icon"`
	ImageURL     *string      `json:"image_url"`
	Position     int          `json:"position"`
	IsActive     bool         `json:"is_active"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`

	ProductCount int `json:"product_count"`
}

// ----------------------------------------------------------------- products

type Product struct {
	ID           uuid.UUID    `json:"id"`
	BusinessID   uuid.UUID    `json:"-"`
	CategoryID   uuid.UUID    `json:"category_id"`
	Translations Translations `json:"translations"`
	Price        float64      `json:"price"`
	ComparePrice *float64     `json:"compare_price"`
	ImageURL     *string      `json:"image_url"`
	Allergens    []string     `json:"allergens"`
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

	ShowVatNote    bool      `json:"show_vat_note"`
	VatNoteText    string    `json:"vat_note_text"`
	ShowPriceDate  bool      `json:"show_price_date"`
	PriceUpdatedAt time.Time `json:"price_updated_at"`

	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	Instagram    *string `json:"instagram"`
	WifiPassword *string `json:"wifi_password"`
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
	ImageURL     *string   `json:"image_url"`
	Allergens    []string  `json:"allergens"`
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
