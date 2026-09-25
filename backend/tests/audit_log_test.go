package tests

// The audit trail over HTTP: every administrative write the contract lists
// leaves exactly one row, saying who, from where, and which fields changed —
// and nothing it must not say.
//
// Beyond "a row exists", the suite pins down the properties that make a trail
// trustworthy:
//
//   - a write and its row commit together: when the row cannot be written the
//     write is rolled back too, and a refused request writes neither;
//   - a write that RetryOnConflict runs again is recorded once;
//   - a Wi-Fi password is stored masked and an account password in no form;
//   - one tenant never reads another's trail.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/config"
	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
)

type auditRow struct {
	ID          string                                        `json:"id"`
	CreatedAt   string                                        `json:"created_at"`
	UserID      *string                                       `json:"user_id"`
	UserEmail   string                                        `json:"user_email"`
	Action      string                                        `json:"action"`
	EntityType  string                                        `json:"entity_type"`
	EntityID    *string                                       `json:"entity_id"`
	EntityLabel string                                        `json:"entity_label"`
	Changes     map[string]struct{ Old, New json.RawMessage } `json:"changes"`
	IP          *string                                       `json:"ip"`
	Port        *int                                          `json:"port"`
	IPSource    string                                        `json:"ip_source"`
}

type auditPage struct {
	Items  []auditRow `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// auditSuite drives every request of one owner from one proven address, so each
// row can be checked for it.
type auditSuite struct {
	h       *harness
	owner   *tenant
	headers map[string]string
}

const (
	auditIP   = "198.51.100.33"
	auditPort = "45678"
)

func (s *auditSuite) do(t *testing.T, method, path string, body any) (*http.Response, []byte) {
	t.Helper()
	return s.h.doWith(method, path, s.owner.session, body, s.headers)
}

func (s *auditSuite) must(t *testing.T, what, method, path string, body any) []byte {
	t.Helper()
	resp, payload := s.do(t, method, path, body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s: %s %s answered %d: %s", what, method, path, resp.StatusCode, payload)
	}
	return payload
}

func (s *auditSuite) logs(t *testing.T, query string) auditPage {
	t.Helper()
	resp, payload := s.h.do(http.MethodGet, "/api/audit-logs"+query, s.owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/audit-logs%s answered %d: %s", query, resp.StatusCode, payload)
	}
	var page auditPage
	decodeInto(t, "audit logs", payload, &page)
	return page
}

// only returns the rows of one action, newest first.
func (s *auditSuite) only(t *testing.T, action string) []auditRow {
	t.Helper()
	return s.logs(t, "?limit=200&action="+action).Items
}

func rawString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("%s is not a string: %v", raw, err)
	}
	return s
}

func TestAuditTrail(t *testing.T) {
	h := newHarnessConfigured(t, nil, func(cfg *config.Config) {
		cfg.EdgeSecretHeader = "X-Edge-Secret"
		cfg.EdgeSecret = analyticsEdgeSecret
		cfg.ClientPortHeader = analyticsPortHeader
	})
	const email = "audit-owner@example.test"
	owner := h.register("owner", "Denetim Kafe", email)
	other := h.register("other", "Öteki Kafe", "audit-other@example.test")
	s := &auditSuite{h: h, owner: owner, headers: fromCloudflare(auditIP, auditPort)}

	// ------------------------------------------------------------ the writes
	var menu menuPayload
	decodeInto(t, "menu", s.must(t, "create menu", http.MethodPost, "/api/menus", map[string]any{
		"name": "Ana Menü", "phone": "+90 555 000 00 00",
	}), &menu)

	s.must(t, "update menu", http.MethodPut, "/api/menus/"+menu.ID, map[string]any{
		"phone":             "+90 555 111 11 11",
		"wifi_ssid":         "Kafe-Misafir",
		"wifi_password":     "cok-gizli-wifi",
		"logo_url":          "/uploads/logo.svg",
		"contact_display":   "footer",
		"languages":         []string{"tr", "en"},
		"show_yerli_uretim": false,
	})
	// A save that changes nothing records nothing.
	s.must(t, "resave menu", http.MethodPut, "/api/menus/"+menu.ID, map[string]any{"phone": "+90 555 111 11 11"})

	category := func(name string) categoryPayload {
		var c categoryPayload
		decodeInto(t, "category", s.must(t, "create category "+name, http.MethodPost, "/api/categories", map[string]any{
			"menu_id": menu.ID, "translations": map[string]any{"tr": map[string]any{"name": name}},
		}), &c)
		return c
	}
	coffee, food := category("Kahveler"), category("Yiyecekler")
	s.must(t, "update category", http.MethodPut, "/api/categories/"+food.ID, map[string]any{
		"translations": map[string]any{
			"tr": map[string]any{"name": "Yiyecekler"},
			"en": map[string]any{"name": "Food"},
		},
	})
	s.must(t, "reorder categories", http.MethodPut, "/api/categories/reorder", map[string]any{
		"ids": []string{food.ID, coffee.ID},
	})

	product := func(categoryID, name string, price float64) productPayload {
		var p productPayload
		decodeInto(t, "product", s.must(t, "create product "+name, http.MethodPost, "/api/products", map[string]any{
			"category_id": categoryID, "translations": map[string]any{"tr": map[string]any{"name": name}},
			"price": price,
		}), &p)
		return p
	}
	latte, mocha := product(coffee.ID, "Latte", 90), product(coffee.ID, "Mocha", 100)
	sandwich := product(food.ID, "Ciabatta Sandwich", 180)

	s.must(t, "update product", http.MethodPut, "/api/products/"+latte.ID, map[string]any{
		"translations": map[string]any{
			"tr": map[string]any{"name": "Latte", "ingredients": "Espresso, süt"},
			"en": map[string]any{"name": "Latte"},
		},
		"price": 95,
	})
	s.must(t, "patch price", http.MethodPatch, "/api/products/"+mocha.ID+"/price", map[string]any{"price": 110})
	s.must(t, "patch the same price", http.MethodPatch, "/api/products/"+mocha.ID+"/price", map[string]any{"price": 110})
	s.must(t, "bulk price", http.MethodPost, "/api/products/bulk-price", map[string]any{
		"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": true,
	})
	s.must(t, "bulk preview", http.MethodPost, "/api/products/bulk-price", map[string]any{
		"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": false,
	})
	s.must(t, "reorder products", http.MethodPut, "/api/products/reorder", map[string]any{
		"category_id": coffee.ID, "ids": []string{mocha.ID, latte.ID},
	})
	s.must(t, "delete product", http.MethodDelete, "/api/products/"+sandwich.ID, nil)
	s.must(t, "delete category", http.MethodDelete, "/api/categories/"+food.ID, nil)
	s.must(t, "update business", http.MethodPut, "/api/business", map[string]any{"name": "Denetim Kafe & Bistro"})
	s.must(t, "change password", http.MethodPost, "/api/auth/change-password", map[string]any{
		"current_password": "karecik-test-password", "new_password": "yeni-parola-2026",
	})

	var second menuPayload
	decodeInto(t, "second menu", s.must(t, "create second menu", http.MethodPost, "/api/menus",
		map[string]any{"name": "Silinecek Menü"}), &second)
	s.must(t, "delete menu", http.MethodDelete, "/api/menus/"+second.ID, nil)

	// A refused write records nothing.
	if resp, _ := s.do(t, http.MethodPut, "/api/products/"+latte.ID, map[string]any{"price": -1}); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a negative price answered %d", resp.StatusCode)
	}
	// Another tenant's product: a 404 and no row in either trail.
	if resp, _ := h.do(http.MethodDelete, "/api/products/"+latte.ID, other.session, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("another tenant's delete answered %d", resp.StatusCode)
	}

	// ------------------------------------------------------------ the trail
	t.Run("one_row_per_write", func(t *testing.T) {
		want := map[string]int{
			audit.ActionMenuCreate:       2,
			audit.ActionMenuUpdate:       1,
			audit.ActionMenuDelete:       1,
			audit.ActionCategoryCreate:   2,
			audit.ActionCategoryUpdate:   1,
			audit.ActionCategoryReorder:  1,
			audit.ActionCategoryDelete:   1,
			audit.ActionProductCreate:    3,
			audit.ActionProductUpdate:    1,
			audit.ActionProductPrice:     1,
			audit.ActionProductBulkPrice: 1,
			audit.ActionProductReorder:   1,
			audit.ActionProductDelete:    1,
			audit.ActionBusinessUpdate:   1,
			audit.ActionPasswordChange:   1,
		}
		all := s.logs(t, "?limit=200")
		got := map[string]int{}
		for _, row := range all.Items {
			got[row.Action]++
		}
		for action, n := range want {
			if got[action] != n {
				t.Errorf("%s: %d row(s), want %d", action, got[action], n)
			}
		}
		total := 0
		for _, n := range want {
			total += n
		}
		if all.Total != total || len(all.Items) != total {
			t.Errorf("the trail holds %d rows (%d listed), want %d: %v", all.Total, len(all.Items), total, got)
		}
		for i := 1; i < len(all.Items); i++ {
			if all.Items[i-1].CreatedAt < all.Items[i].CreatedAt {
				t.Errorf("the trail is not newest first at row %d", i)
			}
		}
	})

	t.Run("who_and_where", func(t *testing.T) {
		for _, row := range s.logs(t, "?limit=200").Items {
			if row.UserID == nil || row.UserEmail != email {
				t.Errorf("%s: user %v <%s>, want the owner", row.Action, row.UserID, row.UserEmail)
			}
			if row.IP == nil || *row.IP != auditIP || row.Port == nil || fmt.Sprint(*row.Port) != auditPort ||
				row.IPSource != "cloudflare" {
				t.Errorf("%s: recorded %v:%v via %s, want %s:%s via cloudflare",
					row.Action, row.IP, row.Port, row.IPSource, auditIP, auditPort)
			}
		}
	})

	t.Run("menu_update_records_every_changed_setting_and_masks_the_wifi_password", func(t *testing.T) {
		rows := s.only(t, audit.ActionMenuUpdate)
		if len(rows) != 1 {
			t.Fatalf("%d menu.update rows", len(rows))
		}
		row := rows[0]
		if row.EntityID == nil || *row.EntityID != menu.ID || row.EntityLabel != "Ana Menü" || row.EntityType != "menu" {
			t.Errorf("menu.update describes %v %q (%s)", row.EntityID, row.EntityLabel, row.EntityType)
		}
		for _, key := range []string{"phone", "wifi_ssid", "wifi_password", "logo_url", "contact_display",
			"contact_in_footer", "languages", "show_yerli_uretim"} {
			if _, ok := row.Changes[key]; !ok {
				t.Errorf("menu.update does not record %s: %v", key, row.Changes)
			}
		}
		if phone := row.Changes["phone"]; rawString(t, phone.Old) != "+90 555 000 00 00" || rawString(t, phone.New) != "+90 555 111 11 11" {
			t.Errorf("phone recorded as %s -> %s", phone.Old, phone.New)
		}
		if wifi := row.Changes["wifi_password"]; string(wifi.Old) != "null" || rawString(t, wifi.New) != audit.MaskedSecret {
			t.Errorf("wifi_password recorded as %s -> %s, want null -> ••••", wifi.Old, wifi.New)
		}
		if len(row.Changes) != 8 {
			t.Errorf("menu.update records %d fields, want exactly the 8 that changed: %v", len(row.Changes), row.Changes)
		}
	})

	t.Run("product_changes_are_spelled_per_language_and_field", func(t *testing.T) {
		update := s.only(t, audit.ActionProductUpdate)[0]
		for _, key := range []string{"price", "translations.en.name", "translations.tr.ingredients"} {
			if _, ok := update.Changes[key]; !ok {
				t.Errorf("product.update lacks %s: %v", key, update.Changes)
			}
		}
		if update.EntityLabel != "Latte" {
			t.Errorf("product.update label %q", update.EntityLabel)
		}

		price := s.only(t, audit.ActionProductPrice)[0]
		if string(price.Changes["price"].Old) != "100" || string(price.Changes["price"].New) != "110" ||
			len(price.Changes) != 1 {
			t.Errorf("product.price recorded %v, want price 100 -> 110 only", price.Changes)
		}

		bulk := s.only(t, audit.ActionProductBulkPrice)[0]
		if bulk.EntityLabel != "Ana Menü" || string(bulk.Changes["percentage"].New) != "10" ||
			string(bulk.Changes["affected"].New) != "3" {
			t.Errorf("product.bulk_price recorded %q %v", bulk.EntityLabel, bulk.Changes)
		}
		var oldPrices, newPrices []struct {
			Name  string  `json:"name"`
			Price float64 `json:"price"`
		}
		_ = json.Unmarshal(bulk.Changes["prices"].Old, &oldPrices)
		_ = json.Unmarshal(bulk.Changes["prices"].New, &newPrices)
		if len(oldPrices) != 3 || len(newPrices) != 3 {
			t.Fatalf("the bulk row lists %d/%d prices, want 3", len(oldPrices), len(newPrices))
		}
		for i := range oldPrices {
			if oldPrices[i].Name != newPrices[i].Name || newPrices[i].Price <= oldPrices[i].Price {
				t.Errorf("bulk price %d: %+v -> %+v", i, oldPrices[i], newPrices[i])
			}
		}

		reorder := s.only(t, audit.ActionProductReorder)[0]
		if reorder.EntityLabel != "Kahveler" || string(reorder.Changes["order"].New) != `["Mocha","Latte"]` {
			t.Errorf("product.reorder recorded %q %s", reorder.EntityLabel, reorder.Changes["order"].New)
		}
		deleted := s.only(t, audit.ActionProductDelete)[0]
		if deleted.EntityLabel != "Ciabatta Sandwich" || deleted.EntityID == nil || *deleted.EntityID != sandwich.ID {
			t.Errorf("product.delete describes %v %q", deleted.EntityID, deleted.EntityLabel)
		}
		if _, ok := deleted.Changes["price"]; !ok {
			t.Errorf("product.delete keeps no snapshot: %v", deleted.Changes)
		}
	})

	t.Run("categories_business_and_menus", func(t *testing.T) {
		if row := s.only(t, audit.ActionCategoryUpdate)[0]; row.EntityLabel != "Yiyecekler" ||
			rawString(t, row.Changes["translations.en.name"].New) != "Food" {
			t.Errorf("category.update: %q %v", row.EntityLabel, row.Changes)
		}
		if row := s.only(t, audit.ActionCategoryReorder)[0]; string(row.Changes["order"].New) != `["Yiyecekler","Kahveler"]` {
			t.Errorf("category.reorder: %s", row.Changes["order"].New)
		}
		if row := s.only(t, audit.ActionCategoryDelete)[0]; row.EntityLabel != "Yiyecekler" ||
			string(row.Changes["deleted_products"].New) != "0" {
			t.Errorf("category.delete: %q %v", row.EntityLabel, row.Changes)
		}
		if row := s.only(t, audit.ActionBusinessUpdate)[0]; rawString(t, row.Changes["name"].New) != "Denetim Kafe & Bistro" ||
			len(row.Changes) != 1 {
			t.Errorf("business.update: %v", row.Changes)
		}
		creates := s.only(t, audit.ActionMenuCreate)
		first := creates[len(creates)-1]
		if first.EntityLabel != "Ana Menü" || rawString(t, first.Changes["phone"].New) != "+90 555 000 00 00" {
			t.Errorf("menu.create: %q %v", first.EntityLabel, first.Changes)
		}
		if _, ok := first.Changes["theme"]; ok {
			t.Error("menu.create records a default the request never named")
		}
		if row := s.only(t, audit.ActionMenuDelete)[0]; row.EntityLabel != "Silinecek Menü" {
			t.Errorf("menu.delete: %q", row.EntityLabel)
		}
	})

	t.Run("no_password_in_any_form", func(t *testing.T) {
		row := s.only(t, audit.ActionPasswordChange)[0]
		if row.EntityType != "account" || row.EntityLabel != email || rawString(t, row.Changes["method"].New) != "dashboard" {
			t.Errorf("account.password_change: %s %q %v", row.EntityType, row.EntityLabel, row.Changes)
		}
		var hash string
		if err := h.pool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE email = $1`, email).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		var dump string
		if err := h.pool.QueryRow(context.Background(),
			`SELECT COALESCE(string_agg(changes::text || entity_label, ' '), '') FROM audit_logs`).Scan(&dump); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"karecik-test-password", "yeni-parola-2026", hash, hash[7:], "cok-gizli-wifi"} {
			if strings.Contains(dump, secret) {
				t.Errorf("the trail holds a secret (%q…)", secret[:4])
			}
		}
	})

	t.Run("filters_paging_and_refusals", func(t *testing.T) {
		products := s.logs(t, "?entity_type=product&limit=200")
		for _, row := range products.Items {
			if row.EntityType != "product" {
				t.Errorf("entity_type=product listed a %s row", row.EntityType)
			}
		}
		if products.Total != 8 {
			t.Errorf("entity_type=product: %d rows, want 8", products.Total)
		}
		page := s.logs(t, "?limit=2&offset=1")
		if len(page.Items) != 2 || page.Limit != 2 || page.Offset != 1 {
			t.Errorf("paging: %d items, limit %d, offset %d", len(page.Items), page.Limit, page.Offset)
		}
		if capped := s.logs(t, "?limit=5000"); capped.Limit != 200 {
			t.Errorf("limit 5000 answered with %d", capped.Limit)
		}
		for _, bad := range []string{"?entity_type=user", "?action=product.explode", "?limit=x", "?offset=-5"} {
			if resp, payload := h.do(http.MethodGet, "/api/audit-logs"+bad, owner.session, nil); resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("%s answered %d: %s", bad, resp.StatusCode, payload)
			}
		}
		if resp, _ := h.do(http.MethodGet, "/api/audit-logs", "", nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("an anonymous trail answered %d", resp.StatusCode)
		}
	})

	t.Run("tenant_isolation", func(t *testing.T) {
		resp, payload := h.do(http.MethodGet, "/api/audit-logs?limit=200", other.session, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the other tenant's trail answered %d", resp.StatusCode)
		}
		var page auditPage
		decodeInto(t, "other trail", payload, &page)
		if page.Total != 0 || len(page.Items) != 0 {
			t.Errorf("the other tenant, who wrote nothing, reads %d rows", page.Total)
		}
	})

	t.Run("a_write_and_its_row_commit_together", func(t *testing.T) {
		// Make every product.price row impossible to write; the price edit it
		// belongs to must then be rolled back with it.
		ctx := context.Background()
		if _, err := h.pool.Exec(ctx, `ALTER TABLE audit_logs ADD CONSTRAINT test_block_price
			CHECK (action <> 'product.price') NOT VALID`); err != nil {
			t.Fatal(err)
		}
		defer h.pool.Exec(ctx, `ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS test_block_price`)

		resp, _ := s.do(t, http.MethodPatch, "/api/products/"+latte.ID+"/price", map[string]any{"price": 777})
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("a price edit whose audit row failed answered %d", resp.StatusCode)
		}
		var price float64
		if err := h.pool.QueryRow(ctx, `SELECT price FROM products WHERE id = $1::text::uuid`, latte.ID).Scan(&price); err != nil {
			t.Fatal(err)
		}
		if price == 777 {
			t.Error("the price was written although its audit row was not")
		}
	})
}

