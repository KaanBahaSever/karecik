package tests

// Black-box coverage of the e-mailed password reset, driven over HTTP the same
// way the isolation suite is.
//
// What is actually worth asserting here is not "the happy path works" — that
// much is obvious from using it once. It is the set of properties that are
// invisible from the outside and easy to regress:
//
//   - the endpoint does not reveal whether an address is registered
//   - a link works exactly once, even if it is clicked twice
//   - resetting ends EVERY session, including ones the person cannot see
//   - the raw token is never what the database holds
//
// Each of those is a security property that a well-meaning refactor can quietly
// remove while every page still looks right.

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"karecik/backend/internal/utils"
)

// recordingMailer stands in for the provider. It records instead of sending, so
// the suite can read the link that would have gone out.
//
// Safe for concurrent use because ForgotPassword sends from a goroutine: the
// handler answers before the mail is dispatched, which is the whole point of
// that design and would otherwise make this a data race.
type recordingMailer struct {
	mu   sync.Mutex
	sent []recordedMail
}

type recordedMail struct {
	to      string
	subject string
	text    string
}

func (m *recordingMailer) Configured() bool { return true }

func (m *recordingMailer) Send(_ context.Context, to, subject, _, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, recordedMail{to: to, subject: subject, text: text})
	return nil
}

