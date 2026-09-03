package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

type categoryRequest struct {
	MenuID       *uuid.UUID          `json:"menu_id"`
	Translations models.Translations `json:"translations"`
	Icon         *string             `json:"icon"`
	ImageURL     *string             `json:"image_url"`
	IsActive     *bool               `json:"is_active"`
}

type reorderRequest struct {
	IDs        []uuid.UUID `json:"ids"`
	CategoryID *uuid.UUID  `json:"category_id"`
}

// ListCategories — GET /api/categories?menu_id=...
// Without the parameter every category of the business is listed; with it the
// list is scoped to that menu exactly the way the customer menu is, so the
// editor shows what the customer will see.
func (h *Handler) ListCategories(c *fiber.Ctx) error {
	businessID := middleware.BusinessID(c)

	var menuID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("menu_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return utils.BadRequest(c, "Geçersiz menü kimliği.")
		}
		// The menu has to belong to this business before it scopes anything.
		if _, err := repository.GetMenu(c.Context(), h.DB, parsed, businessID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.NotFound(c, "Menü bulunamadı.")
			}
			return utils.Internal(c, err)
		}
		menuID = &parsed
	}

	categories, err := repository.ListCategories(c.Context(), h.DB, businessID, menuID)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, categories)
}

// CreateCategory — POST /api/categories
func (h *Handler) CreateCategory(c *fiber.Ctx) error {
	var req categoryRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)
	business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}

	translations, errMessage := sanitizeTranslations(req.Translations, business.DefaultLanguage, "Kategori")
	if errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// A category always lands on a menu: the one the dashboard is editing when
	// it names it, otherwise the default menu of the business.
	var menuID uuid.UUID
	if req.MenuID != nil {
		ok, err := h.ownsMenu(c, businessID, *req.MenuID, "Bu menüye kategori ekleyemezsiniz.")
		if !ok {
			return err
		}
		menuID = *req.MenuID
	} else {
		menu, err := repository.GetDefaultMenu(c.Context(), h.DB, businessID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.Unprocessable(c, "Önce bir menü oluşturmalısınız.")
			}
			return utils.Internal(c, err)
		}
		menuID = menu.ID
	}

	category, err := repository.CreateCategory(c.Context(), h.DB, businessID, menuID,
		translations, req.Icon, req.ImageURL, isActive)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.Created(c, category)
}

// ownsMenu proves that a menu id coming from a request belongs to this
// business before a category is written into it — without the check a forged
// id would move the category into a foreign tenant's menu. The Turkish
// `denied` message is what the caller shows on refusal.
//
// A refused request is reported through the false `ok` after the response has
// already been written, the same way public.go's previewMenuOf does it.
func (h *Handler) ownsMenu(c *fiber.Ctx, businessID, menuID uuid.UUID,
	denied string) (bool, error) {

	if _, err := repository.GetMenu(c.Context(), h.DB, menuID, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, utils.Forbidden(c, denied)
		}
		return false, utils.Internal(c, err)
	}
	return true, nil
}

// UpdateCategory — PUT /api/categories/:id
func (h *Handler) UpdateCategory(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz kategori kimliği.")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)
	fields := make(map[string]any)

	if value, ok := raw["translations"]; ok {
		var translations models.Translations
		if err := json.Unmarshal(value, &translations); err != nil {
			return utils.Unprocessable(c, "Çeviri alanı geçersiz.")
		}
		business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
		if err != nil {
			return utils.Internal(c, err)
		}
		cleaned, errMessage := sanitizeTranslations(translations, business.DefaultLanguage, "Kategori")
		if errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["translations"] = cleaned
	}

	for _, key := range []string{"icon", "image_url"} {
		if value, ok := raw[key]; ok {
			ptr, err := decodeNullableString(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin veya boş olmalıdır.")
			}
			fields[key] = ptr
		}
	}

	if value, ok := raw["is_active"]; ok {
		flag, err := decodeBool(value)
		if err != nil {
			return utils.Unprocessable(c, "is_active alanı true/false olmalıdır.")
		}
		fields["is_active"] = flag
	}

	// Moving a category to another menu is an ownership decision, not a plain
	// column write.
	if value, ok := raw["menu_id"]; ok {
		menuID, err := decodeUUID(value)
		if err != nil || menuID == uuid.Nil {
			return utils.Unprocessable(c, "menu_id alanı geçerli bir menü kimliği olmalıdır.")
		}
		if ok, err := h.ownsMenu(c, businessID, menuID, "Kategoriyi bu menüye taşıyamazsınız."); !ok {
			return err
		}
		fields["menu_id"] = menuID
	}

	category, err := repository.UpdateCategory(c.Context(), h.DB, id, businessID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Kategori bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, category)
}

// DeleteCategory — DELETE /api/categories/:id
// The products inside the category are removed along with it.
func (h *Handler) DeleteCategory(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz kategori kimliği.")
	}

	deleted, err := repository.DeleteCategory(c.Context(), h.DB, id, middleware.BusinessID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Kategori bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	return utils.OK(c, fiber.Map{"success": true, "deleted_products": deleted})
}

// ReorderCategories — PUT /api/categories/reorder
// Persists the new order after a drag and drop.
func (h *Handler) ReorderCategories(c *fiber.Ctx) error {
	var req reorderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}
	if len(req.IDs) == 0 {
		return utils.Unprocessable(c, "Sıralanacak kategori listesi boş olamaz.")
	}

	businessID := middleware.BusinessID(c)
	if err := repository.ReorderCategories(c.Context(), h.DB, businessID, req.IDs); err != nil {
		return utils.Internal(c, err)
	}

	// The drag and drop reorders the whole business, so every category is
	// returned regardless of the menu the editor is showing.
	categories, err := repository.ListCategories(c.Context(), h.DB, businessID, nil)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, categories)
}

// sanitizeTranslations cleans and validates a translations map: unsupported
// language codes are dropped and whitespace is trimmed. It returns an error
// message when no name is present (or the default language has none).
//
// The label argument appears in the returned message, so it is Turkish.
func sanitizeTranslations(in models.Translations, defaultLang, label string) (models.Translations, string) {
	out := models.Translations{}

	for lang, translation := range in {
		if !utils.IsValidLanguage(lang) {
			continue
		}
		cleaned := models.Translation{
			Name:        strings.TrimSpace(translation.Name),
			Description: strings.TrimSpace(translation.Description),
			Ingredients: strings.TrimSpace(translation.Ingredients),
		}
		if len([]rune(cleaned.Name)) > 120 {
			return nil, label + " adı en fazla 120 karakter olabilir."
		}
		if len([]rune(cleaned.Description)) > 500 {
			return nil, label + " açıklaması en fazla 500 karakter olabilir."
		}
		out[lang] = cleaned
	}

	if !out.HasName(defaultLang) {
		return nil, label + " adı zorunludur (varsayılan dilde doldurulmalıdır)."
	}
	return out, ""
}
