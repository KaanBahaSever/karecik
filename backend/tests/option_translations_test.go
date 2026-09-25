package tests

// Black-box coverage of per-language option names, over HTTP against a scratch
// database: the owner stores a name per language for every option group and
// item, the customer menu serves the one of the visitor's language, and an edit
// of those names alone never moves the menu's "prices valid from" date — the
// trigger of migration 010 compares the surcharges only.

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ------------------------------------------------------------- payloads

type optionName struct {
	Name string `json:"name"`
}

// translatedOptionItem and translatedOptionGroup are the option shapes as the
// owner's endpoints return them and as ProductModal sends them back.
type translatedOptionItem struct {
	Name         string                `json:"name"`
	Translations map[string]optionName `json:"translations,omitempty"`
	Price        float64               `json:"price"`
}

type translatedOptionGroup struct {
	Name         string                 `json:"name"`
	Translations map[string]optionName  `json:"translations,omitempty"`
	Type         string                 `json:"type"`
	Required     bool                   `json:"required"`
	Items        []translatedOptionItem `json:"items"`
}

// translatedProduct is dialogProduct with option groups that keep their
// translations, so a re-save sends back everything the owner stored.
type translatedProduct struct {
	dialogProduct
	Options []translatedOptionGroup `json:"options"`
}

func (p translatedProduct) payload() map[string]any {
	body := p.dialogProduct.payload()
	body["options"] = p.Options
	return body
}

// publicOptions is the option part of the customer payload, read as raw JSON
// members so that a translations key the payload must not carry is visible.
type publicOptions []map[string]json.RawMessage

// --------------------------------------------------------------- helpers

func (s *priceDateSuite) translatedProduct(t *testing.T, owner *tenant, id string) translatedProduct {
	t.Helper()
	resp, payload := s.h.do(http.MethodGet, "/api/products", owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listing %s's products failed (got %d: %s)", owner.label, resp.StatusCode, payload)
	}
	// The outer Options is shallower than the embedded one, so encoding/json
	// decodes "options" into it and leaves dialogProduct.Options empty.
	var products []translatedProduct
	decodeInto(t, "list products", payload, &products)
	for _, product := range products {
		if product.ID == id {
			return product
		}
	}
	t.Fatalf("product %s is missing from %s's product list", id, owner.label)
	return translatedProduct{}
}

// publicProductOptions reads the options of one product from the customer
// payload (or the owner's preview when session is set) in the given language;
// an empty lang sends no ?lang= at all.
func (s *priceDateSuite) publicProductOptions(t *testing.T, path, session, lang, productID string) publicOptions {
	t.Helper()
	if lang != "" {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		path += separator + "lang=" + url.QueryEscape(lang)
	}
	resp, payload := s.h.do(http.MethodGet, path, session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s failed (got %d: %s)", path, resp.StatusCode, payload)
	}
	var menu struct {
		Categories []struct {
			Products []struct {
				ID      string        `json:"id"`
				Options publicOptions `json:"options"`
			} `json:"products"`
		} `json:"categories"`
	}
	decodeInto(t, "GET "+path, payload, &menu)
	for _, category := range menu.Categories {
		for _, product := range category.Products {
			if product.ID == productID {
				return product.Options
			}
		}
	}
	t.Fatalf("GET %s: product %s is missing from the payload: %s", path, productID, payload)
	return nil
}

// names flattens resolved options into "group: item, item" lines and fails on
// a translations key, which must never reach a customer.
func (o publicOptions) names(t *testing.T, where string) []string {
	t.Helper()
	lines := make([]string, 0, len(o))
	for _, group := range o {
		if _, ok := group["translations"]; ok {
			t.Errorf("%s: an option group carries its translations map: %s", where, group["translations"])
		}
		var name string
		_ = json.Unmarshal(group["name"], &name)
		var items []map[string]json.RawMessage
		_ = json.Unmarshal(group["items"], &items)
		itemNames := make([]string, 0, len(items))
		for _, item := range items {
			if _, ok := item["translations"]; ok {
				t.Errorf("%s: an option item carries its translations map: %s", where, item["translations"])
			}
			var itemName string
			_ = json.Unmarshal(item["name"], &itemName)
			itemNames = append(itemNames, itemName)
		}
		lines = append(lines, name+": "+strings.Join(itemNames, ", "))
	}
	return lines
}

