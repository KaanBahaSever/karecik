package repository

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

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

// TxDB is a DB that can also open a transaction. *pgxpool.Pool satisfies it,
// and so does pgx.Tx, whose Begin opens a savepoint instead. The functions
// that have to run more than one statement atomically take it instead of DB:
// the deletes, the reorders and the bulk price update, which lock rows before
// they write them, and the creates and moves of a record into a menu or a
// category, which take an advisory lock first (see locks.go).
type TxDB interface {
	DB
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ErrNotFound signals that the requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicate signals a unique constraint violation (email, slug).
var ErrDuplicate = errors.New("record already exists")

// ErrParentNotFound signals that the menu or the category a write moves a
// record into does not exist in the business — or no longer does by the time
// the write holds its locks.
var ErrParentNotFound = errors.New("parent record not found")

// ErrPriceTooLarge signals that a bulk price update would give a product a
// price larger than utils.MaxPrice, the largest one products.price holds.
// ApplyPrices writes nothing when it returns it.
var ErrPriceTooLarge = errors.New("a new price is larger than the price column holds")

// isNoRows detects pgx's "no rows" error.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// UnstorableText reports whether PostgreSQL refuses a text parameter: one that
// holds U+0000, or bytes that are not valid UTF-8. Either fails the statement
// with SQLSTATE 22021, which the caller would answer with a 500. Text decoded
// from a JSON body is always valid UTF-8 — encoding/json replaces a bad byte
// with U+FFFD — but a query string, a header or a form body can carry any
// bytes. No stored text can be such a value, so a lookup by one is answered as
// the miss it is without asking the database.
func UnstorableText(s string) bool {
	return strings.ContainsRune(s, 0) || !utf8.ValidString(s)
}

// sqlState returns the SQLSTATE of the PostgreSQL error in err's chain, and ""
// when the chain holds none.
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation detects the PostgreSQL 23505 (unique_violation) error.
func IsUniqueViolation(err error) bool {
	return sqlState(err) == "23505"
}

// IsForeignKeyViolation detects the PostgreSQL 23503 (foreign_key_violation)
// error.
//
// The handlers check that a menu or a category belongs to the business before
// they write a row that points at it, but a concurrent delete can still remove
// it between that check and the write. The write's foreign key check then
// waits for the delete, finds the row gone and fails with 23503. What the
// request named no longer exists, so the handlers answer 404 rather than 500.
func IsForeignKeyViolation(err error) bool {
	return sqlState(err) == "23503"
}

// IsRetryableConflict detects the two SQLSTATEs PostgreSQL answers a lost
// concurrency conflict with: 40P01 (deadlock_detected), when the deadlock
// detector aborts one side of a lock cycle, and 40001 (serialization_failure).
// The statement or transaction that received either has been aborted, so
// nothing it wrote survives and running it again is safe — see
// RetryOnConflict.
func IsRetryableConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40P01" || pgErr.Code == "40001"
	}
	return false
}
