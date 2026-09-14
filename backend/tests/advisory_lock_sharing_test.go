package tests

// What the shared advisory locks of repository/locks.go promise besides making
// a write wait for a delete of its parent.
//
// Shared locks conflict only with the exclusive lock of a delete, so writes
// into the same menu or the same category never wait on each other there. A
// write into a category takes the lock of the category's menu before the lock
// of the category, like every writer that takes both. And the row lock of a
// category reorder is FOR NO KEY UPDATE, which does not conflict with the KEY
// SHARE lock a product write's foreign key check holds on its category until
// that write commits.

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// sideBySideAnswerWithin is how long a write may take that needs no lock
// another write holds.
const sideBySideAnswerWithin = 2 * time.Second

// TestWritesIntoTheSameParentDoNotWaitOnEachOther: a write into a menu, or into
// a category, waits for a row lock of the test's own while it holds the shared
// advisory locks of that parent. Every other write into the same parent has to
// answer at once.
func TestWritesIntoTheSameParentDoNotWaitOnEachOther(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Yan Yana Kafe", "side-by-side-owner@example.test")

	menu := h.createMenu(owner, "Ortak Menü")
	category := h.createCategory(owner, menu.ID, "Ortak Kategori")
	other := h.createMenu(owner, "Diğer Menü")
	source := h.createCategory(owner, other.ID, "Kaynak")

	type write struct {
		name, method, path string
		body               any
		status             int
	}
	productCreate := func(name string) write {
		return write{"a product create in the category", http.MethodPost, "/api/products", map[string]any{
			"category_id":  category.ID,
			"translations": map[string]any{"tr": map[string]any{"name": name}},
			"price":        25,
		}, http.StatusCreated}
	}
	categoryCreate := func(name string) write {
		return write{"a category create in the menu", http.MethodPost, "/api/categories", map[string]any{
			"menu_id":      menu.ID,
			"translations": map[string]any{"tr": map[string]any{"name": name}},
		}, http.StatusCreated}
	}

	runCase := func(t *testing.T, hold, holdID string, waiting write, others []write) {
		t.Helper()
		var pending []*pendingRequest
		held := holdLockOnOwnConnection(t, h, func() []*pendingRequest { return pending }, hold, holdID)

		first := h.startRequest(waiting.method, waiting.path, owner.session, waiting.body)
		pending = append(pending, first)
		waiter := waitForWaiterBlockedBy(t, h, waiting.name, held.pid)
		t.Logf("%s (backend %d) holds its advisory locks and waits on the held row: %s", waiting.name, waiter.pid,
			waiter.query)

		for _, w := range others {
			request := h.startRequest(w.method, w.path, owner.session, w.body)
			pending = append(pending, request)
			select {
			case <-request.done:
				if request.err != nil || request.status != w.status {
					t.Errorf("%s answered %d %s (%v), want %d", w.name, request.status, shorten(request.body),
						request.err, w.status)
				}
			case <-time.After(sideBySideAnswerWithin):
				t.Errorf("%s did not answer within %s while %s held the shared advisory locks of the same parent",
					w.name, sideBySideAnswerWithin, waiting.name)
			}
		}

		if err := held.tx.Rollback(context.Background()); err != nil {
			t.Fatalf("could not release the held row: %v", err)
		}
		if status, body := first.wait(t, waiting.name); status != waiting.status {
			t.Errorf("%s answered %d, want %d: %s", waiting.name, status, waiting.status, body)
		}
		for _, request := range pending {
			request.wait(t, "a write into the same parent")
		}
	}

	t.Run("category_moves_into_one_menu", func(t *testing.T) {
		moving := h.createCategory(owner, other.ID, "Taşınan 1")
		alongside := h.createCategory(owner, other.ID, "Taşınan 2")
		runCase(t, `SELECT id FROM categories WHERE id = $1::text::uuid FOR UPDATE`, moving.ID,
			write{"the category move", http.MethodPut, "/api/categories/" + moving.ID,
				map[string]any{"menu_id": menu.ID}, http.StatusOK},
			[]write{
				{"a second category move into the menu", http.MethodPut, "/api/categories/" + alongside.ID,
					map[string]any{"menu_id": menu.ID}, http.StatusOK},
				categoryCreate("Yan Yana Kategori 1"),
				productCreate("Yan Yana Ürün 1"),
			})
	})

	t.Run("product_moves_into_one_category", func(t *testing.T) {
		moving := h.createProduct(owner, source.ID, "Taşınan Ürün 1", 10)
		alongside := h.createProduct(owner, source.ID, "Taşınan Ürün 2", 10)
		reordered := h.createProduct(owner, source.ID, "Sıralanan Ürün", 10)
		runCase(t, `SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`, moving.ID,
			write{"the product move", http.MethodPut, "/api/products/" + moving.ID,
				map[string]any{"category_id": category.ID}, http.StatusOK},
			[]write{
				{"a second product move into the category", http.MethodPut, "/api/products/" + alongside.ID,
					map[string]any{"category_id": category.ID}, http.StatusOK},
				{"a reorder into the category", http.MethodPut, "/api/products/reorder",
					map[string]any{"category_id": category.ID, "ids": []string{reordered.ID}}, http.StatusOK},
				productCreate("Yan Yana Ürün 2"),
				categoryCreate("Yan Yana Kategori 2"),
			})
	})
}

