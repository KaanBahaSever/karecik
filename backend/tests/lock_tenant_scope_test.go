package tests

// The locking statements of the multi-row writers — the SELECT ... FOR UPDATE
// of DeleteMenu, DeleteCategory, ReorderProducts and the bulk price update, and
// the SELECT ... FOR NO KEY UPDATE of ReorderCategories — are scoped to the
// business of the session, like every other query. A lock query that lost its
// business predicate would still refuse the foreign write afterwards, so tenant
// isolation tests would stay green; what it would do is lock, or wait on,
// another tenant's rows first. This suite catches exactly that: tenant A holds
// one of its own rows, and tenant B's requests naming A's rows must come back
// at once.
//
// The advisory locks are scoped the same way. The repository checks that the
// business owns a menu or a category before it takes that record's lock, so a
// request of tenant B naming A's record never waits for A's delete of it. And
// the key of a lock is the record's own: while A's delete holds the lock of A's
// record, B's deletes of, writes into and creates in B's own records answer at
// once — a record of B whose id equals the id of A's record, in the table of
// the other kind of parent, included.

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// tenantScopeAnswerWithin is how long B's request may take. Nothing it does
// needs a lock A holds, so it answers in milliseconds; a request that waits on
// A's row never answers before A lets go.
const tenantScopeAnswerWithin = 2 * time.Second

// categoryPosition reads a category's position straight from the table.
func categoryPosition(t *testing.T, h *harness, id string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var position int
	if err := h.pool.QueryRow(ctx, `SELECT position FROM categories WHERE id = $1::text::uuid`, id).Scan(&position); err != nil {
		t.Fatalf("could not read the position of category %s: %v", id, err)
	}
	return position
}

