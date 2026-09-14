package tests

// Black-box coverage of the "prices valid from" date printed at the bottom of
// every customer menu — "Fiyatlarımız dd.mm.yyyy tarihinden itibaren
// geçerlidir." — driven over HTTP the same way the isolation suite is.
//
// No handler moves that date. A trigger does (migration 010), on any UPDATE of
// a product that changes price, compare_price or an option surcharge. So the
// only test worth having goes through every endpoint that writes a product and
// then reads the date back in both places it is shown: to the owner
// (GET /api/menus/:id) and to a customer (the public menu payload, whose footer
// prints it).
//
// "It moved" is only observable against a known starting point, and
// price_updated_at is NOT NULL DEFAULT now(): a menu created a second ago
// already carries this very minute. Every case therefore first pins the menus
// it watches to oldPriceDate through h.pool — no endpoint may write that column,
// which is the whole point — then calls the API, then compares.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// oldPriceDate is the instant every case starts from. 22:30 UTC on purpose: it
// is already the next day in Istanbul, so a footer that formats the date in the
// server's own zone rather than in Europe/Istanbul names 01.01.2020 instead of
// 02.01.2020, and every "does not move" case catches it.
var oldPriceDate = time.Date(2020, 1, 1, 22, 30, 0, 0, time.UTC)

// movedWithin is how far a moved date may sit from this process's clock. It is
// generous because now() is the database server's clock, which belongs to a
// different machine whenever KARECIK_TEST_DATABASE_URL points at one.
const movedWithin = 5 * time.Minute

// ------------------------------------------------------------- payloads

// dialogProduct is a product as GET /api/products returns it, reduced to the
// fields ProductModal.jsx loads into its form.
type dialogProduct struct {
	ID           string                       `json:"id"`
	CategoryID   string                       `json:"category_id"`
	Translations map[string]dialogTranslation `json:"translations"`
	Price        float64                      `json:"price"`
	ComparePrice *float64                     `json:"compare_price"`
	Calories     *int                         `json:"calories"`
	ImageURL     *string                      `json:"image_url"`
	Allergens    []string                     `json:"allergens"`
	Badges       []map[string]any             `json:"badges"`
	Options      []dialogOptionGroup          `json:"options"`
	IsActive     bool                         `json:"is_active"`
	IsFeatured   bool                         `json:"is_featured"`
}

type dialogTranslation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Ingredients string `json:"ingredients"`
}

type dialogOptionGroup struct {
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	Required bool               `json:"required"`
	Items    []dialogOptionItem `json:"items"`
}

type dialogOptionItem struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// payload is the body ProductModal.save() sends: all eleven keys on every save,
// whatever the owner actually touched. That is why the trigger has to compare
// values — price, compare_price and options are named in every one of these
// UPDATEs, changed or not.
func (p dialogProduct) payload() map[string]any {
	return map[string]any{
		"category_id":   p.CategoryID,
		"translations":  p.Translations,
		"price":         p.Price,
		"compare_price": p.ComparePrice,
		"calories":      p.Calories,
		"image_url":     p.ImageURL,
		"allergens":     p.Allergens,
		"badges":        p.Badges,
		"options":       p.Options,
		"is_active":     p.IsActive,
		"is_featured":   p.IsFeatured,
	}
}

type bulkPriceResponse struct {
	Applied  bool `json:"applied"`
	Affected int  `json:"affected"`
	Preview  []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		OldPrice float64 `json:"old_price"`
		NewPrice float64 `json:"new_price"`
	} `json:"preview"`
	PriceUpdatedAt *time.Time `json:"price_updated_at"`
}

// ---------------------------------------------------------------- suite

// priceDateSuite carries what every case needs to read a date back.
type priceDateSuite struct {
	h        *harness
	istanbul *time.Location
}

// watchedMenu is one menu whose date the suite reads, with what it takes to
// read it both as the owner and as a customer.
type watchedMenu struct {
	label        string
	owner        *tenant
	id           string
	businessSlug string
	menuSlug     string
}

