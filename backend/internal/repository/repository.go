package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB, hem *pgxpool.Pool hem de pgx.Tx tarafindan karsilanan ortak arayuzdur.
// Boylece ayni repository fonksiyonlari transaction icinde de kullanilabilir.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ErrNotFound, aranan kaydin bulunamadigini bildirir.
var ErrNotFound = errors.New("kayit bulunamadi")

// ErrDuplicate, benzersizlik kisitinin ihlal edildigini bildirir (e-posta, slug).
var ErrDuplicate = errors.New("kayit zaten mevcut")

// isNoRows, pgx'in "satir yok" hatasini yakalar.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation, PostgreSQL 23505 (unique_violation) hatasini yakalar.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