// waitForMail blocks until n messages have been recorded, or fails.
//
// The send is detached from the request, so "the handler returned 200" does not
// mean the mail has been handed over yet. Polling is the honest way to wait for
// another goroutine here; the alternative — a fixed sleep — is either flaky or
// slow, and usually both.
func (m *recordingMailer) waitForMail(t *testing.T, n int) []recordedMail {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		got := append([]recordedMail(nil), m.sent...)
		m.mu.Unlock()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d e-mail(s), only %d were sent within the deadline", n, len(got))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (m *recordingMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// tokenFromMail pulls the ?token= value out of the recorded message.
func tokenFromMail(t *testing.T, mail recordedMail) string {
	t.Helper()
	const marker = "token="
	index := strings.Index(mail.text, marker)
	if index < 0 {
		t.Fatalf("the reset e-mail carried no token: %q", mail.text)
	}
	token := mail.text[index+len(marker):]
	// The plain-text body puts the link on its own line.
	if cut := strings.IndexAny(token, "\r\n \t"); cut >= 0 {
		token = token[:cut]
	}
	if token == "" {
		t.Fatal("the reset e-mail carried an empty token")
	}
	return token
}

func TestPasswordReset(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)

	const email = "reset-owner@example.com"
	owner := h.register("owner", "Reset Kafe", email)

	// A second session for the same account, opened from another "device". The
	// reset has to kill this one too — that is the entire reason someone resets
	// a password they cannot remember being shared.
	resp, payload := h.do(http.MethodPost, "/api/auth/login", "", map[string]any{
		"email":    email,
		"password": "karecik-test-password",
	})
	h.requireSuccess("second login", resp, payload)
	otherSession := sessionCookie(resp)
	if otherSession == "" || otherSession == owner.session {
		t.Fatalf("the second login did not produce a distinct session cookie")
	}

	// ---------------------------------------------------------- enumeration
	//
	// An unknown address must be answered exactly like a known one. Comparing
	// the two responses is the assertion; hard-coding the expected wording
	// would pass even if both of them started saying "no such account".
	unknownResp, unknownBody := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": "nobody-here@example.com"})
	knownResp, knownBody := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})

	if unknownResp.StatusCode != knownResp.StatusCode {
		t.Errorf("forgot-password leaks account existence through the status: "+
			"unknown=%d known=%d", unknownResp.StatusCode, knownResp.StatusCode)
	}
	if string(unknownBody) != string(knownBody) {
		t.Errorf("forgot-password leaks account existence through the body:\n unknown: %s\n known:   %s",
			unknownBody, knownBody)
	}

	// ...but only the registered address actually gets mail.
	sent := mail.waitForMail(t, 1)
	if len(sent) != 1 {
		t.Fatalf("expected exactly 1 e-mail, got %d", len(sent))
	}
	if sent[0].to != email {
		t.Errorf("the reset e-mail went to %q, expected %q", sent[0].to, email)
	}

	token := tokenFromMail(t, sent[0])

	// -------------------------------------------------- the token at rest
	//
	// What is stored must be the hash, never the value that travelled by
	// e-mail. A dump of this table must not yield a usable link.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storedHash string
	if err := h.pool.QueryRow(ctx,
		`SELECT token_hash FROM password_resets`).Scan(&storedHash); err != nil {
		t.Fatalf("could not read the stored reset token: %v", err)
	}
	if storedHash == token {
		t.Error("the RAW reset token is stored in the database; it must be hashed")
	}
	if storedHash != utils.HashPasswordResetToken(token) {
		t.Errorf("the stored token is not the SHA-256 of the e-mailed one")
	}

	// ------------------------------------------------------ rejected inputs
	badResp, _ := h.do(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token, "password": "short"})
	if badResp.StatusCode < 400 {
		t.Errorf("reset accepted a %d-character password (status %d)", len("short"), badResp.StatusCode)
	}

	wrongResp, _ := h.do(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token + "x", "password": "yeni-guclu-sifre"})
	if wrongResp.StatusCode < 400 {
		t.Errorf("reset accepted an unknown token (status %d)", wrongResp.StatusCode)
	}

	// ------------------------------------------------------------- the reset
	const newPassword = "yeni-guclu-sifre"
	okResp, okBody := h.do(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token, "password": newPassword})
	h.requireSuccess("reset-password", okResp, okBody)

	// Single use. The same link again must not work, whatever the reason.
	replayResp, _ := h.do(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token, "password": "baska-bir-sifre"})
	if replayResp.StatusCode < 400 {
		t.Errorf("the reset link worked a SECOND time (status %d)", replayResp.StatusCode)
	}

	// Nothing outstanding is left behind.
	var remaining int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM password_resets`).Scan(&remaining); err != nil {
		t.Fatalf("could not count the remaining reset tokens: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d reset token(s) survived a successful reset", remaining)
	}

	// -------------------------------------------------- sessions are revoked
	for label, session := range map[string]string{
		"the session that requested it": owner.session,
		"the other device":              otherSession,
	} {
		meResp, mePayload := h.do(http.MethodGet, "/api/auth/me", session, nil)
		if meResp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s still authenticates after a password reset: %d %s",
				label, meResp.StatusCode, mePayload)
		}
	}

	// ------------------------------------------------------ the new password
	oldResp, _ := h.do(http.MethodPost, "/api/auth/login", "",
		map[string]any{"email": email, "password": "karecik-test-password"})
	if oldResp.StatusCode < 400 {
		t.Errorf("the OLD password still logs in after a reset (status %d)", oldResp.StatusCode)
	}

	newResp, newBody := h.do(http.MethodPost, "/api/auth/login", "",
		map[string]any{"email": email, "password": newPassword})
	h.requireSuccess("login with the new password", newResp, newBody)
}

// TestPasswordResetWithoutMailer covers the deployment that never configured a
// provider: the request must be REFUSED, not accepted and dropped.
//
// A silently-swallowed reset is the worst outcome available — the interface
// says a link is on its way, the logs say nothing, and the owner waits for mail
// that was never going to arrive.
func TestPasswordResetWithoutMailer(t *testing.T) {
	h := newHarness(t) // mailer.Disabled

	const email = "no-mailer@example.com"
	h.register("owner", "Postasiz Kafe", email)

	resp, payload := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("with no mailer configured, forgot-password answered %d (%s); expected 503",
			resp.StatusCode, payload)
	}
}

// backdateResetTokens pushes every stored token's created_at into the past, so
// a test can cross the cooldown window without waiting for it.
//
// The row's own clock is what the cooldown compares against (see
// repository.HasRecentPasswordReset), so moving created_at is the honest way to
// simulate elapsed time — no clock injection, no sleeping for half an hour.
func backdateResetTokens(t *testing.T, h *harness, by time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tag, err := h.pool.Exec(ctx,
		`UPDATE password_resets SET created_at = created_at - make_interval(secs => $1)`,
		by.Seconds())
	if err != nil {
		t.Fatalf("could not backdate the reset tokens: %v", err)
	}
	if tag.RowsAffected() == 0 {
		t.Fatal("backdating affected no rows — the fixture never created a token")
	}
}

// TestPasswordResetCooldown covers the per-account quota guard.
//
// The provider's free tier is 100 messages a day, and the per-IP limiter does
// nothing about requests arriving from many hosts. The cooldown is what stops
// one address being used to spend the allowance.
//
// The response must not change. This test asserts that the second request is
// byte-identical to the first; TestPasswordReset separately asserts that a
// request for an UNKNOWN address is byte-identical to one for a known address.
// Together those two make the cooldown response indistinguishable from every
// other outcome, which is the property that matters.
func TestPasswordResetCooldown(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)

	const email = "cooldown@example.com"
	h.register("owner", "Cooldown Kafe", email)

	firstResp, firstBody := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})
	h.requireSuccess("first forgot-password", firstResp, firstBody)
	mail.waitForMail(t, 1)

	// Immediately again: inside the cooldown.
	secondResp, secondBody := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})

	if secondResp.StatusCode != firstResp.StatusCode {
		t.Errorf("the cooldown is observable through the status: first=%d second=%d",
			firstResp.StatusCode, secondResp.StatusCode)
	}
	if string(secondBody) != string(firstBody) {
		t.Errorf("the cooldown is observable through the body:\n first:  %s\n second: %s",
			firstBody, secondBody)
	}

	// The point of the exercise: no second message was handed to the provider.
	if got := mail.count(); got != 1 {
		t.Errorf("the cooldown sent %d e-mails, expected exactly 1", got)
	}

	// And no token was minted either — a row created for a mail that is never
	// sent would burn a maxActiveResets slot for nothing.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var tokens int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM password_resets`).Scan(&tokens); err != nil {
		t.Fatalf("could not count the reset tokens: %v", err)
	}
	if tokens != 1 {
		t.Errorf("the cooldown left %d token(s), expected exactly 1", tokens)
	}
}

