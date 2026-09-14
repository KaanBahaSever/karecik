package tests

// A product write that names a category — a create, a move, a reorder — answers
// a category of another business exactly like a category that does not exist:
// 404 "Kategori bulunamadı.", the same answer it gives when the category is
// deleted while the write runs. One answer for all three means a caller cannot
// tell another tenant's category ids from ids that were never issued.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestProductWritesNamingACategoryTheBusinessDoesNotHaveAnswer404(t *testing.T) {
	h := newHarness(t)
	tenantA := h.register("A", "Kategori Kapısı A", "category-gate-a@example.test")
	tenantB := h.register("B", "Kategori Kapısı B", "category-gate-b@example.test")

	menuA := h.createMenu(tenantA, "A Menüsü")
	categoryA := h.createCategory(tenantA, menuA.ID, "A Kategorisi")
	menuB := h.createMenu(tenantB, "B Menüsü")
	categoryB := h.createCategory(tenantB, menuB.ID, "B Kategorisi")
	productB := h.createProduct(tenantB, categoryB.ID, "B Ürünü", fixturePrice)

	for _, target := range []struct {
		name string
		id   string
	}{
		{"a_category_of_another_business", categoryA.ID},
		{"a_category_that_does_not_exist", uuid.NewString()},
	} {
		t.Run(target.name, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				method string
				path   string
				body   map[string]any
			}{
				{"create", http.MethodPost, "/api/products", map[string]any{
					"category_id":  target.id,
					"translations": map[string]any{"tr": map[string]any{"name": "Kaçak Ürün"}},
					"price":        20,
				}},
				{"move", http.MethodPut, "/api/products/" + productB.ID, map[string]any{"category_id": target.id}},
				{"reorder", http.MethodPut, "/api/products/reorder", map[string]any{
					"category_id": target.id, "ids": []string{productB.ID},
				}},
			} {
				resp, payload := h.do(tc.method, tc.path, tenantB.session, tc.body)
				var refusal struct {
					Error string `json:"error"`
					Code  string `json:"code"`
				}
				if err := json.Unmarshal(payload, &refusal); err != nil {
					t.Fatalf("%s: could not decode %s: %v", tc.name, payload, err)
				}
				if resp.StatusCode != http.StatusNotFound || refusal.Error != "Kategori bulunamadı." ||
					refusal.Code != "NOT_FOUND" {
					t.Errorf("%s %s naming %s answered %d %s, want 404 %q", tc.method, tc.path, target.name,
						resp.StatusCode, payload, "Kategori bulunamadı.")
				}
			}
		})
	}

	// Nothing was written: B's product stayed where it was, and A's category
	// holds no product.
	products := h.listProductsAsOwner(t, "after the refused writes", tenantB, categoryB.ID)
	if product := findProduct(products, productB.ID); product == nil || product.CategoryID != categoryB.ID {
		t.Errorf("tenant B's product %s moved or disappeared: %+v", productB.ID, product)
	}
	if products := h.listProductsAsOwner(t, "after the refused writes", tenantA, categoryA.ID); len(products) != 0 {
		t.Errorf("tenant A's category holds %d product(s) after tenant B's refused writes", len(products))
	}
}
