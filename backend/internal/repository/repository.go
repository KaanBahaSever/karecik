package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB is the common interface satisfied by both *pgxpool.Pool and pgx.Tx, so the
// same repository functions can also run inside a transaction.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ErrNotFound signals that the requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicate signals a unique constraint violation (email, slug).
var ErrDuplicate = errors.New("record already exists")

// ErrLastMenu signals an attempt to delete a business' only menu.
var ErrLastMenu = errors.New("cannot delete the last menu")

// isNoRows detects pgx's "no rows" error.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation detects the PostgreSQL 23505 (unique_violation) error.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
