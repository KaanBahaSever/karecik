package handlers

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Meta — GET /api/meta (public)
// Serves the fixed catalogues the dashboard needs from a single source:
// currencies, themes, fonts, allergens, badge icons, splash exit animations
// with their easings, splash and header display modes, languages and rounding
// modes.
func (h *Handler) Meta(c *fiber.Ctx) error {
	currencies := make([]utils.Currency, 0, len(utils.Currencies))
	for _, code := range []string{"TRY", "USD", "EUR", "GBP", "AZN", "RUB", "SAR", "AED"} {
		currencies = append(currencies, utils.Currencies[code])
	}

	return utils.OK(c, fiber.Map{
		"currencies":             currencies,
		"themes":                 utils.Themes,
		"fonts":                  utils.Fonts,
		"allergens":              utils.Allergens,
		"badge_icons":            utils.BadgeIcons,
		"splash_exit_animations": utils.SplashExitAnimations,
		"splash_easings":         utils.SplashEasings,
		"splash_display_modes":   utils.SplashDisplayModes,
		"header_display_modes":   utils.HeaderDisplayModes,
		"languages":              utils.Languages,
		"rounding_modes": []fiber.Map{
			{"id": utils.RoundNone, "label": "Yuvarlama yok"},
			{"id": utils.RoundInteger, "label": "Tam sayıya (148)"},
			{"id": utils.RoundNearest5, "label": "5'in katına (150)"},
			{"id": utils.RoundNearest10, "label": "10'un katına (150)"},
			{"id": utils.RoundEnds50, "label": "0,50'nin katına (147,50)"},
			{"id": utils.RoundEnds95, "label": "…,95 ile bitir (147,95)"},
			{"id": utils.RoundEnds99, "label": "…,99 ile bitir (147,99)"},
		},
	})
}

