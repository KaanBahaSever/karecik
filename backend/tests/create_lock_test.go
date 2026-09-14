package tests

// Creates wait for a delete of the parent they create into.
//
// A create reaches its parent only through the foreign key check of its INSERT,
// and the KEY SHARE lock that check takes conflicts with nothing a delete holds
// before its DELETE. A create that did not wait would therefore land in the
// middle of a delete, and the record it created would be one the delete never
// locked: a price edit, a reorder or a bulk price update of that record could
// then close a lock cycle with the delete's cascade, and a category delete
// would remove a product it did not count. So every create takes the shared
// advisory lock of its parent first — a category create the lock of its menu,
// a product create the lock of its category's menu and then of the category
// (see repository/locks.go).
//
// Each test stages a delete mid-way with locks of the test's own, starts a
// create into the parent and requires it to wait for the delete, starts the
// writer that would close the cycle, and then releases everything. PostgreSQL
// has to count no deadlock, no retry may be logged, no request may answer 5xx,
// the create has to answer 404 and the rows have to agree with the answers.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// countProductsNamed counts the products whose Turkish name is name.
func countProductsNamed(t *testing.T, h *harness, name string) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM products WHERE translations->'tr'->>'name' = $1`, name).Scan(&n); err != nil {
		t.Fatalf("could not count the products named %q: %v", name, err)
	}
	return n
}

// countCategoriesNamed is countProductsNamed for categories.
func countCategoriesNamed(t *testing.T, h *harness, name string) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM categories WHERE translations->'tr'->>'name' = $1`, name).Scan(&n); err != nil {
		t.Fatalf("could not count the categories named %q: %v", name, err)
	}
	return n
}