// TestPasswordResetCooldownExpires is the other half: the window must REOPEN.
//
// A cooldown that never lifts is not a rate limit, it is a permanent lockout
// for anyone whose first e-mail went astray.
func TestPasswordResetCooldownExpires(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)

	const email = "cooldown-expiry@example.com"
	h.register("owner", "Bekleyen Kafe", email)

	resp, body := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})
	h.requireSuccess("first forgot-password", resp, body)
	mail.waitForMail(t, 1)

	// Step over the window rather than waiting it out.
	backdateResetTokens(t, h, 31*time.Minute)

	resp, body = h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})
	h.requireSuccess("forgot-password after the cooldown", resp, body)

	sent := mail.waitForMail(t, 2)
	if len(sent) != 2 {
		t.Fatalf("expected 2 e-mails once the cooldown had passed, got %d", len(sent))
	}
	// Two different links, not the same one resent.
	if tokenFromMail(t, sent[0]) == tokenFromMail(t, sent[1]) {
		t.Error("the second e-mail carried the SAME token as the first")
	}
}

// TestPasswordResetRateLimits covers the per-IP budgets, and the reason the two
// endpoints do not share one.
//
// /forgot-password spends the mail allowance, so it is held to 2 an hour.
// /reset-password sends nothing and must NOT be throttled on that budget: a
// person who asks for a link and then mistypes their new password would
// otherwise be locked out of finishing the reset they are in the middle of.
func TestPasswordResetRateLimits(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)

	const email = "ratelimit@example.com"
	h.register("owner", "Limit Kafe", email)

	// First request: the real one, and the source of the token used below.
	resp, body := h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})
	h.requireSuccess("forgot-password 1", resp, body)
	token := tokenFromMail(t, mail.waitForMail(t, 1)[0])

	// Second: allowed by the limiter, swallowed by the cooldown.
	resp, body = h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email})
	h.requireSuccess("forgot-password 2", resp, body)

	// Third: over budget. Deliberately aimed at an address that does NOT exist,
	// so the 429 cannot be read as "this account is real" — the limiter has to
	// fire on the host, before anything about the address is considered.
	resp, body = h.do(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": "someone-else@example.com"})
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("the third forgot-password in an hour answered %d (%s); expected 429",
			resp.StatusCode, body)
	}

	// Still exactly one message: neither the cooldown nor the limiter sent.
	if got := mail.count(); got != 1 {
		t.Errorf("%d e-mails were sent across three requests, expected 1", got)
	}

	// The mail budget is spent — but the reset itself must still be reachable.
	// This is the regression the split limiters exist to prevent.
	resp, body = h.do(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token, "password": "kisa"})
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Fatalf("reset-password is sharing the forgot-password budget: %d %s", resp.StatusCode, body)
	}
	if resp.StatusCode < 400 {
		t.Errorf("reset-password accepted a 4-character password (status %d)", resp.StatusCode)
	}

	resp, body = h.do(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token, "password": "gecerli-yeni-sifre"})
	h.requireSuccess("reset-password after the forgot budget was spent", resp, body)
}
