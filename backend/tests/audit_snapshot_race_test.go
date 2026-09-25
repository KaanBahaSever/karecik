package tests

// The "before" half of an audit row is the row the write actually replaced.
//
// Every case below stages the same race, the way product_lock_order_test.go
// stages its own: a transaction of the test's own makes a competing write to
// the record and keeps its row lock; the audited request is sent; the test
// waits until PostgreSQL itself reports the request blocked by that
// transaction, and only then commits it. The request's write therefore lands
// right after the competing one, every time.
//
// When the old copy of a record was read before the write — outside its
// transaction and without a lock — the competing write's changes showed up in
// the request's diff as if the request had made them, the old values were the
// ones from before the competing write, and a request that put back a value
// the competing write had moved recorded nothing at all. Read inside the
// write's transaction under the write's own row lock, the old copy is exactly
// what the request replaced, and the diff is exactly what it changed.

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"testing"
)

// raceBehind makes the competing write hold (with args), sends the request,
// waits until the request is blocked by the competing write, commits it, and
// returns the request's answer.
func raceBehind(t *testing.T, h *harness, owner *tenant, method, path string, body any,
	hold string, args ...any) (int, []byte) {

	t.Helper()
	var requests []*pendingRequest
	held := holdLock(t, h, func() []*pendingRequest { return requests }, hold, args...)
	request := h.startRequest(method, path, owner.session, body)
	requests = append(requests, request)

	what := method + " " + path
	waitForWaiterBlockedBy(t, h, what, held.pid)
	if err := held.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not commit the competing write: %v", err)
	}
	return request.wait(t, what)
}

// auditRowsOf reads the owner's audit rows of one action, newest first.
func auditRowsOf(t *testing.T, h *harness, owner *tenant, action string) []auditRow {
	t.Helper()
	resp, payload := h.do(http.MethodGet, "/api/audit-logs?limit=200&action="+action, owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/audit-logs?action=%s answered %d: %s", action, resp.StatusCode, payload)
	}
	var page auditPage
	decodeInto(t, "audit logs", payload, &page)
	return page.Items
}

// latestFor returns the newest row of an action about one record.
func latestFor(t *testing.T, h *harness, owner *tenant, action, entityID string) auditRow {
	t.Helper()
	for _, row := range auditRowsOf(t, h, owner, action) {
		if row.EntityID != nil && *row.EntityID == entityID {
			return row
		}
	}
	t.Fatalf("no %s row for %s", action, entityID)
	return auditRow{}
}