// TestProductCreatedInAMenuBeingDeletedWaitsForTheDelete: while menu M is being
// deleted, a product is created in a category of M between the delete's product
// and category lock steps, and then a bulk price update of M and a price edit of
// M's product start.
//
// Two connections of the test's own hold a KEY SHARE lock on M's category with
// the smaller id and M's row. The delete locks M's product and waits at that
// category; the create and the bulk price update start there. The category is
// released, the delete moves on to wait at M's row, the price edit starts, and
// M's row is released last. A product created while the delete waited at the
// category would be one the cascade needs but the delete never locked, held by
// the bulk update while that update waited for the product the delete holds.
func TestProductCreatedInAMenuBeingDeletedWaitsForTheDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Oluşturma D Kafe", "create-lock-d-owner@example.test")

	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()
			menu := h.createMenu(owner, fmt.Sprintf("Silinen D%d", run))
			low := insertCategoryWithID(t, h, fixtureUUID(0x0d000000, uint64(10*run+1)), menu.ID, "Küçük", 0)
			high := insertCategoryWithID(t, h, fixtureUUID(0xfd000000, uint64(10*run+2)), menu.ID, "Büyük", 1)
			resident := h.createProduct(owner, low.ID, "Mevcut Ürün", 10)
			pinMenuPriceDate(t, h, menu.ID)
			createdName := fmt.Sprintf("Yeni D%d", run)

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			pend := func() []*pendingRequest { return pending }
			keyShare := holdLockOnOwnConnection(t, h, pend,
				`SELECT id FROM categories WHERE id = $1::text::uuid FOR KEY SHARE`, low.ID)
			menuRow := holdLockOnOwnConnection(t, h, pend,
				`SELECT id FROM menus WHERE id = $1::text::uuid FOR NO KEY UPDATE`, menu.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id at its category step", keyShare.pid)
			t.Logf("the delete (backend %d) waits on the held category while running: %s", deleter.pid, deleter.query)

			createRequest := h.startRequest(http.MethodPost, "/api/products", owner.session, map[string]any{
				"category_id":  high.ID,
				"translations": map[string]any{"tr": map[string]any{"name": createdName}},
				"price":        30,
			})
			pending = append(pending, createRequest)
			known := []int{deleter.pid}
			if creator := requireWaitsOn(t, h, "the product create", createRequest, deleter.pid, known...); creator != 0 {
				known = append(known, creator)
			}

			bulkRequest := h.startRequest(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
				"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": true,
			})
			pending = append(pending, bulkRequest)
			if bulk := requireWaitsOn(t, h, "the bulk price update", bulkRequest, deleter.pid, known...); bulk != 0 {
				known = append(known, bulk)
			}

			if err := keyShare.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held category: %v", err)
			}
			atMenuRow := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id at the menu row", menuRow.pid)
			t.Logf("backend %d now waits on the held menu row while running: %s", atMenuRow.pid, atMenuRow.query)

			editRequest := h.startRequest(http.MethodPatch, "/api/products/"+resident.ID+"/price", owner.session,
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

			createStatus, createBody := createRequest.wait(t, "POST /api/products")
			bulkStatus, bulkBody := bulkRequest.wait(t, "POST /api/products/bulk-price")
			editStatus, editBody := editRequest.wait(t, "PATCH /api/products/:id/price")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			t.Logf("create %d %s | bulk %d %s | price edit %d %s | delete %d %s", createStatus, shorten(createBody),
				bulkStatus, shorten(bulkBody), editStatus, shorten(editBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"POST /api/products": createStatus, "POST /api/products/bulk-price": bulkStatus,
				"PATCH /api/products/:id/price": editStatus, "DELETE /api/menus/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			requireNotFound(t, "the product create", createStatus, createBody, "Kategori bulunamadı.")
			if n := countProductsNamed(t, h, createdName); n != 0 {
				t.Errorf("the create answered %d, but %d product(s) named %q exist", createStatus, n, createdName)
			}
			if rowExists(t, h, "menus", menu.ID) || rowExists(t, h, "categories", low.ID) ||
				rowExists(t, h, "categories", high.ID) || rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the menu, one of its categories or its product is still there",
					deleteStatus)
			}
			if bulkStatus != http.StatusOK && bulkStatus != http.StatusNotFound {
				t.Errorf("POST /api/products/bulk-price answered %d, want 200 or 404: %s", bulkStatus, bulkBody)
			}
			if editStatus != http.StatusOK && editStatus != http.StatusNotFound {
				t.Errorf("PATCH /api/products/:id/price answered %d, want 200 or 404: %s", editStatus, editBody)
			}
		})
	}
}

