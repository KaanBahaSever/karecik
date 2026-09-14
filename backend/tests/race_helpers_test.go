package tests

// Helpers for the suites that stage a race against the API with transactions
// of their own. The pattern is the one product_lock_order_test.go uses: a
// transaction opened through h.pool holds a row lock, a request runs on a
// goroutine, and the test moves on only once PostgreSQL itself reports who is
// waiting for whom — so every interleaving is the same on every run.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// heldLock is a transaction of the test's own that holds at least one lock.
type heldLock struct {
	tx  pgx.Tx
	pid int
}

// holdLock opens a transaction through h.pool and runs one statement in it —
// a SELECT ... FOR <mode> or a write — whose locks the transaction then keeps.
// The cleanup rolls it back, a no-op once the test has committed or rolled back
// itself, and then waits for the pending requests, so a failing test never
// leaves a request blocked behind a lock nobody will release.
func holdLock(t *testing.T, h *harness, pending func() []*pendingRequest, sql string, args ...any) *heldLock {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWaitTimeout)
	defer cancel()

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("could not open the lock-holding transaction: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
		waitForPending(pending)
	})

	return runHoldingStatement(t, ctx, tx, sql, args...)
}

// holdLockOnOwnConnection is holdLock on a connection of its own, outside the
// harness pool. The pool has five connections, and a test that keeps several
// transactions open while requests wait on them would otherwise leave those
// requests without one.
func holdLockOnOwnConnection(t *testing.T, h *harness, pending func() []*pendingRequest,
	sql string, args ...any) *heldLock {

	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWaitTimeout)
	defer cancel()

	adminURL := strings.TrimSpace(os.Getenv(adminURLEnv))
	if adminURL == "" {
		adminURL = defaultAdminURL
	}
	cfg, err := pgx.ParseConfig(adminURL)
	if err != nil {
		t.Fatalf("could not parse the connection string of the lock-holding connection: %v", err)
	}
	cfg.Database = h.dbName
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("could not open the lock-holding connection: %v", err)
	}
	// Registered before the rollback, so it runs after it: t.Cleanup is LIFO.
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("could not open the lock-holding transaction: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
		waitForPending(pending)
	})

	return runHoldingStatement(t, ctx, tx, sql, args...)
}

// runHoldingStatement reads the backend pid of a fresh transaction and runs the
// statement whose locks the transaction keeps.
func runHoldingStatement(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) *heldLock {
	t.Helper()
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("could not read the backend pid of the lock-holding transaction: %v", err)
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("the lock-holding statement failed: %v", err)
	}
	return &heldLock{tx: tx, pid: pid}
}

// waitForPending waits a bounded time for every pending request to answer.
func waitForPending(pending func() []*pendingRequest) {
	if pending == nil {
		return
	}
	for _, request := range pending() {
		select {
		case <-request.done:
		case <-time.After(4 * lockWaitTimeout):
		}
	}
}

// Advisory lock keys, spelled as the repository spells them (see
// repository/locks.go): a namespace per kind of parent and hashtext of the id.
// A test that holds one of these stands in for a delete of that parent.
const (
	menuAdvisoryLock     = `SELECT pg_advisory_xact_lock(1263684942, hashtext($1::text::uuid::text))`
	categoryAdvisoryLock = `SELECT pg_advisory_xact_lock(1263682388, hashtext($1::text::uuid::text))`
)

// lockWaiter is one backend of the scratch database that waits on a lock.
type lockWaiter struct {
	pid      int
	query    string
	blockers []int
}

func (w lockWaiter) blockedBy(pid int) bool {
	for _, blocker := range w.blockers {
		if blocker == pid {
			return true
		}
	}
	return false
}

