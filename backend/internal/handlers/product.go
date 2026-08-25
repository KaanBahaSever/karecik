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

type productRequest struct {
	CategoryID   uuid.UUID           `json:"category_id"`
	Translations models.Translations `json:"translations"`
	Price        float64             `json:"price"`
	ComparePrice *float64            `json:"compare_price"`
	ImageURL     *string             `json:"image_url"`
	Allergens    []string            `json:"allergens"`
	IsActive     *bool               `json:"is_active"`
	IsFeatured   *bool               `json:"is_featured"`
}

type priceRequest struct {
	Price float64 `json:"price"`
}

type bulkPriceRequest struct {
	Percentage  float64     `json:"percentage"`
	Rounding    string      `json:"rounding"`
	CategoryIDs []uuid.UUID `json:"category_ids"`
	Apply       bool        `json:"apply"`
}

// ListProducts — GET /api/products?category_id=...&search=...
func (h *Handler) ListProducts(c *fiber.Ctx) error {
	var categoryID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("category_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return utils.BadRequest(c, "Geçersiz kategori kimliği.")
		}
		categoryID = &parsed
	}

	products, err := repository.ListProducts(c.Context(), h.DB,
		middleware.BusinessID(c), categoryID, c.Query("search"))
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, products)
}

// CreateProduct — POST /api/products
func (h *Handler) CreateProduct(c *fiber.Ctx) error {
	var req productRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)

	if req.CategoryID == uuid.Nil {
		return utils.Unprocessable(c, "Ürünün ekleneceği kategoriyi seçmelisiniz.")
	}
	// Kategori bu isletmeye mi ait?
	if _, err := repository.GetCategory(c.Context(), h.DB, req.CategoryID, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.Forbidden(c, "Bu kategoriye ürün ekleyemezsiniz.")
		}
		return utils.Internal(c, err)
	}

	business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}

	translations, errMsg := sanitizeTranslations(req.Translations, business.DefaultLanguage, "Ürün")
	if errMsg != "" {
		return utils.Unprocessable(c, errMsg)
	}

	if req.Price < 0 {
		return utils.Unprocessable(c, "Fiyat sıfırdan küçük olamaz.")
	}
	if req.ComparePrice != nil && *req.ComparePrice < 0 {
		return utils.Unprocessable(c, "Karşılaştırma fiyatı sıfırdan küçük olamaz.")
	}

	isActive, isFeatured := true, false
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.IsFeatured != nil {
		isFeatured = *req.IsFeatured
	}

	product, err := repository.CreateProduct(c.Context(), h.DB, businessID, req.CategoryID,
		translations, utils.Round2(req.Price), roundPtr(req.ComparePrice), req.ImageURL,
		sanitizeAllergens(req.Allergens), isActive, isFeatured)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.Created(c, product)
}

// UpdateProduct — PUT /api/products/:id
func (h *Handler) UpdateProduct(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz ürün kimliği.")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)
	fields := make(map[string]any)

	if v, ok := raw["category_id"]; ok {
		var categoryID uuid.UUID
		if err := json.Unmarshal(v, &categoryID); err != nil {
			return utils.Unprocessable(c, "Geçersiz kategori kimliği.")
		}
		if _, err := repository.GetCategory(c.Context(), h.DB, categoryID, businessID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.Forbidden(c, "Ürünü bu kategoriye taşıyamazsınız.")
			}
			return utils.Internal(c, err)
		}
		fields["category_id"] = categoryID
	}

	if v, ok := raw["translations"]; ok {
		var translations models.Translations
		if err := json.Unmarshal(v, &translations); err != nil {
			return utils.Unprocessable(c, "Çeviri alanı geçersiz.")
		}
		business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
		if err != nil {
			return utils.Internal(c, err)
		}
		cleaned, errMsg := sanitizeTranslations(translations, business.DefaultLanguage, "Ürün")
		if errMsg != "" {
			return utils.Unprocessable(c, errMsg)
		}
		fields["translations"] = cleaned
	}

	if v, ok := raw["price"]; ok {
		price, err := decodeFloat(v)
		if err != nil || price < 0 {
			return utils.Unprocessable(c, "Fiyat sıfır veya daha büyük bir sayı olmalıdır.")
		}
		fields["price"] = utils.Round2(price)
	}

	if v, ok := raw["compare_price"]; ok {
		if string(v) == "null" {
			fields["compare_price"] = nil
		} else {
			price, err := decodeFloat(v)
			if err != nil || price < 0 {
				return utils.Unprocessable(c, "Karşılaştırma fiyatı geçersiz.")
			}
			rounded := utils.Round2(price)
			fields["compare_price"] = &rounded
		}
	}

	if v, ok := raw["image_url"]; ok {
		ptr, err := decodeNullableString(v)
		if err != nil {
			return utils.Unprocessable(c, "Görsel adresi geçersiz.")
		}
		fields["image_url"] = ptr
	}

	if v, ok := raw["allergens"]; ok {
		var allergens []string
		if err := json.Unmarshal(v, &allergens); err != nil {
			return utils.Unprocessable(c, "Alerjen listesi geçersiz.")
		}
		fields["allergens"] = sanitizeAllergens(allergens)
	}

	for _, key := range []string{"is_active", "is_featured"} {
		if v, ok := raw[key]; ok {
			b, err := decodeBool(v)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = b
		}
	}

	product, err := repository.UpdateProduct(c.Context(), h.DB, id, businessID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Ürün bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, product)
}

