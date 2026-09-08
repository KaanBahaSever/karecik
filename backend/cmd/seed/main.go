// Command seed loads the development fixtures.
//
// This used to run on every boot behind SEED_DEMO. It does not any more: a
// single mis-set environment variable could put a working login into a real
// deployment, and "the server writes sample accounts unless you remember to
// turn it off" is the wrong default for something that creates credentials.
// Seeding is now a command a person types, on a database they name.
//
//	cd backend
//	go run ./cmd/seed                 # add whatever is missing
//	go run ./cmd/seed -refresh        # rewrite the seeded menus from source
//
// -refresh exists because the seed never rewrites a menu it already created —
// which is right for a database someone has been editing, but means a
// CORRECTION to the fixture data can otherwise never reach an older database.
// It is destructive: it deletes those menus and writes them again.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"karecik/backend/internal/config"
	"karecik/backend/internal/database"
)

func main() {
	log.SetFlags(0)

	refresh := flag.Bool("refresh", false,
		"delete the seeded menus and write them again from the fixture data (destructive)")
	force := flag.Bool("force", false,
		"allow seeding even when APP_ENV=production (you almost certainly do not want this)")
	flag.Parse()

	cfg := config.Load()

	// The fixtures are a REAL cafe with a working password. Refusing production
	// by default is the last line between sample credentials and a live system.
	if cfg.IsProduction() && !*force {
		log.Fatal("[karecik] refusing to seed: APP_ENV=production (pass -force if you truly mean it)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[karecik] %v", err)
	}
	defer pool.Close()

	// The fixtures assume the current schema, so the database is brought up to
	// date first rather than failing on a column that does not exist yet.
	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("[karecik] migration error: %v", err)
	}

	if err := database.SeedDevData(ctx, pool, *refresh); err != nil {
		fmt.Fprintf(os.Stderr, "[karecik] seed failed: %v\n", err)
		os.Exit(1)
	}

	log.Println("[karecik] seed complete")
}
