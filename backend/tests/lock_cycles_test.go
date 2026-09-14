package tests

// Lock cycles between a structural move and a delete of the move's new parent.
//
// A move reaches its new parent only through the foreign key check of its own
// UPDATE, after it has locked the row it moves, so row locks alone cannot order
// it against a delete of that parent. Each test below stages one interleaving in
// which a move lands in the middle of a delete and a third writer then needs a
// row one of the two holds. The move has to wait for the delete — at the
// parent's advisory lock, before it has locked anything (see
// repository/locks.go) — and then find its target gone.
//
// A 200 alone proves nothing, because RetryOnConflict runs a write PostgreSQL
// aborted as a deadlock victim again. So every test also asserts that
// PostgreSQL counted no deadlock and that no retry was logged, that no request
// answered 5xx, and that the rows agree with the answers.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// capturedStandardLog collects what the standard logger writes during a test,
// and still passes every line on to the test binary's own output.
type capturedStandardLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *capturedStandardLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	l.buf.Write(p)
	l.mu.Unlock()
	return os.Stderr.Write(p)
}

func (l *capturedStandardLog) reset() {
	l.mu.Lock()
	l.buf.Reset()
	l.mu.Unlock()
}

// linesContaining returns the logged lines that contain substr.
func (l *capturedStandardLog) linesContaining(substr string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var lines []string
	for _, line := range strings.Split(l.buf.String(), "\n") {
		if strings.Contains(line, substr) {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

// captureStandardLog routes the standard logger through a capturedStandardLog
// until the test ends. RetryOnConflict writes its retry lines there.
func captureStandardLog(t *testing.T) *capturedStandardLog {
	t.Helper()
	captured := &capturedStandardLog{}
	previous := log.Writer()
	log.SetOutput(captured)
	t.Cleanup(func() { log.SetOutput(previous) })
	return captured
}

// settledDeadlocks reads how many deadlocks PostgreSQL has counted since
// before. A backend adds its counts to pg_stat_database only once it is idle
// again, so a deadlock detected a moment ago may not show yet when the requests
// have answered; the count is therefore read again for a few seconds, and
// returned as soon as it is not zero.
func settledDeadlocks(t *testing.T, h *harness, before int64) int64 {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		count := deadlocksSoFar(t, h) - before
		if count != 0 || time.Now().After(deadline) {
			return count
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// categoryMenuOf reads the menu a category sits on, straight from the table.
func categoryMenuOf(t *testing.T, h *harness, id string) (bool, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var menuID string
	err := h.pool.QueryRow(ctx, `SELECT menu_id::text FROM categories WHERE id = $1::text::uuid`, id).Scan(&menuID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ""
	}
	if err != nil {
		t.Fatalf("could not read category %s: %v", id, err)
	}
	return true, menuID
}

// requireWaitsOn waits until request is blocked by the backend blocker, and
// fails the test when it answers first or waits on something else. It returns
// the waiting backend, or 0 when there is none.
func requireWaitsOn(t *testing.T, h *harness, what string, request *pendingRequest, blocker int,
	known ...int) int {

	t.Helper()
	waiter, answered := waitForAnswerOrLockWait(t, h, what, request, "", known...)
	switch {
	case answered:
		t.Errorf("%s answered while the delete (backend %d) was still running: it has to wait for the "+
			"delete before it locks anything", what, blocker)
		return 0
	case !waiter.blockedBy(blocker):
		t.Errorf("%s (backend %d) waits on %v, want the delete (backend %d)", what, waiter.pid,
			waiter.blockers, blocker)
	default:
		t.Logf("%s (backend %d) waits on the delete while running: %s", what, waiter.pid, waiter.query)
	}
	return waiter.pid
}

// requireNoConflicts asserts that nothing deadlocked, that no write was retried
// and that no request answered 5xx.
func requireNoConflicts(t *testing.T, h *harness, logs *capturedStandardLog, deadlocksBefore int64,
	answers map[string]int) {

	t.Helper()
	if deadlocks := settledDeadlocks(t, h, deadlocksBefore); deadlocks != 0 {
		t.Errorf("PostgreSQL detected %d deadlock(s)", deadlocks)
	}
	if retries := logs.linesContaining("retrying"); len(retries) > 0 {
		t.Errorf("a write lost a lock conflict and was run again: %q", retries)
	}
	for what, status := range answers {
		if status >= 500 {
			t.Errorf("%s answered %d", what, status)
		}
	}
}

// requireMissingCategory asserts a 404 that names the category.
func requireMissingCategory(t *testing.T, what string, body []byte) {
	t.Helper()
	var refusal struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil || refusal.Error != "Kategori bulunamadı." {
		t.Errorf("%s answered 404 with %s, want %q", what, body, "Kategori bulunamadı.")
	}
}

// TestCategoryMovedIntoAMenuBeingDeletedWaitsForTheDelete: while menu M is being
// deleted, a category of another menu is moved into M, and a product of that
// category gets a new price.
//
// A transaction of the test's own holds M's row, which keeps the delete waiting
// after it has locked M's products and categories. A move that got into M at
// that point would leave the delete's cascade needing the moved category's
// product, held by the price edit, while the price edit — once the product was
// in M — needed M's row, held by the delete.
func TestCategoryMovedIntoAMenuBeingDeletedWaitsForTheDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Döngü A Kafe", "cycle-a-owner@example.test")

	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()
			menu := h.createMenu(owner, fmt.Sprintf("Silinen A%d", run))
			category := h.createCategory(owner, menu.ID, "Silinen Kategori")
			resident := h.createProduct(owner, category.ID, "Mevcut Ürün", 10)
			other := h.createMenu(owner, fmt.Sprintf("Diğer A%d", run))
			moving := h.createCategory(owner, other.ID, "Gelen Kategori")
			traveller := h.createProduct(owner, moving.ID, "Gelen Ürün", 100)
			pinMenuPriceDate(t, h, menu.ID)
			pinMenuPriceDate(t, h, other.ID)

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			held := holdLock(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM menus WHERE id = $1::text::uuid FOR NO KEY UPDATE`, menu.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id", held.pid)
			t.Logf("the delete (backend %d) waits on the held menu row while running: %s", deleter.pid, deleter.query)

			moveRequest := h.startRequest(http.MethodPut, "/api/categories/"+moving.ID, owner.session,
				map[string]any{"menu_id": menu.ID})
			pending = append(pending, moveRequest)
			known := []int{deleter.pid}
			if mover := requireWaitsOn(t, h, "the category move", moveRequest, deleter.pid, known...); mover != 0 {
				known = append(known, mover)
			}

			editRequest := h.startRequest(http.MethodPatch, "/api/products/"+traveller.ID+"/price", owner.session,
				map[string]any{"price": 150})
			pending = append(pending, editRequest)
			if editor, answered := waitForAnswerOrLockWait(t, h, "the price edit", editRequest, "", known...); answered {
				t.Logf("the price edit answered while the delete was still waiting")
			} else {
				t.Logf("the price edit (backend %d) waits on %v while running: %s", editor.pid, editor.blockers, editor.query)
			}

			if err := held.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held menu row: %v", err)
			}

			moveStatus, moveBody := moveRequest.wait(t, "PUT /api/categories/:id")
			editStatus, editBody := editRequest.wait(t, "PATCH /api/products/:id/price")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			t.Logf("move %d %s | price edit %d %s | delete %d %s", moveStatus, shorten(moveBody),
				editStatus, shorten(editBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"PUT /api/categories/:id": moveStatus, "PATCH /api/products/:id/price": editStatus,
				"DELETE /api/menus/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			if rowExists(t, h, "menus", menu.ID) || rowExists(t, h, "categories", category.ID) ||
				rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the menu, its category or its product is still there", deleteStatus)
			}
			if !rowExists(t, h, "menus", other.ID) {
				t.Errorf("the other menu disappeared")
			}

			movedExists, movedMenu := categoryMenuOf(t, h, moving.ID)
			travellerExists, travellerCategory, travellerPrice := productRow(t, h, traveller.ID)
			switch moveStatus {
			case http.StatusNotFound:
				// The record that vanished is the menu the category was moved to.
				requireNotFound(t, "the category move", moveStatus, moveBody, "Menü bulunamadı.")
				if !movedExists || movedMenu != other.ID {
					t.Errorf("the move answered 404, so the category must still be in menu %s; exists=%t in %q",
						other.ID, movedExists, movedMenu)
				}
				if !travellerExists || travellerCategory != moving.ID {
					t.Errorf("the move answered 404, so the product must still be in category %s; exists=%t in %q",
						moving.ID, travellerExists, travellerCategory)
				}
				if editStatus == http.StatusOK && travellerPrice != 150 {
					t.Errorf("the price edit answered 200 but the product costs %.2f", travellerPrice)
				}
			case http.StatusOK:
				if movedExists || travellerExists {
					t.Errorf("the move answered 200, so the category was in the menu when it was deleted — but "+
						"category exists=%t, product exists=%t", movedExists, travellerExists)
				}
			default:
				t.Errorf("PUT /api/categories/:id answered %d, want 404 (the menu was gone) or 200: %s", moveStatus, moveBody)
			}
			if editStatus != http.StatusOK && editStatus != http.StatusNotFound {
				t.Errorf("PATCH /api/products/:id/price answered %d, want 200 or 404: %s", editStatus, editBody)
			}
		})
	}
}

// TestProductMovedIntoAMenuBeingDeletedWaitsForTheDelete: while menu M is being
// deleted, a product of another menu is moved into a category of M between the
// delete's product and category lock steps, and then gets a new price.
//
// Two transactions of the test's own hold a KEY SHARE lock on M's category with
// the smaller id and M's row. The delete locks M's products and waits at that
// category; the move is started there. The category is released, the delete
// moves on to wait at M's row, the price edit is started, and M's row is
// released last. A move that got in while the delete waited at the category
// would have left the cascade needing the moved product, held by the price
// edit, while the price edit needed M's row, held by the delete.
func TestProductMovedIntoAMenuBeingDeletedWaitsForTheDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Döngü B Kafe", "cycle-b-owner@example.test")

	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()
			menu := h.createMenu(owner, fmt.Sprintf("Silinen B%d", run))
			first := h.createCategory(owner, menu.ID, "Birinci")
			second := h.createCategory(owner, menu.ID, "İkinci")
			low, high := first, second
			if second.ID < first.ID {
				low, high = second, first
			}
			resident := h.createProduct(owner, low.ID, "Mevcut Ürün", 10)
			other := h.createMenu(owner, fmt.Sprintf("Diğer B%d", run))
			otherCategory := h.createCategory(owner, other.ID, "Diğer Kategori")
			traveller := h.createProduct(owner, otherCategory.ID, "Gelen Ürün", 100)
			pinMenuPriceDate(t, h, menu.ID)
			pinMenuPriceDate(t, h, other.ID)

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			pend := func() []*pendingRequest { return pending }
			keyShare := holdLock(t, h, pend, `SELECT id FROM categories WHERE id = $1::text::uuid FOR KEY SHARE`, low.ID)
			menuRow := holdLock(t, h, pend, `SELECT id FROM menus WHERE id = $1::text::uuid FOR NO KEY UPDATE`, menu.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id at its category step", keyShare.pid)
			t.Logf("the delete (backend %d) waits on the held category while running: %s", deleter.pid, deleter.query)

			moveRequest := h.startRequest(http.MethodPut, "/api/products/"+traveller.ID, owner.session,
				map[string]any{"category_id": high.ID})
			pending = append(pending, moveRequest)
			known := []int{deleter.pid}
			if mover := requireWaitsOn(t, h, "the product move", moveRequest, deleter.pid, known...); mover != 0 {
				known = append(known, mover)
			}

			if err := keyShare.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held category: %v", err)
			}
			atMenuRow := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id at the menu row", menuRow.pid)
			t.Logf("backend %d now waits on the held menu row while running: %s", atMenuRow.pid, atMenuRow.query)

			editRequest := h.startRequest(http.MethodPatch, "/api/products/"+traveller.ID+"/price", owner.session,
				map[string]any{"price": 150})
			pending = append(pending, editRequest)
			if editor, answered := waitForAnswerOrLockWait(t, h, "the price edit", editRequest, "",
				append(known, atMenuRow.pid)...); answered {
				t.Logf("the price edit answered while the delete was still waiting")
			} else {
				t.Logf("the price edit (backend %d) waits on %v while running: %s", editor.pid, editor.blockers, editor.query)
			}

			if err := menuRow.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held menu row: %v", err)
			}

			moveStatus, moveBody := moveRequest.wait(t, "PUT /api/products/:id")
			editStatus, editBody := editRequest.wait(t, "PATCH /api/products/:id/price")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			t.Logf("move %d %s | price edit %d %s | delete %d %s", moveStatus, shorten(moveBody),
				editStatus, shorten(editBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"PUT /api/products/:id": moveStatus, "PATCH /api/products/:id/price": editStatus,
				"DELETE /api/menus/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			if rowExists(t, h, "menus", menu.ID) || rowExists(t, h, "categories", low.ID) ||
				rowExists(t, h, "categories", high.ID) || rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the menu, one of its categories or its product is still there",
					deleteStatus)
			}
			if !rowExists(t, h, "menus", other.ID) || !rowExists(t, h, "categories", otherCategory.ID) {
				t.Errorf("the other menu or its category disappeared")
			}

			exists, inCategory, price := productRow(t, h, traveller.ID)
			switch moveStatus {
			case http.StatusNotFound:
				requireMissingCategory(t, "the product move", moveBody)
				if !exists || inCategory != otherCategory.ID {
					t.Errorf("the move answered 404, so the product must still be in category %s; exists=%t in %q",
						otherCategory.ID, exists, inCategory)
				}
				if editStatus == http.StatusOK && price != 150 {
					t.Errorf("the price edit answered 200 but the product costs %.2f", price)
				}
			case http.StatusOK:
				if exists {
					t.Errorf("the move answered 200, so the product was in the menu when it was deleted — but it "+
						"still exists in category %s at %.2f", inCategory, price)
				}
			default:
				t.Errorf("PUT /api/products/:id answered %d, want 404 (the category was gone) or 200: %s",
					moveStatus, moveBody)
			}
			if editStatus != http.StatusOK && editStatus != http.StatusNotFound {
				t.Errorf("PATCH /api/products/:id/price answered %d, want 200 or 404: %s", editStatus, editBody)
			}
		})
	}
}

// TestProductMovedAndReorderedIntoACategoryBeingDeletedWaitForTheDelete: while
// category D is being deleted, a product of another category is moved into D,
// and a reorder into D lists that product — which has the smaller id — together
// with the product the delete already holds.
//
// A transaction of the test's own holds a KEY SHARE lock on D, which keeps the
// delete waiting at its DELETE after it has locked D's product. A move that got
// in at that point would have left the reorder holding the moved product and
// waiting for the one the delete holds, while the delete's cascade needed the
// moved product. The answer of the delete also has to count exactly the
// products it removed.
func TestProductMovedAndReorderedIntoACategoryBeingDeletedWaitForTheDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Döngü C Kafe", "cycle-c-owner@example.test")

	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()
			menu := h.createMenu(owner, fmt.Sprintf("Menü C%d", run))
			doomed := h.createCategory(owner, menu.ID, "Silinecek")
			source := h.createCategory(owner, menu.ID, "Kaynak")
			// Chosen ids, so the traveller sorts before the resident on every run.
			resident := insertProductWithID(t, h, fixtureUUID(0xcc000000, uint64(run)), doomed.ID, "Mevcut Ürün", 10, 0)
			traveller := insertProductWithID(t, h, fixtureUUID(0x0c000000, uint64(run)), source.ID, "Gelen Ürün", 20, 0)

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			keyShare := holdLock(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM categories WHERE id = $1::text::uuid FOR KEY SHARE`, doomed.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/categories/"+doomed.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/categories/:id", keyShare.pid)
			t.Logf("the delete (backend %d) waits on the held category while running: %s", deleter.pid, deleter.query)

			moveRequest := h.startRequest(http.MethodPut, "/api/products/"+traveller.ID, owner.session,
				map[string]any{"category_id": doomed.ID})
			pending = append(pending, moveRequest)
			known := []int{deleter.pid}
			if mover := requireWaitsOn(t, h, "the product move", moveRequest, deleter.pid, known...); mover != 0 {
				known = append(known, mover)
			}

			reorderRequest := h.startRequest(http.MethodPut, "/api/products/reorder", owner.session,
				map[string]any{"category_id": doomed.ID, "ids": []string{traveller.ID, resident.ID}})
			pending = append(pending, reorderRequest)
			requireWaitsOn(t, h, "the reorder", reorderRequest, deleter.pid, known...)

			if err := keyShare.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held category: %v", err)
			}

			moveStatus, moveBody := moveRequest.wait(t, "PUT /api/products/:id")
			reorderStatus, reorderBody := reorderRequest.wait(t, "PUT /api/products/reorder")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/categories/:id")
			t.Logf("move %d %s | reorder %d %s | delete %d %s", moveStatus, shorten(moveBody),
				reorderStatus, shorten(reorderBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"PUT /api/products/:id": moveStatus, "PUT /api/products/reorder": reorderStatus,
				"DELETE /api/categories/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Fatalf("DELETE /api/categories/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			if rowExists(t, h, "categories", doomed.ID) || rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered 200 but the category or its product is still there")
			}
			if !rowExists(t, h, "categories", source.ID) {
				t.Errorf("the source category disappeared")
			}

			exists, inCategory, _ := productRow(t, h, traveller.ID)
			removed := 1
			if !exists {
				removed++
			}
			var result struct {
				DeletedProducts int `json:"deleted_products"`
			}
			decodeInto(t, "DELETE /api/categories/:id", deleteBody, &result)
			if result.DeletedProducts != removed {
				t.Errorf("DELETE /api/categories/:id reports deleted_products %d, but it removed %d product(s)",
					result.DeletedProducts, removed)
			}

			for what, answer := range map[string]struct {
				status int
				body   []byte
			}{
				"the product move": {moveStatus, moveBody},
				"the reorder":      {reorderStatus, reorderBody},
			} {
				switch answer.status {
				case http.StatusNotFound:
					requireMissingCategory(t, what, answer.body)
				case http.StatusOK:
				default:
					t.Errorf("%s answered %d, want 404 (the category was gone) or 200: %s", what, answer.status, answer.body)
				}
			}
			if moveStatus == http.StatusNotFound && reorderStatus == http.StatusNotFound &&
				(!exists || inCategory != source.ID) {
				t.Errorf("the move and the reorder answered 404, so the product must still be in category %s; "+
					"exists=%t in %q", source.ID, exists, inCategory)
			}
			if exists && inCategory != source.ID {
				t.Errorf("the moved product survived the delete of its new category in category %q", inCategory)
			}
		})
	}
}

// TestAMoveIntoACategoryThatChangesMenuWaitsForTheNewMenusDelete: a product
// move into category T has read T's menu, and T moves to another menu N before
// the move holds the advisory lock of the menu it read. N is being deleted by
// then, and a price edit of the moved product follows.
//
// A lock of the test's own on the advisory key of T's first menu stands in for
// a delete of that menu and keeps the move waiting there. T is moved to N, and
// N's delete starts and waits at its category step: two connections of the
// test's own hold a KEY SHARE lock on N's category with the smaller id, and N's
// row. Then the first menu's lock is released. The lock the move now holds
// belongs to a menu T has left, so the move has to read T's menu again, start
// over and wait for N's delete. A move that wrote instead would bring a product
// into N that the delete never locked, and the price edit of that product would
// wait for N's row while the delete's cascade needed the product.
func TestAMoveIntoACategoryThatChangesMenuWaitsForTheNewMenusDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Yeniden Okuma Kafe", "reread-owner@example.test")

	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()
			firstMenu := h.createMenu(owner, fmt.Sprintf("İlk Menü %d", run))
			newMenu := h.createMenu(owner, fmt.Sprintf("Yeni Menü %d", run))
			sourceMenu := h.createMenu(owner, fmt.Sprintf("Kaynak Menü %d", run))
			// Chosen ids: the new menu's own category sorts before T, so the
			// delete waits at it before it would reach T.
			low := insertCategoryWithID(t, h, fixtureUUID(0x1f000000, uint64(run)), newMenu.ID, "Küçük", 0)
			target := insertCategoryWithID(t, h, fixtureUUID(0xef000000, uint64(run)), firstMenu.ID, "Hedef", 0)
			resident := h.createProduct(owner, low.ID, "Mevcut Ürün", 10)
			source := h.createCategory(owner, sourceMenu.ID, "Kaynak")
			traveller := h.createProduct(owner, source.ID, "Gezgin Ürün", 100)
			pinMenuPriceDate(t, h, newMenu.ID)
			pinMenuPriceDate(t, h, sourceMenu.ID)

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			pend := func() []*pendingRequest { return pending }
			firstMenuLock := holdLockOnOwnConnection(t, h, pend, menuAdvisoryLock, firstMenu.ID)

			moveRequest := h.startRequest(http.MethodPut, "/api/products/"+traveller.ID, owner.session,
				map[string]any{"category_id": target.ID})
			pending = append(pending, moveRequest)
			mover := waitForWaiterBlockedBy(t, h, "the move at the first menu's advisory lock", firstMenuLock.pid)
			t.Logf("the move (backend %d) waits on the first menu's advisory lock while running: %s",
				mover.pid, mover.query)

			if resp, body := h.do(http.MethodPut, "/api/categories/"+target.ID, owner.session,
				map[string]any{"menu_id": newMenu.ID}); resp.StatusCode != http.StatusOK {
				t.Fatalf("fixture: moving category T to the new menu answered %d: %s", resp.StatusCode, body)
			}

			keyShare := holdLockOnOwnConnection(t, h, pend,
				`SELECT id FROM categories WHERE id = $1::text::uuid FOR KEY SHARE`, low.ID)
			menuRow := holdLockOnOwnConnection(t, h, pend,
				`SELECT id FROM menus WHERE id = $1::text::uuid FOR NO KEY UPDATE`, newMenu.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+newMenu.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "the new menu's delete at its category step", keyShare.pid)
			t.Logf("the delete (backend %d) waits on the held category while running: %s", deleter.pid, deleter.query)

			if err := firstMenuLock.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the first menu's advisory lock: %v", err)
			}
			restarted, answered := waitForAnswerOrWaiterBlockedBy(t, h, "the move after the first menu's lock",
				moveRequest, deleter.pid)
			if answered {
				t.Errorf("the move answered while the new menu's delete was still running: it wrote under the " +
					"lock of a menu its category had left instead of starting over and waiting for that delete")
			} else {
				t.Logf("the move (backend %d) started over and waits on the delete while running: %s",
					restarted.pid, restarted.query)
			}

			if err := keyShare.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held category: %v", err)
			}
			atMenuRow := waitForWaiterBlockedBy(t, h, "the new menu's delete at the menu row", menuRow.pid)
			t.Logf("backend %d now waits on the held menu row while running: %s", atMenuRow.pid, atMenuRow.query)

			editRequest := h.startRequest(http.MethodPatch, "/api/products/"+traveller.ID+"/price", owner.session,
				map[string]any{"price": 150})
			pending = append(pending, editRequest)
			if editor, answered := waitForAnswerOrLockWait(t, h, "the price edit", editRequest, "",
				deleter.pid, restarted.pid); answered {
				t.Logf("the price edit answered while the delete was still waiting")
			} else {
				t.Logf("the price edit (backend %d) waits on %v while running: %s", editor.pid, editor.blockers, editor.query)
			}

			if err := menuRow.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held menu row: %v", err)
			}

			moveStatus, moveBody := moveRequest.wait(t, "PUT /api/products/:id")
			editStatus, editBody := editRequest.wait(t, "PATCH /api/products/:id/price")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			t.Logf("move %d %s | price edit %d %s | delete %d %s", moveStatus, shorten(moveBody),
				editStatus, shorten(editBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"PUT /api/products/:id": moveStatus, "PATCH /api/products/:id/price": editStatus,
				"DELETE /api/menus/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			requireNotFound(t, "the move into the category the delete removed", moveStatus, moveBody,
				"Kategori bulunamadı.")
			if editStatus != http.StatusOK {
				t.Errorf("PATCH /api/products/:id/price answered %d, want 200: %s", editStatus, editBody)
			}
			if exists, inCategory, price := productRow(t, h, traveller.ID); !exists || inCategory != source.ID ||
				!samePrice(price, 150) {
				t.Errorf("the product must stay in category %s at 150.00; it exists=%t in %q at %.2f",
					source.ID, exists, inCategory, price)
			}
			if rowExists(t, h, "menus", newMenu.ID) || rowExists(t, h, "categories", low.ID) ||
				rowExists(t, h, "categories", target.ID) || rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the new menu, one of its categories or its product is still there",
					deleteStatus)
			}
			if !rowExists(t, h, "menus", firstMenu.ID) {
				t.Errorf("the first menu disappeared")
			}
		})
	}
}
