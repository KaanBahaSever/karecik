package repository_test

// The tenant scope of the structural writers, against a real PostgreSQL.
//
// The handlers check that a menu or a category belongs to the business before
// they call a writer, so over HTTP a writer never sees a record of another
// tenant. The writers check it again themselves — before they take the record's
// advisory lock, and before they write anything — and these tests call them
// directly with records of another business while a transaction of the test's
// own holds the advisory lock of each such record, the way a delete by its
// owner would. A writer that took the lock would wait until the test's short
// deadline; a writer that skipped the check would write.
//
// Like the HTTP suites in backend/tests, this one uses the server named by
// KARECIK_TEST_DATABASE_URL (or the local default) only to create a scratch
// database of its own, and it skips when no server can be reached.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/database"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// scopeTestDeadline is how long a writer may take here. A writer that checks
// the business first answers in milliseconds; one that waits for the lock the
// test holds never answers before the deadline.
const scopeTestDeadline = 2 * time.Second

// scratchDatabase creates, migrates and eventually drops a database of this
// test's own, and returns a pool on it.
func scratchDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	adminURL := strings.TrimSpace(os.Getenv("KARECIK_TEST_DATABASE_URL"))
	if adminURL == "" {
		adminURL = "postgres://postgres@localhost:5432/postgres?sslmode=disable"
	}
	adminCfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Skipf("repository scope test skipped: the connection string is not usable: %v", err)
	}
	adminCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err == nil {
		err = admin.Ping(ctx)
	}
	if err != nil {
		t.Skipf("repository scope test skipped: no PostgreSQL server could be reached: %v", err)
	}
	t.Cleanup(admin.Close)

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("could not generate a scratch database name: %v", err)
	}
	// The name is a fixed prefix plus hex, so quoting it cannot be an injection.
	name := "karecik_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		t.Skipf("repository scope test skipped: could not create a scratch database: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		if _, err := admin.Exec(dropCtx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`); err != nil {
			t.Errorf("could not drop the scratch database %q: %v — drop it by hand", name, err)
		}
	})

	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatalf("could not parse the connection string: %v", err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("could not connect to the scratch database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("could not migrate the scratch database: %v", err)
	}
	return pool
}

// scopeTenant is one business with a menu, a category and a product.
type scopeTenant struct {
	business, menu, category, product uuid.UUID
}

// insertScopeTenant writes a business and its records straight into the tables.
func insertScopeTenant(t *testing.T, pool *pgxpool.Pool, label string) scopeTenant {
	t.Helper()
	ctx := context.Background()
	var user uuid.UUID
	var tenant scopeTenant
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, business_name) VALUES ($1, 'x', $2) RETURNING id`,
		"scope-"+label+"@example.test", "Kapsam "+label).Scan(&user); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	// Each step reads the id the step before it returned, so the arguments are
	// built when the step runs.
	for _, step := range []struct {
		sql  string
		args func() []any
		dest *uuid.UUID
	}{
		{`INSERT INTO businesses (user_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
			func() []any { return []any{user, "Kapsam " + label, "kapsam-" + label} }, &tenant.business},
		{`INSERT INTO menus (business_id, name, slug) VALUES ($1, 'Menü', 'menu') RETURNING id`,
			func() []any { return []any{tenant.business} }, &tenant.menu},
		{`INSERT INTO categories (business_id, menu_id, translations) VALUES ($1, $2, '{"tr":{"name":"Kategori"}}') RETURNING id`,
			func() []any { return []any{tenant.business, tenant.menu} }, &tenant.category},
		{`INSERT INTO products (business_id, category_id, translations, price) VALUES ($1, $2, '{"tr":{"name":"Ürün"}}', 10) RETURNING id`,
			func() []any { return []any{tenant.business, tenant.category} }, &tenant.product},
	} {
		if err := pool.QueryRow(ctx, step.sql, step.args()...).Scan(step.dest); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	return tenant
}

// holdAdvisoryLock takes an exclusive advisory lock with the repository's keys
// in a transaction of the test's own, until the test ends.
func holdAdvisoryLock(t *testing.T, pool *pgxpool.Pool, namespace int32, id uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("could not open the lock-holding transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::int4, hashtext($2::uuid::text))`, namespace, id); err != nil {
		t.Fatalf("could not take the advisory lock: %v", err)
	}
}