// TestAWriteIntoACategoryHoldsItsMenusLockWhileItWaitsForTheCategorys: a lock
// of the test's own on a category's advisory key stands in for a delete of that
// category. A create, a move and a reorder into the category wait for it — and
// while they wait they already hold the shared lock of the category's menu, so
// an exclusive try of that menu's lock fails.
func TestAWriteIntoACategoryHoldsItsMenusLockWhileItWaitsForTheCategorys(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Kilit Sırası Kafe", "lock-order-menu-first@example.test")

	menu := h.createMenu(owner, "Sıralı Menü")
	category := h.createCategory(owner, menu.ID, "Sıralı Kategori")
	other := h.createMenu(owner, "Kaynak Menü")
	source := h.createCategory(owner, other.ID, "Kaynak Kategori")
	moving := h.createProduct(owner, source.ID, "Taşınan", 10)
	reordered := h.createProduct(owner, source.ID, "Sıralanan", 10)

	menuLockIsFree := func(t *testing.T) bool {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatalf("could not open the probe transaction: %v", err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var free bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(1263684942, hashtext($1::text::uuid::text))`,
			menu.ID).Scan(&free); err != nil {
			t.Fatalf("could not try the menu's advisory lock: %v", err)
		}
		return free
	}

	for _, w := range []struct {
		name, method, path string
		body               any
		status             int
	}{
		{"a product create", http.MethodPost, "/api/products", map[string]any{
			"category_id":  category.ID,
			"translations": map[string]any{"tr": map[string]any{"name": "Sıralı Yeni"}},
			"price":        15,
		}, http.StatusCreated},
		{"a product move", http.MethodPut, "/api/products/" + moving.ID, map[string]any{"category_id": category.ID},
			http.StatusOK},
		{"a product reorder", http.MethodPut, "/api/products/reorder",
			map[string]any{"category_id": category.ID, "ids": []string{reordered.ID}}, http.StatusOK},
	} {
		t.Run(w.name, func(t *testing.T) {
			if !menuLockIsFree(t) {
				t.Fatalf("fixture: the menu's advisory lock is taken before %s starts", w.name)
			}

			var pending []*pendingRequest
			held := holdLockOnOwnConnection(t, h, func() []*pendingRequest { return pending },
				categoryAdvisoryLock, category.ID)

			request := h.startRequest(w.method, w.path, owner.session, w.body)
			pending = append(pending, request)
			waiter := waitForWaiterBlockedBy(t, h, w.name+" at the category's advisory lock", held.pid)
			t.Logf("%s (backend %d) waits on the category's advisory lock while running: %s", w.name, waiter.pid,
				waiter.query)

			if menuLockIsFree(t) {
				t.Errorf("%s waits for the category's advisory lock without holding its menu's: the menu's lock "+
					"has to be taken first", w.name)
			}

			if err := held.tx.Rollback(context.Background()); err != nil {
				t.Fatalf("could not release the category's advisory lock: %v", err)
			}
			if status, body := request.wait(t, w.name); status != w.status {
				t.Errorf("%s answered %d, want %d: %s", w.name, status, w.status, body)
			}
			if !menuLockIsFree(t) {
				t.Errorf("the menu's advisory lock is still taken after %s answered", w.name)
			}
		})
	}
}

// TestCategoryReorderDoesNotWaitForAProductWriteIntoTheCategory: a product is
// moved into category C1 with a new price, and waits for the menu row — which
// the price date trigger writes — after its foreign key check has taken a KEY
// SHARE lock on C1. A reorder of the menu's categories, C1 included, has to
// answer at once.
func TestCategoryReorderDoesNotWaitForAProductWriteIntoTheCategory(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Anahtar Paylaşımı Kafe", "key-share-owner@example.test")

	menu := h.createMenu(owner, "Paylaşım Menü")
	first := h.createCategory(owner, menu.ID, "Birinci")
	second := h.createCategory(owner, menu.ID, "İkinci")
	product := h.createProduct(owner, second.ID, "Taşınan", 100)
	pinMenuPriceDate(t, h, menu.ID)

	var pending []*pendingRequest
	held := holdLockOnOwnConnection(t, h, func() []*pendingRequest { return pending },
		`SELECT id FROM menus WHERE id = $1::text::uuid FOR NO KEY UPDATE`, menu.ID)

	move := h.startRequest(http.MethodPut, "/api/products/"+product.ID, owner.session,
		map[string]any{"category_id": first.ID, "price": 150})
	pending = append(pending, move)
	waiter := waitForWaiterBlockedBy(t, h, "the move with a new price at the menu row", held.pid)
	t.Logf("the move (backend %d) holds KEY SHARE on the category and waits on the menu row: %s", waiter.pid,
		waiter.query)

	reorder := h.startRequest(http.MethodPut, "/api/categories/reorder", owner.session,
		map[string]any{"ids": []string{second.ID, first.ID}})
	pending = append(pending, reorder)
	requireAnswersWithin(t, "the category reorder", reorder, sideBySideAnswerWithin)

	if err := held.tx.Rollback(context.Background()); err != nil {
		t.Fatalf("could not release the menu row: %v", err)
	}
	if status, body := reorder.wait(t, "PUT /api/categories/reorder"); status != http.StatusOK {
		t.Errorf("PUT /api/categories/reorder answered %d, want 200: %s", status, body)
	}
	if status, body := move.wait(t, "PUT /api/products/:id"); status != http.StatusOK {
		t.Errorf("PUT /api/products/:id answered %d, want 200: %s", status, body)
	}
	if position := categoryPosition(t, h, first.ID); position != 1 {
		t.Errorf("the reordered category %s is at position %d, want 1", first.ID, position)
	}
}
