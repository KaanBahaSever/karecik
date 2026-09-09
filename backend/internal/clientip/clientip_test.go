package clientip

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// headers is a Header backed by a map, so the core can be exercised with no
// HTTP server at all. Lookup is case-insensitive to match what both real
// adapters do (net/http canonicalises, fasthttp compares case-insensitively) —
// a test that only worked with exact casing would pass while the real thing
// missed a header spelled "cf-connecting-ip".
type headers map[string]string

func (h headers) Get(key string) string {
	for k, v := range h {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

const (
	secretHeader = "X-Edge-Secret"
	secretValue  = "correct-horse-battery-staple"
)

func proven() Resolver {
	return Resolver{SecretHeader: secretHeader, Secret: secretValue, PortHeader: HeaderXClientPort}
}

// ---------------------------------------------------------------- normalise

func TestNormalizeIP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"plain ipv4", "203.0.113.9", "203.0.113.9", true},
		{"ipv4 with port", "203.0.113.9:44321", "203.0.113.9", true},
		{"surrounding spaces", "  203.0.113.9  ", "203.0.113.9", true},

		// An unbracketed IPv6 address is full of colons; treating the last one
		// as a port separator is the classic way this breaks.
		{"bare ipv6", "2001:db8::1", "2001:db8::1", true},
		{"bracketed ipv6 with port", "[2001:db8::1]:9000", "2001:db8::1", true},
		{"bracketed ipv6 no port", "[2001:db8::1]", "2001:db8::1", true},
		{"ipv6 loopback", "::1", "::1", true},

		// Same host, two spellings: must collapse to one, or a count of
		// distinct visitors is wrong.
		{"ipv4-mapped ipv6", "::ffff:203.0.113.9", "203.0.113.9", true},
		{"ipv4-mapped with port", "[::ffff:203.0.113.9]:8080", "203.0.113.9", true},

		// A zone names an interface on some other machine.
		{"ipv6 with zone", "fe80::1%eth0", "fe80::1", true},

		{"empty", "", "", false},
		{"whitespace only", "   ", "", false},
		{"not an address", "definitely-not-an-ip", "", false},
		{"header junk", "unknown", "", false},
		{"sql-ish junk", "1.2.3.4'; DROP TABLE users;--", "", false},
		{"truncated", "203.0.113.", "", false},
		{"octet out of range", "203.0.113.999", "", false},
		{"unspecified v4", "0.0.0.0", "", false},
		{"unspecified v6", "::", "", false},
		{"port only", ":8080", "", false},
		{"newline injection", "203.0.113.9\nFAKE LOG LINE", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeIP(tc.in)
			if ok != tc.ok {
				t.Fatalf("normalizeIP(%q) ok=%v, want %v (got %q)", tc.in, ok, tc.ok, got)
			}
			if ok && got != tc.want {
				t.Errorf("normalizeIP(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizePort(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"44321", "44321", true},
		{"1", "1", true},
		{"65535", "65535", true},
		{" 44321 ", "44321", true},
		{"0", "", false},     // never a live source port
		{"65536", "", false}, // one past the top
		{"99999", "", false},
		{"-1", "", false},
		{"", "", false},
		{"abc", "", false},
		{"44321abc", "", false},
		{"0x1234", "", false},
		{"+443", "", false},
		{"44321\n", "", false},
	}

	for _, tc := range cases {
		t.Run("port_"+tc.in, func(t *testing.T) {
			got, ok := normalizePort(tc.in)
			if ok != tc.ok {
				t.Fatalf("normalizePort(%q) ok=%v, want %v (got %q)", tc.in, ok, tc.ok, got)
			}
			if ok && got != tc.want {
				t.Errorf("normalizePort(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRightmostForwarded(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		// The LAST entry is the one the nearest proxy wrote. Taking the first
		// is the bug that makes XFF spoofing work.
		{"chain", "1.1.1.1, 2.2.2.2, 3.3.3.3", "3.3.3.3", true},
		{"single", "203.0.113.9", "203.0.113.9", true},
		{"no spaces", "1.1.1.1,2.2.2.2", "2.2.2.2", true},
		{"ragged spacing", " 1.1.1.1 ,   2.2.2.2 ", "2.2.2.2", true},
		{"ipv6 in chain", "1.1.1.1, 2001:db8::1", "2001:db8::1", true},
		{"trailing junk falls back leftward", "203.0.113.9, garbage", "203.0.113.9", true},
		{"trailing empty entry", "203.0.113.9, ", "203.0.113.9", true},
		{"all junk", "garbage, nonsense", "", false},
		{"empty", "", "", false},
		{"commas only", ",,,", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := rightmostForwarded(tc.in)
			if ok != tc.ok {
				t.Fatalf("rightmostForwarded(%q) ok=%v, want %v (got %q)", tc.in, ok, tc.ok, got)
			}
			if ok && got != tc.want {
				t.Errorf("rightmostForwarded(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ----------------------------------------------------------------- resolve

func TestResolvePrecedence(t *testing.T) {
	cases := []struct {
		name       string
		resolver   Resolver
		hdr        headers
		remote     string
		wantIP     string
		wantPort   string
		wantSource Source
		wantProven bool
	}{
		{
			name:     "verified cloudflare wins and carries the port",
			resolver: proven(),
			hdr: headers{
				secretHeader:         secretValue,
				HeaderCFConnectingIP: "203.0.113.9",
				HeaderXClientPort:    "44321",
				HeaderXRealIP:        "198.51.100.1",
			},
			remote: "10.0.0.5:1234", wantIP: "203.0.113.9", wantPort: "44321",
			wantSource: SourceCloudflare, wantProven: true,
		},
		{
			name:     "wrong secret downgrades to unverified and DROPS the port",
			resolver: proven(),
			hdr: headers{
				secretHeader:         "guessed-wrong",
				HeaderCFConnectingIP: "203.0.113.9",
				HeaderXClientPort:    "44321",
			},
			remote: "10.0.0.5:1234", wantIP: "203.0.113.9", wantPort: PortUnknown,
			wantSource: SourceCloudflareUnverified,
		},
		{
			name:     "no secret configured: CF still used, still unverified",
			resolver: Resolver{PortHeader: HeaderXClientPort},
			hdr: headers{
				HeaderCFConnectingIP: "203.0.113.9",
				HeaderXClientPort:    "44321",
			},
			remote: "10.0.0.5:1234", wantIP: "203.0.113.9", wantPort: PortUnknown,
			wantSource: SourceCloudflareUnverified,
		},
		{
			name:       "no cloudflare: the platform edge header",
			resolver:   proven(),
			hdr:        headers{HeaderXRealIP: "198.51.100.1"},
			remote:     "10.0.0.5:1234",
			wantIP:     "198.51.100.1",
			wantPort:   PortUnknown,
			wantSource: SourceEdge,
		},
		{
			name:       "XFF ignored unless enabled",
			resolver:   proven(),
			hdr:        headers{HeaderXForwardedFor: "203.0.113.9"},
			remote:     "10.0.0.5:1234",
			wantIP:     "10.0.0.5",
			wantPort:   "1234",
			wantSource: SourcePeer,
		},
		{
			name:       "XFF used when explicitly enabled, rightmost entry",
			resolver:   Resolver{TrustForwarded: true},
			hdr:        headers{HeaderXForwardedFor: "1.1.1.1, 198.51.100.1"},
			remote:     "10.0.0.5:1234",
			wantIP:     "198.51.100.1",
			wantPort:   PortUnknown,
			wantSource: SourceForwarded,
		},
		{
			name:       "nothing but the peer",
			resolver:   proven(),
			hdr:        headers{},
			remote:     "[2001:db8::1]:5555",
			wantIP:     "2001:db8::1",
			wantPort:   "5555",
			wantSource: SourcePeer,
		},
		{
			name:       "nothing at all",
			resolver:   proven(),
			hdr:        headers{},
			remote:     "",
			wantIP:     IPUnknown,
			wantPort:   PortUnknown,
			wantSource: SourceUnknown,
		},
		{
			// One junk header must not blank the line; the walk continues.
			name:     "junk CF header falls through to the edge header",
			resolver: proven(),
			hdr: headers{
				secretHeader:         secretValue,
				HeaderCFConnectingIP: "not-an-ip",
				HeaderXRealIP:        "198.51.100.1",
			},
			remote: "10.0.0.5:1234", wantIP: "198.51.100.1", wantPort: PortUnknown,
			wantSource: SourceEdge,
		},
		{
			// Proven origin, but the transform rule is missing or broken.
			name:     "verified cloudflare with an unusable port",
			resolver: proven(),
			hdr: headers{
				secretHeader:         secretValue,
				HeaderCFConnectingIP: "203.0.113.9",
				HeaderXClientPort:    "0",
			},
			remote: "10.0.0.5:1234", wantIP: "203.0.113.9", wantPort: PortUnknown,
			wantSource: SourceCloudflare, wantProven: true,
		},
		{
			name:     "port header not configured at all",
			resolver: Resolver{SecretHeader: secretHeader, Secret: secretValue},
			hdr: headers{
				secretHeader:         secretValue,
				HeaderCFConnectingIP: "203.0.113.9",
				HeaderXClientPort:    "44321",
			},
			remote: "10.0.0.5:1234", wantIP: "203.0.113.9", wantPort: PortUnknown,
			wantSource: SourceCloudflare, wantProven: true,
		},
		{
			name:       "case-insensitive header lookup",
			resolver:   proven(),
			hdr:        headers{"cf-connecting-ip": "203.0.113.9", "x-edge-secret": secretValue},
			remote:     "10.0.0.5:1234",
			wantIP:     "203.0.113.9",
			wantPort:   PortUnknown, // no port header sent
			wantSource: SourceCloudflare,
			wantProven: true,
		},
		{
			name:       "ipv4-mapped ipv6 is normalised",
			resolver:   proven(),
			hdr:        headers{HeaderXRealIP: "::ffff:203.0.113.9"},
			remote:     "10.0.0.5:1234",
			wantIP:     "203.0.113.9",
			wantPort:   PortUnknown,
			wantSource: SourceEdge,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.resolver.Resolve(tc.hdr, tc.remote)
			if got.IP != tc.wantIP {
				t.Errorf("IP = %q, want %q", got.IP, tc.wantIP)
			}
			if got.Port != tc.wantPort {
				t.Errorf("Port = %q, want %q", got.Port, tc.wantPort)
			}
			if got.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q", got.Source, tc.wantSource)
			}
			if got.Verified != tc.wantProven {
				t.Errorf("Verified = %v, want %v", got.Verified, tc.wantProven)
			}
		})
	}
}

// TestUnverifiedNeverCarriesPort is the anti-forgery property stated on its own
// so it cannot be lost in a refactor of the table above.
//
// A port is what attributes a log entry to one subscriber behind carrier-grade
// NAT. Accepting an unproven one would put a precise, wrong identity in an
// audit trail — worse than recording no port at all.
func TestUnverifiedNeverCarriesPort(t *testing.T) {
	for _, r := range []Resolver{
		{PortHeader: HeaderXClientPort}, // no secret configured
		{SecretHeader: secretHeader, Secret: secretValue, PortHeader: HeaderXClientPort}, // secret wrong below
	} {
		got := r.Resolve(headers{
			HeaderCFConnectingIP: "203.0.113.9",
			HeaderXClientPort:    "44321",
			secretHeader:         "wrong",
		}, "10.0.0.5:1234")

		if got.Verified {
			t.Fatal("an unproven request was marked Verified")
		}
		if got.Port != PortUnknown {
			t.Errorf("an unproven request carried port %q; it must be %q", got.Port, PortUnknown)
		}
	}
}

// TestSpoofAttempts walks the request an attacker would actually send.
func TestSpoofAttempts(t *testing.T) {
	r := proven()

	t.Run("forged CF header without the secret is not trusted", func(t *testing.T) {
		got := r.Resolve(headers{HeaderCFConnectingIP: "1.2.3.4"}, "10.0.0.5:1234")
		if got.Verified {
			t.Error("a forged CF-Connecting-IP was accepted as verified")
		}
		if got.Source != SourceCloudflareUnverified {
			t.Errorf("Source = %q, want %q", got.Source, SourceCloudflareUnverified)
		}
	})

	t.Run("XFF stuffing cannot displace the edge header", func(t *testing.T) {
		got := r.Resolve(headers{
			HeaderXForwardedFor: "1.2.3.4, 5.6.7.8, 9.10.11.12",
			HeaderXRealIP:       "198.51.100.1",
		}, "10.0.0.5:1234")
		if got.IP != "198.51.100.1" {
			t.Errorf("IP = %q; X-Forwarded-For displaced the edge header", got.IP)
		}
	})

	t.Run("control characters never reach the output", func(t *testing.T) {
		got := r.Resolve(headers{
			secretHeader:         secretValue,
			HeaderCFConnectingIP: "203.0.113.9\r\n2026-01-01 FAKE ENTRY",
			HeaderXRealIP:        "198.51.100.1",
		}, "10.0.0.5:1234")
		if strings.ContainsAny(got.IP, "\r\n") {
			t.Fatalf("a newline survived into the IP field: %q", got.IP)
		}
		if got.IP != "198.51.100.1" {
			t.Errorf("IP = %q, want the edge header to be used instead", got.IP)
		}
	})
}

func TestSanitizeLogValue(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"clean", "/api/health", "/api/health"},
		{"newline", "/a\nFAKE", "/a\\nFAKE"},
		{"crlf", "/a\r\nFAKE", "/a\\r\\nFAKE"},
		{"tab", "/a\tb", "/a\\tb"},
		{"ansi", "/a\x1b[31mred", "/a\\x1b[31mred"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeLogValue(tc.in, 0); got != tc.want {
				t.Errorf("SanitizeLogValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("truncates", func(t *testing.T) {
		got := SanitizeLogValue(strings.Repeat("a", 100), 10)
		if len([]rune(got)) != 11 { // 10 runes plus the ellipsis
			t.Errorf("SanitizeLogValue truncated to %d runes, want 11", len([]rune(got)))
		}
	})
}

// ----------------------------------------------------------- http adapter

func TestResolveRequest(t *testing.T) {
	r := proven()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(secretHeader, secretValue)
	req.Header.Set(HeaderCFConnectingIP, "203.0.113.9")
	req.Header.Set(HeaderXClientPort, "44321")
	req.RemoteAddr = "10.0.0.5:1234"

	got := r.ResolveRequest(req)
	if got.IP != "203.0.113.9" || got.Port != "44321" || !got.Verified {
		t.Errorf("ResolveRequest = %+v, want 203.0.113.9:44321 verified", got)
	}

	t.Run("nil request does not panic", func(t *testing.T) {
		if got := r.ResolveRequest(nil); got.Source != SourceUnknown {
			t.Errorf("ResolveRequest(nil).Source = %q, want %q", got.Source, SourceUnknown)
		}
	})
}

func TestNilHeaderDoesNotPanic(t *testing.T) {
	got := Resolver{}.Resolve(nil, "203.0.113.9:44321")
	if got.IP != "203.0.113.9" {
		t.Errorf("IP = %q, want the peer to be used", got.IP)
	}
}

func TestIPPort(t *testing.T) {
	ip, port := proven().IPPort(headers{
		secretHeader:         secretValue,
		HeaderCFConnectingIP: "203.0.113.9",
		HeaderXClientPort:    "44321",
	}, "10.0.0.5:1234")

	if ip != "203.0.113.9" || port != "44321" {
		t.Errorf("IPPort = (%q, %q), want (203.0.113.9, 44321)", ip, port)
	}
}

// --------------------------------------------------------------- key safety

// TestKeyIPIsNotAttackerControlled is the regression test for a flaw found by
// adversarially reviewing the first version of this package.
//
// That version keyed the rate limiter on Addr.IP. Behind an unverified
// Cloudflare, Addr.IP comes from CF-Connecting-IP — a header. An attacker
// hitting the origin directly could therefore mint a fresh limiter bucket for
// every single request just by editing it, which is not a weakened limiter but
// an absent one wearing the costume of a limit.
//
// The fix was to separate what is REPORTED from what is KEYED ON. This test
// pins the property down so the two cannot quietly merge again.
func TestKeyIPIsNotAttackerControlled(t *testing.T) {
	r := proven() // secret configured, but the attacker does not know it

	keys := map[string]bool{}
	reported := map[string]bool{}

	for _, forged := range []string{"1.2.3.4", "1.2.3.5", "9.9.9.9", "2001:db8::1", "198.51.100.77"} {
		got := r.Resolve(headers{
			HeaderCFConnectingIP: forged,        // attacker-chosen, no secret
			HeaderXRealIP:        "203.0.113.7", // written by the platform edge
		}, "10.0.0.5:5555")

		keys[got.KeyIP] = true
		reported[got.IP] = true
	}

	if len(keys) != 1 {
		t.Errorf("%d distinct limiter keys from one attacker — the limiter can be bypassed "+
			"by varying a header", len(keys))
	}
	for k := range keys {
		if k != "203.0.113.7" {
			t.Errorf("KeyIP = %q, want the edge address 203.0.113.7", k)
		}
	}

	// The reported address still follows the header, which is the point of
	// having two fields: the log stays useful, the limiter stays honest.
	if len(reported) != 5 {
		t.Errorf("reported %d distinct addresses, want 5 — Addr.IP should still "+
			"follow CF-Connecting-IP for logging", len(reported))
	}
}

// TestKeyIPFallsBackToPeer covers the case with no edge header at all.
func TestKeyIPFallsBackToPeer(t *testing.T) {
	got := Resolver{}.Resolve(headers{HeaderCFConnectingIP: "1.2.3.4"}, "198.51.100.9:5555")
	if got.KeyIP != "198.51.100.9" {
		t.Errorf("KeyIP = %q, want the peer 198.51.100.9", got.KeyIP)
	}
	if got.IP != "1.2.3.4" {
		t.Errorf("IP = %q, want the reported header value 1.2.3.4", got.IP)
	}
}

// TestVerifiedCloudflareIsKeyable: proving the origin upgrades the key from the
// edge address (which may be a whole Cloudflare egress) to the actual visitor.
func TestVerifiedCloudflareIsKeyable(t *testing.T) {
	got := proven().Resolve(headers{
		secretHeader:         secretValue,
		HeaderCFConnectingIP: "203.0.113.9",
		HeaderXRealIP:        "198.51.100.1",
	}, "10.0.0.5:5555")

	if got.KeyIP != "203.0.113.9" {
		t.Errorf("KeyIP = %q, want the proven visitor address", got.KeyIP)
	}
	if !got.Verified {
		t.Error("a matching secret did not mark the result Verified")
	}
}

// TestControlCharactersAreRefused closes a loophole netip leaves open.
//
// netip.ParseAddr accepts ANY bytes after "%" as an IPv6 zone without looking
// at them, so an address carrying a newline used to parse successfully and be
// cleaned up afterwards by the zone strip. The output was safe either way; the
// disposition was not, because an audit path should refuse a malformed header
// rather than quietly repair it.
func TestControlCharactersAreRefused(t *testing.T) {
	for _, in := range []string{
		"fe80::1%eth0\n2026-01-01 FAKE ENTRY",
		"203.0.113.9\r\nFAKE",
		"2001:db8::1%\x00",
		"203.0.113.9\x7f",
	} {
		if got, ok := normalizeIP(in); ok {
			t.Errorf("normalizeIP(%q) accepted the value as %q; control characters must be refused", in, got)
		}
	}
	if _, ok := normalizePort("443\n"); ok {
		t.Error("normalizePort accepted a value containing a newline")
	}
}
