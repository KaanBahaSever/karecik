package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// PublicMenuBySlug — GET /api/public/menu/:slug?lang=tr&menu=<slug>
// Path-based access, used locally and as a fallback in QR links.
func (h *Handler) PublicMenuBySlug(c *fiber.Ctx) error {
	return h.servePublicMenu(c,
		strings.TrimSpace(c.Params("slug")),
		strings.TrimSpace(c.Query("menu")))
}

// PublicMenuByHost — GET /api/public/menu?lang=tr&menu=<slug>
// Subdomain-based access: kahve-duragi.karecik.com -> slug "kahve-duragi".
func (h *Handler) PublicMenuByHost(c *fiber.Ctx) error {
	slug := middleware.SubdomainOf(c, h.Cfg)
	if slug == "" {
		return utils.NotFound(c,
			"Menü adresi çözümlenemedi. Adres <isletme>."+h.Cfg.AppDomain+" biçiminde olmalıdır.")
	}
	return h.servePublicMenu(c, slug, strings.TrimSpace(c.Query("menu")))
}

// PublicMenuByBranch — GET /api/public/b/:branch_slug[/:menu_slug]?lang=tr
// The explicit branch form of the address; the menu slug is optional.
func (h *Handler) PublicMenuByBranch(c *fiber.Ctx) error {
	return h.servePublicMenu(c,
		strings.TrimSpace(c.Params("branch_slug")),
		strings.TrimSpace(c.Params("menu_slug")))
}

// servePublicMenu resolves the tenant behind a slug and renders its menu.
// The slug may name a branch or a business — ResolveTenant tries them in that
// order, so the addresses that existed before branches keep working.
func (h *Handler) servePublicMenu(c *fiber.Ctx, slug, menuSlug string) error {
	if slug == "" {
		return utils.NotFound(c, "Menü bulunamadı.")
	}

	tenant, err := middleware.ResolveTenant(c.Context(), h.DB, slug, menuSlug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Böyle bir menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	menu, err := repository.BuildPublicMenu(c.Context(), h.DB, tenant.Business,
		repository.PublicMenuOptions{
			Lang:            strings.TrimSpace(c.Query("lang")),
			IncludeInactive: false,
			Branch:          tenant.Branch,
			Menu:            tenant.Menu,
		})
	if err != nil {
		return utils.Internal(c, err)
	}

	// Menu content changes rarely; a short cache eases the QR traffic.
	c.Set("Cache-Control", "public, max-age=60")
	return utils.OK(c, menu)
}

// PreviewMenu — GET /api/preview/menu?lang=tr&branch=<slug>&menu=<slug>  (authenticated)
// Backs the dashboard live preview and also returns inactive records. The
// branch and menu slugs are scoped to the business of the token, never to the
// globally unique public namespace.
func (h *Handler) PreviewMenu(c *fiber.Ctx) error {
	businessID := middleware.BusinessID(c)

	business, err := repository.GetBusinessByID(c.Context(), h.DB, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "İşletme bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	opts := repository.PublicMenuOptions{
		Lang:            strings.TrimSpace(c.Query("lang")),
		IncludeInactive: true,
	}

	if slug := strings.TrimSpace(c.Query("branch")); slug != "" {
		// The owner also previews an unpublished branch, which the public
		// repository.GetBranchBySlug would hide, so the scoped lookup is used.
		branch, err := findBranchBySlug(c.Context(), h.DB, businessID, slug)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.NotFound(c, "Şube bulunamadı.")
			}
			return utils.Internal(c, err)
		}
		opts.Branch = branch
	}

	menu, ok, err := h.previewMenuOf(c, businessID, opts.Branch)
	if !ok {
		return err
	}
	opts.Menu = menu

	payload, err := repository.BuildPublicMenu(c.Context(), h.DB, business, opts)
	if err != nil {
		return utils.Internal(c, err)
	}

	c.Set("Cache-Control", "no-store")
	return utils.OK(c, payload)
}

// previewMenuOf picks the menu the preview renders: the requested one, then the
// default menu of the previewed branch, then the default menu of the business.
// A business without menus previews every category, exactly like the customer
// menu does, so a nil menu is a valid answer — a refused request is reported
// through the false `ok` after the response has already been written.
func (h *Handler) previewMenuOf(c *fiber.Ctx, businessID uuid.UUID,
	branch *models.Branch) (*models.Menu, bool, error) {

	if slug := strings.TrimSpace(c.Query("menu")); slug != "" {
		menu, err := repository.GetMenuBySlug(c.Context(), h.DB, businessID, slug)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, false, utils.NotFound(c, "Menü bulunamadı.")
			}
			return nil, false, utils.Internal(c, err)
		}
		return menu, true, nil
	}

	if branch != nil {
		menu, err := repository.DefaultMenuOfBranch(c.Context(), h.DB, branch.ID)
		if err == nil {
			return menu, true, nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, false, utils.Internal(c, err)
		}
	}

	menu, err := repository.GetDefaultMenu(c.Context(), h.DB, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, true, nil
		}
		return nil, false, utils.Internal(c, err)
	}
	return menu, true, nil
}