// lockWaiters lists the backends of the scratch database that are waiting on a
// lock, oldest first, with the statement each one runs and the backends that
// block it.
func lockWaiters(t *testing.T, h *harness) []lockWaiter {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The statements start with indentation, so it is trimmed before a prefix
	// is compared; chr() spells the whitespace without escape sequences.
	rows, err := h.pool.Query(ctx, `
		SELECT pid, ltrim(query, chr(32) || chr(9) || chr(13) || chr(10)), pg_blocking_pids(pid)
		FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock'
		ORDER BY backend_start`)
	if err != nil {
		t.Fatalf("could not read pg_stat_activity: %v", err)
	}
	defer rows.Close()

	var waiters []lockWaiter
	for rows.Next() {
		var w lockWaiter
		var blockers []int32
		if err := rows.Scan(&w.pid, &w.query, &blockers); err != nil {
			t.Fatalf("could not read pg_stat_activity: %v", err)
		}
		w.query = strings.Join(strings.Fields(w.query), " ")
		for _, blocker := range blockers {
			w.blockers = append(w.blockers, int(blocker))
		}
		waiters = append(waiters, w)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("could not read pg_stat_activity: %v", err)
	}
	return waiters
}

// waitForWaiterBlockedBy polls until some backend waits on a lock that the
// backend blockerPID holds, and returns it.
func waitForWaiterBlockedBy(t *testing.T, h *harness, what string, blockerPID int) lockWaiter {
	t.Helper()
	deadline := time.Now().Add(lockWaitTimeout)
	for {
		for _, w := range lockWaiters(t, h) {
			if w.blockedBy(blockerPID) {
				return w
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: no backend started waiting on a lock held by backend %d within %s",
				what, blockerPID, lockWaitTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitForAnswerOrLockWait polls until either the request has answered — true —
// or a backend that is not one of known waits on a lock while running a
// statement that starts with prefix — false, and that backend.
func waitForAnswerOrLockWait(t *testing.T, h *harness, what string, request *pendingRequest,
	prefix string, known ...int) (lockWaiter, bool) {

	t.Helper()
	isKnown := func(pid int) bool {
		for _, k := range known {
			if k == pid {
				return true
			}
		}
		return false
	}

	deadline := time.Now().Add(lockWaitTimeout)
	for {
		select {
		case <-request.done:
			return lockWaiter{}, true
		default:
		}
		for _, w := range lockWaiters(t, h) {
			if !isKnown(w.pid) && strings.HasPrefix(w.query, prefix) {
				return w, false
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: neither answered nor started waiting on a lock within %s", what, lockWaitTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitForAnswerOrWaiterBlockedBy polls until either the request has answered —
// true — or some backend waits on a lock that the backend blockerPID holds —
// false, and that backend. Unlike waitForAnswerOrLockWait it ignores backends
// that wait on anything else, so a request that has just been released from
// one lock is not mistaken for a waiter before it reaches the next.
func waitForAnswerOrWaiterBlockedBy(t *testing.T, h *harness, what string, request *pendingRequest,
	blockerPID int) (lockWaiter, bool) {

	t.Helper()
	deadline := time.Now().Add(lockWaitTimeout)
	for {
		select {
		case <-request.done:
			return lockWaiter{}, true
		default:
		}
		for _, w := range lockWaiters(t, h) {
			if w.blockedBy(blockerPID) {
				return w, false
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: neither answered nor started waiting on backend %d within %s", what, blockerPID, lockWaitTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// insertMenuWithID writes a menu with a chosen id straight into the table, for
// the given business. Every other column keeps its schema default.
func insertMenuWithID(t *testing.T, h *harness, id, businessID, name, slug string) menuPayload {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tag, err := h.pool.Exec(ctx, `
		INSERT INTO menus (id, business_id, name, slug)
		VALUES ($1::text::uuid, $2::text::uuid, $3, $4)`, id, businessID, name, slug)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("fixture: could not insert menu %s: %v", id, err)
	}
	return menuPayload{ID: id, Name: name, Slug: slug}
}

// requireAnswersWithin fails the test when the request has not answered within
// the limit: it is not supposed to wait for any lock.
func requireAnswersWithin(t *testing.T, what string, request *pendingRequest, limit time.Duration) {
	t.Helper()
	select {
	case <-request.done:
	case <-time.After(limit):
		t.Errorf("%s did not answer within %s: it waits for a lock it should not need", what, limit)
	}
}

// pinMenuPriceDate moves a menu's price date into the past, so that a price
// change really makes the products_touch_menu_price_date trigger write — and
// lock — the menu row.
func pinMenuPriceDate(t *testing.T, h *harness, menuID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := h.pool.Exec(ctx,
		`UPDATE menus SET price_updated_at = $1 WHERE id = $2::text::uuid`, lockTestPriceDate, menuID); err != nil {
		t.Fatalf("fixture: could not pin the price date of menu %s: %v", menuID, err)
	}
}

// productRow reads a product straight from the table: whether it exists, the
// category it sits in and its price.
func productRow(t *testing.T, h *harness, id string) (bool, string, float64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var category string
	var price float64
	err := h.pool.QueryRow(ctx,
		`SELECT category_id::text, price::float8 FROM products WHERE id = $1::text::uuid`, id).Scan(&category, &price)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", 0
	}
	if err != nil {
		t.Fatalf("could not read product %s: %v", id, err)
	}
	return true, category, price
}

// rowExists reports whether a menu, category or product row is still there.
func rowExists(t *testing.T, h *harness, table, id string) bool {
	t.Helper()
	switch table {
	case "menus", "categories", "products":
	default:
		t.Fatalf("rowExists: unexpected table %q", table)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var exists bool
	if err := h.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE id = $1::text::uuid)`, table), id).Scan(&exists); err != nil {
		t.Fatalf("could not look for %s %s: %v", table, id, err)
	}
	return exists
}

// fixtureUUID builds a uuid whose place in PostgreSQL's uuid order is chosen by
// the test: uuid values compare byte by byte, which is the order of their
// lowercase hexadecimal spelling, so group decides the first eight digits and
// n the last twelve.
func fixtureUUID(group uint32, n uint64) string {
	return fmt.Sprintf("%08x-0000-4000-8000-%012x", group, n)
}

// insertCategoryWithID writes a category with a chosen id straight into the
// table, into the given menu and its business, so a test can rely on how the id
// sorts. The row is what POST /api/categories stores for a Turkish name.
func insertCategoryWithID(t *testing.T, h *harness, id, menuID, name string, position int) categoryPayload {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tag, err := h.pool.Exec(ctx, `
		INSERT INTO categories (id, business_id, menu_id, translations, position)
		SELECT $1::text::uuid, m.business_id, m.id, $3::jsonb, $4
		FROM menus m WHERE m.id = $2::text::uuid`, id, menuID, turkishName(t, name), position)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("fixture: could not insert category %s into menu %s: %v", id, menuID, err)
	}
	return categoryPayload{ID: id, MenuID: &menuID}
}

// insertProductWithID is insertCategoryWithID for a product.
func insertProductWithID(t *testing.T, h *harness, id, categoryID, name string, price float64,
	position int) productPayload {

	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tag, err := h.pool.Exec(ctx, `
		INSERT INTO products (id, business_id, category_id, translations, price, position)
		SELECT $1::text::uuid, c.business_id, c.id, $3::jsonb, $4::float8, $5
		FROM categories c WHERE c.id = $2::text::uuid`, id, categoryID, turkishName(t, name), price, position)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("fixture: could not insert product %s into category %s: %v", id, categoryID, err)
	}
	return productPayload{ID: id, CategoryID: categoryID, Price: price}
}

// turkishName is the translations value of a record named in Turkish.
func turkishName(t *testing.T, name string) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]map[string]string{"tr": {"name": name, "description": ""}})
	if err != nil {
		t.Fatalf("fixture: could not encode the name %q: %v", name, err)
	}
	return string(encoded)
}

// requireNotFound asserts a 404 whose error message is exactly message.
func requireNotFound(t *testing.T, what string, status int, body []byte, message string) {
	t.Helper()
	var refusal struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil || status != 404 || refusal.Error != message ||
		refusal.Code != "NOT_FOUND" {
		t.Errorf("%s answered %d %s, want 404 %q", what, status, shorten(body), message)
	}
}

// shorten keeps a response body readable in a failure message.
func shorten(body []byte) string {
	if len(body) > 300 {
		return string(body[:300]) + "..."
	}
	return string(body)
}
