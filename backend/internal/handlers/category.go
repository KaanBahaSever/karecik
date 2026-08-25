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
	Translations models.Translations `json:"translations"`
	Icon         *string             `json:"icon"`
	ImageURL     *string             `json:"image_url"`
	IsActive     *bool               `json:"is_active"`
}

type reorderRequest struct {
	IDs        []uuid.UUID `json:"ids"`
	CategoryID *uuid.UUID  `json:"category_id"`
}

// ListCategories — GET /api/categories
func (h *Handler) ListCategories(c *fiber.Ctx) error {
	categories, err := repository.ListCategories(c.Context(), h.DB, middleware.BusinessID(c))
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

	translations, errMsg := sanitizeTranslations(req.Translations, business.DefaultLanguage, "Kategori")
	if errMsg != "" {
		return utils.Unprocessable(c, errMsg)
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	category, err := repository.CreateCategory(c.Context(), h.DB, businessID,
		translations, req.Icon, req.ImageURL, isActive)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.Created(c, category)
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

	if v, ok := raw["translations"]; ok {
		var translations models.Translations
		if err := json.Unmarshal(v, &translations); err != nil {
			return utils.Unprocessable(c, "Çeviri alanı geçersiz.")
		}
		business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
		if err != nil {
			return utils.Internal(c, err)
		}
		cleaned, errMsg := sanitizeTranslations(translations, business.DefaultLanguage, "Kategori")
		if errMsg != "" {
			return utils.Unprocessable(c, errMsg)
		}
		fields["translations"] = cleaned
	}

	for _, key := range []string{"icon", "image_url"} {
		if v, ok := raw[key]; ok {
			ptr, err := decodeNullableString(v)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin veya boş olmalıdır.")
			}
			fields[key] = ptr
		}
	}

	if v, ok := raw["is_active"]; ok {
		b, err := decodeBool(v)
		if err != nil {
			return utils.Unprocessable(c, "is_active alanı true/false olmalıdır.")
		}
		fields["is_active"] = b
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
// Kategoriyle birlikte icindeki urunler de silinir.
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
// Surukle-birak sonrasi yeni sirayi kaydeder.
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

	categories, err := repository.ListCategories(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, categories)
}

// sanitizeTranslations, ceviri sozlugunu temizler ve dogrular.
// Desteklenmeyen dil kodlarini atar, bosluklari kirpar.
// Hicbir dilde ad yoksa (ya da varsayilan dilde ad bossa) hata mesaji doner.
func sanitizeTranslations(in models.Translations, defaultLang, label string) (models.Translations, string) {
	out := models.Translations{}

	for lang, tr := range in {
		if !utils.IsValidLanguage(lang) {
			continue
		}
		cleaned := models.Translation{
			Name:        strings.TrimSpace(tr.Name),
			Description: strings.TrimSpace(tr.Description),
			Ingredients: strings.TrimSpace(tr.Ingredients),
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
