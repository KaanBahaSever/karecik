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
	}

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
