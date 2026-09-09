package config

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds every setting read from the .env file and the environment.
type Config struct {
	DatabaseURL string
	// Session cookie. There is no signing key any more: a session is an entry in
	// this process's memory, not a signed blob the client carries.
	CookieDomain   string // EMPTY (host-only) — see middleware.SetSessionCookie
	CookieSameSite string // "Lax" | "None" | "Strict"
	CookieSecure   bool   // HTTPS only; SameSite=None REQUIRES it

	Host           string // interface the API binds to
	Port           string
	AppDomain      string // production root domain, e.g. karecik.com
	DevDomain      string // local root domain, e.g. localhost
	CORSOrigins    []string
	UploadDir      string
	MaxUploadBytes int64
	ServeStatic    bool
	StaticDir      string
	Env            string

	// Transactional e-mail. Only the password reset flow uses it; leaving both
	// blank disables that flow rather than half-running it — see
	// internal/mailer and handlers.ForgotPassword.
	ResendAPIKey string
	MailFrom     string // "Karecik <noreply@karecik.com>", on a VERIFIED domain

	// --- client address resolution (audit logging, rate limiting)
	//
	// EdgeSecretHeader / EdgeSecret prove a request really arrived through
	// Cloudflare. They are the ONLY proof available here: the usual check —
	// that the TCP peer is a Cloudflare address — cannot work when the peer is
	// always Railway's own proxy, and Authenticated Origin Pulls cannot work
	// when Railway terminates TLS. Set the same value in a Cloudflare transform
	// rule and here. Unset, CF headers are still used but logged as unverified.
	EdgeSecretHeader string
	EdgeSecret       string

	// ClientPortHeader carries the visitor's source port, published by a
	// Cloudflare transform rule from cf.edge.client_port. Nothing supplies it
	// otherwise, so leaving this unset means the port column reads "-".
	ClientPortHeader string

	// TrustForwardedFor allows X-Forwarded-For as a last-resort address source.
	// OFF by default: Railway does not document setting or sanitising that
	// header, so a value in it may be verbatim client input.
	TrustForwardedFor bool

	// PublicURL is the origin the reset link is built from. It has to be the
	// address a person's browser can actually open, which is not derivable
	// from Host/Port: those describe the interface the process binds to,
	// behind whatever proxy terminates TLS.
	PublicURL string
}

// Load reads the .env file and fills in the Config.
// A missing .env is not an error; values then come from the environment or the
// defaults below.
func Load() *Config {
	// The working directory may be cmd/api or backend; try both.
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			log.Printf("[karecik] loaded .env (%s)", path)
			break
		}
	}

	cfg := &Config{
		DatabaseURL:    env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/karecik?sslmode=disable"),
		CookieDomain:   env("COOKIE_DOMAIN", ""),
		CookieSameSite: env("COOKIE_SAMESITE", "Lax"),

		Host:           env("HOST", "127.0.0.1"),
		Port:           env("PORT", "8080"),
		AppDomain:      env("APP_DOMAIN", "karecik.com"),
		DevDomain:      env("DEV_DOMAIN", "localhost"),
		CORSOrigins:    splitAndTrim(env("CORS_ORIGINS", "http://localhost:5173")),
		UploadDir:      env("UPLOAD_DIR", "./uploads"),
		MaxUploadBytes: envInt64("MAX_UPLOAD_BYTES", 5*1024*1024),
		ServeStatic:    envBool("SERVE_STATIC", false),
		StaticDir:      env("STATIC_DIR", "../frontend/dist"),
		Env:            env("APP_ENV", "development"),

		ResendAPIKey: env("RESEND_API_KEY", ""),
		MailFrom:     env("MAIL_FROM", ""),

		EdgeSecretHeader:  env("EDGE_SECRET_HEADER", "X-Edge-Secret"),
		EdgeSecret:        env("EDGE_SECRET", ""),
		ClientPortHeader:  env("CLIENT_PORT_HEADER", "X-Client-Port"),
		TrustForwardedFor: envBool("TRUST_FORWARDED_FOR", false),
	}

	// The default follows the deployment shape rather than being a fixed
	// string: in production the panel and the API share one origin on the app
	// domain, and in development the browser is on the Vite dev server, which
	// is a different port from this process.
	if cfg.Env == "production" {
		cfg.PublicURL = env("PUBLIC_URL", "https://"+cfg.AppDomain)
	} else {
		cfg.PublicURL = env("PUBLIC_URL", "http://localhost:5173")
	}
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")

	// Secure defaults to "on in production", which is where the cookie travels
	// over the public internet. It stays configurable because a developer may
	// run the API behind a local TLS proxy, and because SameSite=None is
	// meaningless without it.
	cfg.CookieSecure = envBool("COOKIE_SECURE", cfg.Env == "production")

	switch cfg.CookieSameSite {
	case "Lax", "None", "Strict":
	default:
		log.Printf("[karecik] WARNING: COOKIE_SAMESITE=%q is not one of Lax/None/Strict, using Lax",
			cfg.CookieSameSite)
		cfg.CookieSameSite = "Lax"
	}

	// A browser silently DROPS SameSite=None without Secure, so the combination
	// below does not fail loudly on its own — every login would simply appear to
	// succeed and no cookie would be stored. Refusing it here turns a baffling
	// symptom into a startup message.
	if cfg.CookieSameSite == "None" && !cfg.CookieSecure {
		if cfg.Env == "production" {
			log.Fatal("[karecik] FATAL: COOKIE_SAMESITE=None requires COOKIE_SECURE=true")
		}
		log.Println("[karecik] WARNING: COOKIE_SAMESITE=None needs Secure; falling back to Lax")
		cfg.CookieSameSite = "Lax"
	}

	// Said at startup rather than discovered by the first locked-out owner.
	// The reset endpoint refuses cleanly when this is unset (mailer.Disabled),
	// so nothing here is fatal — but a deployment where "şifremi unuttum" is
	// visible in the interface and cannot work is worth one loud line.
	if cfg.ResendAPIKey == "" || cfg.MailFrom == "" {
		level := "WARNING"
		if !cfg.IsProduction() {
			level = "note"
		}
		log.Printf("[karecik] %s: RESEND_API_KEY / MAIL_FROM are not both set — "+
			"password reset e-mails are disabled (cmd/resetpw still works)", level)
	}

	cfg.warnAboutEdgeTrust()

	// CORS_ORIGINS defaults to the local Vite server, which is right for
	// development and dangerous in production: the CORS middleware runs with
	// AllowCredentials, so an allowed origin may read authenticated responses.
	// A deployment that simply never sets the variable would ship a standing
	// credentialed grant to whatever is running on the visitor's own machine.
	//
	// Dropping the entry rather than refusing to boot is deliberate: a loopback
	// origin is never a legitimate production caller, so there is nothing to
	// preserve and nothing to weigh up.
	if cfg.IsProduction() {
		cfg.CORSOrigins = withoutLoopback(cfg.CORSOrigins)
	}

	return cfg
}