func TestLockingWritersNeverWaitOnAnotherTenantsRows(t *testing.T) {
	h := newHarness(t)
	tenantA := h.register("A", "Kilit A", "lock-scope-a@example.test")
	tenantB := h.register("B", "Kilit B", "lock-scope-b@example.test")

	menuA := h.createMenu(tenantA, "A Menüsü")
	categoryA := h.createCategory(tenantA, menuA.ID, "A Kategorisi")
	productA := h.createProduct(tenantA, categoryA.ID, "A Ürünü", fixturePrice)
	menuB := h.createMenu(tenantB, "B Menüsü")
	categoryB := h.createCategory(tenantB, menuB.ID, "B Kategorisi")

	// The reorder requests list one of B's own records as well as A's, so B's
	// request always has something of its own to lock and write, and gets as
	// far as its locking statement. The bulk price update names a category of
	// B's next to A's for the same reason.
	productB := h.createProduct(tenantB, categoryB.ID, "B Ürünü", fixturePrice)
	reorderIDs := []string{productB.ID, productA.ID}
	positionA := categoryPosition(t, h, categoryA.ID)

	holdProductA := `SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`
	holdCategoryA := `SELECT id FROM categories WHERE id = $1::text::uuid FOR UPDATE`

	cases := []struct {
		name   string
		hold   string
		holdID string
		method string
		path   string
		body   any
		// clientError: the request names A's record itself, so it must be a
		// 4xx. The other cases name A's record only inside a list of B's own,
		// which the endpoint answers with B's list.
		clientError bool
	}{
		{"delete_a_menu", holdProductA, productA.ID, http.MethodDelete, "/api/menus/" + menuA.ID, nil, true},
		{"delete_a_category", holdProductA, productA.ID, http.MethodDelete, "/api/categories/" + categoryA.ID, nil, true},
		{"reorder_into_a_category_of_a", holdProductA, productA.ID, http.MethodPut, "/api/products/reorder",
			map[string]any{"category_id": categoryA.ID, "ids": reorderIDs}, true},
		{"reorder_into_a_category_of_b", holdProductA, productA.ID, http.MethodPut, "/api/products/reorder",
			map[string]any{"category_id": categoryB.ID, "ids": reorderIDs}, false},
		{"reorder_categories_listing_a_category_of_a", holdCategoryA, categoryA.ID, http.MethodPut,
			"/api/categories/reorder", map[string]any{"ids": []string{categoryB.ID, categoryA.ID}}, false},
		{"bulk_price_apply_on_a_menu_of_a", holdProductA, productA.ID, http.MethodPost, "/api/products/bulk-price",
			map[string]any{"menu_id": menuA.ID, "percentage": 10, "rounding": "none", "apply": true}, true},
		{"bulk_price_apply_listing_a_category_of_a", holdProductA, productA.ID, http.MethodPost,
			"/api/products/bulk-price", map[string]any{
				"menu_id": menuB.ID, "category_ids": []string{categoryA.ID, categoryB.ID},
				"percentage": 10, "rounding": "none", "apply": true,
			}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var request *pendingRequest
			held := holdLock(t, h, func() []*pendingRequest {
				if request == nil {
					return nil
				}
				return []*pendingRequest{request}
			}, tc.hold, tc.holdID)

			request = h.startRequest(tc.method, tc.path, tenantB.session, tc.body)
			select {
			case <-request.done:
			case <-time.After(tenantScopeAnswerWithin):
				t.Errorf("tenant B's %s %s did not answer within %s while tenant A held its own row: "+
					"its locking query is not scoped to B's business", tc.method, tc.path, tenantScopeAnswerWithin)
			}

			if err := held.tx.Rollback(context.Background()); err != nil {
				t.Fatalf("could not release tenant A's row: %v", err)
			}
			status, body := request.wait(t, tc.name)

			switch {
			case status >= 500:
				t.Errorf("tenant B's %s %s answered %d: %s", tc.method, tc.path, status, body)
			case tc.clientError && (status < 400 || status >= 500):
				t.Errorf("tenant B's %s %s answered %d, want a 4xx: %s", tc.method, tc.path, status, body)
			default:
				t.Logf("tenant B's %s %s answered %d", tc.method, tc.path, status)
			}
		})
	}

	// None of it may have touched A's rows.
	h.readMenuAsOwner(t, "after tenant B's requests", tenantA, menuA.ID)
	products := h.listProductsAsOwner(t, "after tenant B's requests", tenantA, categoryA.ID)
	if product := findProduct(products, productA.ID); product == nil || product.CategoryID != categoryA.ID ||
		!samePrice(product.Price, fixturePrice) {
		t.Errorf("tenant A's product %s moved, changed price or disappeared: %+v", productA.ID, product)
	}
	if position := categoryPosition(t, h, categoryA.ID); position != positionA {
		t.Errorf("tenant A's category %s moved from position %d to %d", categoryA.ID, positionA, position)
	}
}

