package tests

// A menu delete racing a product that is moved INTO that menu.
//
// Both races are replayed here exactly, as regression tests. The product move
// locks its product row, then — through the foreign key check on
// products.category_id — takes a KEY SHARE lock on the target category, and
// then, when the price changes too, the menu row through the trigger of
// migration 010. A delete that held the menu row and reached the category only
// through the cascade closed a lock cycle with it, and the API answered one of
// the two with a 500.
//
// repository.DeleteMenu therefore takes the menu's advisory lock, then locks
// products, then categories, then the menu, and a move into one of the menu's
// categories takes the same advisory lock, shared, before it locks anything
// (see repository/locks.go). Each test asserts what that has to guarantee:
// PostgreSQL detects no deadlock at all (the retry net would otherwise hide one
// behind a 200), no request answers 5xx, and the rows end up in a state that
// agrees with the answers.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestMenuDeleteDoesNotDeadlockWithAMoveWithANewPrice: a product is moved into
// a category of menu T with a new price while T is being deleted.
//
// A transaction of the test's own holds a KEY SHARE lock on the target
// category, which keeps the delete waiting on that category. The move starts
// next, and the held lock is released last — at once, or after a pause longer
// than deadlock_timeout.
func TestMenuDeleteDoesNotDeadlockWithAMoveWithANewPrice(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Taşıma Kafe", "move-with-price-owner@example.test")

	for _, variant := range []struct {
		name  string
		pause time.Duration
	}{
		{"release_at_once", 0},
		{"release_after_1500ms", 1500 * time.Millisecond},
	} {
		t.Run(variant.name, func(t *testing.T) {
			ctx := context.Background()

			source := h.createMenu(owner, "Kaynak "+variant.name)
			sourceCategory := h.createCategory(owner, source.ID, "Kaynak Kategori")
			moved := h.createProduct(owner, sourceCategory.ID, "Taşınan Ürün", 100)
			target := h.createMenu(owner, "Hedef "+variant.name)
			targetCategory := h.createCategory(owner, target.ID, "Hedef Kategori")
			resident := h.createProduct(owner, targetCategory.ID, "Hedefteki Ürün", 50)
			pinMenuPriceDate(t, h, target.ID)

			deadlocksBefore := deadlocksSoFar(t, h)

			var pending []*pendingRequest
			held := holdLock(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM categories WHERE id = $1::text::uuid FOR KEY SHARE`, targetCategory.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+target.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id", held.pid)
			t.Logf("the delete (backend %d) waits on the held category lock while running: %s", deleter.pid, deleter.query)

			moveRequest := h.startRequest(http.MethodPut, "/api/products/"+moved.ID, owner.session,
				map[string]any{"category_id": targetCategory.ID, "price": 150})
			pending = append(pending, moveRequest)
			// Any statement: the move waits at the menu's advisory lock, before
			// its UPDATE.
			if waiter, answered := waitForAnswerOrLockWait(t, h, "PUT /api/products/:id", moveRequest,
				"", deleter.pid); answered {
				t.Logf("the move answered while the delete was still waiting")
			} else {
				t.Logf("the move (backend %d) waits on %v while running: %s", waiter.pid, waiter.blockers, waiter.query)
			}

			time.Sleep(variant.pause)
			if err := held.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held category lock: %v", err)
			}

			moveStatus, moveBody := moveRequest.wait(t, "PUT /api/products/:id")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			deadlocks := deadlocksSoFar(t, h) - deadlocksBefore
			t.Logf("move %d %s | delete %d %s | deadlocks +%d", moveStatus, shorten(moveBody),
				deleteStatus, shorten(deleteBody), deadlocks)

			if deadlocks != 0 {
				t.Errorf("PostgreSQL detected %d deadlock(s) between the menu delete and the move with a new "+
					"price: the delete has to lock the menu's categories before the menu row", deadlocks)
			}
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			if moveStatus != http.StatusOK && moveStatus != http.StatusNotFound {
				t.Errorf("PUT /api/products/:id answered %d, want 200 (moved before the delete) or 404 (the "+
					"category was gone): %s", moveStatus, moveBody)
			}

			// The state has to agree with the answers.
			if rowExists(t, h, "menus", target.ID) || rowExists(t, h, "categories", targetCategory.ID) ||
				rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the target menu, its category or its product is still there",
					deleteStatus)
			}
			if !rowExists(t, h, "menus", source.ID) || !rowExists(t, h, "categories", sourceCategory.ID) {
				t.Errorf("the source menu or its category disappeared")
			}
			exists, category, price := productRow(t, h, moved.ID)
			switch moveStatus {
			case http.StatusOK:
				if exists {
					t.Errorf("the move answered 200 before the target menu was deleted, so the product went with "+
						"it — but it still exists in category %s at %.2f", category, price)
				}
			case http.StatusNotFound:
				if !exists || category != sourceCategory.ID || price != 100 {
					t.Errorf("the move answered 404, so the product must be untouched in category %s at 100.00; "+
						"it exists=%t in category %q at %.2f", sourceCategory.ID, exists, category, price)
				}
			}
		})
	}
}

// TestMenuDeleteDoesNotDeadlockWithAProductMovedIntoTheMenu: while menu M is
// being deleted, a product of another menu is moved into a category of M and
// then gets a new price.
//
// A transaction of the test's own holds the menu row, which keeps the delete
// waiting after it has locked M's products and categories. The move and the
// price edit start while it waits, and the menu row is released last.
//
// A move that slips into M at once lets the price edit wait on M's row while
// the delete's cascade needs the moved product the price edit holds. The move
// has to wait for the delete instead — at the menu's advisory lock, before it
// has locked anything — and then find the category gone.
func TestMenuDeleteDoesNotDeadlockWithAProductMovedIntoTheMenu(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Pencere Kafe", "moved-into-menu-owner@example.test")

	for run := 1; run <= 3; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()

			menu := h.createMenu(owner, fmt.Sprintf("Silinen %d", run))
			category := h.createCategory(owner, menu.ID, "Silinen Kategori")
			resident := h.createProduct(owner, category.ID, "Mevcut Ürün", 10)
			other := h.createMenu(owner, fmt.Sprintf("Diğer %d", run))
			otherCategory := h.createCategory(owner, other.ID, "Diğer Kategori")
			moving := h.createProduct(owner, otherCategory.ID, "Gelen Ürün", 100)
			pinMenuPriceDate(t, h, menu.ID)
			pinMenuPriceDate(t, h, other.ID)

			deadlocksBefore := deadlocksSoFar(t, h)

			var pending []*pendingRequest
			held := holdLock(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM menus WHERE id = $1::text::uuid FOR NO KEY UPDATE`, menu.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id", held.pid)
			t.Logf("the delete (backend %d) waits on the held menu row while running: %s", deleter.pid, deleter.query)

			moveRequest := h.startRequest(http.MethodPut, "/api/products/"+moving.ID, owner.session,
				map[string]any{"category_id": category.ID})
			pending = append(pending, moveRequest)
			mover, moveAnswered := waitForAnswerOrLockWait(t, h, "PUT /api/products/:id", moveRequest,
				"", deleter.pid)
			known := []int{deleter.pid}
			switch {
			case moveAnswered:
				t.Errorf("the move into a category of the menu being deleted answered while the delete was still " +
					"waiting: the move does not wait for the delete, so a product the delete never locked got into " +
					"the menu")
			case !mover.blockedBy(deleter.pid):
				t.Errorf("the move (backend %d) waits on %v, want the menu delete (backend %d)",
					mover.pid, mover.blockers, deleter.pid)
				known = append(known, mover.pid)
			default:
				t.Logf("the move (backend %d) waits on the delete while running: %s", mover.pid, mover.query)
				known = append(known, mover.pid)
			}

			editRequest := h.startRequest(http.MethodPatch, "/api/products/"+moving.ID+"/price", owner.session,
				map[string]any{"price": 150})
			pending = append(pending, editRequest)
			if editor, answered := waitForAnswerOrLockWait(t, h, "PATCH /api/products/:id/price", editRequest,
				"", known...); answered {
				t.Logf("the price edit answered while the delete was still waiting")
			} else {
				t.Logf("the price edit (backend %d) waits on %v", editor.pid, editor.blockers)
			}

			if err := held.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held menu row: %v", err)
			}

			moveStatus, moveBody := moveRequest.wait(t, "PUT /api/products/:id")
			editStatus, editBody := editRequest.wait(t, "PATCH /api/products/:id/price")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			deadlocks := deadlocksSoFar(t, h) - deadlocksBefore
			t.Logf("move %d %s | price edit %d %s | delete %d %s | deadlocks +%d", moveStatus, shorten(moveBody),
				editStatus, shorten(editBody), deleteStatus, shorten(deleteBody), deadlocks)

			if deadlocks != 0 {
				t.Errorf("PostgreSQL detected %d deadlock(s) between the menu delete, the move and the price edit",
					deadlocks)
			}
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			if editStatus != http.StatusOK {
				t.Errorf("PATCH /api/products/:id/price answered %d, want 200: %s", editStatus, editBody)
			}
			if moveStatus != http.StatusOK && moveStatus != http.StatusNotFound {
				t.Errorf("PUT /api/products/:id answered %d, want 200 or 404: %s", moveStatus, moveBody)
			}

			if rowExists(t, h, "menus", menu.ID) || rowExists(t, h, "categories", category.ID) ||
				rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the menu, its category or its product is still there", deleteStatus)
			}
			if !rowExists(t, h, "menus", other.ID) || !rowExists(t, h, "categories", otherCategory.ID) {
				t.Errorf("the other menu or its category disappeared")
			}
			exists, inCategory, price := productRow(t, h, moving.ID)
			switch moveStatus {
			case http.StatusNotFound:
				if !exists || inCategory != otherCategory.ID || price != 150 {
					t.Errorf("the move answered 404 and the price edit %d, so the product must stay in category %s "+
						"at 150.00; it exists=%t in category %q at %.2f", editStatus, otherCategory.ID, exists,
						inCategory, price)
				}
			case http.StatusOK:
				if exists {
					t.Errorf("the move answered 200, so the product was in the menu when it was deleted — but it "+
						"still exists in category %s at %.2f", inCategory, price)
				}
			}
		})
	}
}
