package handlers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/config"
	"karecik/backend/internal/models"
)

// Handler carries the dependencies shared by every HTTP endpoint.
//
// NOTE: the error messages returned from these handlers are shown to the end
// user and are therefore written in Turkish on purpose.
type Handler struct {
	DB  *pgxpool.Pool
	Cfg *config.Config
}

// New builds a Handler.
func New(db *pgxpool.Pool, cfg *config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}

// Health reports the service and database status.
func (h *Handler) Health(c *fiber.Ctx) error {
	dbStatus := "up"
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	if err := h.DB.Ping(ctx); err != nil {
		dbStatus = "down"
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"database": dbStatus,
		"version":  "1.0.0",
	})
}

// menuURL builds the customer menu address of a business.
//
//	production  : https://kahve-duragi.karecik.com
//	development : http://kahve-duragi.localhost:5173
func (h *Handler) menuURL(slug string) string {
	if h.Cfg.IsProduction() {
		return fmt.Sprintf("https://%s.%s", slug, h.Cfg.AppDomain)
	}

	port := "5173"
	if len(h.Cfg.CORSOrigins) > 0 {
		if parsed, err := url.Parse(h.Cfg.CORSOrigins[0]); err == nil && parsed.Port() != "" {
			port = parsed.Port()
		}
	}
	return fmt.Sprintf("http://%s.%s:%s", slug, h.Cfg.DevDomain, port)
}

// withMenuURL attaches the computed menu address to a business object.
func (h *Handler) withMenuURL(business *models.Business) *models.Business {
	if business != nil {
		business.MenuURL = h.menuURL(business.Slug)
	}
	return business
}

// ------------------------------------------------------------------ helpers

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
