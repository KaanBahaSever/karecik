package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"karecik/backend/internal/clientip"
)

// ctxClientAddr holds the resolved client address for the current request.
const ctxClientAddr = "karecik_client_addr"

// maxLoggedPath bounds the request path written to a log line.
//
// The path is raw client input and the platform edge accepts up to 32 KB of
// headers, so an unbounded value lets one request flood the log with its own
// content and push everything else out of view.
const maxLoggedPath = 256

// ClientAddr resolves the visitor's address once per request and stores it.
//
// It must be registered BEFORE anything that reads the address — the request
// logger, the rate limiter, and the login handler that records where a session
// was opened from. Resolving once rather than at each call site means those
// three can never disagree about who the caller was, which is the kind of
// discrepancy that makes an audit trail impossible to reason about later.
func ClientAddr(resolver clientip.Resolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// c.Get is fasthttp's case-insensitive header lookup, so the canonical
		// spellings in the clientip package match however the header arrived.
		addr := resolver.Resolve(
			clientip.HeaderFunc(func(key string) string { return c.Get(key) }),
			c.Context().RemoteAddr().String(),
		)
		c.Locals(ctxClientAddr, addr)
		return c.Next()
	}
}

// ClientAddrOf returns the address resolved for this request.
//
// The zero value comes back as an explicit "unknown" rather than an empty
// struct, so a caller that forgot to register ClientAddr logs "unknown"
// instead of a blank column that reads like a formatting bug.
func ClientAddrOf(c *fiber.Ctx) clientip.Addr {
	addr, ok := c.Locals(ctxClientAddr).(clientip.Addr)
	if !ok {
		return clientip.Addr{
			IP:     clientip.IPUnknown,
			Port:   clientip.PortUnknown,
			Source: clientip.SourceUnknown,
		}
	}
	return addr
}

// ClientIPKey is the rate limiter's key function.
//
// This exists because the obvious choice is wrong here. Fiber's c.IP() returns
// the TCP peer unless app.Config.ProxyHeader is set, and behind this platform's
// edge the peer is an internal proxy address that is IDENTICAL for every
// visitor. A limiter keyed on it is not "per client" at all — it is one global
// bucket, so the first two people to ask for a password reset in an hour lock
// out everybody else on the internet.
//
// It keys on Addr.KeyIP, NOT on Addr.IP, and the difference is the whole point.
// Addr.IP is the best guess at who the visitor is and is what gets logged;
// behind an unverified Cloudflare it comes from CF-Connecting-IP, which is a
// header. Keying a limiter on a header the caller writes means the caller mints
// a fresh bucket per request simply by changing it — an unlimited allowance
// wearing the costume of a limit. KeyIP holds the platform edge's own X-Real-IP
// instead: coarser, sometimes a whole Cloudflare egress address rather than one
// person, but never a value the caller picked.
//
// Setting EDGE_SECRET is what upgrades this from coarse to exact: with the
// secret matched, the proven CF-Connecting-IP becomes the key and the limiter
// counts individual visitors.
//
// When nothing unforgeable can be established the key falls back to the source
// label, which deliberately collapses those requests into one shared bucket.
// That is the safe direction: unattributable traffic gets throttled together
// rather than each anonymous request being handed a fresh allowance.
func ClientIPKey(c *fiber.Ctx) string {
	addr := ClientAddrOf(c)
	if addr.KeyIP == "" || addr.KeyIP == clientip.IPUnknown {
		return string(addr.Source)
	}
	return addr.KeyIP
}

// LoggerTags exposes the resolved address to the Fiber request logger.
//
// Deliberately custom tags on the EXISTING logger rather than a second
// middleware calling log.Printf. Two lines per request would double the volume
// in the platform's log viewer and, worse, split one event across two entries
// that have to be correlated by eye. This way the address, the port and the
// status all land on the same line.
//
// Register with:
//
//	logger.New(logger.Config{
//	    Format:     "[karecik] ${time} ${status} ${method} ${path} ip=${clientIP} port=${clientPort} src=${clientSource} (${latency})\n",
//	    TimeFormat: "15:04:05",
//	    CustomTags: middleware.LoggerTags(),
//	})
func LoggerTags() map[string]logger.LogFunc {
	return map[string]logger.LogFunc{
		"clientIP": func(out logger.Buffer, c *fiber.Ctx, _ *logger.Data, _ string) (int, error) {
			return out.WriteString(ClientAddrOf(c).IP)
		},
		"clientPort": func(out logger.Buffer, c *fiber.Ctx, _ *logger.Data, _ string) (int, error) {
			return out.WriteString(ClientAddrOf(c).Port)
		},
		"clientSource": func(out logger.Buffer, c *fiber.Ctx, _ *logger.Data, _ string) (int, error) {
			// Logged on every line on purpose. "cloudflare" is proof;
			// "cloudflare-unverified" is a strong guess; "peer" identifies
			// nobody. An audit entry that does not say which of those it is
			// cannot be weighed when it matters.
			return out.WriteString(string(ClientAddrOf(c).Source))
		},
	}
}

// LogClientAddr is a standalone stdout logger, for when the Fiber logger is not
// in use or a separate audit stream is wanted.
//
// Prefer LoggerTags in this application — see the note there about one line per
// request. This is kept because it is the plain form: no middleware framework,
// no buffer plumbing, just log.Printf to stdout, which is what the platform's
// log viewer streams.
//
// The path is sanitised because it is raw client input: a request for a path
// containing a newline would otherwise write a second, fabricated line into the
// audit log. The address and port need no sanitising — they have already been
// through netip and strconv and cannot contain anything but their own alphabet.
func LogClientAddr() fiber.Handler {
	return func(c *fiber.Ctx) error {
		addr := ClientAddrOf(c)
		log.Printf("[karecik] request method=%s path=%s ip=%s port=%s src=%s",
			c.Method(),
			clientip.SanitizeLogValue(c.Path(), maxLoggedPath),
			addr.IP, addr.Port, addr.Source,
		)
		return c.Next()
	}
}
