package repository_test

// A write that waited for a delete of its parent reports the parent gone.
//
// A create, a move or a reorder into a menu or a category waits at the parent's
// advisory lock while a delete holds that lock exclusively. Once the delete has
// committed, the write checks its parent again under the lock and comes back
// with ErrParentNotFound, having written nothing — rather than running its
// INSERT or UPDATE into the foreign key violation the deleted row would cause.
//
// Each case holds the parent's advisory lock in a transaction of the test's
// own, starts the writer, waits until PostgreSQL reports the writer blocked by
// that transaction, deletes the parent in the same transaction and commits.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
)

// insertSiblingCategory writes a second category with one product into the
// menu of a tenant, and a second menu holding a third category.
func insertSiblings(t *testing.T, pool *pgxpool.Pool, tenant scopeTenant) (category, product, otherMenuCategory uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var otherMenu uuid.UUID
	for _, step := range []struct {
		sql  string
		args func() []any
		dest *uuid.UUID
	}{
		{`INSERT INTO categories (business_id, menu_id, translations) VALUES ($1, $2, '{"tr":{"name":"Kardeş"}}') RETURNING id`,
			func() []any { return []any{tenant.business, tenant.menu} }, &category},
		{`INSERT INTO products (business_id, category_id, translations, price) VALUES ($1, $2, '{"tr":{"name":"Kardeş"}}', 10) RETURNING id`,
			func() []any { return []any{tenant.business, category} }, &product},
		{`INSERT INTO menus (business_id, name, slug) VALUES ($1, 'Öteki Menü', 'oteki-menu') RETURNING id`,
			func() []any { return []any{tenant.business} }, &otherMenu},
		{`INSERT INTO categories (business_id, menu_id, translations) VALUES ($1, $2, '{"tr":{"name":"Öteki"}}') RETURNING id`,
			func() []any { return []any{tenant.business, otherMenu} }, &otherMenuCategory},
	} {
		if err := pool.QueryRow(ctx, step.sql, step.args()...).Scan(step.dest); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	return category, product, otherMenuCategory
}

func TestWritersThatWaitedForADeleteReportTheParentGone(t *testing.T) {
	pool := scratchDatabase(t)
	translations := models.Translations{"tr": {Name: "Bekleyen"}}

	type fixture struct {
		tenant                              scopeTenant
		sibling, siblingProduct, otherMenuC uuid.UUID
	}

	for i, tc := range []struct {
		name string
		// menu: the parent deleted is the tenant's menu; otherwise its category.
		menu  bool
		write func(ctx context.Context, f fixture) error
	}{
		{"CreateCategory_in_a_menu_being_deleted", true, func(ctx context.Context, f fixture) error {
			_, err := repository.CreateCategory(ctx, pool, f.tenant.business, f.tenant.menu, translations, nil, nil, true)
			return err
		}},
		{"UpdateCategory_moving_into_a_menu_being_deleted", true, func(ctx context.Context, f fixture) error {
			_, err := repository.UpdateCategory(ctx, pool, f.otherMenuC, f.tenant.business, map[string]any{"menu_id": f.tenant.menu})
			return err
		}},
		{"CreateProduct_in_a_category_of_a_menu_being_deleted", true, func(ctx context.Context, f fixture) error {
			_, err := repository.CreateProduct(ctx, pool, f.tenant.business, f.tenant.category, translations, 10, nil, nil, nil,
				nil, nil, nil, true, false)
			return err
		}},
		{"CreateProduct_in_a_category_being_deleted", false, func(ctx context.Context, f fixture) error {
			_, err := repository.CreateProduct(ctx, pool, f.tenant.business, f.tenant.category, translations, 10, nil, nil, nil,
				nil, nil, nil, true, false)
			return err
		}},
		{"UpdateProduct_moving_into_a_category_being_deleted", false, func(ctx context.Context, f fixture) error {
			_, err := repository.UpdateProduct(ctx, pool, f.siblingProduct, f.tenant.business,
				map[string]any{"category_id": f.tenant.category})
			return err
		}},
		{"ReorderProducts_into_a_category_being_deleted", false, func(ctx context.Context, f fixture) error {
			return repository.ReorderProducts(ctx, pool, f.tenant.business, f.tenant.category, []uuid.UUID{f.siblingProduct})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tenant := insertScopeTenant(t, pool, "gone-"+string(rune('a'+i)))
			sibling, siblingProduct, otherMenuCategory := insertSiblings(t, pool, tenant)
			f := fixture{tenant: tenant, sibling: sibling, siblingProduct: siblingProduct, otherMenuC: otherMenuCategory}

			holder, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("could not open the delete's transaction: %v", err)
			}
			t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
			var holderPID int
			if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatalf("could not read the backend pid: %v", err)
			}
			namespace, parent, deleteSQL := repository.CategoryLockNamespace, tenant.category, `DELETE FROM categories WHERE id = $1`
			if tc.menu {
				namespace, parent, deleteSQL = repository.MenuLockNamespace, tenant.menu, `DELETE FROM menus WHERE id = $1`
			}
			if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock($1::int4, hashtext($2::uuid::text))`,
				namespace, parent); err != nil {
				t.Fatalf("could not take the parent's advisory lock: %v", err)
			}

			writeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- tc.write(writeCtx, f) }()

			deadline := time.Now().Add(10 * time.Second)
			for waiting := false; !waiting; {
				select {
				case err := <-result:
					t.Fatalf("the writer finished without waiting for the delete of its parent: %v", err)
				default:
				}
				if err := pool.QueryRow(ctx, `
					SELECT EXISTS (SELECT 1 FROM pg_stat_activity
					               WHERE datname = current_database() AND $1::int = ANY(pg_blocking_pids(pid)))`,
					holderPID).Scan(&waiting); err != nil {
					t.Fatalf("could not read pg_stat_activity: %v", err)
				}
				if time.Now().After(deadline) {
					t.Fatalf("the writer never started waiting on the delete's transaction")
				}
				time.Sleep(10 * time.Millisecond)
			}

			if _, err := holder.Exec(ctx, deleteSQL, parent); err != nil {
				t.Fatalf("could not delete the parent: %v", err)
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatalf("could not commit the delete: %v", err)
			}

			select {
			case err := <-result:
				if !errors.Is(err, repository.ErrParentNotFound) {
					t.Errorf("the writer answered %v, want ErrParentNotFound", err)
				}
			case <-time.After(20 * time.Second):
				t.Fatalf("the writer did not finish after the delete committed")
			}

			var bystanders int
			if err := pool.QueryRow(ctx, `
				SELECT (SELECT count(*) FROM products WHERE translations->'tr'->>'name' = 'Bekleyen') +
				       (SELECT count(*) FROM categories WHERE translations->'tr'->>'name' = 'Bekleyen')`).
				Scan(&bystanders); err != nil {
				t.Fatalf("could not count the written rows: %v", err)
			}
			if bystanders != 0 {
				t.Errorf("the writer that found its parent gone still wrote %d row(s)", bystanders)
			}
		})
	}
}
