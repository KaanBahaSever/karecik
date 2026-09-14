package tests

// Numbers a column cannot hold are refused with a 422 before anything is
// written.
//
// products.price and products.compare_price are NUMERIC(12,2), whose largest
// value is 9999999999.99, and menus.position is an INTEGER. A larger number
// fails the write — in PostgreSQL with SQLSTATE 22003, or in pgx before the
// value is sent — and NaN, which a form or an XML body can carry, is stored by
// PostgreSQL in a NUMERIC column and breaks every later read of the product.
// Every case checks the answer and that nothing it names moved.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

const (
	msgPriceTooLarge        = "Fiyat en fazla 9.999.999.999,99 olabilir."
	msgComparePriceTooLarge = "Karşılaştırma fiyatı en fazla 9.999.999.999,99 olabilir."
	msgBulkPriceTooLarge    = "Bu değişiklik bazı fiyatları izin verilen en yüksek değerin (9.999.999.999,99) üzerine çıkarıyor."
	msgPositionTooLarge     = "Sıra değeri çok büyük."
)

// storedNumbers reads what the numeric columns of one product and one menu hold,
// as PostgreSQL spells them.
func storedNumbers(t *testing.T, h *harness, productID, menuID string) (price, comparePrice string, position int, products int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.pool.QueryRow(ctx, `
		SELECT p.price::text, COALESCE(p.compare_price::text, 'NULL'), m.position,
		       (SELECT count(*) FROM products)
		FROM products p, menus m
		WHERE p.id = $1::text::uuid AND m.id = $2::text::uuid`, productID, menuID).
		Scan(&price, &comparePrice, &position, &products); err != nil {
		t.Fatalf("could not read the stored numbers: %v", err)
	}
	return price, comparePrice, position, products
}

// errorMessage reads the error text of an answer.
func errorMessage(body []byte) string {
	var refusal struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &refusal)
	return refusal.Error
}

