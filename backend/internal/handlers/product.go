package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
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
	Calories     *int                `json:"calories"`
	ImageURL     *string             `json:"image_url"`
	Allergens    []string            `json:"allergens"`
	Badges       models.Badges       `json:"badges"`
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
	// Does the category belong to this business?
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

	translations, errMessage := sanitizeTranslations(req.Translations, business.DefaultLanguage, "Ürün")
	if errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	if req.Price < 0 {
		return utils.Unprocessable(c, "Fiyat sıfırdan küçük olamaz.")
	}
	if req.ComparePrice != nil && *req.ComparePrice < 0 {
		return utils.Unprocessable(c, "Karşılaştırma fiyatı sıfırdan küçük olamaz.")
	}
	if errMessage := validateCalories(req.Calories); errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	badges, errMessage := sanitizeBadges(req.Badges)
	if errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	isActive, isFeatured := true, false
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.IsFeatured != nil {
		isFeatured = *req.IsFeatured
	}

	product, err := repository.CreateProduct(c.Context(), h.DB, businessID, req.CategoryID,
		translations, utils.Round2(req.Price), roundPtr(req.ComparePrice), req.Calories,
		req.ImageURL, sanitizeAllergens(req.Allergens), badges, isActive, isFeatured)
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

	if value, ok := raw["category_id"]; ok {
		var categoryID uuid.UUID
		if err := json.Unmarshal(value, &categoryID); err != nil {
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

	if value, ok := raw["translations"]; ok {
		var translations models.Translations
		if err := json.Unmarshal(value, &translations); err != nil {
			return utils.Unprocessable(c, "Çeviri alanı geçersiz.")
		}
		business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
		if err != nil {
			return utils.Internal(c, err)
		}
		cleaned, errMessage := sanitizeTranslations(translations, business.DefaultLanguage, "Ürün")
		if errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["translations"] = cleaned
	}

	if value, ok := raw["price"]; ok {
		price, err := decodeFloat(value)
		if err != nil || price < 0 {
			return utils.Unprocessable(c, "Fiyat sıfır veya daha büyük bir sayı olmalıdır.")
		}
		fields["price"] = utils.Round2(price)
	}

	if value, ok := raw["compare_price"]; ok {
		if string(value) == "null" {
			fields["compare_price"] = nil
		} else {
			price, err := decodeFloat(value)
			if err != nil || price < 0 {
				return utils.Unprocessable(c, "Karşılaştırma fiyatı geçersiz.")
			}
			rounded := utils.Round2(price)
			fields["compare_price"] = &rounded
		}
	}

	// null clears the calorie value, which is optional on every product.
	if value, ok := raw["calories"]; ok {
		if string(value) == "null" {
			fields["calories"] = (*int)(nil)
		} else {
			calories, err := decodeInt(value)
			if err != nil {
				return utils.Unprocessable(c, "Kalori değeri tam sayı olmalıdır.")
			}
			if errMessage := validateCalories(&calories); errMessage != "" {
				return utils.Unprocessable(c, errMessage)
			}
			fields["calories"] = &calories
		}
	}

	if value, ok := raw["image_url"]; ok {
		ptr, err := decodeNullableString(value)
		if err != nil {
			return utils.Unprocessable(c, "Görsel adresi geçersiz.")
		}
		fields["image_url"] = ptr
	}

	if value, ok := raw["allergens"]; ok {
		var allergens []string
		if err := json.Unmarshal(value, &allergens); err != nil {
			return utils.Unprocessable(c, "Alerjen listesi geçersiz.")
		}
		fields["allergens"] = sanitizeAllergens(allergens)
	}

	if value, ok := raw["badges"]; ok {
		var badges models.Badges
		if string(value) != "null" {
			if err := json.Unmarshal(value, &badges); err != nil {
				return utils.Unprocessable(c, "Rozet listesi geçersiz.")
			}
		}
		// badges is a NOT NULL jsonb column, so sanitizeBadges always returns
		// a non-nil list — a nil one would be written as the literal null.
		cleaned, errMessage := sanitizeBadges(badges)
		if errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["badges"] = cleaned
	}

	for _, key := range []string{"is_active", "is_featured"} {
		if value, ok := raw[key]; ok {
			flag, err := decodeBool(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = flag
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
// Backs the inline quick price editing in the menu editor.
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
// Handles both ordering inside a category and moving between categories.
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
// Applies a percentage increase or discount to every product (or to the
// selected categories) with optional price rounding. When apply is false only
// a preview is returned and nothing is written.
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

// ------------------------------------------------------------------ helpers

// Fallback colours of a custom badge, used when the dashboard sends none.
const (
	defaultBadgeBgColor   = "#1d4ed8"
	defaultBadgeTextColor = "#ffffff"
)

// sanitizeBadges validates the custom badges of a product and fills in the
// missing identifiers and colours. It returns the cleaned list plus an error
// message, which is empty when everything is valid.
//
// NOTE: the message is shown to the end user and is therefore Turkish.
func sanitizeBadges(in models.Badges) (models.Badges, string) {
	if len(in) > models.MaxBadges {
		return nil, fmt.Sprintf("En fazla %d rozet ekleyebilirsiniz.", models.MaxBadges)
	}

	out := make(models.Badges, 0, len(in))

	for _, badge := range in {
		badge.Text = strings.TrimSpace(badge.Text)
		if badge.Text == "" {
			return nil, "Rozet metni boş olamaz."
		}
		if len([]rune(badge.Text)) > models.MaxBadgeTextRunes {
			return nil, fmt.Sprintf("Rozet metni en fazla %d karakter olabilir.",
				models.MaxBadgeTextRunes)
		}

		badge.Icon = strings.TrimSpace(badge.Icon)
		if badge.Icon != "" && !utils.IsValidBadgeIcon(badge.Icon) {
			return nil, "Geçersiz rozet simgesi: " + badge.Icon
		}

		badge.BgColor = strings.TrimSpace(badge.BgColor)
		if badge.BgColor == "" {
			badge.BgColor = defaultBadgeBgColor
		}
		badge.TextColor = strings.TrimSpace(badge.TextColor)
		if badge.TextColor == "" {
			badge.TextColor = defaultBadgeTextColor
		}
		if !hexColorPattern.MatchString(badge.BgColor) ||
			!hexColorPattern.MatchString(badge.TextColor) {
			return nil, "Rozet renkleri #RRGGBB biçiminde olmalıdır."
		}

		if strings.TrimSpace(badge.ID) == "" {
			badge.ID = uuid.NewString()
		}

		out = append(out, badge)
	}

	// Normalize is the last pass: it trims the fields and guarantees a non-nil
	// slice, which the NOT NULL jsonb column needs.
	return out.Normalize(), ""
}

// validateCalories checks the optional calorie value of a product; nil means
// "not stated" and is always allowed.
func validateCalories(value *int) string {
	if value == nil {
		return ""
	}
	if *value < 0 || *value > 20000 {
		return "Kalori değeri 0 ile 20000 arasında olmalıdır."
	}
	return ""
}

// sanitizeAllergens drops unknown allergen codes and removes duplicates.
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

func roundPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	rounded := utils.Round2(*value)
	return &rounded
}
