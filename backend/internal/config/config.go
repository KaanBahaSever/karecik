package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config, .env dosyasindan ve ortam degiskenlerinden okunan tum ayarlari tutar.
type Config struct {
	DatabaseURL    string
	JWTSecret      string
	Port           string
	AppDomain      string   // canlidaki kok alan adi, orn. karecik.com
	DevDomain      string   // yerelde kok alan adi, orn. localhost
	CORSOrigins    []string
	UploadDir      string
	MaxUploadBytes int64
	SeedDemo       bool
	ServeStatic    bool
	StaticDir      string
	Env            string
}

// Load, .env dosyasini okur ve Config nesnesini doldurur.
// .env bulunamazsa hata vermez; degerler ortam degiskenlerinden/varsayilanlardan gelir.
func Load() *Config {
	// Calisma dizini cmd/api ya da backend olabilir; iki olasiligi da dene.
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			log.Printf("[karecik] .env yuklendi (%s)", path)
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
			log.Fatal("[karecik] HATA: production ortaminda JWT_SECRET zorunludur")
		}
		cfg.JWTSecret = "karecik-gelistirme-anahtari-uretimde-degistir"
		log.Println("[karecik] UYARI: JWT_SECRET tanimli degil, gelistirme anahtari kullaniliyor")
	}

	return cfg
}

// IsProduction, uygulamanin uretim modunda olup olmadigini soyler.
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
