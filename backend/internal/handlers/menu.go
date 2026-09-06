package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// A menu is what a customer opens: it owns its categories and every setting
// they see, and it answers on {business-slug}.karecik.com/{menu-slug}. The
// subdomain half belongs to the account, so a menu slug is only a path segment
// — unique within its business and never reserved-checked.
//
// A business may own zero menus, permanently. Nothing here creates one on
// anybody's behalf and nothing here refuses to delete the last one.

// ListMenus — GET /api/menus
// The list is legitimately empty for a business that publishes nothing.
func (h *Handler) ListMenus(c *fiber.Ctx) error {
	business, err := h.currentBusiness(c)
	if err != nil {
		return h.businessError(c, err)
	}

	menus, err := repository.ListMenus(c.Context(), h.DB, business.ID)
	if err != nil {
		return utils.Internal(c, err)
	}
	for i := range menus {
		h.withMenuURL(business.Slug, &menus[i])
	}
	return utils.OK(c, menus)
}

// GetMenu — GET /api/menus/:id
func (h *Handler) GetMenu(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz menü kimliği.")
	}

	business, err := h.currentBusiness(c)
	if err != nil {
		return h.businessError(c, err)
	}

	menu, err := repository.GetMenu(c.Context(), h.DB, id, business.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, h.withMenuURL(business.Slug, menu))
}

// CreateMenu — POST /api/menus
// The body may carry any setting a menu owns; whatever it leaves out keeps the
// schema default, so a new menu starts from the same values every existing one
// was given.
//
// The address is never a reason to refuse the request: an omitted slug is
// derived from the name and a slug already used inside this business gets a
// numeric suffix. What was actually taken comes back on the created menu.
func (h *Handler) CreateMenu(c *fiber.Ctx) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	// An empty slug means "derive it from the name".
	if value, ok := raw["slug"]; ok {
		if text, err := decodeString(value); err == nil && strings.TrimSpace(text) == "" {
			delete(raw, "slug")
		}
	}

	business, err := h.currentBusiness(c)
	if err != nil {
		return h.businessError(c, err)
	}

	fields, ok, err := menuFieldsFrom(c, raw)
	if !ok {
		return err
	}

	name, ok := fields["name"].(string)
	if !ok {
		return utils.Unprocessable(c, validateMenuName(""))
	}
	if _, ok := fields["slug"]; !ok {
		fields["slug"] = utils.SlugifyWithFallback(name, "menu")
	}

	// The suffix search runs inside this business only, so a menu called
	// "kahvalti" under another tenant is none of its concern. CreateMenu
	// resolves it once more under its own retry, which is what keeps the
	// composite unique index a safety net rather than an error path.
	slug, err := repository.EnsureUniqueMenuSlug(c.Context(), h.DB, business.ID,
		fields["slug"].(string), nil)
	if err != nil {
		return utils.Internal(c, err)
	}
	fields["slug"] = slug

	menu, err := repository.CreateMenu(c.Context(), h.DB, business.ID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu menü adresi bu işletmede zaten kullanılıyor.")
		}
		return utils.Internal(c, err)
	}
	return utils.Created(c, h.withMenuURL(business.Slug, menu))
}

// UpdateMenu — PUT /api/menus/:id
// Partial update: only the fields present in the body are applied.
func (h *Handler) UpdateMenu(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz menü kimliği.")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	business, err := h.currentBusiness(c)
	if err != nil {
		return h.businessError(c, err)
	}

	// The menu has to belong to this business before anything is written — and
	// before its own id is excluded from the slug search.
	if _, err := repository.GetMenu(c.Context(), h.DB, id, business.ID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	fields, ok, err := menuFieldsFrom(c, raw)
	if !ok {
		return err
	}

	// Excluding the menu itself is what makes re-saving an unchanged address a
	// no-op instead of a bump to -2.
	if requested, ok := fields["slug"].(string); ok {
		slug, err := repository.EnsureUniqueMenuSlug(c.Context(), h.DB, business.ID,
			requested, &id)
		if err != nil {
			return utils.Internal(c, err)
		}
		fields["slug"] = slug
	}

	menu, err := repository.UpdateMenu(c.Context(), h.DB, id, business.ID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu menü adresi bu işletmede zaten kullanılıyor.")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, h.withMenuURL(business.Slug, menu))
}

// DeleteMenu — DELETE /api/menus/:id
// The categories of the menu — and the products inside them — are removed
// along with it, so the dashboard asks for a confirmation first.
//
// Every menu can be deleted, the last one included: a business owning no menus
// is a legitimate state, and no other menu is promoted in its place.
func (h *Handler) DeleteMenu(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz menü kimliği.")
	}

	err = repository.DeleteMenu(c.Context(), h.DB, id, middleware.BusinessID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, fiber.Map{"success": true})
}

// businessError maps a failed account lookup onto its response. The token
// carries a business id, so a miss means the account is gone underneath the
// session.
func (h *Handler) businessError(c *fiber.Ctx, err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return utils.NotFound(c, "İşletme bulunamadı.")
	}
	return utils.Internal(c, err)
}

