package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds every setting read from the .env file and the environment.
type Config struct {
	DatabaseURL    string
	JWTSecret      string
	Port           string
	AppDomain      string // production root domain, e.g. karecik.com
	DevDomain      string // local root domain, e.g. localhost
	CORSOrigins    []string
	UploadDir      string
	MaxUploadBytes int64
	SeedDemo       bool
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
		JWTSecret:      env("JWT_SECRET", ""),
		Port:           env("PORT", "8080"),
		AppDomain:      env("APP_DOMAIN", "karecik.com"),
		DevDomain:      env("DEV_DOMAIN", "localhost"),
		CORSOrigins:    splitAndTrim(env("CORS_ORIGINS", "http://localhost:5173")),
		UploadDir:      env("UPLOAD_DIR", "./uploads"),
		MaxUploadBytes: envInt64("MAX_UPLOAD_BYTES", 5*1024*1024),
		SeedDemo:       envBool("SEED_DEMO", true),
		ServeStatic:    envBool("SERVE_STATIC", false),
		StaticDir:      env("STATIC_DIR", "../frontend/dist"),
		Env:            env("APP_ENV", "development"),
	}

	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			log.Fatal("[karecik] FATAL: JWT_SECRET is required in production")
		}
		cfg.JWTSecret = "karecik-development-secret-change-me-in-production"
		log.Println("[karecik] WARNING: JWT_SECRET is unset, using the development secret")
	}

	return cfg
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