func TestNumbersAColumnCannotHoldAreRefused(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Sayı Sınırı Kafe", "numeric-limits-owner@example.test")
	menu := h.createMenu(owner, "Sınır Menü")
	category := h.createCategory(owner, menu.ID, "Sınır Kategori")
	product := h.createProduct(owner, category.ID, "Sınır Ürün", 10)

	const (
		jsonBody = "application/json"
		formBody = "application/x-www-form-urlencoded"
		xmlBody  = "application/xml"
	)
	productPath := "/api/products/" + product.ID
	pricePath := productPath + "/price"
	create := func(members string) string {
		return `{"category_id":"` + category.ID + `","translations":{"tr":{"name":"Büyük"}},` + members + `}`
	}

	for _, tc := range []struct {
		name, method, target, contentType, body, message string
	}{
		{"create_price_of_ten_billion", http.MethodPost, "/api/products", jsonBody, create(`"price":10000000000`), msgPriceTooLarge},
		{"create_price_just_above_the_limit", http.MethodPost, "/api/products", jsonBody, create(`"price":9999999999.991`), msgPriceTooLarge},
		{"create_price_1e14", http.MethodPost, "/api/products", jsonBody, create(`"price":1e14`), msgPriceTooLarge},
		{"create_price_1e300", http.MethodPost, "/api/products", jsonBody, create(`"price":1e300`), msgPriceTooLarge},
		{"create_compare_price_of_ten_billion", http.MethodPost, "/api/products", jsonBody,
			create(`"price":5,"compare_price":10000000000`), msgComparePriceTooLarge},
		{"create_compare_price_1e300", http.MethodPost, "/api/products", jsonBody,
			create(`"price":5,"compare_price":1e300`), msgComparePriceTooLarge},

		{"update_price_of_ten_billion", http.MethodPut, productPath, jsonBody, `{"price":10000000000}`, msgPriceTooLarge},
		{"update_price_1e14", http.MethodPut, productPath, jsonBody, `{"price":1e14}`, msgPriceTooLarge},
		{"update_price_1e300", http.MethodPut, productPath, jsonBody, `{"price":1e300}`, msgPriceTooLarge},
		{"update_compare_price_of_ten_billion", http.MethodPut, productPath, jsonBody, `{"compare_price":10000000000}`, msgComparePriceTooLarge},
		{"update_compare_price_1e300", http.MethodPut, productPath, jsonBody, `{"compare_price":1e300}`, msgComparePriceTooLarge},

		{"patch_price_of_ten_billion", http.MethodPatch, pricePath, jsonBody, `{"price":10000000000}`, msgPriceTooLarge},
		{"patch_price_1e14", http.MethodPatch, pricePath, jsonBody, `{"price":1e14}`, msgPriceTooLarge},
		{"patch_price_1e300", http.MethodPatch, pricePath, jsonBody, `{"price":1e300}`, msgPriceTooLarge},
		{"patch_price_infinity_in_a_form", http.MethodPatch, pricePath, formBody, "price=Inf", msgPriceTooLarge},
		{"patch_price_minus_infinity_in_a_form", http.MethodPatch, pricePath, formBody, "price=-Inf", "Fiyat sıfırdan küçük olamaz."},
		{"patch_price_nan_in_a_form", http.MethodPatch, pricePath, formBody, "price=NaN",
			"Fiyat sıfır veya daha büyük bir sayı olmalıdır."},
		{"patch_price_nan_in_xml", http.MethodPatch, pricePath, xmlBody, "<priceRequest><Price>NaN</Price></priceRequest>",
			"Fiyat sıfır veya daha büyük bir sayı olmalıdır."},

		{"menu_position_one_above_the_largest_integer", http.MethodPut, "/api/menus/" + menu.ID, jsonBody,
			`{"position":2147483648}`, msgPositionTooLarge},
		{"menu_position_of_three_billion", http.MethodPut, "/api/menus/" + menu.ID, jsonBody,
			`{"position":3000000000}`, msgPositionTooLarge},
		{"new_menu_position_one_above_the_largest_integer", http.MethodPost, "/api/menus", jsonBody,
			`{"name":"Taşan Menü","position":2147483648}`, msgPositionTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			price, comparePrice, position, products := storedNumbers(t, h, product.ID, menu.ID)
			menusBefore := len(h.listMenus(t, owner))

			status, body := sendRawRequest(t, h, tc.method, tc.target, owner.session, nil, tc.contentType, tc.body)
			if status != http.StatusUnprocessableEntity || errorMessage(body) != tc.message {
				t.Errorf("%s %s answered %d %s, want 422 %q", tc.method, tc.target, status, shorten(body), tc.message)
			}

			afterPrice, afterCompare, afterPosition, afterProducts := storedNumbers(t, h, product.ID, menu.ID)
			if afterPrice != price || afterCompare != comparePrice || afterPosition != position || afterProducts != products {
				t.Errorf("the refused request still wrote: price %s -> %s, compare_price %s -> %s, menu position %d -> %d, "+
					"products %d -> %d", price, afterPrice, comparePrice, afterCompare, position, afterPosition,
					products, afterProducts)
			}
			if menus := len(h.listMenus(t, owner)); menus != menusBefore {
				t.Errorf("the refused request changed the number of menus from %d to %d", menusBefore, menus)
			}
		})
	}

	// The limits themselves are accepted, stored exactly and read back.
	t.Run("the_largest_price_is_stored", func(t *testing.T) {
		if status, body := sendRawRequest(t, h, http.MethodPatch, pricePath, owner.session, nil, jsonBody,
			`{"price":9999999999.99}`); status != http.StatusOK {
			t.Fatalf("PATCH the largest price answered %d %s, want 200", status, shorten(body))
		}
		if status, body := sendRawRequest(t, h, http.MethodPut, productPath, owner.session, nil, jsonBody,
			`{"compare_price":9999999999.99}`); status != http.StatusOK {
			t.Fatalf("PUT the largest compare price answered %d %s, want 200", status, shorten(body))
		}
		if price, comparePrice, _, _ := storedNumbers(t, h, product.ID, menu.ID); price != "9999999999.99" ||
			comparePrice != "9999999999.99" {
			t.Errorf("the largest price is stored as %s and the largest compare price as %s", price, comparePrice)
		}
		if status, body := sendRawRequest(t, h, http.MethodPost, "/api/products", owner.session, nil, jsonBody,
			create(`"price":9999999999.99,"compare_price":9999999999.99`)); status != http.StatusCreated {
			t.Fatalf("POST a product at the largest price answered %d %s, want 201", status, shorten(body))
		}
		if resp, body := h.do(http.MethodGet, "/api/products", owner.session, nil); resp.StatusCode != http.StatusOK {
			t.Errorf("GET /api/products answered %d %s after the largest prices were stored", resp.StatusCode, shorten(body))
		}
	})

	t.Run("the_largest_position_is_stored_and_a_new_menu_still_comes_last", func(t *testing.T) {
		if status, body := sendRawRequest(t, h, http.MethodPut, "/api/menus/"+menu.ID, owner.session, nil, jsonBody,
			`{"position":2147483647}`); status != http.StatusOK {
			t.Fatalf("PUT the largest menu position answered %d %s, want 200", status, shorten(body))
		}
		created := h.createMenu(owner, "Sonraki Menü")
		menus := h.listMenus(t, owner)
		if len(menus) == 0 || menus[len(menus)-1].ID != created.ID {
			t.Errorf("the menu created after one at the largest position is not listed last: %+v", menus)
		}
		for _, listed := range menus {
			if listed.ID == created.ID && listed.Position != 2147483647 {
				t.Errorf("the new menu got position %d, want the largest integer, 2147483647", listed.Position)
			}
		}
	})
}

