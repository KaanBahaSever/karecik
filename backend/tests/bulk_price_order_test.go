package tests

// What a bulk price answer lists, and what it counts.
//
// The preview and the apply list the products of the menu in menu editor
// order — category position, then product position — whatever order a query
// returns them in, and both count as affected only the products whose price
// changes, not every product listed.

import (
	"net/http"
	"strings"
	"testing"
)

// bulkPriceOrder reads the product ids of a bulk price answer in its order.
func bulkPriceOrder(t *testing.T, what string, status int, body []byte) ([]string, bulkPriceAnswer) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("%s answered %d, want 200: %s", what, status, shorten(body))
	}
	var answer bulkPriceAnswer
	decodeInto(t, what, body, &answer)
	ids := make([]string, 0, len(answer.Preview))
	for _, row := range answer.Preview {
		ids = append(ids, row.ID)
	}
	return ids, answer
}

func TestBulkPriceListsProductsInMenuEditorOrder(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Editör Sırası Kafe", "bulk-order-owner@example.test")
	menu := h.createMenu(owner, "Editör Sırası Menü")

	// The rows are written with chosen ids, so that no order a query can return
	// them in without sorting is the editor order. The category shown first has
	// the larger id and is inserted second. The products are inserted in the
	// order second/1, first/1, second/0, first/0, and their ids sort as first/1,
	// second/1, first/0, second/0. The editor order is first/0, first/1,
	// second/0, second/1.
	later := insertCategoryWithID(t, h, fixtureUUID(0xc0ffee00, 1), menu.ID, "Sonra Gösterilen", 1)
	earlier := insertCategoryWithID(t, h, fixtureUUID(0xc0ffee00, 2), menu.ID, "Önce Gösterilen", 0)

	laterSecond := insertProductWithID(t, h, fixtureUUID(0xa0ffee00, 2), later.ID, "Sonra 2", 100, 1)
	earlierSecond := insertProductWithID(t, h, fixtureUUID(0xa0ffee00, 1), earlier.ID, "Önce 2", 100, 1)
	laterFirst := insertProductWithID(t, h, fixtureUUID(0xa0ffee00, 4), later.ID, "Sonra 1", 100, 0)
	earlierFirst := insertProductWithID(t, h, fixtureUUID(0xa0ffee00, 3), earlier.ID, "Önce 1", 100, 0)
	want := []string{earlierFirst.ID, earlierSecond.ID, laterFirst.ID, laterSecond.ID}

	for _, apply := range []bool{false, true} {
		what := "the preview"
		if apply {
			what = "the apply"
		}
		resp, body := h.do(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
			"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": apply,
		})
		got, answer := bulkPriceOrder(t, what, resp.StatusCode, body)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s lists the products as %v, want the editor order %v", what, got, want)
		}
		if answer.Affected != len(want) {
			t.Errorf("%s answered affected=%d, want %d", what, answer.Affected, len(want))
		}
	}
}

func TestBulkPriceCountsOnlyTheProductsWhosePriceChanges(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Etkilenen Kafe", "bulk-affected-owner@example.test")
	menu := h.createMenu(owner, "Etkilenen Menü")
	category := h.createCategory(owner, menu.ID, "Karışık")

	// +10% rounded to the nearest 10: 0 stays 0, 100 becomes 110, 150 becomes
	// 170 (165 rounds half away from zero).
	free := h.createProduct(owner, category.ID, "İkram", 0)
	cheap := h.createProduct(owner, category.ID, "Çay", 100)
	round := h.createProduct(owner, category.ID, "Kahve", 150)
	wantPrices := map[string]float64{free.ID: 0, cheap.ID: 110, round.ID: 170}

	for _, step := range []struct {
		name     string
		percent  float64
		apply    bool
		affected int
	}{
		{"preview", 10, false, 2},
		{"apply", 10, true, 2},
		{"apply_of_nothing", 0, true, 0},
	} {
		resp, body := h.do(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
			"menu_id": menu.ID, "percentage": step.percent, "rounding": "nearest_10", "apply": step.apply,
		})
		listed, answer := bulkPriceOrder(t, step.name, resp.StatusCode, body)
		if len(listed) != 3 {
			t.Errorf("%s lists %d products, want all 3: %s", step.name, len(listed), shorten(body))
		}
		if answer.Affected != step.affected {
			t.Errorf("%s answered affected=%d, want %d — only the products whose price changes count: %s",
				step.name, answer.Affected, step.affected, shorten(body))
		}
	}

	for id, want := range wantPrices {
		if _, _, stored := productRow(t, h, id); !samePrice(stored, want) {
			t.Errorf("product %s stores %.2f, want %.2f", id, stored, want)
		}
	}
}