// PatchProductPrice — PATCH /api/products/:id/price
// Menu editorundeki satir ici hizli fiyat degisimi.
func (h *Handler) PatchProductPrice(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz ürün kimliği.")
	}

	var req priceRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}
	if req.Price < 0 {
		return utils.Unprocessable(c, "Fiyat sıfırdan küçük olamaz.")
	}

	product, err := repository.UpdateProduct(c.Context(), h.DB, id, middleware.BusinessID(c),
		map[string]any{"price": utils.Round2(req.Price)})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Ürün bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, product)
}

// DeleteProduct — DELETE /api/products/:id
func (h *Handler) DeleteProduct(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz ürün kimliği.")
	}

	if err := repository.DeleteProduct(c.Context(), h.DB, id, middleware.BusinessID(c)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Ürün bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, fiber.Map{"success": true})
}

// ReorderProducts — PUT /api/products/reorder
// Hem kategori icindeki sirayi hem de kategoriler arasi tasimayi isler.
func (h *Handler) ReorderProducts(c *fiber.Ctx) error {
	var req reorderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}
	if req.CategoryID == nil || *req.CategoryID == uuid.Nil {
		return utils.Unprocessable(c, "Hedef kategori belirtilmelidir.")
	}

	businessID := middleware.BusinessID(c)
	if _, err := repository.GetCategory(c.Context(), h.DB, *req.CategoryID, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.Forbidden(c, "Bu kategori üzerinde işlem yapamazsınız.")
		}
		return utils.Internal(c, err)
	}

	if err := repository.ReorderProducts(c.Context(), h.DB, businessID, *req.CategoryID, req.IDs); err != nil {
		return utils.Internal(c, err)
	}

	products, err := repository.ListProducts(c.Context(), h.DB, businessID, req.CategoryID, "")
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, products)
}

// BulkPrice — POST /api/products/bulk-price
// Tum urunlere (ya da secili kategorilere) yuzde bazli zam/indirim uygular,
// istege bagli fiyat yuvarlamasi yapar. apply=false ise yalnizca onizleme doner.
func (h *Handler) BulkPrice(c *fiber.Ctx) error {
	var req bulkPriceRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	if req.Percentage < -90 || req.Percentage > 1000 {
		return utils.Unprocessable(c, "Yüzde değeri -90 ile 1000 arasında olmalıdır.")
	}
	if req.Rounding == "" {
		req.Rounding = utils.RoundNone
	}
	if !utils.ValidRoundingModes[req.Rounding] {
		return utils.Unprocessable(c, "Geçersiz yuvarlama seçeneği.")
	}

	businessID := middleware.BusinessID(c)
	business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}

	rows, err := repository.ListPriceRows(c.Context(), h.DB, businessID, req.CategoryIDs)
	if err != nil {
		return utils.Internal(c, err)
	}

	preview := make([]models.PriceChangePreview, 0, len(rows))
	changes := make(map[uuid.UUID]float64)

	for _, row := range rows {
		newPrice := utils.RoundPrice(
			utils.ApplyPercentage(row.Price, req.Percentage), req.Rounding)

		preview = append(preview, models.PriceChangePreview{
			ID:       row.ID,
			Name:     row.Translations.Resolve(business.DefaultLanguage, business.DefaultLanguage).Name,
			OldPrice: utils.Round2(row.Price),
			NewPrice: newPrice,
		})

		if newPrice != utils.Round2(row.Price) {
			changes[row.ID] = newPrice
		}
	}

	result := models.BulkPriceResult{
		Applied:  false,
		Affected: len(changes),
		Preview:  preview,
	}

	if !req.Apply {
		return utils.OK(c, result)
	}

	affected, err := repository.ApplyPrices(c.Context(), h.DB, businessID, changes)
	if err != nil {
		return utils.Internal(c, err)
	}

	updatedAt, err := repository.TouchPriceUpdatedAt(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}

	if err := repository.LogPriceUpdate(c.Context(), h.DB, businessID,
		req.Percentage, req.Rounding, affected); err != nil {
		return utils.Internal(c, err)
	}

	result.Applied = true
	result.Affected = affected
	result.PriceUpdatedAt = &updatedAt

	return utils.OK(c, result)
}

// ------------------------------------------------------------- yardimcilar

// sanitizeAllergens, bilinmeyen alerjen kodlarini eler ve tekrarlari temizler.
func sanitizeAllergens(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool)

	for _, code := range in {
		code = strings.TrimSpace(strings.ToLower(code))
		if code == "" || seen[code] || !utils.IsValidAllergen(code) {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}

func roundPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := utils.Round2(*v)
	return &r
}
