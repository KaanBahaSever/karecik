package models

import (
	"math"
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

// ----------------------------------------------------------------- sessions
//
// There is no Session model here any more. A session is not a database record:
// it lives in the API process's memory as a session.Entry, keyed by the SHA-256
// of the cookie. Everything in this file maps to a table, and a session no
// longer does — see internal/session for the store and the trade-off it makes.

// --------------------------------------------------------------- businesses

// Business is the tenant: the account and the address it answers on, nothing
// more. Every published setting (branding, splash, contact, pricing,
// languages) lives on the Menu, because a menu is what a customer opens.
//
// Slug is the subdomain of {business-slug}.karecik.com. It is GLOBALLY unique
// and it is reserved-checked with utils.IsReservedSlug, because it is a
// hostname label — unlike Menu.Slug, which is only a path segment.
type Business struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"-"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Computed — the tenant's root address, with no menu path. It has no
	// column in the database.
	HomeURL string `json:"home_url,omitempty"`
}

// ---------------------------------------------------------------- menus

// Menu is one published menu of a business (kahvaltı, akşam, bar...) and the
// primary entity of the system: it owns its categories and every setting
// below. Slug is the PATH segment of {business-slug}.karecik.com/{menu-slug},
// so it is unique only within its business — two tenants may both publish
// "kahvalti" — and it is never reserved-checked.
//
// There is no default menu. A business may own zero menus, and deleting one
// never promotes another.
type Menu struct {
	ID          uuid.UUID `json:"id"`
	BusinessID  uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	Position    int       `json:"position"`

	LogoURL  *string `json:"logo_url"`
	CoverURL *string `json:"cover_url"`

	// CurrencySymbol is derived from Currency by the repository layer and is
	// read-only to the outside world — no handler and no payload may set it.
	Currency       string `json:"currency"`
	CurrencySymbol string `json:"currency_symbol"`

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
	// SplashSlideFade only applies to the four slide-* animations: true fades
	// the panel out while it slides, false keeps it fully opaque like a
	// curtain.
	//
	// SplashEntrance is the other half of SplashExitAnimation and sits next to
	// it: how the logo and text come IN, before the hold. "fade" brings them
	// up from zero opacity — the behaviour every menu has always had, hence
	// the column default — and "none" simply draws them, with no animation at
	// all, the same off state LogoFadeIn uses in the header.
	SplashLogoURL       *string `json:"splash_logo_url"`
	SplashHeadline      string  `json:"splash_headline"`
	SplashEntrance      string  `json:"splash_entrance"`
	SplashExitAnimation string  `json:"splash_exit_animation"`
	SplashExitDuration  int     `json:"splash_exit_duration"`
	SplashExitEasing    string  `json:"splash_exit_easing"`
	SplashDisplay       string  `json:"splash_display"`
	SplashSlideFade     bool    `json:"splash_slide_fade"`

	// Menu background — a flat colour or an image behind a darkening overlay.
	// BackgroundColor nil means "inherit the theme background".
	BackgroundType           string  `json:"background_type"`
	BackgroundColor          *string `json:"background_color"`
	BackgroundImageURL       *string `json:"background_image_url"`
	BackgroundOverlayOpacity float64 `json:"background_overlay_opacity"`

	// HeaderDisplay picks what the customer menu header shows — logo, name or
	// both. LogoFadeIn brings that logo in with a short fade when the menu
	// opens; false means it is simply there, with no animation at all.
	//
	// Slogan is the one-line tagline printed under the business name, in the
	// owner's own words. It is a plain string and never a pointer: the column
	// is NOT NULL with a '' default and the header treats "no slogan" and
	// "empty slogan" identically, so the empty string is a valid value — it is
	// how a slogan is removed — and it renders nothing. HeaderDisplay does not
	// govern it: the slogan shows in all three modes.
	HeaderDisplay string `json:"header_display"`
	LogoFadeIn    bool   `json:"logo_fade_in"`
	Slogan        string `json:"slogan"`

	// TextColor tints every word on the customer menu — headings, product
	// titles and body text all read one CSS variable, so this single #RRGGBB
	// value drives the lot. It overrides the theme's own text tone.
	TextColor string `json:"text_color"`

	// ShowYerliUretim puts the "Yerli Üretim" badge and the VAT notice in the
	// menu footer. YerliUretimLogoURL is the certified artwork — an uploaded
	// '/uploads/...' path or an absolute URL; nil falls back to a plain text
	// pill, because the official mark is never drawn by this product.
	ShowYerliUretim    bool    `json:"show_yerli_uretim"`
	YerliUretimLogoURL *string `json:"yerli_uretim_logo_url"`

	ShowVatNote    bool      `json:"show_vat_note"`
	VatNoteText    string    `json:"vat_note_text"`
	ShowPriceDate  bool      `json:"show_price_date"`
	PriceUpdatedAt time.Time `json:"price_updated_at"`

	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	Instagram    *string `json:"instagram"`
	WifiSSID     *string `json:"wifi_ssid"`
	WifiPassword *string `json:"wifi_password"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Computed fields — they have no column in the database. MenuURL is the
	// full https://{business-slug}.karecik.com/{menu-slug} address, which is
	// exactly what the QR code encodes.
	CategoryCount int    `json:"category_count"`
	MenuURL       string `json:"menu_url,omitempty"`
}

// PublicMenuRef is the lightweight menu descriptor the customer menu uses to
// render a switcher when a business publishes more than one menu, and the
// tenant directory page uses to list them. Description is the subtitle under
// the name on that directory.
type PublicMenuRef struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// --------------------------------------------------------------- categories

// Category groups the products of one menu. categories.menu_id is NOT NULL
// since migration 005, so MenuID is always set on a row read from the
// database; the pointer only survives so that a request body can leave it out
// — and there is no default menu to fall back to, so the handler answers that
// with 422 rather than guessing.
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

// ---------------------------------------------------------- product options

// ProductOptionItem is one choice inside a group, priced as a surcharge on top
// of the product's own price. A zero price is normal ("Tek" portion).
type ProductOptionItem struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// ProductOptionGroup is one question asked about a product.
// Type is "single" (radio) or "multiple" (checkbox).
type ProductOptionGroup struct {
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	Required bool                `json:"required"`
	Items    []ProductOptionItem `json:"items"`
}

// ProductOptions is stored as a JSONB array on the product.
type ProductOptions []ProductOptionGroup

// Option types and limits. The dashboard mirrors the limits in
// frontend/src/components/dashboard/ProductModal.jsx.
const (
	OptionTypeSingle   = "single"
	OptionTypeMultiple = "multiple"
	MaxOptionGroups    = 8
	MaxOptionItems     = 20
	MaxOptionNameRunes = 60
)

// Normalize trims names, drops groups without a name or without items, drops
// items without a name, coerces an unknown Type to "single", rounds prices to
// two decimals and caps both lists. It always returns a non-nil slice.
//
// Name length and the price range are validated in the handler, which owns the
// Turkish messages; this only trims, coerces, rounds and caps — nothing here
// ever reports an error.
func (o ProductOptions) Normalize() ProductOptions {
	normalized := make(ProductOptions, 0, len(o))

	for _, group := range o {
		group.Name = strings.TrimSpace(group.Name)
		if group.Name == "" {
			continue
		}

		group.Type = strings.ToLower(strings.TrimSpace(group.Type))
		if group.Type != OptionTypeSingle && group.Type != OptionTypeMultiple {
			group.Type = OptionTypeSingle
		}

		items := make([]ProductOptionItem, 0, len(group.Items))
		for _, item := range group.Items {
			item.Name = strings.TrimSpace(item.Name)
			if item.Name == "" {
				continue
			}
			// Prices arrive from a text input, so 25.999999 is possible.
			item.Price = math.Round(item.Price*100) / 100

			items = append(items, item)
			if len(items) == MaxOptionItems {
				break
			}
		}

		// A group with no answers is a question nobody can answer.
		if len(items) == 0 {
			continue
		}
		group.Items = items

		normalized = append(normalized, group)
		if len(normalized) == MaxOptionGroups {
			break
		}
	}

	return normalized
}

// ----------------------------------------------------------------- products

type Product struct {
	ID           uuid.UUID      `json:"id"`
	BusinessID   uuid.UUID      `json:"-"`
	CategoryID   uuid.UUID      `json:"category_id"`
	Translations Translations   `json:"translations"`
	Price        float64        `json:"price"`
	ComparePrice *float64       `json:"compare_price"`
	Calories     *int           `json:"calories"`
	ImageURL     *string        `json:"image_url"`
	Allergens    []string       `json:"allergens"`
	Badges       Badges         `json:"badges"`
	Options      ProductOptions `json:"options"`
	IsActive     bool           `json:"is_active"`
	IsFeatured   bool           `json:"is_featured"`
	Position     int            `json:"position"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ------------------------------------------------ customer-facing menu DTOs
// On the public endpoints the translations are already resolved: instead of the
// translations map the payload carries plain name / description fields.

type PublicMenu struct {
	Business   PublicBusiness   `json:"business"`
	Categories []PublicCategory `json:"categories"`
	Footer     PublicFooter     `json:"footer"`

	// Menus lists every active menu of the same business, in position order.
	// Always non-nil; empty when the tenant publishes nothing.
	Menus []PublicMenuRef `json:"menus"`

	// MenuResolved reports whether Categories and the settings on Business
	// actually come from a menu. It is false when the request identified a
	// business but no single menu — no menu slug in the path and either zero
	// or two-plus active menus — which is what makes the frontend render the
	// tenant directory instead of a menu.
	MenuResolved bool `json:"menu_resolved"`
}

// PublicBusiness is the header block of the customer payload. Its settings are
// built from a Menu, not from a Business: Name and Slug are the menu's name
// and the menu's slug, because the menu is the venue the customer is looking
// at. The tenant identity travels alongside them in BusinessName /
// BusinessSlug. The field set is kept verbatim because the customer frontend
// reads it field by field.
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

	// SplashEntrance mirrors the field of the same name on Menu — the splash
	// screen reads it to decide whether its content fades in or is simply
	// there. See the note next to it.
	SplashLogoURL       *string `json:"splash_logo_url"`
	SplashHeadline      string  `json:"splash_headline"`
	SplashEntrance      string  `json:"splash_entrance"`
	SplashExitAnimation string  `json:"splash_exit_animation"`
	SplashExitDuration  int     `json:"splash_exit_duration"`
	SplashExitEasing    string  `json:"splash_exit_easing"`
	SplashDisplay       string  `json:"splash_display"`
	SplashSlideFade     bool    `json:"splash_slide_fade"`

	BackgroundType           string  `json:"background_type"`
	BackgroundColor          *string `json:"background_color"`
	BackgroundImageURL       *string `json:"background_image_url"`
	BackgroundOverlayOpacity float64 `json:"background_overlay_opacity"`

	// HeaderDisplay, LogoFadeIn and Slogan mirror the three header fields on
	// Menu — the customer view reads them to lay its header out. See the notes
	// there; an empty Slogan renders nothing.
	HeaderDisplay string `json:"header_display"`
	LogoFadeIn    bool   `json:"logo_fade_in"`
	Slogan        string `json:"slogan"`

	// TextColor, ShowYerliUretim and YerliUretimLogoURL mirror the same three
	// fields on Menu — the customer view reads them to tint its text and to
	// build the legal footer. See the notes there.
	TextColor          string  `json:"text_color"`
	ShowYerliUretim    bool    `json:"show_yerli_uretim"`
	YerliUretimLogoURL *string `json:"yerli_uretim_logo_url"`

	ShowVatNote    bool      `json:"show_vat_note"`
	VatNoteText    string    `json:"vat_note_text"`
	ShowPriceDate  bool      `json:"show_price_date"`
	PriceUpdatedAt time.Time `json:"price_updated_at"`

	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	Instagram    *string `json:"instagram"`
	WifiSSID     *string `json:"wifi_ssid"`
	WifiPassword *string `json:"wifi_password"`

	// The tenant, and the menu this payload was built from. BusinessName and
	// BusinessSlug are always filled — even when no menu resolved, which is
	// what lets the directory page title itself; MenuSlug is empty in that
	// case. Together they spell the real address,
	// {business_slug}.karecik.com/{menu_slug}, without a second request.
	BusinessName string `json:"business_name"`
	BusinessSlug string `json:"business_slug"`
	MenuSlug     string `json:"menu_slug"`

	// MenuName is the menu name again — Name above carries it too, and both
	// stay because the frontend reads them field by field.
	MenuName *string `json:"menu_name"`
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
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Ingredients  string         `json:"ingredients,omitempty"`
	Price        float64        `json:"price"`
	ComparePrice *float64       `json:"compare_price"`
	Calories     *int           `json:"calories"`
	ImageURL     *string        `json:"image_url"`
	Allergens    []string       `json:"allergens"`
	Badges       Badges         `json:"badges"`
	Options      ProductOptions `json:"options"`
	IsFeatured   bool           `json:"is_featured"`
	IsActive     bool           `json:"is_active"`
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
