// Command resetpw sets an account's password from the server.
//
// WHY THIS IS A COMMAND AND NOT AN HTTP ENDPOINT
//
// A "forgot password" flow proves you own the address before it lets you change
// the password, and that proof is the e-mail. With no mail delivery in place
// there is nothing to prove ownership WITH, so an unauthenticated reset
// endpoint — however carefully written — would let anyone who knows an address
// take the account. A token generator with no way to deliver the token is the
// same hole wearing a hat: it either has to be readable from somewhere (a log,
// a response body) or it cannot be used at all.
//
// So the reset lives where the trust already is: on the server, run by whoever
// already holds the database credentials. When SMTP arrives, the e-mailed-token
// flow can be added and this command stays useful as the break-glass path.
//
//	cd backend
//	go run ./cmd/resetpw -email owner@example.com                  # prompts, hidden input
//	go run ./cmd/resetpw -email owner@example.com -password '...'  # non-interactive
//
// IT CANNOT SIGN ANYONE OUT, AND THAT MATTERS HERE
//
// This used to delete the user's session rows on its way past. Sessions now
// live in the API process's own memory, and this is a different process: it can
// change the password in the database, but it cannot reach into the running
// server to revoke what is already open there. A reset is usually done because
// an account is suspected compromised, so leaving the intruder's session live
// would defeat the point.
//
// Restarting the API is what ends those sessions — it empties the store, which
// signs out every account on the server, the intruder included. The command
// says so when it finishes rather than leaving it to be remembered.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"karecik/backend/internal/config"
	"karecik/backend/internal/database"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

const minPasswordLength = 8

func main() {
	log.SetFlags(0)

	email := flag.String("email", "", "the account's e-mail address (required)")
	password := flag.String("password", "",
		"the new password; omit it to be prompted instead of leaving it in your shell history")
	flag.Parse()

	*email = strings.ToLower(strings.TrimSpace(*email))
	if *email == "" {
		log.Fatal("[karecik] -email is required")
	}

	newPassword := *password
	if newPassword == "" {
		// Read from stdin rather than a flag so the password does not end up in
		// the shell history or in the process list of a shared machine.
		fmt.Printf("New password for %s (min %d characters): ", *email, minPasswordLength)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			log.Fatalf("[karecik] could not read the password: %v", err)
		}
		newPassword = strings.TrimRight(line, "\r\n")
	}

	if len(newPassword) < minPasswordLength {
		log.Fatalf("[karecik] the password must be at least %d characters", minPasswordLength)
	}

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[karecik] %v", err)
	}
	defer pool.Close()

	user, err := repository.GetUserByEmail(ctx, pool, *email)
	if err != nil {
		// The e-mail is typed by an operator who already has database access, so
		// naming the miss is helpful here rather than an enumeration risk.
		log.Fatalf("[karecik] no account found for %s", *email)
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		log.Fatalf("[karecik] could not hash the password: %v", err)
	}
	if err := repository.UpdatePassword(ctx, pool, user.ID, hash); err != nil {
		log.Fatalf("[karecik] could not update the password: %v", err)
	}

	log.Printf("[karecik] password updated for %s", *email)
	log.Printf("[karecik] NOTE: sessions live in the API process's memory, so this command " +
		"cannot sign anyone out. Restart the API service to end every session that is " +
		"open right now — including any the intruder is holding.")
}
