package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/config"
	"karecik/backend/internal/utils"
)

const (
	ctxUserID     = "karecik_user_id"
	ctxBusinessID = "karecik_business_id"
)

// Protected, Authorization: Bearer <token> basligini dogrular ve
// kullanici/isletme kimligini istek baglamina yazar.
func Protected(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get("Authorization"))
		if header == "" {
			return utils.Unauthorized(c, "Bu işlem için oturum açmanız gerekiyor.")
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return utils.Unauthorized(c, "Geçersiz yetkilendirme başlığı.")
		}

		claims, err := utils.ParseToken(cfg.JWTSecret, strings.TrimSpace(parts[1]))
		if err != nil {
			return utils.Unauthorized(c, "Oturumunuz sona ermiş, lütfen tekrar giriş yapın.")
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			return utils.Unauthorized(c, "Geçersiz oturum bilgisi.")
		}
		businessID, err := uuid.Parse(claims.BusinessID)
		if err != nil {
			return utils.Unauthorized(c, "Geçersiz oturum bilgisi.")
		}

		c.Locals(ctxUserID, userID)
		c.Locals(ctxBusinessID, businessID)
		return c.Next()
	}
}

// UserID, Protected middleware'inin yazdigi kullanici kimligini dondurur.
func UserID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(ctxUserID).(uuid.UUID)
	return id
}

// BusinessID, Protected middleware'inin yazdigi isletme kimligini dondurur.
// Tum panel uclari isletme kapsamini buradan alir; istemci asla business_id gondermez.
func BusinessID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(ctxBusinessID).(uuid.UUID)
	return id
}
