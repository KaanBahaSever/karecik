package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/config"
	"karecik/backend/internal/mailer"
	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/session"
)

// Handler carries the dependencies shared by every HTTP endpoint.
//
// Sessions is not in the database: it is a map in this process. It is a field
// here rather than a package-level variable so that a test can stand up an
// isolated app with its own sessions, the same way it already gets its own
// scratch database.
//
// NOTE: the error messages returned from these handlers are shown to the end
// user and are therefore written in Turkish on purpose.
type Handler struct {
	DB       *pgxpool.Pool
	Cfg      *config.Config
	Sessions *session.Store

	// Mail is an interface so that a test can assert an e-mail WOULD have been
	// sent without a network call, and so that a deployment with no provider
	// configured gets mailer.Disabled — which refuses loudly instead of
	// pretending to deliver.
	Mail mailer.Mailer
}

// New builds a Handler.
func New(db *pgxpool.Pool, cfg *config.Config, sessions *session.Store, mail mailer.Mailer) *Handler {
	if mail == nil {
		mail = mailer.Disabled{}
	}
	return &Handler{DB: db, Cfg: cfg, Sessions: sessions, Mail: mail}
}

// Health reports the service and database status.
//
// It also asserts that the frontend bundle is actually reachable when this
// process is supposed to be serving it, and returns 503 when it is not.
//
// That check earns its place: the deploy where the image shipped without a
// bundle reported HEALTHY the whole time, because this endpoint is an /api
// route and knows nothing about static files. The platform saw a green tick
// while every page on the site answered with a JSON error. A health check that
// cannot see the thing that is broken is worse than none, because it is
// believed.
func (h *Handler) Health(c *fiber.Ctx) error {
	dbStatus := "up"
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	if err := h.DB.Ping(ctx); err != nil {
		dbStatus = "down"
	}

	if h.Cfg.ServeStatic {
		index := filepath.Join(h.Cfg.StaticDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			// The resolved path is in the body on purpose. The failure is almost
			// always STATIC_DIR pointing somewhere the bundle is not, and the
			// answer is unreadable without knowing where it looked.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":   "degraded",
				"database": dbStatus,
				"error":    "frontend bundle is missing: " + index,
				"version":  "1.0.0",
			})
		}
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"database": dbStatus,
		"version":  "1.0.0",
	})
}

// ------------------------------------------------------------------ addresses
// {business-slug}.karecik.com/{menu-slug}: the subdomain names the tenant, the
// path names one menu inside it.

// homeURL is the tenant's root address, with no menu path.
//
//	production  : https://kahve-duragi.karecik.com
//	development : http://kahve-duragi.localhost:5173
func (h *Handler) homeURL(businessSlug string) string {
	if h.Cfg.IsProduction() {
		return fmt.Sprintf("https://%s.%s", businessSlug, h.Cfg.AppDomain)
	}

	port := "5173"
	if len(h.Cfg.CORSOrigins) > 0 {
		if parsed, err := url.Parse(h.Cfg.CORSOrigins[0]); err == nil && parsed.Port() != "" {
			port = parsed.Port()
		}
	}
	return fmt.Sprintf("http://%s.%s:%s", businessSlug, h.Cfg.DevDomain, port)
}

// menuURL is the full address of one menu — the tenant's subdomain plus the
// menu's path segment — and therefore exactly what the QR code encodes.
//
//	production  : https://kahve-duragi.karecik.com/kahvalti
//	development : http://kahve-duragi.localhost:5173/kahvalti
func (h *Handler) menuURL(businessSlug, menuSlug string) string {
	return h.homeURL(businessSlug) + "/" + menuSlug
}

// withMenuURL attaches the computed public address to a menu. It needs the slug
// of the owning business, because half of the address belongs to the tenant.
func (h *Handler) withMenuURL(businessSlug string, menu *models.Menu) *models.Menu {
	if menu != nil {
		menu.MenuURL = h.menuURL(businessSlug, menu.Slug)
	}
	return menu
}

// withHomeURL attaches the tenant's root address to the account record.
func (h *Handler) withHomeURL(business *models.Business) *models.Business {
	if business != nil {
		business.HomeURL = h.homeURL(business.Slug)
	}
	return business
}

// currentBusiness loads the tenant behind the token. Every menu address needs
// its slug, so the menu endpoints read it before they answer.
func (h *Handler) currentBusiness(c *fiber.Ctx) (*models.Business, error) {
	return repository.GetBusinessByID(c.Context(), h.DB, middleware.BusinessID(c))
}

// ------------------------------------------------------------------ helpers

// menuLanguage resolves the language the texts of a record have to be written
// in: the default_language of the menu the record lives on. There is no default
// menu of a business to ask any more, so the menu is always reached through the
// record itself.
//
// Turkish is a last resort for a menu that disappeared between two requests —
// categories.menu_id is NOT NULL, so every live record really has one.
func (h *Handler) menuLanguage(c *fiber.Ctx, businessID uuid.UUID,
	menuID *uuid.UUID) (string, error) {

	if menuID == nil {
		return "tr", nil
	}
	menu, err := repository.GetMenu(c.Context(), h.DB, *menuID, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "tr", nil
		}
		return "", err
	}
	return menu.DefaultLanguage, nil
}

// categoryLanguage walks category -> menu to reach the same answer for a record
// that only knows its category.
func (h *Handler) categoryLanguage(c *fiber.Ctx, businessID, categoryID uuid.UUID) (string, error) {
	category, err := repository.GetCategory(c.Context(), h.DB, categoryID, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "tr", nil
		}
		return "", err
	}
	return h.menuLanguage(c, businessID, category.MenuID)
}

// productLanguage walks product -> category -> menu, for the update path, where
// the body may carry nothing but the new translations.
func (h *Handler) productLanguage(c *fiber.Ctx, businessID, productID uuid.UUID) (string, error) {
	product, err := repository.GetProduct(c.Context(), h.DB, productID, businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "tr", nil
		}
		return "", err
	}
	return h.categoryLanguage(c, businessID, product.CategoryID)
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailPattern.MatchString(strings.TrimSpace(email))
}

// strPtr turns an empty string into NULL and returns a *string.
func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
