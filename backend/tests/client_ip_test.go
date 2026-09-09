package tests

// Proof that per-IP rate limiting is actually per-IP.
//
// It was not. Every limiter here keyed on Fiber's c.IP(), which returns the TCP
// peer unless app.Config.ProxyHeader is set — and behind the hosting platform's
// edge that peer is an internal proxy address, the SAME one for every visitor
// on Earth. So "2 password-reset requests per hour per IP" was really "2 per
// hour, worldwide": the first two people to use the recovery form locked
// everybody else out of it until the window rolled over.
//
// A bug like that is invisible in development (where the peer really is the
// client) and invisible in the code (c.IP() reads exactly like the right
// answer). The only thing that catches it is a test that sends requests as two
// different clients and insists they are counted separately.

import (
	"net/http"
	"testing"

	"karecik/backend/internal/clientip"
)

// forgotFrom sends one password-reset request as a given client address.
//
// It sets X-Real-IP, which is the header the hosting platform's edge actually
// writes, and therefore the one the limiter keys on. Deliberately NOT
// CF-Connecting-IP: that one is a value the caller chooses when no shared
// secret is configured, and TestForgedCFHeaderCannotMintBuckets below exists
// precisely to prove it buys an attacker nothing.
func forgotFrom(t *testing.T, h *harness, clientIP, email string) int {
	t.Helper()

	resp, _ := h.doWith(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email},
		map[string]string{clientip.HeaderXRealIP: clientIP})
	return resp.StatusCode
}

// TestRateLimitIsPerClientNotGlobal is the regression test for the bug above.
func TestRateLimitIsPerClientNotGlobal(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	h.register("owner", "Limit Kafe", "perclient@example.com")

	// Ten different visitors, each making a single request. The forgot-password
	// budget is 2 per hour per client, so if the key were global the third
	// visitor would already be refused — and the tenth certainly would.
	for i, ip := range []string{
		"203.0.113.1", "203.0.113.2", "203.0.113.3", "203.0.113.4", "203.0.113.5",
		"198.51.100.1", "198.51.100.2", "2001:db8::1", "2001:db8::2", "2001:db8::3",
	} {
		if code := forgotFrom(t, h, ip, "perclient@example.com"); code == http.StatusTooManyRequests {
			t.Fatalf("visitor %d (%s) was rate limited by OTHER visitors' requests — "+
				"the limiter key is global, not per client", i+1, ip)
		}
	}

	// ...and the limit must still bite the client that actually exceeds it.
	// A limiter that never refuses anyone is the opposite failure.
	const heavy = "203.0.113.200"
	for i := 1; i <= 2; i++ {
		if code := forgotFrom(t, h, heavy, "perclient@example.com"); code == http.StatusTooManyRequests {
			t.Fatalf("request %d from %s was refused while still inside its own budget", i, heavy)
		}
	}
	if code := forgotFrom(t, h, heavy, "perclient@example.com"); code != http.StatusTooManyRequests {
		t.Errorf("the third request from %s answered %d; the per-client limit did not apply",
			heavy, code)
	}

	// The heavy client's budget must not have spent anybody else's.
	if code := forgotFrom(t, h, "203.0.113.201", "perclient@example.com"); code == http.StatusTooManyRequests {
		t.Error("a fresh visitor was refused after another client exhausted its own budget")
	}
}

// TestUnattributableRequestsShareOneBucket covers the deliberate other half.
//
// When no per-visitor address can be established, every such request keys on
// the same label and shares one allowance. That is the safe direction: the
// alternative — treating each unattributable request as a new client — would
// hand out a fresh budget to anyone who simply sent no headers, which is a
// limiter that cannot limit.
func TestUnattributableRequestsShareOneBucket(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	h.register("owner", "Anonim Kafe", "anon@example.com")

	// No CF-Connecting-IP, no X-Real-IP: nothing identifies these callers.
	var refused bool
	for range [4]struct{}{} {
		resp, _ := h.do(http.MethodPost, "/api/auth/forgot-password", "",
			map[string]any{"email": "anon@example.com"})
		if resp.StatusCode == http.StatusTooManyRequests {
			refused = true
			break
		}
	}
	if !refused {
		t.Error("unattributable requests were never refused — each one was given its own budget")
	}
}

// TestForgedCFHeaderCannotMintBuckets is the end-to-end half of
// clientip.TestKeyIPIsNotAttackerControlled.
//
// One attacker, one real network path, a different forged CF-Connecting-IP on
// every request. If the limiter keyed on the reported address they would get a
// fresh allowance each time and the limit would be decorative.
func TestForgedCFHeaderCannotMintBuckets(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	h.register("owner", "Sahte Kafe", "forge@example.com")

	const realEdgeIP = "203.0.113.55" // what the edge saw: one attacker
	forged := []string{"1.2.3.4", "1.2.3.5", "9.9.9.9", "8.8.8.8"}

	refused := false
	for i, f := range forged {
		resp, _ := h.doWith(http.MethodPost, "/api/auth/forgot-password", "",
			map[string]any{"email": "forge@example.com"},
			map[string]string{
				clientip.HeaderXRealIP:        realEdgeIP,
				clientip.HeaderCFConnectingIP: f,
			})
		if resp.StatusCode == http.StatusTooManyRequests {
			refused = true
			t.Logf("refused on request %d (forged %s) — the limiter held", i+1, f)
			break
		}
	}

	if !refused {
		t.Errorf("%d requests from one attacker all passed by varying CF-Connecting-IP — "+
			"the limiter is keyed on a value the caller controls", len(forged))
	}
}
