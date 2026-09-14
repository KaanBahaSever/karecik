package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"karecik/backend/internal/repository"
)

func TestIsForeignKeyViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"foreign_key_violation 23503", &pgconn.PgError{Code: "23503"}, true},
		{"a wrapped 23503", fmt.Errorf("could not move the product: %w", &pgconn.PgError{Code: "23503"}), true},
		{"23503 joined with a cancellation", errors.Join(&pgconn.PgError{Code: "23503"}, context.Canceled), true},
		{"unique_violation 23505", &pgconn.PgError{Code: "23505"}, false},
		{"deadlock_detected 40P01", &pgconn.PgError{Code: "40P01"}, false},
		{"ErrNotFound", repository.ErrNotFound, false},
		{"a plain error", errors.New("23503"), false},
		{"nil", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := repository.IsForeignKeyViolation(tc.err); got != tc.want {
				t.Fatalf("IsForeignKeyViolation(%v) = %t, want %t", tc.err, got, tc.want)
			}
		})
	}
}
