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

// hexColorPattern guards every colour the dashboard sends — menu settings and
// product badges alike.
var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Meta — GET /api/meta (public)
// Serves the fixed catalogues the dashboard needs from a single source:
// currencies, themes, fonts, allergens, badge icons, splash exit animations
// with their easings and slide styles, splash and header display modes,
// languages and rounding modes.
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
		"slide_fade_modes":       utils.SlideFadeModes,
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
// The account: its name, the slug it answers on — the subdomain of
// {business-slug}.karecik.com — and the computed root address. Every published
// setting lives on the menus and is read and written through /api/menus.
func (h *Handler) GetBusiness(c *fiber.Ctx) error {
	business, err := repository.GetBusinessByID(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "İşletme bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, h.withHomeURL(business))
}

// UpdateBusiness — PUT /api/business
// Exactly two fields may be written, because the account owns exactly two: the
// display name and the slug. Everything a customer sees belongs to a menu and
// goes through /api/menus/:id.
//
// The slug here IS a hostname label, so — unlike a menu slug — it is
// reserved-checked and its namespace is global: an address somebody else
// already answers on is a 409, never a silent numeric suffix.
func (h *Handler) UpdateBusiness(c *fiber.Ctx) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)
	fields := make(map[string]any)

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
		text, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "İşletme adresi metin olmalıdır.")
		}
		slug := utils.Slugify(text)
		if !utils.IsValidSlug(slug) {
			return utils.Unprocessable(c,
				"İşletme adresi yalnızca harf, rakam ve tire içerebilir (en az 2 karakter).")
		}
		if utils.IsReservedSlug(slug) {
			return utils.Conflict(c, "Bu adres sistem tarafından ayrılmıştır.")
		}
		taken, err := repository.BusinessSlugTaken(c.Context(), h.DB, slug, businessID)
		if err != nil {
			return utils.Internal(c, err)
		}
		if taken {
			return utils.Conflict(c, "Bu adres başka bir işletme tarafından kullanılıyor.")
		}
		fields["slug"] = slug
	}

	business, err := repository.UpdateBusiness(c.Context(), h.DB, businessID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu adres başka bir işletme tarafından kullanılıyor.")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "İşletme bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, h.withHomeURL(business))
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