// TestProductCreatedInACategoryBeingDeletedWaitsForTheDelete: while category D
// is being deleted, a product is created in D, and then a writer that locks D's
// product together with a product of another category starts — a reorder into
// that category, or a bulk price update of the menu.
//
// A connection of the test's own holds a KEY SHARE lock on D, which keeps the
// delete waiting at its DELETE after it has locked D's product. A product
// created in D at that point would be removed by the cascade without having
// been locked or counted. The answer of the delete has to count exactly the
// products it removed.
func TestProductCreatedInACategoryBeingDeletedWaitsForTheDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Oluşturma E Kafe", "create-lock-e-owner@example.test")

	for i, variant := range []string{"reorder_into_another_category", "bulk_price_update"} {
		t.Run(variant, func(t *testing.T) {
			ctx := context.Background()
			menu := h.createMenu(owner, "Menü E "+variant)
			doomed := h.createCategory(owner, menu.ID, "Silinecek")
			other := h.createCategory(owner, menu.ID, "Diğer")
			resident := insertProductWithID(t, h, fixtureUUID(0xee000000, uint64(10*i+1)), doomed.ID, "Mevcut", 10, 0)
			neighbour := insertProductWithID(t, h, fixtureUUID(0x0e000000, uint64(10*i+2)), other.ID, "Komşu", 20, 0)
			createdName := "Yeni E " + variant

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			keyShare := holdLockOnOwnConnection(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM categories WHERE id = $1::text::uuid FOR KEY SHARE`, doomed.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/categories/"+doomed.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/categories/:id", keyShare.pid)
			t.Logf("the delete (backend %d) waits on the held category while running: %s", deleter.pid, deleter.query)

			createRequest := h.startRequest(http.MethodPost, "/api/products", owner.session, map[string]any{
				"category_id":  doomed.ID,
				"translations": map[string]any{"tr": map[string]any{"name": createdName}},
				"price":        30,
			})
			pending = append(pending, createRequest)
			known := []int{deleter.pid}
			if creator := requireWaitsOn(t, h, "the product create", createRequest, deleter.pid, known...); creator != 0 {
				known = append(known, creator)
			}

			var third *pendingRequest
			if variant == "bulk_price_update" {
				third = h.startRequest(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
					"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": true,
				})
			} else {
				third = h.startRequest(http.MethodPut, "/api/products/reorder", owner.session, map[string]any{
					"category_id": other.ID, "ids": []string{neighbour.ID, resident.ID},
				})
			}
			pending = append(pending, third)
			requireWaitsOn(t, h, variant, third, deleter.pid, known...)

			if err := keyShare.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held category: %v", err)
			}

			createStatus, createBody := createRequest.wait(t, "POST /api/products")
			thirdStatus, thirdBody := third.wait(t, variant)
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/categories/:id")
			t.Logf("create %d %s | %s %d %s | delete %d %s", createStatus, shorten(createBody), variant,
				thirdStatus, shorten(thirdBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"POST /api/products": createStatus, variant: thirdStatus, "DELETE /api/categories/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Fatalf("DELETE /api/categories/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			requireNotFound(t, "the product create", createStatus, createBody, "Kategori bulunamadı.")
			if thirdStatus != http.StatusOK {
				t.Errorf("%s answered %d, want 200: %s", variant, thirdStatus, thirdBody)
			}

			created := countProductsNamed(t, h, createdName)
			if created != 0 {
				t.Errorf("the create answered %d, but %d product(s) named %q exist", createStatus, created, createdName)
			}
			if rowExists(t, h, "categories", doomed.ID) || rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered 200 but the category or its product is still there")
			}
			if exists, inCategory, _ := productRow(t, h, neighbour.ID); !exists || inCategory != other.ID {
				t.Errorf("the product of the other category is gone or moved: exists=%t in %q", exists, inCategory)
			}

			var result struct {
				DeletedProducts int `json:"deleted_products"`
			}
			decodeInto(t, "DELETE /api/categories/:id", deleteBody, &result)
			if result.DeletedProducts != 1 {
				t.Errorf("DELETE /api/categories/:id reports deleted_products %d, but it removed exactly 1 product",
					result.DeletedProducts)
			}
		})
	}
}

// TestCategoryCreatedInAMenuBeingDeletedWaitsForTheDelete: while menu M is
// being deleted, a category is created in M, and then a category reorder and a
// price edit that need M's category and product start.
//
// A connection of the test's own holds a KEY SHARE lock on M's row, which keeps
// the delete waiting at its DELETE after it has locked M's product and
// category. A category created in M at that point would be one the cascade
// needs but the delete never locked.
func TestCategoryCreatedInAMenuBeingDeletedWaitsForTheDelete(t *testing.T) {
	h := newHarness(t)
	logs := captureStandardLog(t)
	owner := h.register("owner", "Oluşturma F Kafe", "create-lock-f-owner@example.test")

	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("run_%d", run), func(t *testing.T) {
			ctx := context.Background()
			menu := h.createMenu(owner, fmt.Sprintf("Silinen F%d", run))
			old := h.createCategory(owner, menu.ID, "Eski")
			resident := h.createProduct(owner, old.ID, "Mevcut Ürün", 10)
			other := h.createMenu(owner, fmt.Sprintf("Diğer F%d", run))
			otherCategory := h.createCategory(owner, other.ID, "Diğer Kategori")
			pinMenuPriceDate(t, h, menu.ID)
			createdName := fmt.Sprintf("Yeni F%d", run)

			deadlocksBefore := deadlocksSoFar(t, h)
			logs.reset()

			var pending []*pendingRequest
			keyShare := holdLockOnOwnConnection(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM menus WHERE id = $1::text::uuid FOR KEY SHARE`, menu.ID)

			deleteRequest := h.startRequest(http.MethodDelete, "/api/menus/"+menu.ID, owner.session, nil)
			pending = append(pending, deleteRequest)
			deleter := waitForWaiterBlockedBy(t, h, "DELETE /api/menus/:id at the menu row", keyShare.pid)
			t.Logf("the delete (backend %d) waits on the held menu row while running: %s", deleter.pid, deleter.query)

			createRequest := h.startRequest(http.MethodPost, "/api/categories", owner.session, map[string]any{
				"menu_id":      menu.ID,
				"translations": map[string]any{"tr": map[string]any{"name": createdName}},
			})
			pending = append(pending, createRequest)
			known := []int{deleter.pid}
			if creator := requireWaitsOn(t, h, "the category create", createRequest, deleter.pid, known...); creator != 0 {
				known = append(known, creator)
			}

			reorderRequest := h.startRequest(http.MethodPut, "/api/categories/reorder", owner.session,
				map[string]any{"ids": []string{otherCategory.ID, old.ID}})
			pending = append(pending, reorderRequest)
			if reorderer := requireWaitsOn(t, h, "the category reorder", reorderRequest, deleter.pid, known...); reorderer != 0 {
				known = append(known, reorderer)
			}

			editRequest := h.startRequest(http.MethodPatch, "/api/products/"+resident.ID+"/price", owner.session,
				map[string]any{"price": 150})
			pending = append(pending, editRequest)
			requireWaitsOn(t, h, "the price edit", editRequest, deleter.pid, known...)

			if err := keyShare.tx.Commit(ctx); err != nil {
				t.Fatalf("could not release the held menu row: %v", err)
			}

			createStatus, createBody := createRequest.wait(t, "POST /api/categories")
			reorderStatus, reorderBody := reorderRequest.wait(t, "PUT /api/categories/reorder")
			editStatus, editBody := editRequest.wait(t, "PATCH /api/products/:id/price")
			deleteStatus, deleteBody := deleteRequest.wait(t, "DELETE /api/menus/:id")
			t.Logf("create %d %s | reorder %d %s | price edit %d %s | delete %d %s", createStatus, shorten(createBody),
				reorderStatus, shorten(reorderBody), editStatus, shorten(editBody), deleteStatus, shorten(deleteBody))

			requireNoConflicts(t, h, logs, deadlocksBefore, map[string]int{
				"POST /api/categories": createStatus, "PUT /api/categories/reorder": reorderStatus,
				"PATCH /api/products/:id/price": editStatus, "DELETE /api/menus/:id": deleteStatus,
			})
			if deleteStatus != http.StatusOK {
				t.Errorf("DELETE /api/menus/:id answered %d, want 200: %s", deleteStatus, deleteBody)
			}
			requireNotFound(t, "the category create", createStatus, createBody, "Menü bulunamadı.")
			if n := countCategoriesNamed(t, h, createdName); n != 0 {
				t.Errorf("the create answered %d, but %d categor(ies) named %q exist", createStatus, n, createdName)
			}
			if rowExists(t, h, "menus", menu.ID) || rowExists(t, h, "categories", old.ID) ||
				rowExists(t, h, "products", resident.ID) {
				t.Errorf("the delete answered %d but the menu, its category or its product is still there", deleteStatus)
			}
			if !rowExists(t, h, "categories", otherCategory.ID) {
				t.Errorf("the category of the other menu disappeared")
			}
			if reorderStatus != http.StatusOK {
				t.Errorf("PUT /api/categories/reorder answered %d, want 200: %s", reorderStatus, reorderBody)
			}
			requireNotFound(t, "the price edit of the deleted menu's product", editStatus, editBody, "Ürün bulunamadı.")
		})
	}
}
