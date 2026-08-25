package handlers

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Meta — GET /api/meta (herkese acik)
// Panelin ihtiyac duydugu sabit listeler tek kaynaktan gelir:
// para birimleri, temalar, yazi tipleri, alerjenler, diller, yuvarlama modlari.
func (h *Handler) Meta(c *fiber.Ctx) error {
	currencies := make([]utils.Currency, 0, len(utils.Currencies))
	for _, code := range []string{"TRY", "USD", "EUR", "GBP", "AZN", "RUB", "SAR", "AED"} {
		currencies = append(currencies, utils.Currencies[code])
	}

	return utils.OK(c, fiber.Map{
		"currencies": currencies,
		"themes":     utils.Themes,
		"fonts":      utils.Fonts,
		"allergens":  utils.Allergens,
		"languages":  utils.Languages,
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
// Kismi guncelleme: yalnizca govdede bulunan alanlar uygulanir.
func (h *Handler) UpdateBusiness(c *fiber.Ctx) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)
	fields := make(map[string]any)

	// --- metin alanlari
	if v, ok := raw["name"]; ok {
		name, err := decodeString(v)
		if err != nil {
			return utils.Unprocessable(c, "İşletme adı metin olmalıdır.")
		}
		name = strings.TrimSpace(name)
		if len([]rune(name)) < 2 || len([]rune(name)) > 100 {
			return utils.Unprocessable(c, "İşletme adı 2 ile 100 karakter arasında olmalıdır.")
		}
		fields["name"] = name
	}

	if v, ok := raw["slug"]; ok {
		slug, err := decodeString(v)
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

	// --- bosaltilabilir metinler (null -> NULL)
	for _, key := range []string{"logo_url", "cover_url", "phone", "address", "instagram", "wifi_password"} {
		if v, ok := raw[key]; ok {
			ptr, err := decodeNullableString(v)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin veya boş olmalıdır.")
			}
			fields[key] = ptr
		}
	}

	// --- para birimi
	if v, ok := raw["currency"]; ok {
		code, err := decodeString(v)
		if err != nil || !utils.IsValidCurrency(strings.ToUpper(code)) {
			return utils.Unprocessable(c, "Geçersiz para birimi.")
		}
		fields["currency"] = strings.ToUpper(code)
	}

	// --- gorunum
	if v, ok := raw["theme"]; ok {
		theme, err := decodeString(v)
		if err != nil || !utils.IsValidTheme(theme) {
			return utils.Unprocessable(c, "Geçersiz tasarım teması.")
		}
		fields["theme"] = theme
	}
	if v, ok := raw["font_family"]; ok {
		font, err := decodeString(v)
		if err != nil || !utils.IsValidFont(font) {
			return utils.Unprocessable(c, "Geçersiz yazı tipi.")
		}
		fields["font_family"] = font
	}
	for _, key := range []string{"primary_color", "splash_bg_color"} {
		if v, ok := raw[key]; ok {
			color, err := decodeString(v)
			if err != nil || !hexColorPattern.MatchString(color) {
				return utils.Unprocessable(c, "Renk değeri #RRGGBB biçiminde olmalıdır.")
			}
			fields[key] = color
		}
	}

	// --- diller
	if v, ok := raw["default_language"]; ok {
		lang, err := decodeString(v)
		if err != nil || !utils.IsValidLanguage(lang) {
			return utils.Unprocessable(c, "Geçersiz varsayılan dil.")
		}
		fields["default_language"] = lang
	}
	if v, ok := raw["languages"]; ok {
		var langs []string
		if err := json.Unmarshal(v, &langs); err != nil || len(langs) == 0 {
			return utils.Unprocessable(c, "En az bir dil seçmelisiniz.")
		}
		for _, l := range langs {
			if !utils.IsValidLanguage(l) {
				return utils.Unprocessable(c, "Desteklenmeyen dil kodu: "+l)
			}
		}
		fields["languages"] = langs
	}

	// --- karsilama animasyonu
	for _, key := range []string{"splash_enabled", "show_vat_note", "show_price_date", "is_active"} {
		if v, ok := raw[key]; ok {
			b, err := decodeBool(v)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = b
		}
	}
	if v, ok := raw["splash_duration"]; ok {
		d, err := decodeInt(v)
		if err != nil || d < 300 || d > 5000 {
			return utils.Unprocessable(c,
				"Karşılama süresi 300 ile 5000 milisaniye arasında olmalıdır.")
		}
		fields["splash_duration"] = d
	}
	for _, key := range []string{"splash_text", "vat_note_text"} {
		if v, ok := raw[key]; ok {
			text, err := decodeString(v)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin olmalıdır.")
			}
			if len([]rune(text)) > 200 {
				return utils.Unprocessable(c, "Metin en fazla 200 karakter olabilir.")
			}
			fields[key] = strings.TrimSpace(text)
		}
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

// ------------------------------------------------------- JSON cozumleyiciler

func decodeString(raw json.RawMessage) (string, error) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}

// decodeNullableString, null degerini NULL olarak yorumlar; bos metin de NULL olur.
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