// cents pins a float to two decimals, the precision every price is stored at.
func cents(v float64) float64 {
	return math.Round(v*100) / 100
}

// businessSlug reads the tenant's subdomain slug, which the public address
// needs and register does not keep.
func (s *priceDateSuite) businessSlug(t *testing.T, owner *tenant) string {
	t.Helper()
	resp, payload := s.h.do(http.MethodGet, "/api/business", owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fixture: reading %s's business failed (got %d: %s)",
			owner.label, resp.StatusCode, payload)
	}
	var business struct {
		Slug string `json:"slug"`
	}
	decodeInto(t, "fixture business", payload, &business)
	if business.Slug == "" {
		t.Fatalf("fixture: %s's business carried no slug: %s", owner.label, payload)
	}
	return business.Slug
}

// write performs the request a case is about and insists it succeeded. A
// refused write leaves the date where it was, which would make every "does not
// move" assertion pass for the wrong reason.
func (s *priceDateSuite) write(t *testing.T, what, method, path string, owner *tenant, body any) []byte {
	t.Helper()
	resp, payload := s.h.do(method, path, owner.session, body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s: %s %s was refused (got %d: %s)", what, method, path, resp.StatusCode, payload)
	}
	return payload
}

// product re-reads one product through the list endpoint, the way the menu
// editor loads it before the dialog opens.
func (s *priceDateSuite) product(t *testing.T, owner *tenant, id string) dialogProduct {
	t.Helper()
	resp, payload := s.h.do(http.MethodGet, "/api/products", owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listing %s's products failed (got %d: %s)", owner.label, resp.StatusCode, payload)
	}
	var products []dialogProduct
	decodeInto(t, "list products", payload, &products)
	for _, product := range products {
		if product.ID == id {
			return product
		}
	}
	t.Fatalf("product %s is missing from %s's product list", id, owner.label)
	return dialogProduct{}
}

