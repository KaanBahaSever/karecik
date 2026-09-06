package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// The customer endpoints all read one address: {business-slug}.karecik.com/
// {menu-slug}. The subdomain (or the first path segment of the local fallback)
// names the TENANT; the path segment after it names one menu inside that
// tenant. A tenant that exists is never answered with a 404 — when no single
// menu can be picked the payload comes back with menu_resolved false, which is
// what makes the frontend render the directory or its "no active menus"
// placeholder.

// PublicMenuByHost — GET /api/public/menu?lang=tr&menu=<menu-slug>
// Subdomain access: kahve-duragi.karecik.com -> business slug "kahve-duragi",
// with the menu named by the optional ?menu parameter.
func (h *Handler) PublicMenuByHost(c *fiber.Ctx) error {
	businessSlug := middleware.SubdomainOf(c, h.Cfg)
	if businessSlug == "" {
		return utils.NotFound(c,
			"İşletme adresi çözümlenemedi. Adres <isletme>."+h.Cfg.AppDomain+" biçiminde olmalıdır.")
	}
	return h.servePublicMenu(c, businessSlug, strings.TrimSpace(c.Query("menu")))
}

// PublicMenuByPath — GET /api/public/menu/:businessSlug[/:menuSlug]?lang=tr
// Path-based access, used locally and as the fallback in QR links. The menu
// segment is optional and absent on the tenant landing page.
func (h *Handler) PublicMenuByPath(c *fiber.Ctx) error {
	return h.servePublicMenu(c,
		strings.TrimSpace(c.Params("businessSlug")),
		strings.TrimSpace(c.Params("menuSlug")))
}

// servePublicMenu is the one resolution both public forms share:
//
//  1. resolve the business; 404 when there is no such tenant;
//  2. when a menu slug was supplied, resolve it INSIDE that business — 404 when
//     it names nothing published there;
//  3. otherwise take the only active menu when there is exactly one;
//  4. with zero or two-plus active menus and no slug, answer 200 with
//     menu_resolved false, an empty category list and the menu list.
func (h *Handler) servePublicMenu(c *fiber.Ctx, businessSlug, menuSlug string) error {
	opts := repository.PublicMenuOptions{
		Lang:            strings.TrimSpace(c.Query("lang")),
		IncludeInactive: false,
	}

	business, err := middleware.ResolveBusiness(c.Context(), h.DB, businessSlug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Böyle bir menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	var menu *models.Menu

	if menuSlug != "" {
		menu, err = repository.GetMenuBySlug(c.Context(), h.DB, business.ID, menuSlug)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.NotFound(c, "Böyle bir menü bulunamadı.")
			}
			return utils.Internal(c, err)
		}
	} else {
		menus, err := repository.ListActiveMenus(c.Context(), h.DB, business.ID)
		if err != nil {
			return utils.Internal(c, err)
		}
		// Exactly one menu is the common case: the customer typed the bare
		// subdomain and there is nothing to choose between, so the frontend
		// resolves it and rewrites the address bar to the real link.
		if len(menus) == 1 {
			menu = &menus[0]
		}
	}

	// Menu content changes rarely; a short cache eases the QR traffic. The
	// directory answer is just as public, so it is cached the same way.
	c.Set("Cache-Control", "public, max-age=60")

	if menu == nil {
		payload, err := repository.BuildPublicDirectory(c.Context(), h.DB, business, opts)
		if err != nil {
			return utils.Internal(c, err)
		}
		return utils.OK(c, payload)
	}

	payload, err := repository.BuildPublicMenu(c.Context(), h.DB, business, menu, opts)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, payload)
}

// PreviewMenu — GET /api/preview/menu?lang=tr&menu=<slug>  (authenticated)
// Backs the dashboard live preview and also returns inactive records, so the
// owner can look at a menu before publishing it. The menu slug is scoped to the
// business of the token.
//
// A business with no menus at all is answered with menu_resolved false and an
// empty category list — 200, never a 500 — which is what LivePreview renders
// its placeholder from.
func (h *Handler) PreviewMenu(c *fiber.Ctx) error {
	business, err := h.currentBusiness(c)
	if err != nil {
		return h.businessError(c, err)
	}

	opts := repository.PublicMenuOptions{
		Lang:            strings.TrimSpace(c.Query("lang")),
		IncludeInactive: true,
	}

	menu, ok, err := h.previewMenuOf(c, business.ID)
	if !ok {
		return err
	}

	c.Set("Cache-Control", "no-store")

	if menu == nil {
		payload, err := repository.BuildPublicDirectory(c.Context(), h.DB, business, opts)
		if err != nil {
			return utils.Internal(c, err)
		}
		return utils.OK(c, payload)
	}

	payload, err := repository.BuildPublicMenu(c.Context(), h.DB, business, menu, opts)
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, payload)
}

// previewMenuOf picks the menu the preview renders: the requested one, else the
// first menu by position — the same one the dashboard selects when the stored
// choice is gone. A nil menu with a true `ok` means the business owns none,
// which is answered with the empty payload rather than an error. A refused
// request is reported through the false `ok` after the response has already
// been written.
func (h *Handler) previewMenuOf(c *fiber.Ctx, businessID uuid.UUID) (*models.Menu, bool, error) {
	if slug := strings.TrimSpace(c.Query("menu")); slug != "" {
		menu, err := findMenuBySlug(c.Context(), h.DB, businessID, slug)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, false, utils.NotFound(c, "Menü bulunamadı.")
			}
			return nil, false, utils.Internal(c, err)
		}
		return menu, true, nil
	}

	menus, err := repository.ListMenus(c.Context(), h.DB, businessID)
	if err != nil {
		return nil, false, utils.Internal(c, err)
	}
	if len(menus) == 0 {
		return nil, true, nil
	}
	return &menus[0], true, nil
}

// findMenuBySlug looks a menu up inside one business, unpublished ones
// included. repository.GetMenuBySlug is the public lookup and only sees
// published menus, which is not what the owner's own preview needs.
func findMenuBySlug(ctx context.Context, db repository.DB, businessID uuid.UUID,
	slug string) (*models.Menu, error) {

	slug = strings.ToLower(strings.TrimSpace(slug))

	menus, err := repository.ListMenus(ctx, db, businessID)
	if err != nil {
		return nil, err
	}
	for i := range menus {
		if menus[i].Slug == slug {
			return &menus[i], nil
		}
	}
	return nil, repository.ErrNotFound
}
