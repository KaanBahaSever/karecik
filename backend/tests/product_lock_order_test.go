package tests

// Lock order of the product writers, proven against a real PostgreSQL.
//
// Migration 010 put a trigger on products that locks the menu row of a product
// whose price changes, so a price edit locks its product row and then the menu
// row. Anything that takes those two locks the other way round can deadlock
// with it, and two writers that lock the same product rows in different orders
// can deadlock with each other. The rule the repository follows: every
// multi-row product writer locks the product rows in ascending id order first,
// and the menu row last.
//
// Neither test hopes for a race. A transaction of the test's own, opened
// through h.pool, holds one product row; the writer under test is started
// through the API on a goroutine; and the test waits until PostgreSQL itself
// reports that writer as blocked by the held transaction (pg_blocking_pids)
// before it takes the next step. Every interleaving below is therefore the
// same on every run.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"karecik/backend/internal/utils"
)

// lockWaitTimeout bounds every wait on the database in this file, so a writer
// that never blocks or never finishes is a failure with a message rather than a
// hung test.
const lockWaitTimeout = 15 * time.Second

// lockTestPriceDate is where the menu's price date is pinned before a price
// edit, so the trigger has a date to move and really locks the menu row.
var lockTestPriceDate = time.Date(2021, 3, 4, 10, 0, 0, 0, time.UTC)

// pendingRequest is a request running on a goroutine of its own. result may
// only be read once done is closed.
type pendingRequest struct {
	done   chan struct{}
	status int
	body   []byte
	err    error
}

// startRequest sends one request through the app under test on its own
// goroutine. It never touches t: only the test's own goroutine may fail it.
func (h *harness) startRequest(method, path, session string, body any) *pendingRequest {
	pending := &pendingRequest{done: make(chan struct{})}

	var raw []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			pending.err = err
			close(pending.done)
			return pending
		}
		raw = encoded
	}

	go func() {
		defer close(pending.done)

		var reader io.Reader
		if raw != nil {
			reader = bytes.NewReader(raw)
		}
		req := httptest.NewRequest(method, path, reader)
		if raw != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if session != "" {
			req.AddCookie(&http.Cookie{Name: utils.SessionCookieName, Value: session})
		}

		resp, err := h.app.Test(req, int(3*lockWaitTimeout/time.Millisecond))
		if err != nil {
			pending.err = err
			return
		}
		defer resp.Body.Close()
		pending.status = resp.StatusCode
		pending.body, pending.err = io.ReadAll(resp.Body)
	}()
	return pending
}

// wait blocks until the request has answered, failing the test if it does not
// within the limit.
func (p *pendingRequest) wait(t *testing.T, what string) (int, []byte) {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(4 * lockWaitTimeout):
		t.Fatalf("%s: no response within %s", what, 4*lockWaitTimeout)
	}
	if p.err != nil {
		t.Fatalf("%s: the request never completed: %v", what, p.err)
	}
	return p.status, p.body
}

// heldProduct is one product row locked FOR UPDATE by a transaction of the
// test's own.
type heldProduct struct {
	tx  pgx.Tx
	pid int
}

// holdProduct opens a transaction through h.pool and locks one product row in
// it. The cleanup rolls it back — a no-op once the test has committed or rolled
// back itself — and then waits for the pending requests, so a failing test
// never leaves a writer blocked behind a lock nobody will release.
func holdProduct(t *testing.T, h *harness, productID string, pending func() []*pendingRequest) *heldProduct {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWaitTimeout)
	defer cancel()

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("could not open the lock-holding transaction: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
		for _, request := range pending() {
			select {
			case <-request.done:
			case <-time.After(4 * lockWaitTimeout):
			}
		}
	})

	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("could not read the backend pid of the lock-holding transaction: %v", err)
	}
	var locked string
	if err := tx.QueryRow(ctx,
		`SELECT id::text FROM products WHERE id = $1::text::uuid FOR UPDATE`, productID).Scan(&locked); err != nil {
		t.Fatalf("could not lock product %s: %v", productID, err)
	}
	return &heldProduct{tx: tx, pid: pid}
}

