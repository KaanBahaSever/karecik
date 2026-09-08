package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/mailer"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// maxActiveResets caps how many unexpired links one account may hold.
//
// The limiter in front of the endpoint is per IP; this is per address. Without
// it, a distributed request could still fill one person's inbox, and every
// extra live token is another chance for one of them to leak.
const maxActiveResets = 3

// resetCooldown is the minimum gap between two reset e-mails to one account.
//
// This is the quota guard. The provider's free tier allows 100 messages a day
// and 3,000 a month, and a per-IP limiter does nothing about requests arriving
// from many hosts — which is precisely how one would go about draining it, or
// burying one owner's inbox.
//
// Thirty minutes rather than fifteen, because it costs a legitimate person
// nothing. They are already held to a couple of requests an hour by the
// limiter on the route, so this window only ever bites the case it is aimed
// at: many sources asking for the same address. Paired with maxActiveResets it
// bounds one account to two messages an hour rather than four, which is the
// difference between roughly half the daily allowance and three quarters of it.
//
// A request inside the window creates NO token and sends NO mail. The link
// already sitting in the inbox stays valid for its full hour, so nothing the
// person is waiting for is invalidated by asking again.
const resetCooldown = 30 * time.Minute

// forgotPasswordAnswer is returned no matter what happened.
//
// This is the whole design of the endpoint. Answering "no such account" would
// turn it into a membership oracle: anyone could learn which addresses are
// registered by asking. The message is written so that it stays honest under
// either outcome — it promises that a mail was sent IF the address is
// registered, and never asserts that one was.
func forgotPasswordAnswer(c *fiber.Ctx) error {
	return utils.OK(c, fiber.Map{
		"success": true,
		"message": "Bu adres kayıtlıysa, şifre sıfırlama bağlantısı gönderildi. " +
			"Gelen kutunuzu ve spam klasörünüzü kontrol edin.",
	})
}

// ForgotPassword issues a reset link.
//
// WHAT THIS ENDPOINT MUST NOT REVEAL
//
// Every path below returns the same body and the same status. That includes
// the address being unknown, a link having been sent inside the cooldown, the
// account already holding the maximum number of live tokens, and the e-mail
// provider refusing the message. The only failures that answer differently are
// ones that are true regardless of which address was submitted: a malformed
// body, and e-mail not being configured at all.
//
// The quota rules are layered because each covers what the others cannot:
//
//	the route limiter    per IP     stops one host hammering the endpoint
//	resetCooldown        per account  stops MANY hosts targeting one address
//	maxActiveResets      per account  bounds how many live links can exist at once
func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req forgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		return utils.Unprocessable(c, "E-posta adresinizi girin.")
	}

	// Refused up front, and this one is safe to say out loud: it depends on the
	// deployment, not on the address. Accepting the request and dropping the
	// mail would leave someone waiting for a link that was never going to come.
	if !h.Mail.Configured() {
		return utils.Fail(c, fiber.StatusServiceUnavailable, "MAIL_NOT_CONFIGURED",
			"Şifre sıfırlama e-postası şu an gönderilemiyor. Lütfen bizimle iletişime geçin.")
	}

	user, err := repository.GetUserByEmail(c.Context(), h.DB, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return forgotPasswordAnswer(c)
		}
		return utils.Internal(c, err)
	}

	// The cooldown is checked BEFORE a token is minted, so a request inside the
	// window costs nothing at all: no row, no message, no quota. The link from
	// the previous request is still in the inbox and still valid.
	recent, err := repository.HasRecentPasswordReset(c.Context(), h.DB, user.ID, resetCooldown)
	if err != nil {
		return utils.Internal(c, err)
	}
	if recent {
		log.Printf("[karecik] password reset: %s asked again inside the %s cooldown, not sending",
			user.ID, resetCooldown)
		return forgotPasswordAnswer(c)
	}

	active, err := repository.CountActivePasswordResets(c.Context(), h.DB, user.ID)
	if err != nil {
		return utils.Internal(c, err)
	}
	if active >= maxActiveResets {
		log.Printf("[karecik] password reset: %s already holds %d live tokens, not sending another",
			user.ID, active)
		return forgotPasswordAnswer(c)
	}

	token, err := utils.NewPasswordResetToken()
	if err != nil {
		return utils.Internal(c, err)
	}
	expiresAt := time.Now().Add(utils.PasswordResetTTL)

	if err := repository.CreatePasswordReset(c.Context(), h.DB,
		utils.HashPasswordResetToken(token), user.ID, expiresAt); err != nil {
		return utils.Internal(c, err)
	}

	// Sent in the background, for two reasons.
	//
	// The provider call takes up to ten seconds, and holding the request open
	// for it would make a registered address measurably slower to answer than
	// an unregistered one — handing back, as a timing signal, exactly the fact
	// the identical response bodies above are there to hide.
	//
	// It also means the person is not staring at a spinner while an external
	// API is consulted. The token is already committed, so the link works the
	// moment the mail lands.
	//
	// c.Context() is invalid once this handler returns, so the goroutine gets
	// its own.
	link := fmt.Sprintf("%s/sifre-sifirla?token=%s", h.Cfg.PublicURL, token)
	go h.sendResetMail(user.Email, link)

	return forgotPasswordAnswer(c)
}

