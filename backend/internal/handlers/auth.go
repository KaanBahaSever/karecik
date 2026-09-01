package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

type registerRequest struct {
	BusinessName string `json:"business_name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token    string           `json:"token"`
	User     *models.User     `json:"user"`
	Business *models.Business `json:"business"`
}

// Register — POST /api/auth/register
// Creates the user and business records, derives the subdomain slug and
// returns a token.
func (h *Handler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	req.BusinessName = strings.TrimSpace(req.BusinessName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if len([]rune(req.BusinessName)) < 2 || len([]rune(req.BusinessName)) > 100 {
		return utils.Unprocessable(c, "İşletme adı 2 ile 100 karakter arasında olmalıdır.")
	}
	if !isValidEmail(req.Email) {
		return utils.Unprocessable(c, "Geçerli bir e-posta adresi giriniz.")
	}
	if len(req.Password) < 8 {
		return utils.Unprocessable(c, "Şifreniz en az 8 karakter olmalıdır.")
	}

	exists, err := repository.EmailExists(c.Context(), h.DB, req.Email)
	if err != nil {
		return utils.Internal(c, err)
	}
	if exists {
		return utils.Conflict(c, "Bu e-posta adresi zaten kayıtlı. Giriş yapmayı deneyin.")
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return utils.Internal(c, err)
	}

	user, business, err := repository.CreateAccount(c.Context(), h.DB,
		req.BusinessName, req.Email, hash)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return utils.Conflict(c, "Bu e-posta adresi zaten kayıtlı.")
		}
		return utils.Internal(c, err)
	}

	token, err := utils.GenerateToken(h.Cfg.JWTSecret, user.ID, business.ID)
	if err != nil {
		return utils.Internal(c, err)
	}

	return utils.Created(c, authResponse{
		Token:    token,
		User:     user,
		Business: h.withMenuURL(business),
	})
}

// Login — POST /api/auth/login
func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		return utils.Unprocessable(c, "E-posta ve şifre alanları zorunludur.")
	}

	user, err := repository.GetUserByEmail(c.Context(), h.DB, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Do not reveal which of the two fields was wrong.
			return utils.Unauthorized(c, "E-posta veya şifre hatalı.")
		}
		return utils.Internal(c, err)
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return utils.Unauthorized(c, "E-posta veya şifre hatalı.")
	}

	business, err := repository.GetBusinessByUserID(c.Context(), h.DB, user.ID)
	if err != nil {
		return utils.Internal(c, err)
	}

	token, err := utils.GenerateToken(h.Cfg.JWTSecret, user.ID, business.ID)
	if err != nil {
		return utils.Internal(c, err)
	}

	return utils.OK(c, authResponse{
		Token:    token,
		User:     user,
		Business: h.withMenuURL(business),
	})
}

// Me — GET /api/auth/me
// Returns the session owner and their business when the token is valid.
func (h *Handler) Me(c *fiber.Ctx) error {
	user, err := repository.GetUserByID(c.Context(), h.DB, middleware.UserID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.Unauthorized(c, "Oturum bulunamadı, lütfen tekrar giriş yapın.")
		}
		return utils.Internal(c, err)
	}

	business, err := repository.GetBusinessByID(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		return utils.Internal(c, err)
	}

	return utils.OK(c, fiber.Map{
		"user":     user,
		"business": h.withMenuURL(business),
	})
}
