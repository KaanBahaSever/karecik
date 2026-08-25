// Karecik API sunucusu.
//
// Calistirmak icin:
//
//	cd backend
//	go mod tidy
//	go run ./cmd/api
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"karecik/backend/internal/config"
	"karecik/backend/internal/database"
	"karecik/backend/internal/handlers"
	"karecik/backend/internal/router"
	"karecik/backend/internal/utils"
)

func main() {
	log.SetFlags(0)

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --- veritabani
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[karecik] %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("[karecik] migration hatasi: %v", err)
	}

	if cfg.SeedDemo {
		if err := database.SeedDemo(ctx, pool); err != nil {
			log.Printf("[karecik] UYARI: demo verisi olusturulamadi: %v", err)
		}
	}

	// --- yukleme klasoru
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("[karecik] yukleme klasoru olusturulamadi (%s): %v", cfg.UploadDir, err)
	}

	// --- HTTP sunucusu
	app := fiber.New(fiber.Config{
		AppName:               "Karecik API",
		BodyLimit:             int(cfg.MaxUploadBytes) + 1024*1024, // gorsel + JSON payi
		DisableStartupMessage: true,
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          30 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				return utils.Fail(c, fiberErr.Code, "HTTP_ERROR", fiberErr.Message)
			}
			return utils.Internal(c, err)
		},
	})

	router.Setup(app, handlers.New(pool, cfg), cfg)

	// --- kapanis sinyallerini dinle (Ctrl+C)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		log.Println("[karecik] kapatiliyor...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Printf("[karecik] kapanis hatasi: %v", err)
		}
	}()

	addr := ":" + cfg.Port
	log.Printf("[karecik] sunucu calisiyor -> http://localhost:%s", cfg.Port)
	log.Printf("[karecik] saglik kontrolu  -> http://localhost:%s/api/health", cfg.Port)
	if !cfg.IsProduction() {
		log.Printf("[karecik] demo menu       -> http://localhost:%s/api/public/menu/demo-kafe", cfg.Port)
	}

	if err := app.Listen(addr); err != nil {
		log.Fatalf("[karecik] sunucu baslatilamadi: %v", err)
	}
}
