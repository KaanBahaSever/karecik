package tests

// Lock order of the category writers.
//
// DeleteMenu locks a menu's categories in ascending id order — after its
// products, before the menu row — so every other writer that locks several
// category rows has to take them in that same order. Two writers that take the
// same categories in different orders can each end up holding one and waiting
// for the other; PostgreSQL then aborts one after deadlock_timeout, and only
// the retry net keeps that from being a 500.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// categoryIsLocked reports whether another transaction holds a lock on a
// category row that conflicts with FOR UPDATE. NOWAIT turns "would wait" into
// an immediate 55P03; on a free row the probe's own lock ends with its
// statement.
func categoryIsLocked(t *testing.T, h *harness, id string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.pool.Exec(ctx, `SELECT 1 FROM categories WHERE id = $1::text::uuid FOR UPDATE NOWAIT`, id)
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return true
	}
	t.Fatalf("probing the lock on category %s failed: %v", id, err)
	return false
}

// TestCategoryReorderDoesNotDeadlockWithAMenuDelete replays the deadlock
// between PUT /api/categories/reorder and DELETE /api/menus/:id that forms when
// the delete locks categories in id order and the reorder locks them in any
// other order.
//
// The fixture has two categories whose creation order is the opposite of their
// id order. The test holds the one created first. A reorder that locks in
// creation order waits on it holding nothing; the delete locks the other one —
// the smaller id — and waits too. When the held row is released the reorder
// takes it and then needs the category the delete holds, while the delete
// waits for the one the reorder now holds. A reorder that locks in id order
// instead takes the smaller id first, so the delete waits for it before it
// holds anything the reorder needs.
func TestCategoryReorderDoesNotDeadlockWithAMenuDelete(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Sıra Kafe", "category-reorder-owner@example.test")

	var (
		menu          menuPayload
		first, second categoryPayload
	)
	for attempt := 1; ; attempt++ {
		if attempt > 40 {
			t.Fatalf("fixture: 40 pairs of categories never had the second one's id sort before the first one's")
		}
		menu = h.createMenu(owner, fmt.Sprintf("Sıra Menü %d", attempt))
		first = h.createCategory(owner, menu.ID, "Önce Açılan")
		second = h.createCategory(owner, menu.ID, "Sonra Açılan")
		// uuid values compare byte by byte, which is the order of their
		// lowercase hexadecimal spelling.
		if second.ID < first.ID {
			break
		}
	}
	h.createProduct(owner, first.ID, "Birinci Ürün", 10)
	h.createProduct(owner, second.ID, "İkinci Ürün", 20)

	deadlocksBefore := deadlocksSoFar(t, h)

	var pending []*pendingRequest
	held := holdLock(t, h, func() []*pendingRequest { return pending },
		`SELECT id FROM categories WHERE id = $1::text::uuid FOR UPDATE`, first.ID)

	reorderRequest := h.startRequest(http.MethodPut, "/api/categories/reorder", owner.session,
		map[string]any{"ids": []string{first.ID, second.ID}})
	pending = append(pending, reorderRequest)
	reorderer := waitForWaiterBlockedBy(t, h, "PUT /api/categories/reorder", held.pid)
	t.Logf("the reorder (backend %d) waits on the held category while running: %s", reorderer.pid, reorderer.query)

	deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
	pending = append(pending, deleteRequest)
	deleter, answered := waitForAnswerOrLockWait(t, h, "DELETE /api/menus/:id", deleteRequest, "", reorderer.pid, held.pid)
	if answered {
		t.Fatalf("fixture: the menu delete answered while a category of its menu was held")
	}
	t.Logf("the delete (backend %d) waits on %v while running: %s", deleter.pid, deleter.blockers, deleter.query)

	if err := held.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not release the held category: %v", err)
	}
	reorderStatus, reorderBody := reorderRequest.wait(t, "PUT /api/categories/reorder")
	deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")

	if deadlocks := deadlocksSoFar(t, h) - deadlocksBefore; deadlocks != 0 {
		t.Errorf("PostgreSQL detected %d deadlock(s) between the category reorder and the menu delete: the "+
			"reorder has to lock its categories in ascending id order, the order the delete uses", deadlocks)
	}
	if reorderStatus != http.StatusOK {
		t.Errorf("PUT /api/categories/reorder answered %d, want 200: %s", reorderStatus, reorderBody)
	}
	if deleteStatus != http.StatusOK {
		t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
	}
	if rowExists(t, h, "menus", menu.ID) || rowExists(t, h, "categories", first.ID) ||
		rowExists(t, h, "categories", second.ID) {
		t.Errorf("the delete answered %d but the menu or one of its categories is still there", deleteStatus)
	}
}