// cloudflareRefusesToSet reports whether Cloudflare will not let a transform
// rule write this header name.
//
// Cloudflare reserves the cf- and x-cf- prefixes outright, and separately
// refuses to modify the headers that conventionally carry a visitor address or
// scheme. Configuring one of those names here is not a harmless typo: the
// transform rule can never exist, so nothing upstream ever writes the header,
// and the only thing that CAN write it is the caller. The setting would then be
// doing the exact opposite of its job — presenting client input as edge proof.
func unusableAsEdgeHeader(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(n, "cf-") || strings.HasPrefix(n, "x-cf-") {
		return true
	}
	switch n {
	case "x-forwarded-for", "true-client-ip", "x-real-ip", "x-forwarded-proto":
		return true
	}
	return false
}

// warnAboutEdgeTrust says at startup what the client-address machinery can and
// cannot currently prove.
//
// These are warnings and never fatal. The app is correct in every one of these
// states — it degrades to a coarser rate-limit key and an unverified label in
// the log — and refusing to boot a small business's menu site over a logging
// nicety would be the wrong trade. But each of these is invisible at runtime
// unless it is said out loud once: the symptom is a log column reading "-", or
// a limiter that is quietly coarser than the comment next to it claims.
func (c *Config) warnAboutEdgeTrust() {
	level := "WARNING"
	if !c.IsProduction() {
		level = "note"
	}

	for label, name := range map[string]string{
		"EDGE_SECRET_HEADER": c.EdgeSecretHeader,
		"CLIENT_PORT_HEADER": c.ClientPortHeader,
	} {
		if name != "" && unusableAsEdgeHeader(name) {
			log.Printf("[karecik] WARNING: %s=%q cannot serve as proof — it is either "+
				"reserved by Cloudflare (cf-*, true-client-ip) or already written by "+
				"the host platform's edge (x-real-ip, x-forwarded-*). Either way the "+
				"value arriving in it is not something only Cloudflare could have set. "+
				"Choose a name of your own, such as X-Edge-Secret.", label, name)
		}
	}

	if c.EdgeSecret == "" || c.EdgeSecretHeader == "" {
		log.Printf("[karecik] %s: EDGE_SECRET is not set — requests through Cloudflare "+
			"cannot be PROVEN, so log lines read src=cloudflare-unverified and the rate "+
			"limiter keys on Cloudflare's egress address rather than the visitor "+
			"(safe, but coarse). Fix: a Cloudflare transform rule setting %s to a random "+
			"value, and the same value in EDGE_SECRET here.", level, c.EdgeSecretHeader)
	}

	if c.ClientPortHeader == "" {
		log.Printf("[karecik] %s: CLIENT_PORT_HEADER is empty — the source port column "+
			"will always read \"-\".", level)
	} else {
		log.Printf("[karecik] note: source port is read from %s. It is non-empty ONLY for "+
			"requests through Cloudflare on a zone with a transform rule setting that "+
			"header to to_string(cf.edge.client_port). Nothing else supplies it.",
			c.ClientPortHeader)
	}

	if c.TrustForwardedFor {
		log.Printf("[karecik] WARNING: TRUST_FORWARDED_FOR=true — X-Forwarded-For is not " +
			"documented as sanitised by the host platform, so it may be verbatim client " +
			"input. It is reported in logs but never used as a rate-limit key.")
	}
}

// withoutLoopback drops localhost and 127.0.0.1 origins, naming each one it
// removes — a silently narrowed allow-list is its own kind of confusing.
func withoutLoopback(origins []string) []string {
	kept := make([]string, 0, len(origins))
	for _, origin := range origins {
		host := origin
		if parsed, err := url.Parse(origin); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
		switch strings.ToLower(host) {
		case "localhost", "127.0.0.1", "::1", "[::1]":
			log.Printf("[karecik] WARNING: dropped the loopback CORS origin %q — "+
				"it cannot be a real caller in production", origin)
		default:
			kept = append(kept, origin)
		}
	}
	return kept
}

// IsProduction reports whether the app runs in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
