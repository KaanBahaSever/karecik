package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

type productRequest struct {
	CategoryID   uuid.UUID             `json:"category_id"`
	Translations models.Translations   `json:"translations"`
	Price        float64               `json:"price"`
	ComparePrice *float64              `json:"compare_price"`
	Calories     *int                  `json:"calories"`
	ImageURL     *string               `json:"image_url"`
	Allergens    []string              `json:"allergens"`
	Badges       models.Badges         `json:"badges"`
	Options      models.ProductOptions `json:"options"`
	IsActive     *bool                 `json:"is_active"`
	IsFeatured   *bool                 `json:"is_featured"`
}

type priceRequest struct {
	Price float64 `json:"price"`
}

// MenuID scopes a bulk price update to one menu and is required: prices belong
// to the menu they are printed on and there is no default menu to guess with.
type bulkPriceRequest struct {
	MenuID      uuid.UUID   `json:"menu_id"`
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
	// Does the category belong to this business? A category of another business
	// is answered exactly like one that does not exist, and like one deleted
	// while this request runs (below): all three are a category this business
	// does not have.
	category, err := repository.GetCategory(c.Context(), h.DB, req.CategoryID, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Kategori bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	// The texts are required in the language of the menu the category sits on.
	lang, err := h.menuLanguage(c, businessID, category.MenuID)
	if err != nil {
		return utils.Internal(c, err)
	}

	translations, errMessage := SanitizeTranslations(req.Translations, lang, "Ürün")
	if errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	if errMessage := CheckPrice(req.Price, msgPriceNegative, msgPriceInvalid,
		MsgPriceTooLarge); errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}
	if req.ComparePrice != nil {
		if errMessage := CheckPrice(*req.ComparePrice, msgComparePriceNegative,
			msgComparePriceInvalid, MsgComparePriceTooLarge); errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
	}
	if errMessage := validateCalories(req.Calories); errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	badges, errMessage := SanitizeBadges(req.Badges)
	if errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	options, errMessage := SanitizeOptions(req.Options)
	if errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	// The update path reads image_url through DecodeNullableString, which
	// refuses a text PostgreSQL cannot store; this body was decoded by
	// BodyParser instead — possibly from a form, which can carry any bytes — so
	// the check is made here, with the update path's message.
	if req.ImageURL != nil && UnstorableText(*req.ImageURL) {
		return utils.Unprocessable(c, "Görsel adresi geçersiz.")
	}

	isActive, isFeatured := true, false
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.IsFeatured != nil {
		isFeatured = *req.IsFeatured
	}

	// image_url is trimmed and a blank value is stored as NULL — exactly what
	// UpdateProduct stores through DecodeNullableString, so a product created
	// without an image and one whose image was later cleared look the same.
	imageURL, allergens := optionalStrPtr(req.ImageURL), sanitizeAllergens(req.Allergens)

	// CreateProduct is one transaction of its own — the advisory locks of the
	// category's menu and of the category, then the INSERT — so a run
	// PostgreSQL aborted over a lock conflict wrote nothing and is simply run
	// again. See repository.RetryOnConflict.
	product, err := repository.RetryOnConflictValue(c.Context(), "CreateProduct",
		func() (*models.Product, error) {
			return repository.CreateProduct(c.Context(), h.DB, businessID, req.CategoryID,
				translations, utils.Round2(req.Price), roundPtr(req.ComparePrice), req.Calories,
				imageURL, allergens, badges, options, isActive, isFeatured)
		})
	if err != nil {
		// The category was checked above, but a delete of it — or of its menu —
		// can still land before the create holds its locks: the create waits for
		// that delete at its advisory locks and then finds the category gone
		// (ErrParentNotFound). A delete that took no advisory lock is caught by
		// the INSERT's foreign key check (23503) instead. Either way the category
		// is gone: a 404, not a server error.
		if errors.Is(err, repository.ErrParentNotFound) || repository.IsForeignKeyViolation(err) {
			return utils.NotFound(c, "Kategori bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.Created(c, product)
}

// The refusals of a price a request carries. The too-large messages name
// utils.MaxPrice the way the dashboard writes a price, with Turkish digit
// grouping.
const (
	msgPriceNegative        = "Fiyat sıfırdan küçük olamaz."
	msgPriceInvalid         = "Fiyat sıfır veya daha büyük bir sayı olmalıdır."
	MsgPriceTooLarge        = "Fiyat en fazla 9.999.999.999,99 olabilir."
	msgComparePriceNegative = "Karşılaştırma fiyatı sıfırdan küçük olamaz."
	msgComparePriceInvalid  = "Karşılaştırma fiyatı geçersiz."
	MsgComparePriceTooLarge = "Karşılaştırma fiyatı en fazla 9.999.999.999,99 olabilir."

	// MsgBulkPriceTooLarge refuses a bulk price update that would give a
	// product a price above utils.MaxPrice, on preview and apply alike.
	MsgBulkPriceTooLarge = "Bu değişiklik bazı fiyatları izin verilen en yüksek değerin (9.999.999.999,99) üzerine çıkarıyor."
)

// CheckPrice returns the refusal of a price a request carries, or "" when the
// price can be stored: negative for a price below zero, notANumber for NaN and
// tooLarge for a price above utils.MaxPrice, which products.price and
// products.compare_price cannot hold. The price is compared as it was sent,
// before utils.Round2, so a price that passes also fits once rounded.
//
// NaN needs a case of its own. A JSON body cannot carry it, but a form or an
// XML body can — BodyParser reads "NaN" as a float64 — and NaN is neither below
// zero nor above the limit. PostgreSQL stores it in a NUMERIC column, where the
// CHECK (price >= 0) lets it through, and every later read of the product would
// then fail to encode its price. +Inf and -Inf need nothing extra: one is above
// the limit and the other below zero.
func CheckPrice(price float64, negative, notANumber, tooLarge string) string {
	switch {
	case math.IsNaN(price):
		return notANumber
	case price < 0:
		return negative
	case price > utils.MaxPrice:
		return tooLarge
	}
	return ""
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

	// The target category comes first: it decides which menu — and therefore
	// which language — the new texts have to satisfy.
	var target *models.Category
	if value, ok := raw["category_id"]; ok {
		var categoryID uuid.UUID
		if err := json.Unmarshal(value, &categoryID); err != nil {
			return utils.Unprocessable(c, "Geçersiz kategori kimliği.")
		}
		category, err := repository.GetCategory(c.Context(), h.DB, categoryID, businessID)
		if err != nil {
			// A category of another business is answered like one that does not
			// exist, and like one deleted while the move runs (below).
			if errors.Is(err, repository.ErrNotFound) {
				return utils.NotFound(c, "Kategori bulunamadı.")
			}
			return utils.Internal(c, err)
		}
		target = category
		fields["category_id"] = categoryID
	}

	if value, ok := raw["translations"]; ok {
		var translations models.Translations
		if err := json.Unmarshal(value, &translations); err != nil {
			return utils.Unprocessable(c, "Çeviri alanı geçersiz.")
		}
		var (
			lang string
			err  error
		)
		if target != nil {
			lang, err = h.menuLanguage(c, businessID, target.MenuID)
		} else {
			lang, err = h.productLanguage(c, businessID, id)
		}
		if err != nil {
			return utils.Internal(c, err)
		}
		cleaned, errMessage := SanitizeTranslations(translations, lang, "Ürün")
		if errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["translations"] = cleaned
	}

	// A negative price gets this path's invalid-value message, the one it gives
	// a value that is not a number at all.
	if value, ok := raw["price"]; ok {
		price, err := decodeFloat(value)
		if err != nil {
			return utils.Unprocessable(c, msgPriceInvalid)
		}
		if errMessage := CheckPrice(price, msgPriceInvalid, msgPriceInvalid,
			MsgPriceTooLarge); errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["price"] = utils.Round2(price)
	}

	if value, ok := raw["compare_price"]; ok {
		if string(value) == "null" {
			fields["compare_price"] = nil
		} else {
			price, err := decodeFloat(value)
			if err != nil {
				return utils.Unprocessable(c, msgComparePriceInvalid)
			}
			if errMessage := CheckPrice(price, msgComparePriceInvalid, msgComparePriceInvalid,
				MsgComparePriceTooLarge); errMessage != "" {
				return utils.Unprocessable(c, errMessage)
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
		ptr, err := DecodeNullableString(value)
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
		// badges is a NOT NULL jsonb column, so SanitizeBadges always returns
		// a non-nil list — a nil one would be written as the literal null.
		cleaned, errMessage := SanitizeBadges(badges)
		if errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["badges"] = cleaned
	}

	// An empty array clears the options of a product; null does the same, so a
	// dashboard that sends either gets the same result. options is a NOT NULL
	// jsonb column, so SanitizeOptions never returns a nil list.
	if value, ok := raw["options"]; ok {
		var options models.ProductOptions
		if string(value) != "null" {
			if err := json.Unmarshal(value, &options); err != nil {
				return utils.Unprocessable(c, "Seçenek listesi geçersiz.")
			}
		}
		cleaned, errMessage := SanitizeOptions(options)
		if errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["options"] = cleaned
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

	// No timestamp code here, on purpose. When this UPDATE changes price,
	// compare_price or an option surcharge, the products_touch_menu_price_date
	// trigger of migration 010 moves the "prices valid from" date of the menu
	// the product ends up in. A second mechanism in Go would only drift from it.
	//
	// The update is one statement — the trigger runs inside it — or, when the
	// body names a category, one transaction of its own, so a run PostgreSQL
	// aborted over a lock conflict wrote nothing and is simply run again. See
	// repository.RetryOnConflict.
	product, err := repository.RetryOnConflictValue(c.Context(), "UpdateProduct",
		func() (*models.Product, error) {
			return repository.UpdateProduct(c.Context(), h.DB, id, businessID, fields)
		})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Ürün bulunamadı.")
		}
		// A move into a category that was deleted after GetCategory checked it.
		// The move waits for a delete of the category or of its menu at its
		// advisory lock and then finds the category gone (ErrParentNotFound);
		// a delete that took no advisory lock is caught by the foreign key check
		// (23503). Either way the category is gone: a 404 rather than a 500.
		if errors.Is(err, repository.ErrParentNotFound) || repository.IsForeignKeyViolation(err) {
			return utils.NotFound(c, "Kategori bulunamadı.")
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
	if errMessage := CheckPrice(req.Price, msgPriceNegative, msgPriceInvalid,
		MsgPriceTooLarge); errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	// A different price moves the menu's "prices valid from" date through the
	// trigger of migration 010, and re-sending the same price does not. No
	// timestamp code belongs here — see UpdateProduct, which also explains the
	// retry.
	businessID := middleware.BusinessID(c)
	fields := map[string]any{"price": utils.Round2(req.Price)}
	product, err := repository.RetryOnConflictValue(c.Context(), "PatchProductPrice",
		func() (*models.Product, error) {
			return repository.UpdateProduct(c.Context(), h.DB, id, businessID, fields)
		})
	if err != nil {
		// A foreign key violation is mapped here as on every other product
		// write, with this path's own not-found message: a price edit names no
		// category, only the product.
		if errors.Is(err, repository.ErrNotFound) || repository.IsForeignKeyViolation(err) {
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

	// One self-contained DELETE, so a run PostgreSQL aborted over a lock
	// conflict wrote nothing and is simply run again.
	businessID := middleware.BusinessID(c)
	err = repository.RetryOnConflict(c.Context(), "DeleteProduct", func() error {
		return repository.DeleteProduct(c.Context(), h.DB, id, businessID)
	})
	if err != nil {
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
	// A category of another business is answered like one that does not exist,
	// and like one deleted while the reorder runs (below).
	if _, err := repository.GetCategory(c.Context(), h.DB, *req.CategoryID, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Kategori bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	// ReorderProducts is one transaction of its own — advisory locks, then the
	// row locks, then the write — so a run PostgreSQL aborted over a lock
	// conflict wrote nothing and is simply run again.
	err := repository.RetryOnConflict(c.Context(), "ReorderProducts", func() error {
		return repository.ReorderProducts(c.Context(), h.DB, businessID, *req.CategoryID, req.IDs)
	})
	if err != nil {
		// The target category was checked above, but a delete of it — or of its
		// menu — can still land before the reorder holds its locks: the reorder
		// waits for it and then finds the category gone (ErrParentNotFound), or,
		// for a delete that took no advisory lock, fails its foreign key check
		// (23503). The category is gone: a 404 rather than a 500.
		if errors.Is(err, repository.ErrParentNotFound) || repository.IsForeignKeyViolation(err) {
			return utils.NotFound(c, "Kategori bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	products, err := repository.ListProducts(c.Context(), h.DB, businessID, req.CategoryID, "")
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, products)
}

// BulkPrice — POST /api/products/bulk-price
// Applies a percentage increase or discount to every product of one menu (or
// to the selected categories of it) with optional price rounding. When apply is
// false only a preview is returned and nothing is written.
//
// Prices belong to the menu they are printed on, so the update is scoped to the
// single menu named by menu_id. That field is required: raising every price of
// a menu the user did not pick is not a fallback worth having, so a missing one
// is a plain 422.
//
// A preview reads the prices without locking them. An apply does not reuse
// that read: repository.ApplyPrices locks the products, reads their prices
// under the lock and writes the new ones in one transaction, and the answer —
// preview and affected alike — is built from what it read and wrote. A price
// edited while the apply waited for its locks is therefore the price the
// percentage applies to.
//
// An apply moves the menu's "prices valid from" date only when it really
// changes a price, and this handler never moves it itself: see the note after
// ApplyPrices.
func (h *Handler) BulkPrice(c *fiber.Ctx) error {
	var req bulkPriceRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	// Written as "not within" so that NaN, which a form or an XML body can
	// carry and which no comparison holds for, is refused as well: every price
	// it touched would become NaN.
	if !(req.Percentage >= -90 && req.Percentage <= 1000) {
		return utils.Unprocessable(c, "Yüzde değeri -90 ile 1000 arasında olmalıdır.")
	}
	if req.Rounding == "" {
		req.Rounding = utils.RoundNone
	}
	if !utils.ValidRoundingModes[req.Rounding] {
		return utils.Unprocessable(c, "Geçersiz yuvarlama seçeneği.")
	}

	businessID := middleware.BusinessID(c)

	if req.MenuID == uuid.Nil {
		return utils.Unprocessable(c, "Önce bir menü oluşturmalısınız.")
	}
	menu, err := h.ownsMenu(c, businessID, req.MenuID, "Bu menü üzerinde işlem yapamazsınız.")
	if menu == nil {
		return err
	}

	if !req.Apply {
		rows, err := repository.ListPriceRows(c.Context(), h.DB, businessID, menu.ID, req.CategoryIDs)
		if err != nil {
			return utils.Internal(c, err)
		}
		changes := repository.PlanPriceChanges(rows, req.Percentage, req.Rounding)
		// Refused exactly as an apply of the same prices would be, so a preview
		// never promises prices the apply then refuses to write.
		if repository.PriceLimitExceeded(changes) {
			return utils.Unprocessable(c, MsgBulkPriceTooLarge)
		}
		affected := 0
		for _, change := range changes {
			if change.Changed() {
				affected++
			}
		}
		return utils.OK(c, bulkPriceResult(menu, changes, affected))
	}

	// ApplyPrices is one transaction of its own, so a run PostgreSQL aborted
	// over a lock conflict wrote nothing and is simply run again; the answer is
	// built from the rows the last run read and wrote. See
	// repository.RetryOnConflict.
	var changes []repository.PriceChange
	affected, err := repository.RetryOnConflictValue(c.Context(), "BulkPrice", func() (int, error) {
		priced, written, err := repository.ApplyPrices(c.Context(), h.DB, businessID, menu.ID,
			req.CategoryIDs, req.Percentage, req.Rounding)
		changes = priced
		return written, err
	})
	if err != nil {
		// ApplyPrices refuses a plan with a price above the limit before it
		// writes anything, exactly as the preview refuses it.
		if errors.Is(err, repository.ErrPriceTooLarge) {
			return utils.Unprocessable(c, MsgBulkPriceTooLarge)
		}
		return utils.Internal(c, err)
	}

	// The "prices valid from" date is not written here. The UPDATE inside
	// ApplyPrices fired the products_touch_menu_price_date trigger of migration
	// 010, which moved the date if at least one of those rows really changed
	// price and left it exactly where it was otherwise — an apply with nothing
	// to change included. The response reports the menu's current value.
	//
	// The business id is passed again even though ownsMenu already vouched for
	// the menu: the repository never reads a menu on a bare id, and the only way
	// this can now come back empty is the menu being deleted mid-request.
	updatedAt, err := repository.GetMenuPriceUpdatedAt(c.Context(), h.DB, menu.ID, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	if err := repository.LogPriceUpdate(c.Context(), h.DB, businessID,
		req.Percentage, req.Rounding, affected); err != nil {
		return utils.Internal(c, err)
	}

	result := bulkPriceResult(menu, changes, affected)
	result.Applied = true
	result.PriceUpdatedAt = &updatedAt

	return utils.OK(c, result)
}

// bulkPriceResult is the answer of the bulk price endpoint: every priced product
// with its name in the menu's default language, the price it was read at and
// the price the update gives it, and how many products get a new price — or, for
// an apply, got one.
func bulkPriceResult(menu *models.Menu, changes []repository.PriceChange, affected int) models.BulkPriceResult {
	preview := make([]models.PriceChangePreview, 0, len(changes))
	for _, change := range changes {
		preview = append(preview, models.PriceChangePreview{
			ID:       change.ID,
			Name:     change.Translations.Resolve(menu.DefaultLanguage, menu.DefaultLanguage).Name,
			OldPrice: utils.Round2(change.Price),
			NewPrice: change.NewPrice,
		})
	}
	return models.BulkPriceResult{Affected: affected, Preview: preview}
}

// ------------------------------------------------------------------ helpers

// Fallback colours of a custom badge, used when the dashboard sends none.
const (
	defaultBadgeBgColor   = "#1d4ed8"
	defaultBadgeTextColor = "#ffffff"
)

// SanitizeBadges validates the custom badges of a product and fills in the
// missing identifiers and colours. It returns the cleaned list plus an error
// message, which is empty when everything is valid.
//
// NOTE: the message is shown to the end user and is therefore Turkish.
func SanitizeBadges(in models.Badges) (models.Badges, string) {
	if len(in) > models.MaxBadges {
		return nil, fmt.Sprintf("En fazla %d rozet ekleyebilirsiniz.", models.MaxBadges)
	}

	out := make(models.Badges, 0, len(in))

	for _, badge := range in {
		// Every field of a badge is stored in the jsonb column, the id included,
		// so a text PostgreSQL cannot store in any of them refuses the list (see
		// UnstorableText).
		if UnstorableText(badge.ID) || UnstorableText(badge.Text) || UnstorableText(badge.Icon) ||
			UnstorableText(badge.BgColor) || UnstorableText(badge.TextColor) {
			return nil, "Rozet listesi geçersiz."
		}

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

// maxOptionItemPrice caps the surcharge of a single option. It is a surcharge
// on top of the product price, not a price of its own, so the ceiling is
// deliberately generous — it exists to catch a stray keystroke, not to have an
// opinion about what a cafe may charge.
const maxOptionItemPrice = 100000

// SanitizeOptions validates the option groups of a product and returns the
// cleaned list plus an error message, which is empty when everything is valid.
//
// It checks the RAW payload before normalising, because Normalize truncates
// silently: it caps the list at MaxOptionGroups and each group at
// MaxOptionItems and drops what it cannot use, so a ninth group would be
// quietly accepted instead of refused if the counts were read afterwards.
// The type comparison is case-insensitive for the same reason — Normalize
// lowercases before it compares, so "Single" has to be accepted here too or
// the model and the handler would disagree about the same payload.
//
// A group with a valid name but no usable items is NOT an error: the dashboard
// drops such a group rather than refusing to save, and Normalize drops it here
// as well.
//
// NOTE: the messages are shown to the end user and are therefore Turkish.
func SanitizeOptions(in models.ProductOptions) (models.ProductOptions, string) {
	if len(in) > models.MaxOptionGroups {
		return nil, fmt.Sprintf("En fazla %d seçenek grubu ekleyebilirsiniz.",
			models.MaxOptionGroups)
	}

	for _, group := range in {
		// The group and item names are stored in the jsonb column, so a text
		// PostgreSQL cannot store in one of them refuses the list (see
		// UnstorableText). The type is not stored as sent — an unknown one is
		// refused below — so it needs no check of its own.
		if UnstorableText(group.Name) {
			return nil, "Seçenek listesi geçersiz."
		}
		name := strings.TrimSpace(group.Name)
		if name == "" {
			return nil, "Seçenek grubunun adı zorunludur."
		}
		if len([]rune(name)) > models.MaxOptionNameRunes {
			return nil, fmt.Sprintf("Seçenek grubunun adı en fazla %d karakter olabilir.",
				models.MaxOptionNameRunes)
		}

		switch strings.ToLower(strings.TrimSpace(group.Type)) {
		case models.OptionTypeSingle, models.OptionTypeMultiple:
		default:
			return nil, "Seçenek türü tek veya çoklu olmalıdır."
		}

		if len(group.Items) > models.MaxOptionItems {
			return nil, fmt.Sprintf("Bir grupta en fazla %d seçenek olabilir.",
				models.MaxOptionItems)
		}

		for _, item := range group.Items {
			if UnstorableText(item.Name) {
				return nil, "Seçenek listesi geçersiz."
			}
			itemName := strings.TrimSpace(item.Name)
			if itemName == "" {
				return nil, "Seçenek adı zorunludur."
			}
			if len([]rune(itemName)) > models.MaxOptionNameRunes {
				return nil, fmt.Sprintf("Seçenek adı en fazla %d karakter olabilir.",
					models.MaxOptionNameRunes)
			}
			if item.Price < 0 {
				return nil, "Seçenek fiyatı sıfırdan küçük olamaz."
			}
			if item.Price > maxOptionItemPrice {
				return nil, "Seçenek fiyatı çok yüksek."
			}
		}
	}

	// Normalize is the last pass: it trims the fields, coerces the type, rounds
	// the prices, drops the groups that carry no answer and guarantees a
	// non-nil slice, which the NOT NULL jsonb column needs.
	return in.Normalize(), ""
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