// A write that RetryOnConflict runs again is recorded by the run that commits,
// once. The first run's hook writes its row and then fails as PostgreSQL does
// when it aborts a deadlock victim; the retry succeeds.
func TestAuditRowOfARetriedWriteIsNotDuplicated(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Tekrar Kafe", "retry-owner@example.test")
	menu := h.createMenu(owner, "Ana Menü")
	category := h.createCategory(owner, menu.ID, "Kahveler")
	product := h.createProduct(owner, category.ID, "Latte", 50)

	businessID := uuid.MustParse(owner.businessID)
	productID := uuid.MustParse(product.ID)
	ctx := context.Background()

	runs := 0
	hook := func(ctx context.Context, tx pgx.Tx, change repository.Update[*models.Product]) error {
		runs++
		// Every run reads the product afresh, inside its own transaction.
		if change.Before == nil || change.Before.Price != 50 || change.After.Price != 60 {
			t.Errorf("run %d was handed %+v -> %+v, want price 50 -> 60", runs, change.Before, change.After)
		}
		if err := repository.InsertAuditLog(ctx, tx, audit.Entry{
			BusinessID: businessID, Action: audit.ActionProductPrice, EntityType: audit.EntityProduct,
			EntityID: change.After.ID.String(),
			Changes:  audit.Changes{"price": {Old: change.Before.Price, New: change.After.Price}},
		}); err != nil {
			return err
		}
		if runs == 1 {
			return &pgconn.PgError{Code: "40P01", Message: "deadlock detected (simulated)"}
		}
		return nil
	}

	_, err := repository.RetryOnConflictValue(ctx, "TestRetry", func() (*models.Product, error) {
		return repository.UpdateProduct(ctx, h.pool, productID, businessID, map[string]any{"price": 60.0}, hook)
	})
	if err != nil {
		t.Fatalf("the retried write failed: %v", err)
	}
	if runs != 2 {
		t.Fatalf("the hook ran %d times, want 2 (one aborted run, one retry)", runs)
	}
	var rows int
	// The fixture's own product.create row shares the entity id; only the
	// price rows are this write's.
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE entity_id = $1 AND action = 'product.price'`, product.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("%d audit rows for one retried write, want 1", rows)
	}
}

// Every action and entity type of internal/audit satisfies the CHECK
// constraints of migration 014, so no audited write can fail on its own
// vocabulary.
func TestEveryAuditActionFitsTheSchema(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Sözlük Kafe", "vocabulary@example.test")
	businessID := uuid.MustParse(owner.businessID)
	ctx := context.Background()

	for _, action := range audit.Actions {
		entity := strings.SplitN(action, ".", 2)[0]
		if err := repository.InsertAuditLog(ctx, h.pool, audit.Entry{
			BusinessID: businessID, Action: action, EntityType: entity,
			IP: "-", Port: "-", IPSource: "",
		}); err != nil {
			t.Errorf("%s (%s) does not fit the schema: %v", action, entity, err)
		}
	}
	for _, entity := range audit.EntityTypes {
		if err := repository.InsertAuditLog(ctx, h.pool, audit.Entry{
			BusinessID: businessID, Action: audit.ActionUploadCreate, EntityType: entity,
		}); err != nil {
			t.Errorf("entity type %s does not fit the schema: %v", entity, err)
		}
	}
	// "-" became NULL, and the blank source "unknown".
	var nullIP, nullPort bool
	var source string
	if err := h.pool.QueryRow(ctx, `SELECT ip IS NULL, port IS NULL, ip_source FROM audit_logs LIMIT 1`).
		Scan(&nullIP, &nullPort, &source); err != nil {
		t.Fatal(err)
	}
	if !nullIP || !nullPort || source != "unknown" {
		t.Errorf("an unknown address was stored as ip-null=%v port-null=%v source=%q", nullIP, nullPort, source)
	}
	if err := repository.InsertAuditLog(ctx, h.pool, audit.Entry{
		BusinessID: businessID, Action: "product.explode", EntityType: "product",
	}); err == nil {
		t.Error("an unknown action was accepted by the schema")
	}
}

// A reset through the e-mailed link has no session, and is recorded all the
// same: under the account's business, by the account's user, with the method.
func TestPasswordResetIsAudited(t *testing.T) {
	mail := &recordingMailer{}
	h := newHarnessWith(t, mail)
	const email = "reset-audit@example.test"
	owner := h.register("owner", "Sıfırlama Kafe", email)

	resp, payload := h.doWith(http.MethodPost, "/api/auth/forgot-password", "",
		map[string]any{"email": email}, map[string]string{"X-Real-IP": "203.0.113.61"})
	h.requireSuccess("forgot password", resp, payload)
	token := tokenFromMail(t, mail.waitForMail(t, 1)[0])

	resp, payload = h.doWith(http.MethodPost, "/api/auth/reset-password", "",
		map[string]any{"token": token, "password": "tamamen-yeni-parola"},
		map[string]string{"X-Real-IP": "203.0.113.61"})
	h.requireSuccess("reset password", resp, payload)

	resp, payload = h.do(http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": email, "password": "tamamen-yeni-parola",
	})
	h.requireSuccess("login with the new password", resp, payload)
	session := sessionCookie(resp)

	resp, payload = h.do(http.MethodGet, "/api/audit-logs?action=account.password_change", session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit logs answered %d: %s", resp.StatusCode, payload)
	}
	var page auditPage
	decodeInto(t, "reset trail", payload, &page)
	if len(page.Items) != 1 {
		t.Fatalf("%d password_change rows, want 1: %s", len(page.Items), payload)
	}
	row := page.Items[0]
	if row.UserEmail != email || rawString(t, row.Changes["method"].New) != "reset_link" ||
		row.IP == nil || *row.IP != "203.0.113.61" || row.IPSource != "edge" {
		t.Errorf("the reset row: %+v", row)
	}
	if bytes.Contains(payload, []byte("tamamen-yeni-parola")) {
		t.Error("the trail holds the new password")
	}
	_ = owner
}
