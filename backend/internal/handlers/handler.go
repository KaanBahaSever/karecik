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

// Handler, tum HTTP uclarinin ortak bagimliliklarini tasir.
type Handler struct {
	DB  *pgxpool.Pool
	Cfg *config.Config
}

// New, yeni bir Handler olusturur.
func New(db *pgxpool.Pool, cfg *config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}

// Health, servis ve veritabani durumunu dondurur.
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

// menuURL, isletmenin musteri menusu adresini uretir.
//
//	production : https://kahve-duragi.karecik.com
//	gelistirme : http://kahve-duragi.localhost:5173
func (h *Handler) menuURL(slug string) string {
	if h.Cfg.IsProduction() {
		return fmt.Sprintf("https://%s.%s", slug, h.Cfg.AppDomain)
	}

	port := "5173"
	if len(h.Cfg.CORSOrigins) > 0 {
		if u, err := url.Parse(h.Cfg.CORSOrigins[0]); err == nil && u.Port() != "" {
			port = u.Port()
		}
	}
	return fmt.Sprintf("http://%s.%s:%s", slug, h.Cfg.DevDomain, port)
}

// withMenuURL, hesaplanan menu adresini isletme nesnesine ekler.
func (h *Handler) withMenuURL(b *models.Business) *models.Business {
	if b != nil {
		b.MenuURL = h.menuURL(b.Slug)
	}
	return b
}

// ---------------------------------------------------------------- yardimcilar

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailPattern.MatchString(strings.TrimSpace(email))
}

// strPtr, bos stringi NULL'a cevirerek *string dondurur.
func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