// listedMenu is a menu of GET /api/menus, reduced to its id and position.
type listedMenu struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}

// listMenus reads the owner's menus in the order the dashboard lists them.
func (h *harness) listMenus(t *testing.T, owner *tenant) []listedMenu {
	t.Helper()
	resp, body := h.do(http.MethodGet, "/api/menus", owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/menus answered %d: %s", resp.StatusCode, shorten(body))
	}
	var menus []listedMenu
	decodeInto(t, "GET /api/menus", body, &menus)
	return menus
}

// TestBulkPriceRefusesPricesAboveTheLimit: a stored price the column can hold,
// raised or rounded past what it can hold, is refused on preview and apply
// alike, and nothing is written — no price, no price date and no log row.
func TestBulkPriceRefusesPricesAboveTheLimit(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Toplu Sınır Kafe", "bulk-limit-owner@example.test")
	menu := h.createMenu(owner, "Toplu Sınır Menü")
	category := h.createCategory(owner, menu.ID, "Pahalılar")
	large := h.createProduct(owner, category.ID, "Çok Pahalı", 9999999999)
	largest := h.createProduct(owner, category.ID, "En Pahalı", 9999999999.99)
	plain := h.createProduct(owner, category.ID, "Sıradan", 100)
	pinMenuPriceDate(t, h, menu.ID)

	stored := func(t *testing.T) (string, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var prices, state string
		if err := h.pool.QueryRow(ctx, `
			SELECT string_agg(price::text, ',' ORDER BY id),
			       (SELECT price_updated_at::text FROM menus WHERE id = $2::text::uuid) || '/' ||
			       (SELECT count(*) FROM price_update_logs)::text
			FROM products WHERE id = ANY($1::text[]::uuid[])`,
			[]string{large.ID, largest.ID, plain.ID}, menu.ID).Scan(&prices, &state); err != nil {
			t.Fatalf("could not read the stored prices: %v", err)
		}
		return prices, state
	}
	prices, state := stored(t)

	for _, tc := range []struct {
		name, contentType, body, message string
	}{
		{"ten_percent_on_9999999999_preview", "application/json",
			`{"menu_id":"` + menu.ID + `","percentage":10,"rounding":"none","apply":false}`, msgBulkPriceTooLarge},
		{"ten_percent_on_9999999999_apply", "application/json",
			`{"menu_id":"` + menu.ID + `","percentage":10,"rounding":"none","apply":true}`, msgBulkPriceTooLarge},
		{"nothing_added_but_rounded_up_to_ten_preview", "application/json",
			`{"menu_id":"` + menu.ID + `","percentage":0,"rounding":"nearest_10","apply":false}`, msgBulkPriceTooLarge},
		{"nothing_added_but_rounded_up_to_ten_apply", "application/json",
			`{"menu_id":"` + menu.ID + `","percentage":0,"rounding":"nearest_10","apply":true}`, msgBulkPriceTooLarge},
		{"nan_percentage_in_xml_preview", "application/xml",
			"<bulkPriceRequest><MenuID>" + menu.ID + "</MenuID><Percentage>NaN</Percentage></bulkPriceRequest>",
			"Yüzde değeri -90 ile 1000 arasında olmalıdır."},
		{"nan_percentage_in_xml_apply", "application/xml",
			"<bulkPriceRequest><MenuID>" + menu.ID + "</MenuID><Percentage>NaN</Percentage><Apply>true</Apply></bulkPriceRequest>",
			"Yüzde değeri -90 ile 1000 arasında olmalıdır."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, body := sendRawRequest(t, h, http.MethodPost, "/api/products/bulk-price", owner.session, nil,
				tc.contentType, tc.body)
			if status != http.StatusUnprocessableEntity || errorMessage(body) != tc.message {
				t.Errorf("POST /api/products/bulk-price answered %d %s, want 422 %q", status, shorten(body), tc.message)
			}
			if afterPrices, afterState := stored(t); afterPrices != prices || afterState != state {
				t.Errorf("the refused update still wrote: prices %s -> %s, price date and log rows %s -> %s",
					prices, afterPrices, state, afterState)
			}
		})
	}

	// Without the product the percentage would push over the limit, the same
	// update is an ordinary apply.
	if resp, body := h.do(http.MethodDelete, "/api/products/"+large.ID, owner.session, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("fixture: could not delete the product at 9999999999: %d %s", resp.StatusCode, shorten(body))
	}
	resp, body := h.do(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
		"menu_id": menu.ID, "percentage": 0, "rounding": "none", "apply": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("an apply that keeps the largest price where it is answered %d %s, want 200", resp.StatusCode, shorten(body))
	}
}