func TestWritersRefuseAnotherBusinessesRecordsBeforeLockingThem(t *testing.T) {
	pool := scratchDatabase(t)
	a := insertScopeTenant(t, pool, "a")
	b := insertScopeTenant(t, pool, "b")

	// b's menu and category are locked exclusively for the whole test.
	holdAdvisoryLock(t, pool, repository.MenuLockNamespace, b.menu)
	holdAdvisoryLock(t, pool, repository.CategoryLockNamespace, b.category)

	call := func(t *testing.T) (context.Context, context.CancelFunc) {
		t.Helper()
		return context.WithTimeout(context.Background(), scopeTestDeadline)
	}
	var (
		ctx          = context.Background()
		translations = models.Translations{"tr": {Name: "Kaçak"}}
	)
	productIn := func(t *testing.T, id uuid.UUID) uuid.UUID {
		t.Helper()
		var category uuid.UUID
		if err := pool.QueryRow(ctx, `SELECT category_id FROM products WHERE id = $1`, id).Scan(&category); err != nil {
			t.Fatalf("could not read product %s: %v", id, err)
		}
		return category
	}
	categoryOn := func(t *testing.T, id uuid.UUID) uuid.UUID {
		t.Helper()
		var menu uuid.UUID
		if err := pool.QueryRow(ctx, `SELECT menu_id FROM categories WHERE id = $1`, id).Scan(&menu); err != nil {
			t.Fatalf("could not read category %s: %v", id, err)
		}
		return menu
	}
	countNamed := func(t *testing.T, table string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM `+table+` WHERE translations->'tr'->>'name' = 'Kaçak'`).Scan(&n); err != nil {
			t.Fatalf("could not count the rows of %s: %v", table, err)
		}
		return n
	}

	t.Run("CreateProduct_in_a_category_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		product, err := repository.CreateProduct(callCtx, pool, a.business, b.category, translations, 10, nil, nil, nil,
			nil, nil, nil, true, false)
		if product != nil || !errors.Is(err, repository.ErrParentNotFound) {
			t.Errorf("CreateProduct = (%v, %v), want ErrParentNotFound at once", product, err)
		}
		if n := countNamed(t, "products"); n != 0 {
			t.Errorf("%d product(s) were created in another business's category", n)
		}
	})

	t.Run("UpdateProduct_into_a_category_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		product, err := repository.UpdateProduct(callCtx, pool, a.product, a.business, map[string]any{"category_id": b.category})
		if product != nil || !errors.Is(err, repository.ErrParentNotFound) {
			t.Errorf("UpdateProduct = (%v, %v), want ErrParentNotFound at once", product, err)
		}
		if category := productIn(t, a.product); category != a.category {
			t.Errorf("the product moved to category %s", category)
		}
	})

	t.Run("ReorderProducts_into_a_category_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		if err := repository.ReorderProducts(callCtx, pool, a.business, b.category, []uuid.UUID{a.product}); !errors.Is(err, repository.ErrParentNotFound) {
			t.Errorf("ReorderProducts = %v, want ErrParentNotFound at once", err)
		}
		if category := productIn(t, a.product); category != a.category {
			t.Errorf("the product moved to category %s", category)
		}
	})

	t.Run("CreateCategory_in_a_menu_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		category, err := repository.CreateCategory(callCtx, pool, a.business, b.menu, translations, nil, nil, true)
		if category != nil || !errors.Is(err, repository.ErrParentNotFound) {
			t.Errorf("CreateCategory = (%v, %v), want ErrParentNotFound at once", category, err)
		}
		if n := countNamed(t, "categories"); n != 0 {
			t.Errorf("%d categor(ies) were created in another business's menu", n)
		}
	})

	t.Run("UpdateCategory_into_a_menu_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		category, err := repository.UpdateCategory(callCtx, pool, a.category, a.business, map[string]any{"menu_id": b.menu})
		if category != nil || !errors.Is(err, repository.ErrParentNotFound) {
			t.Errorf("UpdateCategory = (%v, %v), want ErrParentNotFound at once", category, err)
		}
		if menu := categoryOn(t, a.category); menu != a.menu {
			t.Errorf("the category moved to menu %s", menu)
		}
	})

	t.Run("DeleteMenu_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		if err := repository.DeleteMenu(callCtx, pool, b.menu, a.business); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("DeleteMenu = %v, want ErrNotFound at once", err)
		}
	})

	t.Run("DeleteCategory_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		if count, err := repository.DeleteCategory(callCtx, pool, b.category, a.business); count != 0 || !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("DeleteCategory = (%d, %v), want ErrNotFound at once", count, err)
		}
	})

	t.Run("ApplyPrices_on_a_menu_of_another_business", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		changes, written, err := repository.ApplyPrices(callCtx, pool, a.business, b.menu, []uuid.UUID{b.category}, 10, utils.RoundNone)
		if err != nil || written != 0 || len(changes) != 0 {
			t.Errorf("ApplyPrices = (%d changes, %d written, %v), want nothing", len(changes), written, err)
		}
	})

	// The writers still work on the business's own records while the other
	// business's locks are held.
	t.Run("own_records", func(t *testing.T) {
		callCtx, cancel := call(t)
		defer cancel()
		if _, err := repository.CreateProduct(callCtx, pool, a.business, a.category, models.Translations{"tr": {Name: "Kendi"}},
			10, nil, nil, nil, nil, nil, nil, true, false); err != nil {
			t.Errorf("CreateProduct in the business's own category = %v", err)
		}
		if _, err := repository.CreateCategory(callCtx, pool, a.business, a.menu, models.Translations{"tr": {Name: "Kendi"}},
			nil, nil, true); err != nil {
			t.Errorf("CreateCategory in the business's own menu = %v", err)
		}
	})

	var price float64
	if err := pool.QueryRow(ctx, `SELECT price::float8 FROM products WHERE id = $1`, b.product).Scan(&price); err != nil ||
		price != 10 {
		t.Errorf("the other business's product costs %v (%v), want 10", price, err)
	}
}
