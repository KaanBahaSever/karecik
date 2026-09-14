// Command resetpw sets an account's password from the server.
//
// # WHY THIS IS A COMMAND AS WELL AS AN E-MAILED LINK
//
// POST /api/auth/forgot-password proves that the person asking owns the address
// before it lets them change the password, and that proof is the e-mail. When
// mail is not configured or not being delivered there is nothing to prove
// ownership WITH, and an unauthenticated reset endpoint — however carefully
// written — would let anyone who knows an address take the account.
//
// So this break-glass path lives where the trust already is: on the server, run
// by whoever already holds the database credentials.
//
//	cd backend
//	go run ./cmd/resetpw -email owner@example.com                  # prompts for the password
//	go run ./cmd/resetpw -email owner@example.com -password '...'  # non-interactive
//
// It applies the password rules of the API: at least 8 characters and at most
// utils.MaxPasswordBytes bytes, the longest password bcrypt hashes. A longer one
// is refused with that limit named, before the database is contacted.
//
// # IT CANNOT SIGN ANYONE OUT, AND THAT MATTERS HERE
//
// Sessions live in the API process's own memory, and this is a different
// process: it can change the password in the database, but it cannot reach
// into the running server to revoke what is already open there. A reset is
// usually done because an account is suspected compromised, so leaving the
// intruder's session live would defeat the point.
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
	"karecik/backend/internal/resetpw"
	"karecik/backend/internal/utils"
)

func main() {
	log.SetFlags(0)

	emailFlag := flag.String("email", "", "the account's e-mail address (required)")
	password := flag.String("password", "",
		"the new password; omit it to be prompted instead of leaving it in your shell history")
	flag.Parse()

	email, err := resetpw.NormalizeEmail(*emailFlag)
	if err != nil {
		log.Fatalf("[karecik] %v", err)
	}

	newPassword := *password
	if newPassword == "" {
		// Read from stdin rather than a flag so the password does not end up in
		// the shell history or in the process list of a shared machine.
		fmt.Printf("New password for %s (at least %d characters, at most %d bytes): ",
			email, resetpw.MinPasswordLength, utils.MaxPasswordBytes)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			log.Fatalf("[karecik] could not read the password: %v", err)
		}
		newPassword = strings.TrimRight(line, "\r\n")
	}

	if err := resetpw.CheckNewPassword(newPassword); err != nil {
		log.Fatalf("[karecik] %v", err)
	}

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[karecik] %v", err)
	}
	defer pool.Close()

	user, err := repository.GetUserByEmail(ctx, pool, email)
	if err != nil {
		// The e-mail is typed by an operator who already has database access, so
		// naming the miss is helpful here rather than an enumeration risk.
		log.Fatalf("[karecik] no account found for %s", email)
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		log.Fatalf("[karecik] could not hash the password: %v", err)
	}
	if err := repository.UpdatePassword(ctx, pool, user.ID, hash); err != nil {
		log.Fatalf("[karecik] could not update the password: %v", err)
	}

	log.Printf("[karecik] password updated for %s", email)
	log.Printf("[karecik] NOTE: sessions live in the API process's memory, so this command " +
		"cannot sign anyone out. Restart the API service to end every session that is " +
		"open right now — including any the intruder is holding.")
}