// pin puts the given menus back on oldPriceDate. It writes through h.pool
// because no endpoint may write this column — that is exactly what is under
// test.
func (s *priceDateSuite) pin(t *testing.T, menus ...watchedMenu) {
	t.Helper()

	ids := make([]string, 0, len(menus))
	for _, menu := range menus {
		ids = append(ids, menu.id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tag, err := s.h.pool.Exec(ctx,
		`UPDATE menus SET price_updated_at = $1 WHERE id = ANY($2::text[]::uuid[])`,
		oldPriceDate, ids)
	if err != nil {
		t.Fatalf("could not pin the price dates to %s: %v", oldPriceDate.Format(time.RFC3339), err)
	}
	if tag.RowsAffected() != int64(len(ids)) {
		t.Fatalf("pinning the price dates matched %d menus, expected %d", tag.RowsAffected(), len(ids))
	}
}

// expect reads one menu's date as its owner and as a customer and asserts on
// both. moved false means exactly oldPriceDate; moved true means strictly after
// it and within movedWithin of now. It returns the stored instant.
func (s *priceDateSuite) expect(t *testing.T, what string, menu watchedMenu, moved bool) time.Time {
	t.Helper()
	where := what + " — " + menu.label

	// --- the owner's view
	resp, payload := s.h.do(http.MethodGet, "/api/menus/"+menu.id, menu.owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: reading the menu as %s failed (got %d: %s)",
			where, menu.owner.label, resp.StatusCode, payload)
	}
	var owned struct {
		PriceUpdatedAt time.Time `json:"price_updated_at"`
	}
	decodeInto(t, where, payload, &owned)
	stored := owned.PriceUpdatedAt

	switch {
	case !moved && !stored.Equal(oldPriceDate):
		t.Errorf("%s: the price date MOVED to %s although no price changed — it had to stay at %s",
			where, stored.UTC().Format(time.RFC3339Nano), oldPriceDate.Format(time.RFC3339))
	case moved && !stored.After(oldPriceDate):
		t.Errorf("%s: the price date DID NOT MOVE — it is still %s after a price change",
			where, stored.UTC().Format(time.RFC3339Nano))
	case moved:
		if drift := time.Since(stored); drift > movedWithin || drift < -movedWithin {
			t.Errorf("%s: the price date moved to %s, which is %s away from now — expected within %s",
				where, stored.UTC().Format(time.RFC3339Nano), drift.Round(time.Second), movedWithin)
		}
	}

	// --- the customer's view
	path := "/api/public/menu/" + menu.businessSlug + "/" + menu.menuSlug
	resp, payload = s.h.do(http.MethodGet, path, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: GET %s failed (got %d: %s)", where, path, resp.StatusCode, payload)
	}
	var public struct {
		Business struct {
			PriceUpdatedAt time.Time `json:"price_updated_at"`
		} `json:"business"`
		Footer struct {
			PriceNote string `json:"price_note"`
		} `json:"footer"`
	}
	decodeInto(t, where, payload, &public)

	if !public.Business.PriceUpdatedAt.Equal(stored) {
		t.Errorf("%s: the public payload carries price_updated_at %s, the owner's view %s",
			where, public.Business.PriceUpdatedAt.UTC().Format(time.RFC3339Nano),
			stored.UTC().Format(time.RFC3339Nano))
	}
	day := stored.In(s.istanbul).Format("02.01.2006")
	if !strings.Contains(public.Footer.PriceNote, day) {
		t.Errorf("%s: footer.price_note is %q — expected it to name %s, the stored instant %s "+
			"on the Europe/Istanbul calendar", where, public.Footer.PriceNote, day,
			stored.UTC().Format(time.RFC3339))
	}
	return stored
}

// requireSurcharges stops a case whose fixture lost the option group it edits,
// so an index panic cannot pose as a finding.
func requireSurcharges(t *testing.T, product dialogProduct) {
	t.Helper()
	if len(product.Options) == 0 || len(product.Options[0].Items) < 2 {
		t.Fatalf("fixture: product %s no longer carries its option group of two items: %+v",
			product.ID, product.Options)
	}
}

// optionsSpelledWithDecimals renders option groups as JSON with every whole
// surcharge written as a decimal literal — 10 becomes 10.0 — so a payload can
// carry the same surcharges spelled differently. encoding/json would write 10.
func optionsSpelledWithDecimals(t *testing.T, groups []dialogOptionGroup) json.RawMessage {
	t.Helper()

	var b strings.Builder
	b.WriteByte('[')
	for gi, group := range groups {
		if gi > 0 {
			b.WriteByte(',')
		}
		name, _ := json.Marshal(group.Name)
		kind, _ := json.Marshal(group.Type)
		fmt.Fprintf(&b, `{"name":%s,"type":%s,"required":%t,"items":[`, name, kind, group.Required)
		for ii, item := range group.Items {
			if ii > 0 {
				b.WriteByte(',')
			}
			itemName, _ := json.Marshal(item.Name)
			price := strconv.FormatFloat(item.Price, 'f', -1, 64)
			if !strings.Contains(price, ".") {
				price += ".0"
			}
			fmt.Fprintf(&b, `{"name":%s,"price":%s}`, itemName, price)
		}
		b.WriteString("]}")
	}
	b.WriteByte(']')

	raw := json.RawMessage(b.String())
	if !strings.Contains(string(raw), `.0}`) {
		t.Fatalf("fixture: no surcharge was re-spelled as a decimal literal: %s", raw)
	}
	return raw
}

// ---------------------------------------------------------------- the test

// TestPriceChangeDate is the requirement itself: whenever a price changes — one
// product or a bulk update — the menu's "prices valid from" date moves, and
// whenever nothing priced changes, it stays exactly where it was.
func TestPriceChangeDate(t *testing.T) {
	// This test deliberately leaves time.Local alone. It used to switch the
	// process to UTC so that a footer formatted in the server's own zone would
	// fail even on a machine that is itself on Turkish time. But the server
	// starts goroutines that read the clock and outlive the harness — the race
	// detector has reported both fasthttp's server-date updater and the request
	// logger — so putting time.Local back afterwards was a data race. The
	// Istanbul formatting is proven independently of the machine's zone by
	// TestBuildFooterFormatsThePriceDateInIstanbul, and oldPriceDate is chosen so
	// that this suite also catches the regression on a UTC machine such as CI.

	istanbul, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatalf("could not load Europe/Istanbul to compute the expected footer day: %v", err)
	}

	h := newHarness(t)
	s := &priceDateSuite{h: h, istanbul: istanbul}

	// --- fixtures: two menus of one business and a menu of another tenant.
	owner := h.register("owner", "Fiyat Kafe", "price-owner@example.test")
	stranger := h.register("stranger", "Komşu Kafe", "price-stranger@example.test")
	ownerSlug := s.businessSlug(t, owner)
	strangerSlug := s.businessSlug(t, stranger)

	createdA := h.createMenu(owner, "Ana Menü")
	createdB := h.createMenu(owner, "Bar Menüsü")
	createdForeign := h.createMenu(stranger, "Komşu Menüsü")

	menuA := watchedMenu{label: "menu A", owner: owner, id: createdA.ID,
		businessSlug: ownerSlug, menuSlug: createdA.Slug}
	menuB := watchedMenu{label: "menu B of the same business", owner: owner, id: createdB.ID,
		businessSlug: ownerSlug, menuSlug: createdB.Slug}
	menuForeign := watchedMenu{label: "the other tenant's menu", owner: stranger, id: createdForeign.ID,
		businessSlug: strangerSlug, menuSlug: createdForeign.Slug}

	categoryA := h.createCategory(owner, menuA.id, "Sıcak İçecekler")
	categoryB := h.createCategory(owner, menuB.id, "Soğuk İçecekler")
	categoryForeign := h.createCategory(stranger, menuForeign.id, "Tatlılar")

	// The latte carries every field the dialog edits, option surcharges
	// included, so the full-payload cases re-send something real.
	var latte dialogProduct
	decodeInto(t, "fixture create latte",
		s.write(t, "fixture create latte", http.MethodPost, "/api/products", owner, map[string]any{
			"category_id": categoryA.ID,
			"translations": map[string]any{"tr": map[string]any{
				"name": "Latte", "description": "Sıcak ve yumuşak", "ingredients": "Espresso, süt",
			}},
			"price":         145,
			"compare_price": nil,
			"calories":      220,
			"image_url":     nil,
			"allergens":     []string{"sut"},
			"badges":        []map[string]any{{"text": "Yeni"}},
			"options": []map[string]any{{
				"name": "Süt Tercihi", "type": "single", "required": false,
				"items": []map[string]any{
					{"name": "Yulaf Sütü", "price": 25},
					{"name": "Laktozsuz", "price": 10},
				},
			}},
			"is_active":   true,
			"is_featured": false,
		}), &latte)
	if latte.ID == "" {
		t.Fatalf("fixture create latte: no id in the response")
	}
	h.createProduct(owner, categoryA.ID, "Filtre Kahve", 90)
	h.createProduct(owner, categoryB.ID, "Limonata", 110)
	h.createProduct(stranger, categoryForeign.ID, "Sütlaç", 120)

	// Every read path has to work before a single case can mean anything.
	s.pin(t, menuA, menuB, menuForeign)
	for _, menu := range []watchedMenu{menuA, menuB, menuForeign} {
		s.expect(t, "fixture baseline", menu, false)
	}
	if t.Failed() {
		t.FailNow()
	}

	lattePath := "/api/products/" + latte.ID

	t.Run("1_put_with_a_changed_price_moves", func(t *testing.T) {
		s.pin(t, menuA)
		product := s.product(t, owner, latte.ID)
		want := cents(product.Price + 5)

		body := product.payload()
		body["price"] = want
		s.write(t, "PUT with a new price", http.MethodPut, lattePath, owner, body)

		if after := s.product(t, owner, latte.ID); after.Price != want {
			t.Fatalf("PUT with a new price: the price was not written (%.2f, expected %.2f)",
				after.Price, want)
		}
		s.expect(t, "PUT with a new price", menuA, true)
	})

	t.Run("2_put_with_the_same_price_and_a_new_description_does_not_move", func(t *testing.T) {
		s.pin(t, menuA)
		product := s.product(t, owner, latte.ID)
		text := product.Translations["tr"]
		text.Description = "Yeni tarif, aynı fiyat"
		product.Translations["tr"] = text

		s.write(t, "PUT with the same price and a new description", http.MethodPut, lattePath,
			owner, product.payload())

		if after := s.product(t, owner, latte.ID); after.Translations["tr"].Description != text.Description {
			t.Fatalf("PUT with the same price and a new description: the description was not "+
				"written (%q)", after.Translations["tr"].Description)
		}
		s.expect(t, "PUT with the same price and a new description", menuA, false)
	})

	t.Run("3_patch_price", func(t *testing.T) {
		t.Run("a_new_price_moves", func(t *testing.T) {
			s.pin(t, menuA)
			product := s.product(t, owner, latte.ID)
			s.write(t, "PATCH a new price", http.MethodPatch, lattePath+"/price", owner,
				map[string]any{"price": cents(product.Price + 1)})
			s.expect(t, "PATCH a new price", menuA, true)
		})

		t.Run("the_same_price_does_not_move", func(t *testing.T) {
			s.pin(t, menuA)
			product := s.product(t, owner, latte.ID)
			s.write(t, "PATCH the same price", http.MethodPatch, lattePath+"/price", owner,
				map[string]any{"price": product.Price})
			s.expect(t, "PATCH the same price", menuA, false)
		})
	})

	t.Run("4_put_with_only_compare_price_changed_moves", func(t *testing.T) {
		s.pin(t, menuA)
		product := s.product(t, owner, latte.ID)
		compare := cents(product.Price + 30)
		if product.ComparePrice != nil && *product.ComparePrice == compare {
			compare = cents(compare + 10)
		}

		body := product.payload()
		body["compare_price"] = compare
		s.write(t, "PUT with only compare_price changed", http.MethodPut, lattePath, owner, body)

		if after := s.product(t, owner, latte.ID); after.ComparePrice == nil || *after.ComparePrice != compare {
			t.Fatalf("PUT with only compare_price changed: compare_price was not written (%v, expected %.2f)",
				after.ComparePrice, compare)
		}
		s.expect(t, "PUT with only compare_price changed", menuA, true)
	})

	t.Run("5_put_options", func(t *testing.T) {
		t.Run("a_changed_surcharge_moves", func(t *testing.T) {
			s.pin(t, menuA)
			product := s.product(t, owner, latte.ID)
			requireSurcharges(t, product)
			product.Options[0].Items[0].Price = cents(product.Options[0].Items[0].Price + 5)
			want := product.Options[0].Items[0].Price

			s.write(t, "PUT with a changed surcharge", http.MethodPut, lattePath, owner, product.payload())

			if after := s.product(t, owner, latte.ID); after.Options[0].Items[0].Price != want {
				t.Fatalf("PUT with a changed surcharge: the surcharge was not written (%.2f, expected %.2f)",
					after.Options[0].Items[0].Price, want)
			}
			s.expect(t, "PUT with a changed surcharge", menuA, true)
		})

		t.Run("a_renamed_option_with_identical_surcharges_does_not_move", func(t *testing.T) {
			s.pin(t, menuA)
			product := s.product(t, owner, latte.ID)
			requireSurcharges(t, product)
			const renamed = "Laktozsuz Süt"
			product.Options[0].Items[1].Name = renamed

			s.write(t, "PUT renaming an option", http.MethodPut, lattePath, owner, product.payload())

			if after := s.product(t, owner, latte.ID); after.Options[0].Items[1].Name != renamed {
				t.Fatalf("PUT renaming an option: the name was not written (%q)", after.Options[0].Items[1].Name)
			}
			s.expect(t, "PUT renaming an option, surcharges identical", menuA, false)
		})

		t.Run("identical_surcharges_spelled_10_0_do_not_move", func(t *testing.T) {
			s.pin(t, menuA)
			product := s.product(t, owner, latte.ID)
			requireSurcharges(t, product)
			decimals := optionsSpelledWithDecimals(t, product.Options)

			body := product.payload()
			body["options"] = decimals
			s.write(t, "PUT re-sending the surcharges as 10.0", http.MethodPut, lattePath, owner, body)
			s.expect(t, "PUT re-sending the surcharges as 10.0", menuA, false)

			// The handler decodes every surcharge into a float64, so whatever the
			// payload spelled, the column received 10 again — the step above cannot
			// tell a numeric comparison from a textual one. Writing the decimal
			// spelling into the column itself can: jsonb keeps 10.0 as written,
			// and only a numeric comparison calls it the same surcharge.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := h.pool.Exec(ctx,
				`UPDATE products SET options = $1::jsonb WHERE id = $2::text::uuid`,
				string(decimals), latte.ID); err != nil {
				t.Fatalf("could not store the decimal spelling of the surcharges: %v", err)
			}
			var column string
			if err := h.pool.QueryRow(ctx,
				`SELECT options::text FROM products WHERE id = $1::text::uuid`, latte.ID).Scan(&column); err != nil {
				t.Fatalf("could not read the options column back: %v", err)
			}
			if !strings.Contains(column, ".0") {
				t.Fatalf("fixture: the options column did not keep the decimal spelling: %s", column)
			}
			s.expect(t, "the surcharges rewritten as 10.0 in the column itself", menuA, false)

			// And back again: the dialog re-saves what it loaded, and the column
			// receives 10 where it held 10.0.
			s.write(t, "PUT re-saving surcharges stored as 10.0", http.MethodPut, lattePath, owner,
				s.product(t, owner, latte.ID).payload())
			s.expect(t, "PUT re-saving surcharges stored as 10.0", menuA, false)
		})
	})

	t.Run("6_put_is_active_only_does_not_move", func(t *testing.T) {
		s.pin(t, menuA)
		s.write(t, "PUT is_active false", http.MethodPut, lattePath, owner,
			map[string]any{"is_active": false})
		if s.product(t, owner, latte.ID).IsActive {
			t.Fatalf("PUT is_active false: the flag was not written")
		}
		s.expect(t, "PUT hiding a product", menuA, false)

		s.write(t, "PUT is_active true", http.MethodPut, lattePath, owner,
			map[string]any{"is_active": true})
		s.expect(t, "PUT showing it again", menuA, false)
	})

	t.Run("7_create_does_not_move", func(t *testing.T) {
		s.pin(t, menuA)
		s.write(t, "POST a new product", http.MethodPost, "/api/products", owner, map[string]any{
			"category_id":  categoryA.ID,
			"translations": map[string]any{"tr": map[string]any{"name": "Kurabiye"}},
			"price":        45,
		})
		s.expect(t, "POST a new product", menuA, false)
	})

	t.Run("8_bulk_price", func(t *testing.T) {
		t.Run("apply_with_a_percentage_moves", func(t *testing.T) {
			s.pin(t, menuA)
			var result bulkPriceResponse
			decodeInto(t, "bulk +10%",
				s.write(t, "bulk +10%", http.MethodPost, "/api/products/bulk-price", owner, map[string]any{
					"menu_id": menuA.id, "percentage": 10, "rounding": "ends_95", "apply": true,
				}), &result)
			if !result.Applied || result.Affected == 0 {
				t.Fatalf("bulk +10%%: expected an applied update that changed prices, got applied=%t affected=%d",
					result.Applied, result.Affected)
			}

			stored := s.expect(t, "bulk +10%", menuA, true)
			if result.PriceUpdatedAt == nil {
				t.Fatalf("bulk +10%%: the response carried no price_updated_at")
			}
			if !result.PriceUpdatedAt.Equal(stored) {
				t.Errorf("bulk +10%%: the response says price_updated_at %s but the menu stores %s",
					result.PriceUpdatedAt.UTC().Format(time.RFC3339Nano), stored.UTC().Format(time.RFC3339Nano))
			}

			// The prices now travel as one numeric[] instead of one parameter per
			// row, so every promised price is checked against what is stored, to
			// the kuruş — ends_95 makes them all two-decimal values.
			for _, row := range result.Preview {
				if got := s.product(t, owner, row.ID).Price; got != row.NewPrice {
					t.Errorf("bulk +10%%: %q was previewed at %.2f but stores %.2f", row.Name, row.NewPrice, got)
				}
			}
		})

		t.Run("a_preview_does_not_move", func(t *testing.T) {
			s.pin(t, menuA)
			var result bulkPriceResponse
			decodeInto(t, "bulk preview",
				s.write(t, "bulk preview", http.MethodPost, "/api/products/bulk-price", owner, map[string]any{
					"menu_id": menuA.id, "percentage": 10, "rounding": "none", "apply": false,
				}), &result)
			if result.Applied {
				t.Fatalf("bulk preview: the response says applied=true for apply:false")
			}
			s.expect(t, "bulk preview (apply:false)", menuA, false)
		})

		t.Run("apply_with_nothing_to_change_does_not_move", func(t *testing.T) {
			s.pin(t, menuA)
			var result bulkPriceResponse
			decodeInto(t, "bulk 0%",
				s.write(t, "bulk 0%", http.MethodPost, "/api/products/bulk-price", owner, map[string]any{
					"menu_id": menuA.id, "percentage": 0, "rounding": "none", "apply": true,
				}), &result)
			if result.Affected != 0 {
				t.Errorf("bulk 0%%: affected = %d, expected 0 — a zero percentage changes no price",
					result.Affected)
			}

			stored := s.expect(t, "bulk apply:true with percentage 0", menuA, false)
			if result.PriceUpdatedAt == nil || !result.PriceUpdatedAt.Equal(stored) {
				t.Errorf("bulk 0%%: the response's price_updated_at %v is not the menu's stored %s",
					result.PriceUpdatedAt, stored.UTC().Format(time.RFC3339Nano))
			}
		})
	})

	t.Run("9_a_price_change_leaves_other_menus_untouched", func(t *testing.T) {
		s.pin(t, menuA, menuB, menuForeign)
		product := s.product(t, owner, latte.ID)
		s.write(t, "PATCH a price in menu A", http.MethodPatch, lattePath+"/price", owner,
			map[string]any{"price": cents(product.Price + 1)})

		s.expect(t, "a price change in menu A", menuA, true)
		s.expect(t, "a price change in menu A", menuB, false)
		s.expect(t, "a price change in menu A", menuForeign, false)
	})

	t.Run("10_moving_into_menu_b_with_a_new_price_dates_menu_b_only", func(t *testing.T) {
		s.pin(t, menuA, menuB)
		product := s.product(t, owner, latte.ID)

		body := product.payload()
		body["category_id"] = categoryB.ID
		body["price"] = cents(product.Price + 2)
		s.write(t, "PUT moving the product into menu B with a new price", http.MethodPut, lattePath,
			owner, body)

		if after := s.product(t, owner, latte.ID); after.CategoryID != categoryB.ID {
			t.Fatalf("PUT moving the product into menu B: it is still in category %s", after.CategoryID)
		}
		s.expect(t, "moving a product into menu B with a new price", menuB, true)
		s.expect(t, "moving a product into menu B with a new price", menuA, false)
	})
}
