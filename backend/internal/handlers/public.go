package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// PublicMenuBySlug — GET /api/public/menu/:slug?lang=tr
// Path-based access, used locally and as a fallback in QR links.
func (h *Handler) PublicMenuBySlug(c *fiber.Ctx) error {
	return h.servePublicMenu(c, strings.TrimSpace(c.Params("slug")))
}

// PublicMenuByHost — GET /api/public/menu?lang=tr
// Subdomain-based access: kahve-duragi.karecik.com -> slug "kahve-duragi".
func (h *Handler) PublicMenuByHost(c *fiber.Ctx) error {
	slug := middleware.SubdomainOf(c, h.Cfg)
	if slug == "" {
		return utils.NotFound(c,
			"Menü adresi çözümlenemedi. Adres <isletme>."+h.Cfg.AppDomain+" biçiminde olmalıdır.")
	}
	return h.servePublicMenu(c, slug)
}

func (h *Handler) servePublicMenu(c *fiber.Ctx, slug string) error {
	if slug == "" {
		return utils.NotFound(c, "Menü bulunamadı.")
	}

	business, err := repository.GetBusinessBySlug(c.Context(), h.DB, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "Böyle bir menü bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	menu, err := repository.BuildPublicMenu(c.Context(), h.DB, business,
		strings.TrimSpace(c.Query("lang")), false)
	if err != nil {
		return utils.Internal(c, err)
	}

	// Menu content changes rarely; a short cache eases the QR traffic.
	c.Set("Cache-Control", "public, max-age=60")
	return utils.OK(c, menu)
}

// PreviewMenu — GET /api/preview/menu?lang=tr  (authenticated)
// Backs the dashboard live preview and also returns inactive records.
func (h *Handler) PreviewMenu(c *fiber.Ctx) error {
	business, err := repository.GetBusinessByID(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, "İşletme bulunamadı.")
		}
		return utils.Internal(c, err)
	}

	menu, err := repository.BuildPublicMenu(c.Context(), h.DB, business,
		strings.TrimSpace(c.Query("lang")), true)
	if err != nil {
		return utils.Internal(c, err)
	}

	c.Set("Cache-Control", "no-store")
	return utils.OK(c, menu)
}
