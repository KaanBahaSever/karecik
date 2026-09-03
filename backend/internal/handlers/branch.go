package handlers

import (
	"context"
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

type menuRequest struct {
	Name        string  `json:"name"`
	Slug        *string `json:"slug"`
	Description string  `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

type branchRequest struct {
	Name         string  `json:"name"`
	Slug         *string `json:"slug"`
	Phone        *string `json:"phone"`
	Address      *string `json:"address"`
	WifiSSID     *string `json:"wifi_ssid"`
	WifiPassword *string `json:"wifi_password"`
	IsDefault    *bool   `json:"is_default"`
	IsActive     *bool   `json:"is_active"`
}

type branchMenusRequest struct {
	MenuIDs       []uuid.UUID `json:"menu_ids"`
	DefaultMenuID uuid.UUID   `json:"default_menu_id"`
}

type branchPriceRequest struct {
	Price        *float64 `json:"price"`
	ComparePrice *float64 `json:"compare_price"`
	IsAvailable  *bool    `json:"is_available"`
}

// ------------------------------------------------------------------- menus

// ListMenus — GET /api/menus
func (h *Handler) ListMenus(c *fiber.Ctx) error {
	menus, err := repository.ListMenus(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, menus)
}

// CreateMenu — POST /api/menus
func (h *Handler) CreateMenu(c *fiber.Ctx) error {
	var req menuRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)

	name := strings.TrimSpace(req.Name)
	if errMessage := validateMenuName(name); errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 200 {
		return utils.Unprocessable(c, "Menü açıklaması en fazla 200 karakter olabilir.")
	}

	// An empty slug is derived from the name, exactly like the sign-up does.
	raw := name
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		raw = *req.Slug
	}
	slug, ok, err := h.validateMenuSlug(c, businessID, raw, uuid.Nil)
	if !ok {
		return err
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	menu, err := repository.CreateMenu(c.Context(), h.DB, businessID,
		name, slug, description, isActive)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu menü adresi zaten kullanılıyor.")
		}
		return utils.Internal(c, err)
	}
	return utils.Created(c, menu)
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

	businessID := middleware.BusinessID(c)
	fields := make(map[string]any)

	if value, ok := raw["name"]; ok {
		name, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "Menü adı metin olmalıdır.")
		}
		name = strings.TrimSpace(name)
		if errMessage := validateMenuName(name); errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["name"] = name
	}

	if value, ok := raw["slug"]; ok {
		text, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "Menü adresi metin olmalıdır.")
		}
		slug, valid, err := h.validateMenuSlug(c, businessID, text, id)
		if !valid {
			return err
		}
		fields["slug"] = slug
	}

	if value, ok := raw["description"]; ok {
		description, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "Menü açıklaması metin olmalıdır.")
		}
		if len([]rune(description)) > 200 {
			return utils.Unprocessable(c, "Menü açıklaması en fazla 200 karakter olabilir.")
		}
		fields["description"] = strings.TrimSpace(description)
	}

	for _, key := range []string{"is_default", "is_active"} {
		if value, ok := raw[key]; ok {
			flag, err := decodeBool(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = flag
		}
	}

	if value, ok := raw["position"]; ok {
		position, err := decodeInt(value)
		if err != nil || position < 0 {
			return utils.Unprocessable(c, "Sıra değeri sıfır veya daha büyük olmalıdır.")
		}
		fields["position"] = position
	}

	menu, err := repository.UpdateMenu(c.Context(), h.DB, id, businessID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu menü adresi zaten kullanılıyor.")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, menu)
}

// DeleteMenu — DELETE /api/menus/:id
// The categories of the menu — and the products inside them — are removed
// along with it, so the dashboard asks for a confirmation first.
func (h *Handler) DeleteMenu(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz menü kimliği.")
	}

	businessID := middleware.BusinessID(c)

	// The ownership check comes first: DeleteMenu counts the menus of the
	// business before it deletes, so a foreign id would otherwise be answered
	// with "the last menu cannot be deleted" instead of a plain 404.
	if _, err := repository.GetMenu(c.Context(), h.DB, id, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	err = repository.DeleteMenu(c.Context(), h.DB, id, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrLastMenu) {
			return utils.Unprocessable(c, "Son menü silinemez. İşletmenin en az bir menüsü olmalıdır.")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, fiber.Map{"success": true})
}

// ---------------------------------------------------------------- branches

// ListBranches — GET /api/branches
func (h *Handler) ListBranches(c *fiber.Ctx) error {
	branches, err := repository.ListBranches(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		return utils.Internal(c, err)
	}
	for i := range branches {
		branches[i].MenuURL = h.menuURL(branches[i].Slug)
	}
	return utils.OK(c, branches)
}

// CreateBranch — POST /api/branches
func (h *Handler) CreateBranch(c *fiber.Ctx) error {
	var req branchRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)

	name := strings.TrimSpace(req.Name)
	if errMessage := validateBranchName(name); errMessage != "" {
		return utils.Unprocessable(c, errMessage)
	}

	raw := name
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		raw = *req.Slug
	}
	slug, ok, err := h.validateBranchSlug(c, raw, uuid.Nil)
	if !ok {
		return err
	}

	in := models.Branch{
		Name:         name,
		Slug:         slug,
		Phone:        strPtrOrNil(req.Phone),
		Address:      strPtrOrNil(req.Address),
		WifiSSID:     strPtrOrNil(req.WifiSSID),
		WifiPassword: strPtrOrNil(req.WifiPassword),
		IsActive:     true,
	}
	if req.IsDefault != nil {
		in.IsDefault = *req.IsDefault
	}
	if req.IsActive != nil {
		in.IsActive = *req.IsActive
	}

	branch, err := repository.CreateBranch(c.Context(), h.DB, businessID, in)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu şube adresi başka bir işletme tarafından kullanılıyor.")
		}
		return utils.Internal(c, err)
	}

	// A fresh branch serves the default menu of the business until the owner
	// picks the menus itself; otherwise its address would resolve to nothing.
	if menu, err := repository.GetDefaultMenu(c.Context(), h.DB, businessID); err == nil {
		if err := repository.SetBranchMenus(c.Context(), h.DB, branch.ID,
			[]uuid.UUID{menu.ID}, menu.ID); err != nil {
			return utils.Internal(c, err)
		}
		branch.MenuIDs = []uuid.UUID{menu.ID}
	} else if !errors.Is(err, repository.ErrNotFound) {
		return utils.Internal(c, err)
	}

	branch.MenuURL = h.menuURL(branch.Slug)
	return utils.Created(c, branch)
}

// UpdateBranch — PUT /api/branches/:id
// Partial update: only the fields present in the body are applied.
func (h *Handler) UpdateBranch(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz şube kimliği.")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)

	// The branch has to belong to this business before anything is written.
	if _, err := repository.GetBranch(c.Context(), h.DB, id, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Şube bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	fields := make(map[string]any)

	if value, ok := raw["name"]; ok {
		name, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "Şube adı metin olmalıdır.")
		}
		name = strings.TrimSpace(name)
		if errMessage := validateBranchName(name); errMessage != "" {
			return utils.Unprocessable(c, errMessage)
		}
		fields["name"] = name
	}

	if value, ok := raw["slug"]; ok {
		text, err := decodeString(value)
		if err != nil {
			return utils.Unprocessable(c, "Şube adresi metin olmalıdır.")
		}
		slug, valid, err := h.validateBranchSlug(c, text, id)
		if !valid {
			return err
		}
		fields["slug"] = slug
	}

	// --- clearable text fields (null -> NULL)
	for _, key := range []string{"phone", "address", "wifi_ssid", "wifi_password"} {
		if value, ok := raw[key]; ok {
			ptr, err := decodeNullableString(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı metin veya boş olmalıdır.")
			}
			fields[key] = ptr
		}
	}

	for _, key := range []string{"is_default", "is_active"} {
		if value, ok := raw[key]; ok {
			flag, err := decodeBool(value)
			if err != nil {
				return utils.Unprocessable(c, key+" alanı true/false olmalıdır.")
			}
			fields[key] = flag
		}
	}

	if value, ok := raw["position"]; ok {
		position, err := decodeInt(value)
		if err != nil || position < 0 {
			return utils.Unprocessable(c, "Sıra değeri sıfır veya daha büyük olmalıdır.")
		}
		fields["position"] = position
	}

	branch, err := repository.UpdateBranch(c.Context(), h.DB, id, businessID, fields)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu şube adresi başka bir işletme tarafından kullanılıyor.")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Şube bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	branch.MenuURL = h.menuURL(branch.Slug)
	return utils.OK(c, branch)
}

// DeleteBranch — DELETE /api/branches/:id
// The menu links and the price overrides of the branch are removed with it.
func (h *Handler) DeleteBranch(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz şube kimliği.")
	}

	if err := repository.DeleteBranch(c.Context(), h.DB, id, middleware.BusinessID(c)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Şube bulunamadı.")
		}
		return utils.Internal(c, err)
	}
	return utils.OK(c, fiber.Map{"success": true})
}

// SetBranchMenus — PUT /api/branches/:id/menus
// Body: {"menu_ids": [...], "default_menu_id": "..."}
func (h *Handler) SetBranchMenus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz şube kimliği.")
	}

	var req branchMenusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	businessID := middleware.BusinessID(c)

	if _, err := repository.GetBranch(c.Context(), h.DB, id, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Şube bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	if len(req.MenuIDs) == 0 {
		return utils.Unprocessable(c, "Şubeye en az bir menü seçmelisiniz.")
	}

	// Every menu id has to belong to this business — the repository would
	// silently drop a foreign one, which would hide the mistake.
	menus, err := repository.ListMenus(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}
	owned := make(map[uuid.UUID]bool, len(menus))
	for _, menu := range menus {
		owned[menu.ID] = true
	}

	seen := make(map[uuid.UUID]bool, len(req.MenuIDs))
	menuIDs := make([]uuid.UUID, 0, len(req.MenuIDs))
	for _, menuID := range req.MenuIDs {
		if !owned[menuID] {
			return utils.Forbidden(c, "Seçilen menülerden biri bu işletmeye ait değil.")
		}
		if seen[menuID] {
			continue
		}
		seen[menuID] = true
		menuIDs = append(menuIDs, menuID)
	}

	if req.DefaultMenuID != uuid.Nil && !seen[req.DefaultMenuID] {
		return utils.Unprocessable(c, "Varsayılan menü, şubenin menüleri arasında olmalıdır.")
	}

	if err := repository.SetBranchMenus(c.Context(), h.DB, id, menuIDs, req.DefaultMenuID); err != nil {
		return utils.Internal(c, err)
	}

	branch, err := repository.GetBranch(c.Context(), h.DB, id, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}
	branch.MenuURL = h.menuURL(branch.Slug)
	return utils.OK(c, branch)
}

// ---------------------------------------------------------- branch prices

// ListBranchPrices — GET /api/branches/:id/prices
// Returns the price overrides of the branch as a list; a product without an
// override simply has no entry and inherits its own price.
func (h *Handler) ListBranchPrices(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz şube kimliği.")
	}

	businessID := middleware.BusinessID(c)
	if _, err := repository.GetBranch(c.Context(), h.DB, id, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Şube bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	priceByProduct, err := repository.ListBranchPrices(c.Context(), h.DB, id)
	if err != nil {
		return utils.Internal(c, err)
	}

	prices := make([]models.BranchPrice, 0, len(priceByProduct))
	for _, price := range priceByProduct {
		prices = append(prices, price)
	}
	return utils.OK(c, prices)
}

// SetBranchPrice — PUT /api/branches/:id/prices/:productId
// Body: {"price": 120, "compare_price": null, "is_available": true}
// A null price means the branch inherits the price of the product itself.
func (h *Handler) SetBranchPrice(c *fiber.Ctx) error {
	branchID, productID, ok, err := h.branchAndProduct(c)
	if !ok {
		return err
	}

	var req branchPriceRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	if req.Price != nil && *req.Price < 0 {
		return utils.Unprocessable(c, "Fiyat sıfırdan küçük olamaz.")
	}
	if req.ComparePrice != nil && *req.ComparePrice < 0 {
		return utils.Unprocessable(c, "Karşılaştırma fiyatı sıfırdan küçük olamaz.")
	}

	price := models.BranchPrice{
		BranchID:     branchID,
		ProductID:    productID,
		Price:        roundPtr(req.Price),
		ComparePrice: roundPtr(req.ComparePrice),
		IsAvailable:  true,
	}
	if req.IsAvailable != nil {
		price.IsAvailable = *req.IsAvailable
	}

	if err := repository.UpsertBranchPrice(c.Context(), h.DB, price); err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, price)
}

// DeleteBranchPrice — DELETE /api/branches/:id/prices/:productId
// Clearing an override that does not exist is not an error: the product
// already inherits its own price, which is exactly what was asked for.
func (h *Handler) DeleteBranchPrice(c *fiber.Ctx) error {
	branchID, productID, ok, err := h.branchAndProduct(c)
	if !ok {
		return err
	}

	err = repository.DeleteBranchPrice(c.Context(), h.DB, branchID, productID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return utils.Internal(c, err)
	}
	return utils.OK(c, fiber.Map{"success": true})
}

// ----------------------------------------------------------------- helpers

// branchAndProduct parses both path parameters and verifies that the branch
// and the product belong to the business of the token. Like the slug helpers
// it answers the request itself when it refuses it, which the false `ok`
// reports to the caller.
func (h *Handler) branchAndProduct(c *fiber.Ctx) (uuid.UUID, uuid.UUID, bool, error) {
	branchID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, false, utils.BadRequest(c, "Geçersiz şube kimliği.")
	}
	productID, err := uuid.Parse(c.Params("productId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, false, utils.BadRequest(c, "Geçersiz ürün kimliği.")
	}

	businessID := middleware.BusinessID(c)

	if _, err := repository.GetBranch(c.Context(), h.DB, branchID, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return uuid.Nil, uuid.Nil, false, utils.NotFound(c, "Şube bulunamadı.")
		}
		return uuid.Nil, uuid.Nil, false, utils.Internal(c, err)
	}
	if _, err := repository.GetProduct(c.Context(), h.DB, productID, businessID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return uuid.Nil, uuid.Nil, false, utils.NotFound(c, "Ürün bulunamadı.")
		}
		return uuid.Nil, uuid.Nil, false, utils.Internal(c, err)
	}
	return branchID, productID, true, nil
}

// findBranchBySlug looks a branch up inside one business, inactive ones
// included. repository.GetBranchBySlug is the public lookup and only sees
// published branches of published businesses, which is not what the owner's
// own preview needs.
func findBranchBySlug(ctx context.Context, db repository.DB, businessID uuid.UUID,
	slug string) (*models.Branch, error) {

	slug = strings.ToLower(strings.TrimSpace(slug))

	branches, err := repository.ListBranches(ctx, db, businessID)
	if err != nil {
		return nil, err
	}
	for i := range branches {
		if branches[i].Slug == slug {
			return &branches[i], nil
		}
	}
	return nil, repository.ErrNotFound
}

func validateMenuName(name string) string {
	if length := len([]rune(name)); length < 2 || length > 60 {
		return "Menü adı 2 ile 60 karakter arasında olmalıdır."
	}
	return ""
}

func validateBranchName(name string) string {
	if length := len([]rune(name)); length < 2 || length > 60 {
		return "Şube adı 2 ile 60 karakter arasında olmalıdır."
	}
	return ""
}

// The helpers below answer the request themselves when they refuse it, because
// a slug can fail with three different statuses. They report that through the
// `ok` flag: when it is false the response is already written and the handler
// only has to return the error value alongside it — which the utils helpers
// leave nil unless writing the body itself failed.

// validateMenuSlug mirrors the business slug logic: slugify, validate, refuse
// the reserved words and finally check it against the menus of the business.
func (h *Handler) validateMenuSlug(c *fiber.Ctx, businessID uuid.UUID,
	raw string, exceptID uuid.UUID) (string, bool, error) {

	slug := utils.Slugify(raw)
	if !utils.IsValidSlug(slug) {
		return "", false, utils.Unprocessable(c,
			"Menü adresi yalnızca harf, rakam ve tire içerebilir (en az 2 karakter).")
	}
	if utils.IsReservedSlug(slug) {
		return "", false, utils.Conflict(c, "Bu menü adresi sistem tarafından ayrılmıştır.")
	}

	taken, err := repository.MenuSlugTaken(c.Context(), h.DB, businessID, slug, exceptID)
	if err != nil {
		return "", false, utils.Internal(c, err)
	}
	if taken {
		return "", false, utils.Conflict(c, "Bu menü adresi zaten kullanılıyor.")
	}
	return slug, true, nil
}

// validateBranchSlug does the same for a branch, whose slug is a subdomain and
// therefore shares one namespace with the business slugs.
func (h *Handler) validateBranchSlug(c *fiber.Ctx, raw string,
	exceptID uuid.UUID) (string, bool, error) {

	slug := utils.Slugify(raw)
	if !utils.IsValidSlug(slug) {
		return "", false, utils.Unprocessable(c,
			"Şube adresi yalnızca harf, rakam ve tire içerebilir (en az 2 karakter).")
	}
	if utils.IsReservedSlug(slug) {
		return "", false, utils.Conflict(c, "Bu şube adresi sistem tarafından ayrılmıştır.")
	}

	taken, err := repository.BranchSlugTaken(c.Context(), h.DB, slug, exceptID)
	if err != nil {
		return "", false, utils.Internal(c, err)
	}
	if taken {
		return "", false, utils.Conflict(c,
			"Bu şube adresi başka bir işletme tarafından kullanılıyor.")
	}
	return slug, true, nil
}

// strPtrOrNil trims an optional text field and turns an empty one into NULL.
func strPtrOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	return strPtr(*value)
}
