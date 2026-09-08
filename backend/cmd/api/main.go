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
	"karecik/backend/internal/session"
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

	// Nothing is seeded on boot any more.
	//
	// It used to create a working login from an environment flag, which meant a
	// single mis-set variable could put sample accounts into a real deployment.
	// Sample data is now an explicit, human-run command instead:
	//
	//	go run ./cmd/seed          (development fixtures)
	//	go run ./cmd/resetpw       (set a password without e-mail)

	// --- sessions
	//
	// Sessions live in this process, not in the database. Starting the store
	// here — rather than inside the handlers — is what makes the lifetime
	// obvious: it is created with the server and dies with it, so every browser
	// signed in before this line ran is signed out now.
	//
	// The janitor is the only thing keeping the map from growing for the life
	// of the process: an expired entry is refused by the lookup, but refusing
	// one does not free it.
	sessions := session.New()
	sessions.StartJanitor(session.DefaultJanitorInterval)
	defer sessions.Stop()

	// --- upload directory
	// The hint is not padding. In a container this path is a mounted volume, and
	// platforms mount volumes as root while this image runs as an unprivileged
	// user — so the first boot after someone attaches a disk fails here, over and
	// over, with a bare "permission denied" and a deploy that never goes live.
	// Railway's answer is RAILWAY_RUN_UID=0 on the service.
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("[karecik] could not create the upload directory (%s): %v\n"+
			"[karecik] if this is a mounted volume, the container user probably cannot write to it "+
			"(on Railway set RAILWAY_RUN_UID=0 on the service)", cfg.UploadDir, err)
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

	router.Setup(app, handlers.New(pool, cfg, sessions), cfg)

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
	// Said out loud on every boot because the consequence is user-visible and
	// easy to mistake for a bug: everyone who was signed in a moment ago is not
	// any more, and will not be after the next deploy either.
	log.Printf("[karecik] sessions           -> in memory, single instance only "+
		"(a restart signs everyone out; janitor every %s)", session.DefaultJanitorInterval)
	if !cfg.IsProduction() {
		log.Printf("[karecik] seeded menu        -> http://localhost:%s/api/public/menu/melly-coffee/suadiye", cfg.Port)
	}

	if err := app.Listen(addr); err != nil {
		log.Fatalf("[karecik] could not start the server: %v", err)
	}
}