func expectNames(t *testing.T, where string, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("%s: option names\n  got  %q\n  want %q", where, got, want)
	}
}

// ---------------------------------------------------------------- the test

func TestOptionTranslations(t *testing.T) {
	istanbul, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatalf("could not load Europe/Istanbul: %v", err)
	}

	h := newHarness(t)
	s := &priceDateSuite{h: h, istanbul: istanbul}

	owner := h.register("owner", "Çeviri Kafe", "option-translations@example.test")
	businessSlug := s.businessSlug(t, owner)
	created := h.createMenu(owner, "Ana Menü")
	s.write(t, "fixture languages", http.MethodPut, "/api/menus/"+created.ID, owner, map[string]any{
		"default_language": "tr",
		"languages":        []string{"tr", "en", "de", "ar"},
	})
	menu := watchedMenu{label: "the menu", owner: owner, id: created.ID,
		businessSlug: businessSlug, menuSlug: created.Slug}
	publicPath := "/api/public/menu/" + businessSlug + "/" + created.Slug
	previewPath := "/api/preview/menu?menu=" + url.QueryEscape(created.Slug)

	category := h.createCategory(owner, created.ID, "Sıcak İçecekler")

	// Exactly what ProductModal sends for a latte whose owner filled in English
	// and Arabic but no German.
	var latte translatedProduct
	decodeInto(t, "fixture create latte",
		s.write(t, "fixture create latte", http.MethodPost, "/api/products", owner, map[string]any{
			"category_id": category.ID,
			"translations": map[string]any{
				"tr": map[string]any{"name": "Latte"},
				"en": map[string]any{"name": "Latte"},
			},
			"price": 145,
			"options": []map[string]any{
				{
					"name": "Süt Tercihi", "type": "single", "required": true,
					"translations": map[string]any{
						"tr": map[string]any{"name": "Süt Tercihi"},
						"en": map[string]any{"name": "Milk"},
						"ar": map[string]any{"name": "الحليب"},
					},
					"items": []map[string]any{
						{"name": "Tam Yağlı", "price": 0, "translations": map[string]any{
							"en": map[string]any{"name": "Whole milk"},
							"ar": map[string]any{"name": "حليب كامل الدسم"},
						}},
						{"name": "Yulaf Sütü", "price": 25, "translations": map[string]any{
							"en": map[string]any{"name": "Oat milk"},
						}},
					},
				},
				// A group nobody translated keeps its pre-translation shape.
				{
					"name": "Ekstra", "type": "multiple", "required": false,
					"items": []map[string]any{{"name": "Şurup", "price": 10}},
				},
			},
		}), &latte)
	if latte.ID == "" {
		t.Fatalf("fixture create latte: no id in the response")
	}
	lattePath := "/api/products/" + latte.ID

	t.Run("the_owner_reads_every_language_back", func(t *testing.T) {
		product := s.translatedProduct(t, owner, latte.ID)
		if len(product.Options) != 2 || len(product.Options[0].Items) != 2 {
			t.Fatalf("the stored options lost a group or an item: %+v", product.Options)
		}
		milk := product.Options[0]
		if milk.Name != "Süt Tercihi" || milk.Translations["en"].Name != "Milk" ||
			milk.Translations["ar"].Name != "الحليب" || milk.Translations["tr"].Name != "Süt Tercihi" {
			t.Errorf("group names were not stored per language: %+v", milk)
		}
		// The item's default language is stored as a copy of its name.
		whole := milk.Items[0]
		if whole.Name != "Tam Yağlı" || whole.Translations["tr"].Name != "Tam Yağlı" ||
			whole.Translations["en"].Name != "Whole milk" {
			t.Errorf("item names were not stored per language: %+v", whole)
		}
		if product.Options[1].Translations != nil || product.Options[1].Items[0].Translations != nil {
			t.Errorf("an untranslated group gained translations: %+v", product.Options[1])
		}
	})

	t.Run("the_customer_menu_serves_the_requested_language", func(t *testing.T) {
		for _, tc := range []struct {
			lang string
			want []string
		}{
			{"en", []string{"Milk: Whole milk, Oat milk", "Ekstra: Şurup"}},
			{"ar", []string{"الحليب: حليب كامل الدسم, Yulaf Sütü", "Ekstra: Şurup"}},
			// No German names at all: the default language answers.
			{"de", []string{"Süt Tercihi: Tam Yağlı, Yulaf Sütü", "Ekstra: Şurup"}},
			{"tr", []string{"Süt Tercihi: Tam Yağlı, Yulaf Sütü", "Ekstra: Şurup"}},
			{"", []string{"Süt Tercihi: Tam Yağlı, Yulaf Sütü", "Ekstra: Şurup"}},
			// French is not one of this menu's languages.
			{"fr", []string{"Süt Tercihi: Tam Yağlı, Yulaf Sütü", "Ekstra: Şurup"}},
		} {
			where := "public ?lang=" + tc.lang
			options := s.publicProductOptions(t, publicPath, "", tc.lang, latte.ID)
			expectNames(t, where, options.names(t, where), tc.want...)
		}

		where := "preview ?lang=en"
		options := s.publicProductOptions(t, previewPath, owner.session, "en", latte.ID)
		expectNames(t, where, options.names(t, where), "Milk: Whole milk, Oat milk", "Ekstra: Şurup")
	})

	t.Run("a_translation_only_edit_does_not_move_the_price_date", func(t *testing.T) {
		s.pin(t, menu)
		product := s.translatedProduct(t, owner, latte.ID)

		// Rename in English, add German to a group and to an item, and
		// translate the group that had no translations — every surcharge stays.
		product.Options[0].Translations["en"] = optionName{Name: "Choice of milk"}
		product.Options[0].Translations["de"] = optionName{Name: "Milchsorte"}
		product.Options[0].Items[1].Translations["de"] = optionName{Name: "Hafermilch"}
		product.Options[1].Translations = map[string]optionName{"en": {Name: "Extras"}}

		s.write(t, "PUT with translated option names", http.MethodPut, lattePath, owner, product.payload())
		s.expect(t, "PUT with translated option names", menu, false)

		where := "public ?lang=de after the edit"
		options := s.publicProductOptions(t, publicPath, "", "de", latte.ID)
		expectNames(t, where, options.names(t, where), "Milchsorte: Tam Yağlı, Hafermilch", "Ekstra: Şurup")
		where = "public ?lang=en after the edit"
		options = s.publicProductOptions(t, publicPath, "", "en", latte.ID)
		expectNames(t, where, options.names(t, where), "Choice of milk: Whole milk, Oat milk", "Extras: Şurup")

		// Re-saving what the owner reads back changes nothing either.
		s.write(t, "PUT the same options again", http.MethodPut, lattePath, owner,
			s.translatedProduct(t, owner, latte.ID).payload())
		s.expect(t, "PUT the same options again", menu, false)

		// And removing every translation again is a name edit as well.
		plain := s.translatedProduct(t, owner, latte.ID)
		for gi := range plain.Options {
			plain.Options[gi].Translations = nil
			for ii := range plain.Options[gi].Items {
				plain.Options[gi].Items[ii].Translations = nil
			}
		}
		s.write(t, "PUT without translations", http.MethodPut, lattePath, owner, plain.payload())
		s.expect(t, "PUT without translations", menu, false)
		if after := s.translatedProduct(t, owner, latte.ID); after.Options[0].Translations != nil {
			t.Errorf("the translations survived a save that removed them: %+v", after.Options[0])
		}
	})

	t.Run("a_surcharge_edit_next_to_a_translation_still_moves_it", func(t *testing.T) {
		s.pin(t, menu)
		product := s.translatedProduct(t, owner, latte.ID)
		product.Options[0].Translations = map[string]optionName{"en": {Name: "Milk"}}
		product.Options[0].Items[1].Price = cents(product.Options[0].Items[1].Price + 5)

		s.write(t, "PUT with a translation and a new surcharge", http.MethodPut, lattePath, owner,
			product.payload())
		s.expect(t, "PUT with a translation and a new surcharge", menu, true)
	})

	t.Run("an_unsupported_or_overlong_translation", func(t *testing.T) {
		product := s.translatedProduct(t, owner, latte.ID)
		product.Options[0].Translations = map[string]optionName{
			"en": {Name: strings.Repeat("a", 61)},
		}
		resp, payload := h.do(http.MethodPut, lattePath, owner.session, product.payload())
		if resp.StatusCode != http.StatusUnprocessableEntity ||
			!strings.Contains(string(payload), "Seçenek grubunun adı (EN) en fazla 60 karakter olabilir.") {
			t.Errorf("an overlong English group name: got %d: %s", resp.StatusCode, payload)
		}

		product.Options[0].Translations = map[string]optionName{
			"it": {Name: "Latte"}, "en": {Name: "Milk"},
		}
		s.write(t, "PUT with an unsupported language", http.MethodPut, lattePath, owner, product.payload())
		if after := s.translatedProduct(t, owner, latte.ID); after.Options[0].Translations["it"].Name != "" {
			t.Errorf("a translation in an unsupported language was stored: %+v", after.Options[0])
		}
	})

	t.Run("changing_the_default_language_loses_no_name", func(t *testing.T) {
		product := s.translatedProduct(t, owner, latte.ID)
		product.Options[0].Translations = map[string]optionName{
			"en": {Name: "Milk"}, "ar": {Name: "الحليب"},
		}
		s.write(t, "PUT the fixture names", http.MethodPut, lattePath, owner, product.payload())

		s.write(t, "switch the default language", http.MethodPut, "/api/menus/"+created.ID, owner,
			map[string]any{"default_language": "en"})

		// German has no name of its own and now falls back to English, the
		// way the product name does; Turkish keeps its own name.
		where := "public ?lang=de after the switch"
		options := s.publicProductOptions(t, publicPath, "", "de", latte.ID)
		if got := options.names(t, where)[0]; !strings.HasPrefix(got, "Milk:") {
			t.Errorf("%s: group name %q, want the new default language's %q", where, got, "Milk")
		}
		where = "public ?lang=tr after the switch"
		options = s.publicProductOptions(t, publicPath, "", "tr", latte.ID)
		if got := options.names(t, where)[0]; !strings.HasPrefix(got, "Süt Tercihi:") {
			t.Errorf("%s: group name %q, want %q", where, got, "Süt Tercihi")
		}

		// The dashboard opens the English tab on the English name and re-saves;
		// the Turkish one stays under tr.
		product = s.translatedProduct(t, owner, latte.ID)
		product.Options[0].Name = product.Options[0].Translations["en"].Name
		s.write(t, "PUT after the switch", http.MethodPut, lattePath, owner, product.payload())

		after := s.translatedProduct(t, owner, latte.ID)
		group := after.Options[0]
		if group.Name != "Milk" || group.Translations["en"].Name != "Milk" ||
			group.Translations["tr"].Name != "Süt Tercihi" || group.Translations["ar"].Name != "الحليب" {
			t.Errorf("a name was lost across the default-language switch: %+v", group)
		}
	})
}