// TestAdvisoryLocksNeverMakeAnotherTenantWait: tenant A deletes its own menu,
// or its own category, and the delete holds the record's advisory lock while
// it waits on a product row the test holds.
//
// Tenant B's delete of the same id must answer at once, with the 404 of a
// record B does not have. So must every write of B into B's own records while
// A's delete holds that lock — each of them takes advisory locks keyed on B's
// records, which share no key with A's: a save naming a category, a product
// create, a category create, a delete of B's record whose id is the id of A's
// record in the table of the other kind of parent, and deletes of B's own
// category and menu.
func TestAdvisoryLocksNeverMakeAnotherTenantWait(t *testing.T) {
	h := newHarness(t)
	tenantA := h.register("A", "Danışma Kilidi A", "advisory-scope-a@example.test")
	tenantB := h.register("B", "Danışma Kilidi B", "advisory-scope-b@example.test")

	for _, kind := range []string{"menu", "category"} {
		t.Run("delete_a_"+kind, func(t *testing.T) {
			menu := h.createMenu(tenantA, "A Silinen "+kind)
			category := h.createCategory(tenantA, menu.ID, "A Silinen Kategori")
			product := h.createProduct(tenantA, category.ID, "A Ürünü", fixturePrice)
			path := "/api/menus/" + menu.ID
			if kind == "category" {
				path = "/api/categories/" + category.ID
			}

			// B's own records, written before A's delete holds its lock.
			ownMenu := h.createMenu(tenantB, "B Kendi "+kind)
			ownCategory := h.createCategory(tenantB, ownMenu.ID, "B Kendi Kategori")
			ownProduct := h.createProduct(tenantB, ownCategory.ID, "B Kendi Ürün", fixturePrice)
			spareMenu := h.createMenu(tenantB, "B Yedek "+kind)
			var twinPath string
			if kind == "menu" {
				insertCategoryWithID(t, h, menu.ID, spareMenu.ID, "B İkiz Kategori", 0)
				twinPath = "/api/categories/" + menu.ID
			} else {
				insertMenuWithID(t, h, category.ID, tenantB.businessID, "B İkiz Menü", "b-ikiz-menu")
				twinPath = "/api/menus/" + category.ID
			}

			var pending []*pendingRequest
			held := holdProduct(t, h, product.ID, func() []*pendingRequest { return pending })

			own := h.startRequest(http.MethodDelete, path, tenantA.session, nil)
			pending = append(pending, own)
			waiter := waitForWaiterBlockedBy(t, h, "tenant A's delete", held.pid)
			t.Logf("tenant A's delete (backend %d) holds the advisory lock and waits on the held product: %s",
				waiter.pid, waiter.query)

			foreign := h.startRequest(http.MethodDelete, path, tenantB.session, nil)
			pending = append(pending, foreign)
			select {
			case <-foreign.done:
			case <-time.After(tenantScopeAnswerWithin):
				t.Errorf("tenant B's DELETE %s did not answer within %s while tenant A's delete of it held the "+
					"advisory lock: the lock is taken before the business is checked", path, tenantScopeAnswerWithin)
			}

			for _, write := range []struct {
				name, method, path string
				body               any
				status             int
			}{
				{"a save of B's product naming B's category", http.MethodPut, "/api/products/" + ownProduct.ID,
					map[string]any{"category_id": ownCategory.ID}, http.StatusOK},
				{"a product create in B's category", http.MethodPost, "/api/products", map[string]any{
					"category_id":  ownCategory.ID,
					"translations": map[string]any{"tr": map[string]any{"name": "B Yeni Ürün"}},
					"price":        20,
				}, http.StatusCreated},
				{"a category create in B's menu", http.MethodPost, "/api/categories", map[string]any{
					"menu_id":      ownMenu.ID,
					"translations": map[string]any{"tr": map[string]any{"name": "B Yeni Kategori"}},
				}, http.StatusCreated},
				{"a delete of B's record that has the id of A's", http.MethodDelete, twinPath, nil, http.StatusOK},
				{"a delete of B's own category", http.MethodDelete, "/api/categories/" + ownCategory.ID, nil, http.StatusOK},
				{"a delete of B's own menu", http.MethodDelete, "/api/menus/" + ownMenu.ID, nil, http.StatusOK},
			} {
				request := h.startRequest(write.method, write.path, tenantB.session, write.body)
				pending = append(pending, request)
				select {
				case <-request.done:
					if request.err != nil || request.status != write.status {
						t.Errorf("%s answered %d %s (%v), want %d", write.name, request.status,
							shorten(request.body), request.err, write.status)
					}
				case <-time.After(tenantScopeAnswerWithin):
					t.Errorf("%s did not answer within %s while tenant A's delete held the advisory lock of A's %s: "+
						"a lock of B's record shares its key", write.name, tenantScopeAnswerWithin, kind)
				}
			}

			if err := held.tx.Rollback(context.Background()); err != nil {
				t.Fatalf("could not release the held product: %v", err)
			}
			if status, body := foreign.wait(t, "tenant B's delete"); status != http.StatusNotFound {
				t.Errorf("tenant B's DELETE %s answered %d, want 404: %s", path, status, body)
			}
			if status, body := own.wait(t, "tenant A's delete"); status != http.StatusOK {
				t.Errorf("tenant A's DELETE %s answered %d, want 200: %s", path, status, body)
			}
			for _, request := range pending {
				request.wait(t, "a request of tenant B")
			}
		})
	}
}
