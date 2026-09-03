// Karecik API server.
//
// To run it:
//
//	cd backend
//	go mod tidy
//	go run ./cmd/api
package main

import (
	"context"
	"errors"
	"log"
	"net"
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

	// --- database
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[karecik] %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("[karecik] migration error: %v", err)
	}

	if cfg.SeedDemo {
		if err := database.SeedDemo(ctx, pool); err != nil {
			log.Printf("[karecik] WARNING: could not create the demo data: %v", err)
		}
	}

	// --- upload directory
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("[karecik] could not create the upload directory (%s): %v", cfg.UploadDir, err)
	}

	// --- HTTP server
	app := fiber.New(fiber.Config{
		AppName:               "Karecik API",
		BodyLimit:             int(cfg.MaxUploadBytes) + 1024*1024, // image plus JSON headroom
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

	// --- listen for shutdown signals (Ctrl+C)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		log.Println("[karecik] shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Printf("[karecik] shutdown error: %v", err)
		}
	}()

	// Bind to the loopback interface by default rather than every interface.
	// ":8080" would listen on 0.0.0.0, which makes Windows Firewall pop up its
	// "allow this app on your network?" dialog on every rebuild — the binary is
	// new each time, so the granted exception never sticks. Loopback-only needs
	// no exception at all.
	//
	// Set HOST=0.0.0.0 in .env to expose the API on the LAN (to open the menu on
	// a real phone, say); the firewall prompt comes back with it.
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	log.Printf("[karecik] server listening   -> http://localhost:%s (bound to %s)", cfg.Port, cfg.Host)
	log.Printf("[karecik] health check       -> http://localhost:%s/api/health", cfg.Port)
	if !cfg.IsProduction() {
		log.Printf("[karecik] demo menu          -> http://localhost:%s/api/public/menu/demo-kafe", cfg.Port)
	}

	if err := app.Listen(addr); err != nil {
		log.Fatalf("[karecik] could not start the server: %v", err)
	}
}
