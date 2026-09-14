package tests

// A write into a menu or a category that is deleted while the write runs.
//
// The handlers check ownership first, and that check sees the parent row. Here
// the delete is a statement of the test's own, which takes none of the advisory
// locks of repository/locks.go, and it lands after that check, before the
// write's foreign key check: that check waits for the delete, finds the parent
// gone and fails with SQLSTATE 23503. The record the write named no longer
// exists, so the answer has to be a 404 rather than a 500.
//
// Each case is staged the same deterministic way: a transaction of the test's
// own deletes the target menu and stays open, the request starts and is seen
// waiting on that transaction, and only then does the transaction commit.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestWriteIntoAParentDeletedMidRequestAnswers404(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Yarış Kafe", "foreign-key-race-owner@example.test")

	type fixture struct {
		source         menuPayload
		sourceCategory categoryPayload
		product        productPayload
		target         menuPayload
		targetCategory categoryPayload
	}
	n := 0
	newFixture := func() fixture {
		n++
		source := h.createMenu(owner, "Kaynak Menü "+string(rune('A'+n)))
		sourceCategory := h.createCategory(owner, source.ID, "Kaynak Kategori")
		product := h.createProduct(owner, sourceCategory.ID, "Kalan Ürün", 100)
		target := h.createMenu(owner, "Silinen Menü "+string(rune('A'+n)))
		targetCategory := h.createCategory(owner, target.ID, "Silinen Kategori")
		return fixture{source, sourceCategory, product, target, targetCategory}
	}

	productUntouched := func(t *testing.T, f fixture) {
		t.Helper()
		exists, category, price := productRow(t, h, f.product.ID)
		if !exists || category != f.sourceCategory.ID || price != 100 {
			t.Errorf("the refused request still changed the product: exists=%t, category %q (want %s), price %.2f",
				exists, category, f.sourceCategory.ID, price)
		}
	}

	cases := []struct {
		name    string
		method  string
		path    func(f fixture) string
		body    func(f fixture) any
		message string
		check   func(t *testing.T, f fixture)
	}{
		{
			name:    "move_a_product_into_a_category_of_the_deleted_menu",
			method:  http.MethodPut,
			path:    func(f fixture) string { return "/api/products/" + f.product.ID },
			body:    func(f fixture) any { return map[string]any{"category_id": f.targetCategory.ID} },
			message: "Kategori bulunamadı.",
			check:   productUntouched,
		},
		{
			name:    "move_a_product_with_a_new_price_into_a_category_of_the_deleted_menu",
			method:  http.MethodPut,
			path:    func(f fixture) string { return "/api/products/" + f.product.ID },
			body:    func(f fixture) any { return map[string]any{"category_id": f.targetCategory.ID, "price": 175} },
			message: "Kategori bulunamadı.",
			check:   productUntouched,
		},
		{
			name:   "reorder_a_product_into_a_category_of_the_deleted_menu",
			method: http.MethodPut,
			path:   func(f fixture) string { return "/api/products/reorder" },
			body: func(f fixture) any {
				return map[string]any{"category_id": f.targetCategory.ID, "ids": []string{f.product.ID}}
			},
			message: "Kategori bulunamadı.",
			check:   productUntouched,
		},
		{
			name:   "create_a_product_in_a_category_of_the_deleted_menu",
			method: http.MethodPost,
			path:   func(f fixture) string { return "/api/products" },
			body: func(f fixture) any {
				return map[string]any{
					"category_id":  f.targetCategory.ID,
					"translations": map[string]any{"tr": map[string]any{"name": "Yetim Ürün"}},
					"price":        20,
				}
			},
			message: "Kategori bulunamadı.",
		},
		{
			name:   "create_a_category_in_the_deleted_menu",
			method: http.MethodPost,
			path:   func(f fixture) string { return "/api/categories" },
			body: func(f fixture) any {
				return map[string]any{
					"menu_id":      f.target.ID,
					"translations": map[string]any{"tr": map[string]any{"name": "Yetim Kategori"}},
				}
			},
			// The row that vanished is the menu the category was to be added to.
			message: "Menü bulunamadı.",
		},
		{
			name:   "move_a_category_into_the_deleted_menu",
			method: http.MethodPut,
			path:   func(f fixture) string { return "/api/categories/" + f.sourceCategory.ID },
			body:   func(f fixture) any { return map[string]any{"menu_id": f.target.ID} },
			// The row that vanished is the menu the category was moved to.
			message: "Menü bulunamadı.",
			check: func(t *testing.T, f fixture) {
				t.Helper()
				categories := h.listCategoriesAsOwner(t, "after the refused category move", owner, f.source.ID)
				moved := findCategory(categories, f.sourceCategory.ID)
				if moved == nil || moved.MenuID == nil || *moved.MenuID != f.source.ID {
					t.Errorf("the refused move still changed category %s: %+v", f.sourceCategory.ID, moved)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			ctx := context.Background()

			var request *pendingRequest
			held := holdLock(t, h, func() []*pendingRequest {
				if request == nil {
					return nil
				}
				return []*pendingRequest{request}
			}, `DELETE FROM menus WHERE id = $1::text::uuid`, f.target.ID)

			request = h.startRequest(tc.method, tc.path(f), owner.session, tc.body(f))
			waiter := waitForWaiterBlockedBy(t, h, tc.name, held.pid)
			t.Logf("the request waits on the uncommitted delete while running: %s", waiter.query)

			if err := held.tx.Commit(ctx); err != nil {
				t.Fatalf("could not commit the menu delete: %v", err)
			}

			status, body := request.wait(t, tc.name)
			if status != http.StatusNotFound {
				t.Fatalf("%s %s answered %d once the parent was deleted under it, want 404: %s",
					tc.method, tc.path(f), status, body)
			}
			var refusal struct {
				Error string `json:"error"`
				Code  string `json:"code"`
			}
			if err := json.Unmarshal(body, &refusal); err != nil {
				t.Fatalf("could not decode the 404 body %s: %v", body, err)
			}
			if refusal.Error != tc.message || refusal.Code != "NOT_FOUND" {
				t.Errorf("the 404 says %q (%s), want %q (NOT_FOUND)", refusal.Error, refusal.Code, tc.message)
			}

			if rowExists(t, h, "menus", f.target.ID) {
				t.Fatalf("fixture: the target menu survived its committed delete")
			}
			if tc.check != nil {
				tc.check(t, f)
			}
		})
	}
}