// GetBusiness — GET /api/business
func (h *Handler) GetBusiness(c *fiber.Ctx) error {
	business, err := repository.GetBusinessByID(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "İşletme bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, h.withMenuURL(business))
}

// UpdateBusiness — PUT /api/business
// Partial update: only the fields present in the body are applied.
func (h *Handler) UpdateBusiness(c *fiber.Ctx) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)
	fields := make(map[string]any)

	// --- plain text fields
	if value, ok := raw["name"]; ok {
		name, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "İşletme adı metin olmalıdır.")
		}
		name = strings.TrimSpace(name)
		if len([]rune(name)) < 2 || len([]rune(name)) > 100 {
			return utils.Unprocessable(c, "İşletme adı 2 ile 100 karakter arasında olmalıdır.")
		}
		fields["name"] = name
	}

	if value, ok := raw["slug"]; ok {
		slug, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "Menü adresi metin olmalıdır.")
		}
		slug = utils.Slugify(slug)
		if !utils.IsValidSlug(slug) {
			return utils.Unprocessable(c,
				"Menü adresi yalnızca harf, rakam ve tire içerebilir (en az 2 karakter).")
		}
		if utils.IsReservedSlug(slug) {
			return utils.Conflict(c, "Bu menü adresi sistem tarafından ayrılmıştır.")
		}
		taken, err := repository.SlugTaken(c.Context(), h.DB, slug, businessID)
		if err != nil {
			return utils.Internal(c, err)
		}
		if taken {
			return utils.Conflict(c, "Bu menü adresi başka bir işletme tarafından kullanılıyor.")
		}
		fields["slug"] = slug
	}

	// --- clearable text fields (null -> NULL)
	for _, key := range []string{
		"logo_url", "cover_url", "phone", "address", "instagram",
		"wifi_ssid", "wifi_password", "splash_logo_url",
		"background_color", "background_image_url",
	} {
		if value, ok := raw[key]; ok {
			ptr, err := decodeNullableString(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin veya boş olmalıdır.")
			}
			fields[key] = ptr
		}
	}

	// The menu background colour is optional, but when it is set it has to be
	// a colour like every other one.
	if ptr, ok := fields["background_color"].(*string); ok && ptr != nil {
		if !hexColorPattern.MatchString(*ptr) {
			return utils.Unprocessable(c, "Renk değeri #RRGGBB biçiminde olmalıdır.")
		}
	}

	// --- currency
	if value, ok := raw["currency"]; ok {
		code, err := decodeString(value)
		if err != nil || !utils.IsValidCurrency(strings.ToUpper(code)) {
			return utils.Unprocessable(c, "Geçersiz para birimi.")
		}
		fields["currency"] = strings.ToUpper(code)
	}

	// --- appearance
	if value, ok := raw["theme"]; ok {
		theme, err := decodeString(value)
		if err != nil || !utils.IsValidTheme(theme) {
			return utils.Unprocessable(c, "Geçersiz tasarım teması.")
		}
		fields["theme"] = theme
	}
	if value, ok := raw["font_family"]; ok {
		font, err := decodeString(value)
		if err != nil || !utils.IsValidFont(font) {
			return utils.Unprocessable(c, "Geçersiz yazı tipi.")
		}
		fields["font_family"] = font
	}
	for _, key := range []string{"primary_color", "splash_bg_color"} {
		if value, ok := raw[key]; ok {
			color, err := decodeString(value)
			if err != nil || !hexColorPattern.MatchString(color) {
				return utils.Unprocessable(c, "Renk değeri #RRGGBB biçiminde olmalıdır.")
			}
			fields[key] = color
		}
	}

	// --- languages
	if value, ok := raw["default_language"]; ok {
		lang, err := decodeString(value)
		if err != nil || !utils.IsValidLanguage(lang) {
			return utils.Unprocessable(c, "Geçersiz varsayılan dil.")
		}
		fields["default_language"] = lang
	}
	if value, ok := raw["languages"]; ok {
		var languages []string
		if err := json.Unmarshal(value, &languages); err != nil || len(languages) == 0 {
			return utils.Unprocessable(c, "En az bir dil seçmelisiniz.")
		}
		for _, lang := range languages {
			if !utils.IsValidLanguage(lang) {
				return utils.Unprocessable(c, "Desteklenmeyen dil kodu: "+lang)
			}
		}
		fields["languages"] = languages
	}

	// --- splash screen and footer switches
	for _, key := range []string{"splash_enabled", "show_vat_note", "show_price_date", "is_active"} {
		if value, ok := raw[key]; ok {
			flag, err := decodeBool(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = flag
		}
	}
	if value, ok := raw["splash_duration"]; ok {
		duration, err := decodeInt(value)
		if err != nil || duration < 300 || duration > 5000 {
			return utils.Unprocessable(c,
				"Karşılama süresi 300 ile 5000 milisaniye arasında olmalıdır.")
		}
		fields["splash_duration"] = duration
	}
	for _, key := range []string{"splash_text", "splash_headline", "vat_note_text"} {
		if value, ok := raw[key]; ok {
			text, err := decodeString(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin olmalıdır.")
			}
			if len([]rune(text)) > 200 {
				return utils.Unprocessable(c, "Metin en fazla 200 karakter olabilir.")
			}
			fields[key] = strings.TrimSpace(text)
		}
	}
	if value, ok := raw["splash_exit_animation"]; ok {
		animation, err := decodeString(value)
		if err != nil || !utils.IsValidSplashExitAnimation(animation) {
			return utils.Unprocessable(c, "Geçersiz çıkış animasyonu.")
		}
		fields["splash_exit_animation"] = animation
	}
	if value, ok := raw["splash_exit_easing"]; ok {
		easing, err := decodeString(value)
		if err != nil || !utils.IsValidSplashEasing(easing) {
			return utils.Unprocessable(c, "Geçersiz animasyon eğrisi.")
		}
		fields["splash_exit_easing"] = easing
	}
	if value, ok := raw["splash_display"]; ok {
		mode, err := decodeString(value)
		if err != nil || !utils.IsValidSplashDisplay(mode) {
			return utils.Unprocessable(c, "Geçersiz karşılama ekranı görünümü.")
		}
		fields["splash_display"] = mode
	}
	if value, ok := raw["splash_exit_duration"]; ok {
		duration, err := decodeInt(value)
		if err != nil || duration < 100 || duration > 2000 {
			return utils.Unprocessable(c,
				"Çıkış animasyonu süresi 100 ile 2000 milisaniye arasında olmalıdır.")
		}
		fields["splash_exit_duration"] = duration
	}

	// --- menu background
	if value, ok := raw["background_type"]; ok {
		backgroundType, err := decodeString(value)
		if err != nil || !utils.IsValidBackgroundType(backgroundType) {
			return utils.Unprocessable(c, "Geçersiz arka plan türü.")
		}
		fields["background_type"] = backgroundType
	}
	if value, ok := raw["background_overlay_opacity"]; ok {
		opacity, err := decodeFloat(value)
		if err != nil || opacity < 0 || opacity > 1 {
			return utils.Unprocessable(c, "Arka plan karartma değeri 0 ile 1 arasında olmalıdır.")
		}
		fields["background_overlay_opacity"] = opacity
	}

	// --- customer menu header
	if value, ok := raw["header_display"]; ok {
		mode, err := decodeString(value)
		if err != nil || !utils.IsValidHeaderDisplay(mode) {
			return utils.Unprocessable(c, "Geçersiz başlık görünümü.")
		}
		fields["header_display"] = mode
	}

	business, err := repository.UpdateBusiness(c.Context(), h.DB, businessID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu menü adresi zaten kullanılıyor.")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "İşletme bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	return utils.OK(c, h.withMenuURL(business))
}

// ------------------------------------------------------------ JSON decoders

func decodeString(raw json.RawMessage) (string, error) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}

// decodeNullableString maps JSON null to NULL; an empty string becomes NULL too.
func decodeNullableString(raw json.RawMessage) (*string, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	s, err := decodeString(raw)
	if err != nil {
		return nil, err
	}
	return strPtr(s), nil
}

func decodeBool(raw json.RawMessage) (bool, error) {
	var b bool
	err := json.Unmarshal(raw, &b)
	return b, err
}

// decodeUUID reads an identifier. JSON null leaves the value at uuid.Nil
// without an error, so callers reject that themselves.
func decodeUUID(raw json.RawMessage) (uuid.UUID, error) {
	var id uuid.UUID
	err := json.Unmarshal(raw, &id)
	return id, err
}

func decodeInt(raw json.RawMessage) (int, error) {
	var n int
	err := json.Unmarshal(raw, &n)
	return n, err
}

func decodeFloat(raw json.RawMessage) (float64, error) {
	var f float64
	err := json.Unmarshal(raw, &f)
	return f, err
}