// sendResetMail delivers one reset link. Runs detached from the request.
//
// A failure here is logged and goes no further: the caller has already been
// answered, and it could not have been told anyway without revealing that the
// address is registered. The log line is what makes a broken sender domain
// findable, so it names the provider's own complaint.
func (h *Handler) sendResetMail(to, link string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// The link is the only thing interpolated into the HTML, and it is built
	// from PublicURL plus a base64url token — no user input reaches it. It is
	// escaped anyway, because that reasoning is one refactor away from being
	// wrong and an unescaped href is how it would go wrong.
	safeLink := html.EscapeString(link)

	subject := "Karecik — şifre sıfırlama"
	text := "Şifrenizi sıfırlamak için bu bağlantıyı açın:\n\n" + link +
		"\n\nBağlantı 1 saat geçerlidir ve yalnızca bir kez kullanılabilir.\n" +
		"Bu isteği siz yapmadıysanız bu e-postayı yok sayabilirsiniz; şifreniz değişmez.\n"
	body := `<div style="font-family:system-ui,-apple-system,Segoe UI,sans-serif;font-size:15px;color:#111827;line-height:1.6">
  <p>Merhaba,</p>
  <p>Karecik hesabınızın şifresini sıfırlamak için aşağıdaki bağlantıya tıklayın:</p>
  <p><a href="` + safeLink + `" style="display:inline-block;background:#1d4ed8;color:#ffffff;padding:12px 20px;border-radius:8px;text-decoration:none;font-weight:600">Şifremi sıfırla</a></p>
  <p style="color:#6b7280;font-size:13px">Bağlantı <strong>1 saat</strong> geçerlidir ve yalnızca bir kez kullanılabilir.</p>
  <p style="color:#6b7280;font-size:13px">Bu isteği siz yapmadıysanız bu e-postayı yok sayabilirsiniz — şifreniz değişmez.</p>
  <p style="color:#9ca3af;font-size:12px;word-break:break-all">Buton çalışmazsa: ` + safeLink + `</p>
</div>`

	if err := h.Mail.Send(ctx, to, subject, body, text); err != nil {
		if errors.Is(err, mailer.ErrNotConfigured) {
			// Reachable only if the provider was torn down mid-request.
			log.Printf("[karecik] password reset: mail is not configured, link not delivered")
			return
		}
		log.Printf("[karecik] password reset: could not send to %s: %v", to, err)
	}
}

// ResetPassword exchanges a valid token for a new password.
//
// The token is consumed in the same statement that reads it (see
// repository.ConsumePasswordReset), so a link cannot be used twice even if two
// requests arrive at once.
func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req resetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		return utils.Unprocessable(c, "Sıfırlama bağlantısı geçersiz.")
	}
	if len(req.Password) < minPasswordLength {
		return utils.Unprocessable(c, "Yeni şifreniz en az 8 karakter olmalıdır.")
	}

	userID, err := repository.ConsumePasswordReset(c.Context(), h.DB,
		utils.HashPasswordResetToken(token))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Expired, already used and never existed are one answer on
			// purpose — telling them apart would confirm that a token was
			// real, which is halfway to using it.
			return utils.Fail(c, fiber.StatusGone, "RESET_TOKEN_INVALID",
				"Bu bağlantı geçersiz veya süresi dolmuş. Lütfen yeni bir sıfırlama isteyin.")
		}
		return utils.Internal(c, err)
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return utils.Internal(c, err)
	}
	if err := repository.UpdatePassword(c.Context(), h.DB, userID, hash); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// The account went away between the token being issued and used.
			return utils.Fail(c, fiber.StatusGone, "RESET_TOKEN_INVALID",
				"Bu bağlantı artık kullanılamıyor.")
		}
		return utils.Internal(c, err)
	}

	// Any other link requested moments earlier — by the owner double-clicking,
	// or by whoever prompted this reset — dies with this one.
	if _, err := repository.DeleteUserPasswordResets(c.Context(), h.DB, userID); err != nil {
		// Not fatal: the password is already changed, and the remaining tokens
		// expire on their own. Worth a line, because it means they are live
		// until they do.
		log.Printf("[karecik] password reset: could not clear remaining tokens for %s: %v", userID, err)
	}

	// EVERY session, with no exception kept.
	//
	// This is the opposite of ChangePassword, which spares the caller's own
	// session. Someone resetting a password is not signed in, and the usual
	// reason to reset is that somebody else might be — so the whole point is to
	// end the sessions this person cannot see.
	revoked := h.Sessions.DeleteUser(userID, "")

	return utils.OK(c, fiber.Map{
		"success":          true,
		"revoked_sessions": revoked,
		"message":          "Şifreniz güncellendi. Yeni şifrenizle giriş yapabilirsiniz.",
	})
}
