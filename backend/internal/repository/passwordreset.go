package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreatePasswordReset stores one reset token for a user.
//
// tokenHash is the SHA-256 of the value that goes in the e-mail; the raw token
// never reaches this layer. See migrations/009_password_resets.sql.
func CreatePasswordReset(ctx context.Context, db DB,
	tokenHash string, userID uuid.UUID, expiresAt time.Time) error {

	_, err := db.Exec(ctx,
		`INSERT INTO password_resets (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("could not store the password reset token: %w", err)
	}
	return nil
}

// ConsumePasswordReset exchanges a token hash for the user it belongs to, and
// deletes it in the same statement.
//
// The DELETE ... RETURNING is deliberate and is what makes the token single
// use even under concurrency: two requests arriving with the same token race
// on the same row, and PostgreSQL lets exactly one of them delete it. A
// SELECT-then-DELETE would leave a window in which both reads succeed.
//
// The expiry is checked in the WHERE clause rather than in Go for the same
// reason — one round trip, one decision, no gap between reading and acting.
// An expired or unknown token both come back as ErrNotFound, which is all the
// caller should be able to tell apart.
func ConsumePasswordReset(ctx context.Context, db DB, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := db.QueryRow(ctx,
		`DELETE FROM password_resets
		  WHERE token_hash = $1 AND expires_at > now()
		  RETURNING user_id`, tokenHash).Scan(&userID)
	if err != nil {
		if isNoRows(err) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("could not read the password reset token: %w", err)
	}
	return userID, nil
}

// DeleteUserPasswordResets removes every outstanding token of one user.
//
// Called after a successful reset. Without it, a second link requested moments
// earlier — by the owner in a double-click, or by an attacker probing the
// address — would still be live against the account that was just recovered.
func DeleteUserPasswordResets(ctx context.Context, db DB, userID uuid.UUID) (int64, error) {
	tag, err := db.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("could not clear the password reset tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CountActivePasswordResets reports how many unexpired tokens a user holds.
//
// Used to cap outstanding requests per account. The rate limiter in front of
// the endpoint is per IP and this is per address, so between them neither a
// single host nor a distributed attempt can flood one inbox.
func CountActivePasswordResets(ctx context.Context, db DB, userID uuid.UUID) (int, error) {
	var count int
	err := db.QueryRow(ctx,
		`SELECT count(*) FROM password_resets WHERE user_id = $1 AND expires_at > now()`,
		userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("could not count the password reset tokens: %w", err)
	}
	return count, nil
}

// HasRecentPasswordReset reports whether a link was issued to this user inside
// the given window.
//
// This is the cooldown behind the e-mail quota. The provider's free tier allows
// 100 messages a day, and without it one address can be used to spend the lot:
// the per-IP limiter in front of the endpoint does nothing against requests
// arriving from many hosts, which is exactly the shape of an attempt to drain
// the quota or bury someone's inbox.
//
// The comparison is made in SQL rather than in Go on purpose. created_at is
// written by the database's clock (DEFAULT now()), so comparing it against the
// same clock removes any question of skew between the two machines — a Go-side
// time.Now() that ran slightly behind would reopen the window early.
//
// Expiry is deliberately NOT part of the predicate. The question is "did we
// send recently", not "is a token still live"; a row that has expired still
// cost a message when it was created.
func HasRecentPasswordReset(ctx context.Context, db DB,
	userID uuid.UUID, within time.Duration) (bool, error) {

	var recent bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM password_resets
		      WHERE user_id = $1 AND created_at > now() - make_interval(secs => $2))`,
		userID, within.Seconds()).Scan(&recent)
	if err != nil {
		return false, fmt.Errorf("could not check the password reset cooldown: %w", err)
	}
	return recent, nil
}

// SweepPasswordResets deletes expired rows.
//
// Expired tokens are already refused by ConsumePasswordReset, so this is purely
// housekeeping: without it the table grows for the lifetime of the deployment.
func SweepPasswordResets(ctx context.Context, db DB) (int64, error) {
	tag, err := db.Exec(ctx, `DELETE FROM password_resets WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("could not sweep the password reset tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}
