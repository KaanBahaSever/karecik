package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"karecik/backend/internal/repository"
)

// The lookups by a text a stranger typed answer a text PostgreSQL cannot store
// — U+0000, or bytes that are not UTF-8 — as the miss it is, without a query
// (see UnstorableText). Their callers check such a text as well, so over HTTP
// the guards never fire; these tests call them with a database that fails the
// test when it is asked anything.

// refusingDB is a DB that fails the test on every call.
type refusingDB struct{ t *testing.T }

var errNoDatabase = errors.New("refusingDB: no query was expected")

func (d refusingDB) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	d.t.Errorf("unexpected query: %s", sql)
	return nil, errNoDatabase
}

func (d refusingDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	d.t.Errorf("unexpected query: %s", sql)
	return refusedRow{}
}

func (d refusingDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	d.t.Errorf("unexpected statement: %s", sql)
	return pgconn.CommandTag{}, errNoDatabase
}

type refusedRow struct{}

func (refusedRow) Scan(...any) error { return errNoDatabase }

// unstorableTexts are texts PostgreSQL refuses as a parameter, built from code
// points and byte values so that this file holds none of them.
func unstorableTexts() map[string]string {
	return map[string]string{
		"nul":                "ali" + string(rune(0)) + "@example.test",
		"byte_ff":            "ali" + string([]byte{0xff}) + "@example.test",
		"truncated_sequence": "kahve-duragi" + string([]byte{0xc3}),
	}
}

func TestLookupsByUnstorableTextAskNoDatabase(t *testing.T) {
	ctx := context.Background()

	for name, text := range unstorableTexts() {
		t.Run(name, func(t *testing.T) {
			db := refusingDB{t}

			if exists, err := repository.EmailExists(ctx, db, text); exists || err != nil {
				t.Errorf("EmailExists = (%t, %v), want (false, nil)", exists, err)
			}
			if user, err := repository.GetUserByEmail(ctx, db, text); user != nil || !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("GetUserByEmail = (%v, %v), want ErrNotFound", user, err)
			}
			if business, err := repository.GetBusinessBySlug(ctx, db, text); business != nil || !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("GetBusinessBySlug = (%v, %v), want ErrNotFound", business, err)
			}
			if menu, err := repository.GetMenuBySlug(ctx, db, uuid.New(), text); menu != nil || !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("GetMenuBySlug = (%v, %v), want ErrNotFound", menu, err)
			}
			if products, err := repository.ListProducts(ctx, db, uuid.New(), nil, text); err != nil || products == nil || len(products) != 0 {
				t.Errorf("ListProducts with the text as its search = (%v, %v), want an empty list", products, err)
			}
		})
	}
}
