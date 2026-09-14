package tests

// Create and update have to store the same thing for the same input. The update
// paths always turned "" and whitespace into NULL for a category's icon and
// image and for a product's image; the create paths stored whatever arrived, so
// a category created without an emoji carried "" while one whose emoji was
// cleared later carried NULL. These cases hold the create half to the update
// half.

import (
	"fmt"
	"net/http"
	"testing"
)

// nullableText is how icon and image_url come back: a JSON string or null.
type nullableText struct {
	Icon     *string `json:"icon"`
	ImageURL *string `json:"image_url"`
}

// assertStoredText checks one nullable column as the create response returned
// it. want "" means NULL.
func assertStoredText(t *testing.T, field string, got *string, want string, payload []byte) {
	t.Helper()
	switch {
	case want == "" && got != nil:
		t.Errorf("%s: stored %q, expected NULL — the update path stores a blank value as NULL, "+
			"so create has to as well: %s", field, *got, payload)
	case want != "" && got == nil:
		t.Errorf("%s: stored NULL, expected %q: %s", field, want, payload)
	case want != "" && *got != want:
		t.Errorf("%s: stored %q, expected %q: %s", field, *got, want, payload)
	}
}

func TestCreateStoresBlankIconAndImageAsNull(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Boş Alan Kafe", "blank-owner@example.test")
	menu := h.createMenu(owner, "Ana Menü")
	category := h.createCategory(owner, menu.ID, "Ürünler")

	cases := []struct {
		name      string
		fields    map[string]any
		wantIcon  string
		wantImage string
	}{
		{"empty_strings", map[string]any{"icon": "", "image_url": ""}, "", ""},
		{"whitespace_only", map[string]any{"icon": "   ", "image_url": " \t "}, "", ""},
		{"null", map[string]any{"icon": nil, "image_url": nil}, "", ""},
		{"left_out", map[string]any{}, "", ""},
		{"padded_values_are_trimmed",
			map[string]any{"icon": " ☕ ", "image_url": " /uploads/kahve.png "},
			"☕", "/uploads/kahve.png"},
	}

	for i, tc := range cases {
		t.Run("category_"+tc.name, func(t *testing.T) {
			body := map[string]any{
				"menu_id":      menu.ID,
				"translations": map[string]any{"tr": map[string]any{"name": fmt.Sprintf("Kategori %d", i+1)}},
			}
			for key, value := range tc.fields {
				body[key] = value
			}

			resp, payload := h.do(http.MethodPost, "/api/categories", owner.session, body)
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("creating the category failed (got %d: %s)", resp.StatusCode, payload)
			}
			var stored nullableText
			decodeInto(t, "create category", payload, &stored)
			assertStoredText(t, "category icon", stored.Icon, tc.wantIcon, payload)
			assertStoredText(t, "category image_url", stored.ImageURL, tc.wantImage, payload)
		})

		t.Run("product_"+tc.name, func(t *testing.T) {
			body := map[string]any{
				"category_id":  category.ID,
				"translations": map[string]any{"tr": map[string]any{"name": fmt.Sprintf("Ürün %d", i+1)}},
				"price":        10,
			}
			if value, ok := tc.fields["image_url"]; ok {
				body["image_url"] = value
			}

			resp, payload := h.do(http.MethodPost, "/api/products", owner.session, body)
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("creating the product failed (got %d: %s)", resp.StatusCode, payload)
			}
			var stored nullableText
			decodeInto(t, "create product", payload, &stored)
			assertStoredText(t, "product image_url", stored.ImageURL, tc.wantImage, payload)
		})
	}
}