// TestCategoryWritersLockInAscendingIDOrder checks the order itself on the two
// writers that lock several categories: the menu delete and the category
// reorder.
//
// Each writer gets a menu of its own. The test holds the category with the
// third-smallest id and waits until the writer is blocked on it; a writer that
// locks in ascending id order then holds exactly the two smaller ids and none
// of the larger ones. The fixture is grown until its creation order disagrees
// with its id order around the held category, so a writer that followed
// creation (or position) order would show up as well, and the reorder request
// lists the ids in descending order to catch one that followed the request.
func TestCategoryWritersLockInAscendingIDOrder(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Kategori Sıra Kafe", "category-order-owner@example.test")

	const heldIndex = 2

	sameSet := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		seen := make(map[string]bool, len(a))
		for _, id := range a {
			seen[id] = true
		}
		for _, id := range b {
			if !seen[id] {
				return false
			}
		}
		return true
	}

	// newFixture returns a menu and its category ids in ascending id order.
	newFixture := func(label string) (menuPayload, []string) {
		menu := h.createMenu(owner, label+" Menüsü")
		created := make([]string, 0, 12)
		for len(created) < 30 {
			category := h.createCategory(owner, menu.ID, fmt.Sprintf("%s %d", label, len(created)+1))
			created = append(created, category.ID)
			if len(created) < 6 {
				continue
			}
			sorted := append([]string(nil), created...)
			sort.Strings(sorted)
			heldAt := -1
			for i, id := range created {
				if id == sorted[heldIndex] {
					heldAt = i
				}
			}
			if heldAt != heldIndex || !sameSet(created[:heldIndex], sorted[:heldIndex]) {
				return menu, sorted
			}
		}
		t.Fatalf("fixture: 30 categories were created in ascending id order around the held one")
		return menuPayload{}, nil
	}

	descending := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for i := len(ids) - 1; i >= 0; i-- {
			out = append(out, ids[i])
		}
		return out
	}

	deleteMenu, deleteIDs := newFixture("Menü Silme")
	reorderMenu, reorderIDs := newFixture("Kategori Sıralama")
	_ = reorderMenu

	writers := []struct {
		name   string
		ids    []string
		method string
		path   string
		body   any
	}{
		{"menu_delete", deleteIDs, http.MethodDelete, "/api/menus/" + deleteMenu.ID, nil},
		{"category_reorder", reorderIDs, http.MethodPut, "/api/categories/reorder",
			map[string]any{"ids": descending(reorderIDs)}},
	}

	deadlocksBefore := deadlocksSoFar(t, h)

	for _, w := range writers {
		t.Run(w.name, func(t *testing.T) {
			var request *pendingRequest
			held := holdLock(t, h, func() []*pendingRequest {
				if request == nil {
					return nil
				}
				return []*pendingRequest{request}
			}, `SELECT id FROM categories WHERE id = $1::text::uuid FOR UPDATE`, w.ids[heldIndex])

			request = h.startRequest(w.method, w.path, owner.session, w.body)
			waiter := waitForWaiterBlockedBy(t, h, w.name, held.pid)
			t.Logf("%s is blocked by the held category while running: %s", w.name, waiter.query)

			var lockedEarly, freeEarly []string
			for i, id := range w.ids {
				if i == heldIndex {
					continue
				}
				locked := categoryIsLocked(t, h, id)
				switch {
				case i < heldIndex && !locked:
					freeEarly = append(freeEarly, fmt.Sprintf("#%d %s", i, id))
				case i > heldIndex && locked:
					lockedEarly = append(lockedEarly, fmt.Sprintf("#%d %s", i, id))
				}
			}

			if err := held.tx.Rollback(context.Background()); err != nil {
				t.Fatalf("could not release the held category: %v", err)
			}
			status, body := request.wait(t, w.name)
			if status != http.StatusOK {
				t.Errorf("%s answered %d once the held category was released, want 200: %s", w.name, status, body)
			}

			if len(lockedEarly) > 0 || len(freeEarly) > 0 {
				t.Errorf("%s does not lock its categories in ascending id order. While it waited for category "+
					"#%d it already held larger ids %v and had not yet locked smaller ids %v (ids in order: %v)",
					w.name, heldIndex, lockedEarly, freeEarly, w.ids)
			}
		})
	}

	if deadlocks := deadlocksSoFar(t, h) - deadlocksBefore; deadlocks != 0 {
		t.Errorf("PostgreSQL detected %d deadlock(s) while the writers ran one at a time", deadlocks)
	}
}