// changedFields lists the fields of a row, sorted.
func changedFields(row auditRow) []string {
	fields := make([]string, 0, len(row.Changes))
	for field := range row.Changes {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

// requireOnly asserts that a row records exactly one field, from old to new,
// both given as their JSON spelling.
func requireOnly(t *testing.T, row auditRow, field, old, new string) {
	t.Helper()
	change, ok := row.Changes[field]
	if !ok || len(row.Changes) != 1 {
		t.Errorf("%s recorded %v, want %s alone", row.Action, changedFields(row), field)
		return
	}
	if string(change.Old) != old || string(change.New) != new {
		t.Errorf("%s recorded %s %s -> %s, want %s -> %s", row.Action, field, change.Old, change.New, old, new)
	}
}

func jsonString(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestAuditBeforeCopyIsTheRowTheWriteReplaced(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Yaris Kafe", "snapshot-owner@example.test")
	menu := h.createMenu(owner, "Ana Menu")
	category := h.createCategory(owner, menu.ID, "Kahveler")

	renamedProduct := turkishName(t, "Latte Grande")

	t.Run("a_price_edit_behind_a_concurrent_edit_records_its_own_change_only", func(t *testing.T) {
		product := h.createProduct(owner, category.ID, "Latte", 10)
		status, body := raceBehind(t, h, owner, http.MethodPatch, "/api/products/"+product.ID+"/price",
			map[string]any{"price": 20},
			`UPDATE products SET price = 15, translations = $2::jsonb WHERE id = $1::text::uuid`,
			product.ID, renamedProduct)
		if status != http.StatusOK {
			t.Fatalf("the price edit answered %d: %s", status, body)
		}
		// Not 10 -> 20, and no rename: the other write made that one.
		requireOnly(t, latestFor(t, h, owner, "product.price", product.ID), "price", "15", "20")
	})

	t.Run("a_price_edit_that_undoes_a_concurrent_edit_is_still_recorded", func(t *testing.T) {
		product := h.createProduct(owner, category.ID, "Mocha", 10)
		status, body := raceBehind(t, h, owner, http.MethodPatch, "/api/products/"+product.ID+"/price",
			map[string]any{"price": 10},
			`UPDATE products SET price = 15 WHERE id = $1::text::uuid`, product.ID)
		if status != http.StatusOK {
			t.Fatalf("the price edit answered %d: %s", status, body)
		}
		requireOnly(t, latestFor(t, h, owner, "product.price", product.ID), "price", "15", "10")
	})

	t.Run("a_product_save_that_moves_records_its_own_change_only", func(t *testing.T) {
		// Naming the category runs the write under the category's advisory
		// locks; the before-read comes after them.
		product := h.createProduct(owner, category.ID, "Flat White", 10)
		status, body := raceBehind(t, h, owner, http.MethodPut, "/api/products/"+product.ID,
			map[string]any{"category_id": category.ID, "price": 30},
			`UPDATE products SET price = 15, translations = $2::jsonb WHERE id = $1::text::uuid`,
			product.ID, renamedProduct)
		if status != http.StatusOK {
			t.Fatalf("the product save answered %d: %s", status, body)
		}
		requireOnly(t, latestFor(t, h, owner, "product.update", product.ID), "price", "15", "30")
	})

	t.Run("a_category_save_records_its_own_change_only", func(t *testing.T) {
		target := h.createCategory(owner, menu.ID, "Tatlilar")
		status, body := raceBehind(t, h, owner, http.MethodPut, "/api/categories/"+target.ID,
			map[string]any{"icon": "cake"},
			`UPDATE categories SET is_active = false, translations = $2::jsonb WHERE id = $1::text::uuid`,
			target.ID, turkishName(t, "Pastalar"))
		if status != http.StatusOK {
			t.Fatalf("the category save answered %d: %s", status, body)
		}
		row := latestFor(t, h, owner, "category.update", target.ID)
		requireOnly(t, row, "icon", "null", jsonString(t, "cake"))
		if row.EntityLabel != "Pastalar" {
			t.Errorf("category.update is labelled %q, want the name it has now", row.EntityLabel)
		}
	})

	t.Run("a_menu_save_records_its_own_change_only", func(t *testing.T) {
		target := h.createMenu(owner, "Aksam Menusu")
		status, body := raceBehind(t, h, owner, http.MethodPut, "/api/menus/"+target.ID,
			map[string]any{"phone": "+90 555 123 45 67"},
			`UPDATE menus SET wifi_password = 'competing-secret', description = 'Competing'
			 WHERE id = $1::text::uuid`, target.ID)
		if status != http.StatusOK {
			t.Fatalf("the menu save answered %d: %s", status, body)
		}
		requireOnly(t, latestFor(t, h, owner, "menu.update", target.ID), "phone", "null",
			jsonString(t, "+90 555 123 45 67"))
	})

	t.Run("a_business_save_records_the_name_it_replaced", func(t *testing.T) {
		status, body := raceBehind(t, h, owner, http.MethodPut, "/api/business",
			map[string]any{"name": "Yaris Kafe & Bistro"},
			`UPDATE businesses SET name = 'Yaris Kafe (competing)' WHERE id = $1::text::uuid`, owner.businessID)
		if status != http.StatusOK {
			t.Fatalf("the business save answered %d: %s", status, body)
		}
		requireOnly(t, latestFor(t, h, owner, "business.update", owner.businessID), "name",
			jsonString(t, "Yaris Kafe (competing)"), jsonString(t, "Yaris Kafe & Bistro"))
	})

	t.Run("a_product_delete_snapshots_the_product_it_removed", func(t *testing.T) {
		product := h.createProduct(owner, category.ID, "Americano", 10)
		status, body := raceBehind(t, h, owner, http.MethodDelete, "/api/products/"+product.ID, nil,
			`UPDATE products SET price = 15, translations = $2::jsonb WHERE id = $1::text::uuid`,
			product.ID, renamedProduct)
		if status != http.StatusOK {
			t.Fatalf("the delete answered %d: %s", status, body)
		}
		row := latestFor(t, h, owner, "product.delete", product.ID)
		if row.EntityLabel != "Latte Grande" || string(row.Changes["price"].Old) != "15" {
			t.Errorf("product.delete snapshots %q at price %s, want the removed Latte Grande at 15",
				row.EntityLabel, row.Changes["price"].Old)
		}
	})

	t.Run("a_category_delete_snapshots_the_category_it_removed", func(t *testing.T) {
		target := h.createCategory(owner, menu.ID, "Soguk Icecekler")
		status, body := raceBehind(t, h, owner, http.MethodDelete, "/api/categories/"+target.ID, nil,
			`UPDATE categories SET translations = $2::jsonb WHERE id = $1::text::uuid`,
			target.ID, turkishName(t, "Buzlu Icecekler"))
		if status != http.StatusOK {
			t.Fatalf("the delete answered %d: %s", status, body)
		}
		row := latestFor(t, h, owner, "category.delete", target.ID)
		if row.EntityLabel != "Buzlu Icecekler" ||
			string(row.Changes["translations.tr.name"].Old) != jsonString(t, "Buzlu Icecekler") {
			t.Errorf("category.delete snapshots %q (name %s), want the removed Buzlu Icecekler",
				row.EntityLabel, row.Changes["translations.tr.name"].Old)
		}
	})

	t.Run("a_menu_delete_snapshots_the_menu_it_removed", func(t *testing.T) {
		target := h.createMenu(owner, "Silinecek Menu")
		status, body := raceBehind(t, h, owner, http.MethodDelete, "/api/menus/"+target.ID, nil,
			`UPDATE menus SET name = 'Yeniden Adlandirilan' WHERE id = $1::text::uuid`, target.ID)
		if status != http.StatusOK {
			t.Fatalf("the delete answered %d: %s", status, body)
		}
		row := latestFor(t, h, owner, "menu.delete", target.ID)
		if row.EntityLabel != "Yeniden Adlandirilan" || string(row.Changes["name"].Old) != jsonString(t, "Yeniden Adlandirilan") {
			t.Errorf("menu.delete snapshots %q (name %s), want the removed Yeniden Adlandirilan",
				row.EntityLabel, row.Changes["name"].Old)
		}
	})

	// Nothing above is recorded twice: every request wrote one row.
	t.Run("one_row_per_request", func(t *testing.T) {
		for action, want := range map[string]int{
			"product.price": 2, "product.update": 1, "category.update": 1, "menu.update": 1,
			"business.update": 1, "product.delete": 1, "category.delete": 1, "menu.delete": 1,
		} {
			if got := len(auditRowsOf(t, h, owner, action)); got != want {
				t.Errorf("%s: %d rows, want %d", action, got, want)
			}
		}
	})
}
