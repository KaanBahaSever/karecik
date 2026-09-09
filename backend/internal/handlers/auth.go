package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/middleware"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/session"
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

// authResponse no longer carries a token. The session travels in an HttpOnly
// cookie the browser attaches on its own, so there is nothing for the client to
// store — and nothing for a script on the page to steal.
type authResponse struct {
	User     *models.User     `json:"user"`
	Business *models.Business `json:"business"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// minPasswordLength is enforced on register, on change, and by the reset CLI.
const minPasswordLength = 8

// startSession issues a session, stores its hash and writes the cookie.
//
// The raw token exists only here and in the Set-Cookie header: it is hashed
// before it reaches the store, so what is held in memory cannot be replayed.
//
// The business id is recorded with the session because the middleware answers
// every subsequent request from this entry alone. It is passed in rather than
// looked up here for the same reason — both callers have just read it.
func (h *Handler) startSession(c *fiber.Ctx, userID, businessID uuid.UUID) error {
	token, err := utils.NewSessionToken()
	if err != nil {
		return err
	}

	now := time.Now()
	expiresAt := now.Add(utils.SessionTTL)

	h.Sessions.Create(utils.HashSessionToken(token), session.Entry{
		UserID:     userID,
		BusinessID: businessID,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
		// NOT c.IP(): behind the platform edge that is an internal proxy
		// address, the same one for every visitor, which would make this
		// field record where the request was relayed from rather than where
		// it came from — an audit column that is identical on every row.
		IPAddress: middleware.ClientAddrOf(c).IP,
		UserAgent: c.Get("User-Agent"),
	})

	middleware.SetSessionCookie(c, h.Cfg, token, expiresAt)
	return nil
}

// Register — POST /api/auth/register
// Creates the user and the business records, derives the subdomain slug from
// the business name and returns a token.
//
// It creates NO menu: a fresh tenant owns zero of them and the dashboard's
// empty states invite the owner to create the first one themselves.
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

	if err := h.startSession(c, user.ID, business.ID); err != nil {
		return utils.Internal(c, err)
	}

	return utils.Created(c, authResponse{
		User:     user,
		Business: h.withHomeURL(business),
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

	if err := h.startSession(c, user.ID, business.ID); err != nil {
		return utils.Internal(c, err)
	}

	return utils.OK(c, authResponse{
		User:     user,
		Business: h.withHomeURL(business),
	})
}

// Me — GET /api/auth/me
// Returns the session owner and their business when the token is valid.
func (h *Handler) Me(c *fiber.Ctx) error {
	user, err := repository.GetUserByID(c.Context(), h.DB, middleware.UserID(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.SessionExpired(c, "Oturum bulunamadı, lütfen tekrar giriş yapın.")
		}
		return utils.Internal(c, err)
	}

	business, err := repository.GetBusinessByID(c.Context(), h.DB, middleware.BusinessID(c))
	if err != nil {
		return utils.Internal(c, err)
	}

	return utils.OK(c, fiber.Map{
		"user":     user,
		"business": h.withHomeURL(business),
	})
}

// Logout — POST /api/auth/logout
//
// Drops the SESSION, not just the cookie. Clearing the cookie alone would leave
// a working credential behind: anyone holding a copy of the value — a shared
// machine, a captured request — could keep using it until it expired. Removing
// the entry ends it for every copy at once.
//
// It is deliberately forgiving. A cookie that matches nothing is already logged
// out, so the answer is still 200 and the cookie is still cleared; a logout
// button that can fail is a logout button people stop trusting.
func (h *Handler) Logout(c *fiber.Ctx) error {
	if raw := c.Cookies(utils.SessionCookieName); raw != "" {
		h.Sessions.Delete(utils.HashSessionToken(raw))
	}

	middleware.ClearSessionCookie(c, h.Cfg)
	return utils.OK(c, fiber.Map{"success": true})
}

// ChangePassword — POST /api/auth/change-password  (authenticated)
//
// The current password is required even though the caller is already signed in.
// That is the point: it proves the person at the keyboard is the account owner
// and not someone who sat down at an unlocked laptop, and it is the only thing
// standing between a borrowed session and a permanent takeover.
//
// Every OTHER session of the user is destroyed afterwards. If the reason for
// the change is that the old password leaked, leaving the attacker's session
// alive would make the change pointless. The caller's own session survives, so
// the dashboard they are looking at keeps working.
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		return utils.Unprocessable(c, "Mevcut ve yeni şifre alanları zorunludur.")
	}
	if len(req.NewPassword) < minPasswordLength {
		return utils.Unprocessable(c, "Yeni şifreniz en az 8 karakter olmalıdır.")
	}
	if req.NewPassword == req.CurrentPassword {
		return utils.Unprocessable(c, "Yeni şifreniz mevcut şifrenizden farklı olmalıdır.")
	}

	userID := middleware.UserID(c)
	user, err := repository.GetUserByID(c.Context(), h.DB, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.SessionExpired(c, "Oturum bulunamadı, lütfen tekrar giriş yapın.")
		}
		return utils.Internal(c, err)
	}

	if !utils.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		return utils.Unauthorized(c, "Mevcut şifreniz hatalı.")
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return utils.Internal(c, err)
	}
	if err := repository.UpdatePassword(c.Context(), h.DB, userID, hash); err != nil {
		return utils.Internal(c, err)
	}

	// Sparing the current session is why the hash is threaded through the
	// request context rather than recomputed here.
	//
	// The revocation reaches only the sessions this process holds. That is
	// every session there is while the API runs as one instance — the store's
	// package comment explains why it has to.
	revoked := h.Sessions.DeleteUser(userID, middleware.SessionHash(c))

	return utils.OK(c, fiber.Map{
		"success":          true,
		"revoked_sessions": revoked,
	})
}
