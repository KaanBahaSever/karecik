// Package mailer sends the few transactional e-mails Karecik needs.
//
// WHY AN HTTPS API AND NOT SMTP
//
// The platform this runs on disables outbound SMTP on its free and hobby
// plans to keep spam off its address space, so port 25/465/587 simply does not
// connect there. An HTTPS provider API is unaffected by that, needs no long-
// lived connection, and fails fast with a readable status code instead of
// hanging on a blocked socket.
//
// The interface exists so the transport is not baked into the handlers: the
// reset flow calls Send and does not care whether a real provider or Disabled
// is behind it.
package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotConfigured is what Disabled returns. Handlers check for it so they can
// answer "e-mail is not set up on this deployment" instead of a bare 500.
var ErrNotConfigured = errors.New("mailer: no e-mail provider is configured")

// Mailer sends one message. Implementations must be safe for concurrent use.
type Mailer interface {
	Send(ctx context.Context, to, subject, html, text string) error
	// Configured reports whether Send can actually deliver. It lets the caller
	// refuse a request up front rather than accepting it and dropping the mail.
	Configured() bool
}

// ---------------------------------------------------------------- disabled

// Disabled is the mailer used when no provider is configured.
//
// It REFUSES rather than silently succeeding. A password-reset flow whose mail
// step is a no-op looks healthy from the outside while locking people out, and
// that is the failure this type exists to make loud.
type Disabled struct{}

func (Disabled) Configured() bool { return false }

func (Disabled) Send(context.Context, string, string, string, string) error {
	return ErrNotConfigured
}

// ------------------------------------------------------------------ resend

const resendEndpoint = "https://api.resend.com/emails"

// Resend delivers through the Resend HTTPS API.
type Resend struct {
	apiKey string
	from   string
	client *http.Client
}

// NewResend builds a Resend mailer. An empty key or from address yields
// Disabled, so a deployment that has not set them up degrades to a clear
// refusal rather than a half-working flow.
//
// `from` must be an address on a domain verified with the provider — an
// unverified sender is accepted by this constructor and rejected by the API at
// send time, which is the one failure this cannot check for in advance.
func New(apiKey, from string) Mailer {
	apiKey = strings.TrimSpace(apiKey)
	from = strings.TrimSpace(from)
	if apiKey == "" || from == "" {
		return Disabled{}
	}
	return &Resend{
		apiKey: apiKey,
		from:   from,
		// A bounded client, because this call sits inside an HTTP request. The
		// default http.Client has NO timeout, so a provider that accepts the
		// connection and then stalls would hold the request goroutine open
		// indefinitely.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *Resend) Configured() bool { return true }

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

// Send posts one message to the provider.
//
// The returned error never contains the API key. It does contain the provider's
// own message, which is what makes a misconfigured sender domain diagnosable
// from the logs.
func (r *Resend) Send(ctx context.Context, to, subject, html, text string) error {
	body, err := json.Marshal(resendRequest{
		From:    r.from,
		To:      []string{to},
		Subject: subject,
		HTML:    html,
		Text:    text,
	})
	if err != nil {
		return fmt.Errorf("mailer: could not encode the request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailer: could not build the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("mailer: the provider could not be reached: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Read a bounded amount: an error body is small, and an unbounded read of
	// an unexpected response is a memory hazard on a path anyone can trigger.
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("mailer: the provider refused the message (%s): %s",
		resp.Status, strings.TrimSpace(string(detail)))
}