// waitUntilBlockedBy polls until a backend of the scratch database is waiting
// for a lock that pid holds, and returns the statement that backend is running.
func waitUntilBlockedBy(t *testing.T, h *harness, what string, pid int) string {
	t.Helper()
	deadline := time.Now().Add(lockWaitTimeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var query string
		err := h.pool.QueryRow(ctx, `
			SELECT query FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
			  AND $1::int = ANY(pg_blocking_pids(pid))
			LIMIT 1`, pid).Scan(&query)
		cancel()

		switch {
		case err == nil:
			return strings.Join(strings.Fields(query), " ")
		case !errors.Is(err, pgx.ErrNoRows):
			t.Fatalf("%s: could not read pg_stat_activity: %v", what, err)
		case time.Now().After(deadline):
			t.Fatalf("%s: no backend started waiting for the row held by backend %d within %s — the "+
				"writer never reached the held product", what, pid, lockWaitTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// deadlocksSoFar reads how many deadlocks PostgreSQL has detected in the
// scratch database. The counter only grows and nothing else runs in that
// database, so any growth across a scenario is the scenario's.
func deadlocksSoFar(t *testing.T, h *harness) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var count int64
	if err := h.pool.QueryRow(ctx,
		`SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`).Scan(&count); err != nil {
		t.Fatalf("could not read pg_stat_database.deadlocks: %v", err)
	}
	return count
}

// rowIsLocked reports whether some other transaction holds a lock on a product
// row that conflicts with FOR UPDATE. NOWAIT turns "would wait" into an
// immediate 55P03; on a free row the probe's own lock ends with its statement.
func rowIsLocked(t *testing.T, h *harness, productID string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.pool.Exec(ctx,
		`SELECT 1 FROM products WHERE id = $1::text::uuid FOR UPDATE NOWAIT`, productID)
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return true
	}
	t.Fatalf("probing the lock on product %s failed: %v", productID, err)
	return false
}

// idOrder returns the ids sorted the way PostgreSQL orders uuid values.
func idOrder(t *testing.T, h *harness, ids []string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := h.pool.Query(ctx,
		`SELECT id::text FROM products WHERE id = ANY($1::text[]::uuid[]) ORDER BY id`, ids)
	if err != nil {
		t.Fatalf("could not sort the product ids: %v", err)
	}
	sorted, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("could not sort the product ids: %v", err)
	}
	if len(sorted) != len(ids) {
		t.Fatalf("fixture: %d of %d products exist", len(sorted), len(ids))
	}
	return sorted
}

// TestMenuDeleteDoesNotDeadlockWithAPriceEdit replays the deadlock between a
// menu delete and a price edit in that menu, as a regression test.
//
// A plain DELETE FROM menus locked the menu row and then waited, through the
// cascade, on a product row a price edit held — while the price edit's trigger
// waited on the menu row. PostgreSQL broke that cycle after deadlock_timeout by
// aborting one of the two, and the API answered 500 when the victim was the
// delete.
//
// The retry net in the handler would hide the second half of that story — the
// aborted delete runs again and answers 200 — so a 200 alone proves nothing.
// The test therefore also reads PostgreSQL's own deadlock counter: it must not
// move at all.
func TestMenuDeleteDoesNotDeadlockWithAPriceEdit(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Kilit Kafe", "lock-delete-owner@example.test")
	menu := h.createMenu(owner, "Silinecek Menü")
	category := h.createCategory(owner, menu.ID, "Sıcak İçecekler")
	latte := h.createProduct(owner, category.ID, "Latte", 145)
	h.createProduct(owner, category.ID, "Filtre Kahve", 90)
	h.createProduct(owner, category.ID, "Çay", 30)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The trigger writes a menu only when its date is not already the writing
	// transaction's now(). A date in the past guarantees the price edit below
	// really takes the menu row lock — the half of the cycle this is about.
	if _, err := h.pool.Exec(ctx,
		`UPDATE menus SET price_updated_at = $1 WHERE id = $2::text::uuid`, lockTestPriceDate, menu.ID); err != nil {
		t.Fatalf("fixture: could not pin the menu's price date: %v", err)
	}

	deadlocksBefore := deadlocksSoFar(t, h)

	var deleteRequest *pendingRequest
	held := holdProduct(t, h, latte.ID, func() []*pendingRequest {
		if deleteRequest == nil {
			return nil
		}
		return []*pendingRequest{deleteRequest}
	})

	deleteRequest = h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
	waiting := waitUntilBlockedBy(t, h, "DELETE /api/menus/:id", held.pid)
	t.Logf("the delete is blocked by the held product row while running: %s", waiting)

	// The price edit, in the transaction that already holds the product row.
	// The only lock it can still wait for is the menu row its trigger writes.
	started := time.Now()
	tag, err := held.tx.Exec(ctx,
		`UPDATE products SET price = price + 1 WHERE id = $1::text::uuid`, latte.ID)
	editTook := time.Since(started)
	if err != nil {
		t.Fatalf("the price edit failed after %s while the delete was waiting: %v",
			editTook.Round(time.Millisecond), err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("the price edit matched %d rows, want 1", tag.RowsAffected())
	}
	if err := held.tx.Commit(ctx); err != nil {
		t.Fatalf("committing the price edit failed after the edit took %s: %v",
			editTook.Round(time.Millisecond), err)
	}
	t.Logf("the price edit took %s and committed", editTook.Round(time.Millisecond))

	status, body := deleteRequest.wait(t, "DELETE /api/menus/:id")
	if status != http.StatusOK {
		t.Fatalf("DELETE /api/menus/:id answered %d once the price edit had committed, want 200: %s",
			status, body)
	}

	if deadlocks := deadlocksSoFar(t, h) - deadlocksBefore; deadlocks != 0 {
		t.Fatalf("PostgreSQL detected %d deadlock(s) between the menu delete and the price edit (the "+
			"edit took %s). The delete answered 200 only because the retry ran it again; it has to lock "+
			"the products before the menu row so that no cycle forms at all",
			deadlocks, editTook.Round(time.Millisecond))
	}

	resp, payload := h.do(http.MethodGet, "/api/menus/"+menu.ID, owner.session, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("after the delete answered 200, GET /api/menus/:id still answers %d: %s", resp.StatusCode, payload)
	}
	var remaining int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM products WHERE category_id = $1::text::uuid`, category.ID).Scan(&remaining); err != nil {
		t.Fatalf("could not count the products left behind: %v", err)
	}
	if remaining != 0 {
		t.Errorf("the deleted menu left %d products behind", remaining)
	}
}

// TestProductWritersLockInAscendingIDOrder checks the rule itself on every
// multi-row product writer: the menu delete, the category delete, the bulk
// price update and the product reorder.
//
// Each writer gets six products of its own. The test holds the third one in id
// order and waits until the writer is blocked on it. A writer that locks in
// ascending id order is blocked there holding exactly the two smaller ids and
// none of the three larger ones. A writer that takes a larger id before the
// held one, or reaches the held one before a smaller id, shows up as a larger
// id already locked or a smaller one still free. The reorder request lists its
// ids in descending order, so a writer that simply followed the request would
// be caught too.
func TestProductWritersLockInAscendingIDOrder(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Sıra Kafe", "lock-order-owner@example.test")

	const productsPerWriter = 6
	const heldIndex = 2

	type fixture struct {
		menuID     string
		categoryID string
		ids        []string // in id order
	}
	newFixture := func(label string) fixture {
		menu := h.createMenu(owner, label+" Menüsü")
		category := h.createCategory(owner, menu.ID, label)
		ids := make([]string, 0, productsPerWriter)
		for i := 0; i < productsPerWriter; i++ {
			product := h.createProduct(owner, category.ID, fmt.Sprintf("%s %d", label, i+1), float64(10*(i+1)))
			ids = append(ids, product.ID)
		}
		return fixture{menuID: menu.ID, categoryID: category.ID, ids: idOrder(t, h, ids)}
	}

	reversed := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for i := len(ids) - 1; i >= 0; i-- {
			out = append(out, ids[i])
		}
		return out
	}

	type writer struct {
		name    string
		fixture fixture
		method  string
		path    string
		body    any
		check   func(t *testing.T, f fixture, status int, body []byte)
	}

	requireOK := func(t *testing.T, what string, status int, body []byte) {
		t.Helper()
		if status != http.StatusOK {
			t.Fatalf("%s answered %d once the held row was released, want 200: %s", what, status, body)
		}
	}

	menuDelete := newFixture("Menü Silme")
	categoryDelete := newFixture("Kategori Silme")
	bulkPrice := newFixture("Toplu Fiyat")
	reorder := newFixture("Sıralama")

	writers := []writer{
		{
			name: "menu_delete", fixture: menuDelete,
			method: http.MethodDelete, path: "/api/menus/" + menuDelete.menuID,
			check: func(t *testing.T, f fixture, status int, body []byte) {
				requireOK(t, "DELETE /api/menus/:id", status, body)
			},
		},
		{
			name: "category_delete", fixture: categoryDelete,
			method: http.MethodDelete, path: "/api/categories/" + categoryDelete.categoryID,
			check: func(t *testing.T, f fixture, status int, body []byte) {
				requireOK(t, "DELETE /api/categories/:id", status, body)
				var result struct {
					DeletedProducts int `json:"deleted_products"`
				}
				decodeInto(t, "DELETE /api/categories/:id", body, &result)
				if result.DeletedProducts != productsPerWriter {
					t.Errorf("DELETE /api/categories/:id reports deleted_products %d, want %d",
						result.DeletedProducts, productsPerWriter)
				}
			},
		},
		{
			name: "bulk_price_apply", fixture: bulkPrice,
			method: http.MethodPost, path: "/api/products/bulk-price",
			body: map[string]any{"menu_id": bulkPrice.menuID, "percentage": 10, "rounding": "none", "apply": true},
			check: func(t *testing.T, f fixture, status int, body []byte) {
				requireOK(t, "POST /api/products/bulk-price", status, body)
				var result struct {
					Applied  bool `json:"applied"`
					Affected int  `json:"affected"`
				}
				decodeInto(t, "POST /api/products/bulk-price", body, &result)
				if !result.Applied || result.Affected != productsPerWriter {
					t.Errorf("POST /api/products/bulk-price reports applied=%t affected=%d, want true and %d",
						result.Applied, result.Affected, productsPerWriter)
				}
			},
		},
		{
			name: "product_reorder", fixture: reorder,
			method: http.MethodPut, path: "/api/products/reorder",
			body: map[string]any{"category_id": reorder.categoryID, "ids": reversed(reorder.ids)},
			check: func(t *testing.T, f fixture, status int, body []byte) {
				requireOK(t, "PUT /api/products/reorder", status, body)
				var products []productPayload
				decodeInto(t, "PUT /api/products/reorder", body, &products)
				got := make([]string, 0, len(products))
				for _, product := range products {
					got = append(got, product.ID)
				}
				if want := reversed(f.ids); strings.Join(got, ",") != strings.Join(want, ",") {
					t.Errorf("PUT /api/products/reorder left the order %v, want the requested %v", got, want)
				}
			},
		},
	}

	deadlocksBefore := deadlocksSoFar(t, h)

	for _, w := range writers {
		t.Run(w.name, func(t *testing.T) {
			ids := w.fixture.ids
			var request *pendingRequest
			held := holdProduct(t, h, ids[heldIndex], func() []*pendingRequest {
				if request == nil {
					return nil
				}
				return []*pendingRequest{request}
			})

			request = h.startRequest(w.method, w.path, owner.session, w.body)
			waiting := waitUntilBlockedBy(t, h, w.name, held.pid)
			t.Logf("%s is blocked by the held product row while running: %s", w.name, waiting)

			var lockedEarly, freeEarly []string
			for i, id := range ids {
				if i == heldIndex {
					continue
				}
				locked := rowIsLocked(t, h, id)
				switch {
				case i < heldIndex && !locked:
					freeEarly = append(freeEarly, fmt.Sprintf("#%d %s", i, id))
				case i > heldIndex && locked:
					lockedEarly = append(lockedEarly, fmt.Sprintf("#%d %s", i, id))
				}
			}

			if err := held.tx.Rollback(context.Background()); err != nil {
				t.Fatalf("could not release the held product row: %v", err)
			}
			status, body := request.wait(t, w.name)
			w.check(t, w.fixture, status, body)

			if len(lockedEarly) > 0 || len(freeEarly) > 0 {
				t.Errorf("%s does not lock its products in ascending id order. While it waited for "+
					"product #%d it already held larger ids %v and had not yet locked smaller ids %v "+
					"(ids in order: %v)", w.name, heldIndex, lockedEarly, freeEarly, ids)
			}
		})
	}

	if deadlocks := deadlocksSoFar(t, h) - deadlocksBefore; deadlocks != 0 {
		t.Errorf("PostgreSQL detected %d deadlock(s) while the writers ran one at a time", deadlocks)
	}
}