// ----------------------------------------------------------- validation

// menuFieldsFrom validates a partial menu payload and returns the columns to
// write. Only the keys present in the body are looked at, so both the create
// and the update path can use it.
//
// currency_symbol is deliberately not read: the repository derives it from the
// currency code, so a body carrying it is ignored rather than rejected. So is
// the uniqueness of the slug, which is resolved — never refused — by
// repository.EnsureUniqueMenuSlug once the business is known.
//
// It answers the request itself when it refuses it, which the false `ok`
// reports to the caller.
func menuFieldsFrom(c *fiber.Ctx, raw map[string]json.RawMessage) (map[string]any, bool, error) {
	fields := make(map[string]any)

	// --- identity
	if value, ok := raw["name"]; ok {
		name, err := decodeString(value)
		if err != nil {
			return nil, false, utils.Unprocessable(c, "Menü adı metin olmalıdır.")
		}
		name = strings.TrimSpace(name)
		if errMessage := validateMenuName(name); errMessage != "" {
			return nil, false, utils.Unprocessable(c, errMessage)
		}
		fields["name"] = name
	}

	// A menu slug is a PATH SEGMENT under the tenant's own subdomain, so it is
	// cleaned and shape-checked but never compared against utils.IsReservedSlug:
	// "admin" is a perfectly good menu name under somebody's subdomain. Only the
	// business slug is a hostname label and reserved-checked.
	if value, ok := raw["slug"]; ok {
		text, err := decodeString(value)
		if err != nil {
			return nil, false, utils.Unprocessable(c, "Menü adresi metin olmalıdır.")
		}
		slug := utils.Slugify(text)
		if !utils.IsValidSlug(slug) {
			return nil, false, utils.Unprocessable(c,
				"Menü adresi yalnızca harf, rakam ve tire içerebilir (en az 2 karakter).")
		}
		fields["slug"] = slug
	}

	if value, ok := raw["description"]; ok {
		description, err := decodeString(value)
		if err != nil {
			return nil, false, utils.Unprocessable(c, "Menü açıklaması metin olmalıdır.")
		}
		if len([]rune(description)) > 200 {
			return nil, false, utils.Unprocessable(c,
				"Menü açıklaması en fazla 200 karakter olabilir.")
		}
		fields["description"] = strings.TrimSpace(description)
	}

	// --- clearable text fields (null -> NULL)
	//
	// yerli_uretim_logo_url belongs here rather than to a shape check of its
	// own: it holds an uploaded '/uploads/...' path just as often as an
	// absolute URL, so the only thing that can be said about it is that it is
	// either text or absent — and absent is how a business removes the badge
	// artwork and falls back to the plain text pill.
	for _, key := range []string{
		"logo_url", "cover_url", "phone", "address", "instagram",
		"wifi_ssid", "wifi_password", "splash_logo_url",
		"background_color", "background_image_url",
		"yerli_uretim_logo_url",
	} {
		if value, ok := raw[key]; ok {
			ptr, err := decodeNullableString(value)
			if err != nil {
				return nil, false, utils.Unprocessable(c, key+" alanı metin veya boş olmalıdır.")
			}
			fields[key] = ptr
		}
	}

	// The menu background colour is optional, but when it is set it has to be
	// a colour like every other one.
	if ptr, ok := fields["background_color"].(*string); ok && ptr != nil {
		if !hexColorPattern.MatchString(*ptr) {
			return nil, false, utils.Unprocessable(c, "Renk değeri #RRGGBB biçiminde olmalıdır.")
		}
	}

	// --- currency
	if value, ok := raw["currency"]; ok {
		code, err := decodeString(value)
		if err != nil || !utils.IsValidCurrency(strings.ToUpper(code)) {
			return nil, false, utils.Unprocessable(c, "Geçersiz para birimi.")
		}
		fields["currency"] = strings.ToUpper(code)
	}

	// --- appearance
	if value, ok := raw["theme"]; ok {
		theme, err := decodeString(value)
		if err != nil || !utils.IsValidTheme(theme) {
			return nil, false, utils.Unprocessable(c, "Geçersiz tasarım teması.")
		}
		fields["theme"] = theme
	}
	if value, ok := raw["font_family"]; ok {
		font, err := decodeString(value)
		if err != nil || !utils.IsValidFont(font) {
			return nil, false, utils.Unprocessable(c, "Geçersiz yazı tipi.")
		}
		fields["font_family"] = font
	}
	// text_color joins the loop because it is a colour like the other two, and
	// it is the one of the three the DATABASE also checks: migration 006
	// attaches menus_text_color_check with exactly the pattern hexColorPattern
	// holds. Both layers therefore refuse the same values, so a colour this
	// loop lets through can never turn into a constraint violation — a 500
	// where the caller has already earned a 422.
	for _, key := range []string{"primary_color", "splash_bg_color", "text_color"} {
		if value, ok := raw[key]; ok {
			color, err := decodeString(value)
			if err != nil || !hexColorPattern.MatchString(color) {
				return nil, false, utils.Unprocessable(c,
					"Renk değeri #RRGGBB biçiminde olmalıdır.")
			}
			fields[key] = color
		}
	}

	// --- languages
	if value, ok := raw["default_language"]; ok {
		lang, err := decodeString(value)
		if err != nil || !utils.IsValidLanguage(lang) {
			return nil, false, utils.Unprocessable(c, "Geçersiz varsayılan dil.")
		}
		fields["default_language"] = lang
	}
	if value, ok := raw["languages"]; ok {
		var languages []string
		if err := json.Unmarshal(value, &languages); err != nil || len(languages) == 0 {
			return nil, false, utils.Unprocessable(c, "En az bir dil seçmelisiniz.")
		}
		for _, lang := range languages {
			if !utils.IsValidLanguage(lang) {
				return nil, false, utils.Unprocessable(c, "Desteklenmeyen dil kodu: "+lang)
			}
		}
		fields["languages"] = languages
	}

	// --- splash screen, header, footer and publication switches
	//
	// logo_fade_in belongs to the customer menu header rather than the splash
	// screen, and show_yerli_uretim to the footer, but both are the same plain
	// true/false decode, so they join the loop instead of repeating it.
	for _, key := range []string{
		"splash_enabled", "splash_slide_fade",
		"logo_fade_in",
		"show_vat_note", "show_price_date", "show_yerli_uretim",
		"is_active",
	} {
		if value, ok := raw[key]; ok {
			flag, err := decodeBool(value)
			if err != nil {
				return nil, false, utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = flag
		}
	}
	if value, ok := raw["splash_duration"]; ok {
		duration, err := decodeInt(value)
		if err != nil || duration < 300 || duration > 5000 {
			return nil, false, utils.Unprocessable(c,
				"Karşılama süresi 300 ile 5000 milisaniye arasında olmalıdır.")
		}
		fields["splash_duration"] = duration
	}
	// --- bounded free text
	//
	// Every one of these is plain prose the business writes itself, so the only
	// thing to check is the length — and each carries its own limit and its own
	// message, because the slogan is shorter than the other three and says so
	// in its own words rather than through the generic one.
	//
	// An empty string is a VALID value throughout: clearing the field is how a
	// splash tagline, a VAT note or a slogan is removed, so "" is stored rather
	// than refused. The cap is measured in RUNES, not bytes, so "ş" costs one
	// character and not two.
	for _, field := range []struct {
		key     string
		limit   int
		message string
	}{
		{"splash_text", 200, "Metin en fazla 200 karakter olabilir."},
		{"splash_headline", 200, "Metin en fazla 200 karakter olabilir."},
		{"vat_note_text", 200, "Metin en fazla 200 karakter olabilir."},
		{"slogan", 120, "Slogan en fazla 120 karakter olabilir."},
	} {
		if value, ok := raw[field.key]; ok {
			text, err := decodeString(value)
			if err != nil {
				return nil, false, utils.Unprocessable(c, field.key+" alanı metin olmalıdır.")
			}
			if len([]rune(text)) > field.limit {
				return nil, false, utils.Unprocessable(c, field.message)
			}
			fields[field.key] = strings.TrimSpace(text)
		}
	}
	// The entrance is checked before the exit because that is the order the
	// splash screen plays them in, and because the database CHECK behind it —
	// menus_splash_entrance_check — accepts exactly utils.SplashEntrances. An
	// id this validator lets through therefore always satisfies the constraint,
	// so a bad value is a 422 here and never a 500 from the UPDATE.
	if value, ok := raw["splash_entrance"]; ok {
		entrance, err := decodeString(value)
		if err != nil || !utils.IsValidSplashEntrance(entrance) {
			return nil, false, utils.Unprocessable(c, "Geçersiz karşılama giriş animasyonu.")
		}
		fields["splash_entrance"] = entrance
	}
	if value, ok := raw["splash_exit_animation"]; ok {
		animation, err := decodeString(value)
		if err != nil || !utils.IsValidSplashExitAnimation(animation) {
			return nil, false, utils.Unprocessable(c, "Geçersiz çıkış animasyonu.")
		}
		fields["splash_exit_animation"] = animation
	}
	if value, ok := raw["splash_exit_easing"]; ok {
		easing, err := decodeString(value)
		if err != nil || !utils.IsValidSplashEasing(easing) {
			return nil, false, utils.Unprocessable(c, "Geçersiz animasyon eğrisi.")
		}
		fields["splash_exit_easing"] = easing
	}
	if value, ok := raw["splash_display"]; ok {
		mode, err := decodeString(value)
		if err != nil || !utils.IsValidSplashDisplay(mode) {
			return nil, false, utils.Unprocessable(c, "Geçersiz karşılama ekranı görünümü.")
		}
		fields["splash_display"] = mode
	}
	if value, ok := raw["splash_exit_duration"]; ok {
		duration, err := decodeInt(value)
		if err != nil || duration < 100 || duration > 2000 {
			return nil, false, utils.Unprocessable(c,
				"Çıkış animasyonu süresi 100 ile 2000 milisaniye arasında olmalıdır.")
		}
		fields["splash_exit_duration"] = duration
	}

	// --- menu background
	if value, ok := raw["background_type"]; ok {
		backgroundType, err := decodeString(value)
		if err != nil || !utils.IsValidBackgroundType(backgroundType) {
			return nil, false, utils.Unprocessable(c, "Geçersiz arka plan türü.")
		}
		fields["background_type"] = backgroundType
	}
	if value, ok := raw["background_overlay_opacity"]; ok {
		opacity, err := decodeFloat(value)
		if err != nil || opacity < 0 || opacity > 1 {
			return nil, false, utils.Unprocessable(c,
				"Arka plan karartma değeri 0 ile 1 arasında olmalıdır.")
		}
		fields["background_overlay_opacity"] = opacity
	}

	// --- customer menu header
	if value, ok := raw["header_display"]; ok {
		mode, err := decodeString(value)
		if err != nil || !utils.IsValidHeaderDisplay(mode) {
			return nil, false, utils.Unprocessable(c, "Geçersiz başlık görünümü.")
		}
		fields["header_display"] = mode
	}

	// --- order in the dashboard
	if value, ok := raw["position"]; ok {
		position, err := decodeInt(value)
		if err != nil || position < 0 {
			return nil, false, utils.Unprocessable(c,
				"Sıra değeri sıfır veya daha büyük olmalıdır.")
		}
		fields["position"] = position
	}

	return fields, true, nil
}

// validateMenuName reports the Turkish message of an invalid menu name, and an
// empty string when the name is fine.
func validateMenuName(name string) string {
	if length := len([]rune(name)); length < 2 || length > 60 {
		return "Menü adı 2 ile 60 karakter arasında olmalıdır."
	}
	return ""
}
