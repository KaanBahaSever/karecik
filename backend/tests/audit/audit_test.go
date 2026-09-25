package audit_test

// The rules that decide what an audit row says, without a database: which
// fields count as changed, how translations are spelled, what a create and a
// delete record, and that a Wi-Fi password never reaches the trail.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/models"
)

func strPtr(s string) *string { return &s }

func product() *models.Product {
	compare := 60.0
	return &models.Product{
		ID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CategoryID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Translations: models.Translations{
			"tr": {Name: "Latte", Description: "Sütlü espresso", Ingredients: "Espresso, süt"},
			"en": {Name: "Latte", Description: "Espresso with milk"},
		},
		Price:        55,
		ComparePrice: &compare,
		Allergens:    []string{"sut"},
		Badges:       models.Badges{},
		Options:      models.ProductOptions{},
		IsActive:     true,
		Position:     3,
	}
}

func TestDiffProductRecordsOnlyWhatChanged(t *testing.T) {
	before := product()
	after := product()
	after.Price = 60
	after.Translations["en"] = models.Translation{Name: "Latte", Description: "Espresso and steamed milk"}
	after.Translations["de"] = models.Translation{Name: "Latte"}
	after.Position = 9 // a reorder's business, never an update's

	changes := audit.DiffProduct(before, after)
	want := []string{"price", "translations.de.name", "translations.en.description"}
	if got := changes.Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("changed fields %q, want %q", got, want)
	}
	if c := changes["price"]; c.Old != 55.0 || c.New != 60.0 {
		t.Errorf("price change %+v, want 55 -> 60", c)
	}
	if c := changes["translations.de.name"]; c.Old != nil || c.New != "Latte" {
		t.Errorf("a new translation reads %+v, want null -> Latte", c)
	}

	if len(audit.DiffProduct(before, product())) != 0 {
		t.Error("two identical products differ")
	}
}

// A translation field that goes from "" to absent — a language removed that
// never had a description — is not a change anybody made.
func TestDiffProductTreatsEmptyAndAbsentAlike(t *testing.T) {
	before := product()
	before.Translations["de"] = models.Translation{Name: "Latte", Description: ""}
	after := product()
	after.Translations["de"] = models.Translation{Name: "Latte"}
	if changes := audit.DiffProduct(before, after); len(changes) != 0 {
		t.Fatalf("\"\" -> absent was recorded: %v", changes.Keys())
	}
}

func TestCreateAndDeleteRecordTheValuesThatSaySomething(t *testing.T) {
	p := product()
	p.IsFeatured = false
	created := audit.DiffProduct(nil, p)
	for _, key := range []string{"price", "compare_price", "is_active", "allergens",
		"translations.tr.name", "translations.tr.ingredients", "category_id"} {
		change, ok := created[key]
		if !ok {
			t.Errorf("a create does not record %s: %v", key, created.Keys())
			continue
		}
		if change.Old != nil {
			t.Errorf("a create records %s with an old value %v", key, change.Old)
		}
	}
	for _, key := range []string{"is_featured", "badges", "options", "calories", "image_url", "id", "position"} {
		if _, ok := created[key]; ok {
			t.Errorf("a create records the empty or bookkeeping field %s", key)
		}
	}

	deleted := audit.DiffProduct(p, nil)
	if c, ok := deleted["price"]; !ok || c.Old != 55.0 || c.New != nil {
		t.Errorf("a delete records price as %+v, want 55 -> null", c)
	}
}

func TestDiffMenuCoversEverySettingAndMasksTheWifiPassword(t *testing.T) {
	before := &models.Menu{
		ID: uuid.New(), Name: "Ana Menü", Slug: "ana-menu",
		Phone: strPtr("+90 555 000 00 00"), WifiPassword: strPtr("eski-sifre"),
		LogoURL: strPtr("/uploads/old.png"), Theme: "modern-light",
		Languages: []string{"tr"}, ContactDisplay: "inline",
		Links:         models.MenuLinks{},
		ShowVatNote:   true,
		ShowPriceDate: true,
	}
	after := *before
	after.Phone = strPtr("+90 555 111 11 11")
	after.WifiPassword = strPtr("yeni-sifre")
	after.LogoURL = strPtr("/uploads/new.svg")
	after.Theme = "dark-elegant"
	after.Languages = []string{"tr", "en", "ar"}
	after.ContactDisplay = "hidden"
	after.ContactInFooter = true
	after.Links = models.MenuLinks{{ID: "a", Label: "Rezervasyon", URL: "https://r.example"}}
	after.ShowVatNote = false
	after.ShowYerliUretim = true
	after.CurrencySymbol = "$" // derived, never recorded on its own
	after.CategoryCount = 7    // computed
	after.MenuURL = "https://x"

	changes := audit.DiffMenu(before, &after)
	want := []string{"contact_display", "contact_in_footer", "languages", "links", "logo_url",
		"phone", "show_vat_note", "show_yerli_uretim", "theme", "wifi_password"}
	if got := changes.Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("changed settings %q, want %q", got, want)
	}

	wifi := changes["wifi_password"]
	if wifi.Old != audit.MaskedSecret || wifi.New != audit.MaskedSecret {
		t.Errorf("wifi_password recorded as %+v, want both sides masked", wifi)
	}
	for key, change := range changes {
		for _, secret := range []string{"eski-sifre", "yeni-sifre"} {
			if strings.Contains(stringOf(change.Old)+stringOf(change.New), secret) {
				t.Errorf("%s carries the Wi-Fi password in clear", key)
			}
		}
	}

	// Removing the password reads differently from changing it.
	removed := *before
	removed.WifiPassword = nil
	if c := audit.DiffMenu(before, &removed)["wifi_password"]; c.Old != audit.MaskedSecret || c.New != nil {
		t.Errorf("removing the Wi-Fi password recorded %+v, want •••• -> null", c)
	}
}

func TestOnlyKeepsTheNamedFields(t *testing.T) {
	menu := &models.Menu{Name: "Bar", Slug: "bar", Theme: "modern-light", Phone: strPtr("1")}
	got := audit.DiffMenu(nil, menu).Only("name", "slug", "phone", "missing")
	if keys := got.Keys(); !reflect.DeepEqual(keys, []string{"name", "phone", "slug"}) {
		t.Fatalf("Only kept %q", keys)
	}
}

func TestDiffBusiness(t *testing.T) {
	before := &models.Business{ID: uuid.New(), Name: "Melly", Slug: "melly", HomeURL: "a"}
	after := *before
	after.Slug = "melly-coffee"
	after.HomeURL = "b"
	if keys := audit.DiffBusiness(before, &after).Keys(); !reflect.DeepEqual(keys, []string{"slug"}) {
		t.Fatalf("business changes %q, want only slug", keys)
	}
}

func TestTheVocabularyIsClosed(t *testing.T) {
	for _, action := range audit.Actions {
		if !audit.IsAction(action) {
			t.Errorf("IsAction(%q) = false", action)
		}
		if !strings.Contains(action, ".") {
			t.Errorf("action %q is not <entity>.<verb>", action)
		}
	}
	for _, entity := range audit.EntityTypes {
		if !audit.IsEntityType(entity) {
			t.Errorf("IsEntityType(%q) = false", entity)
		}
	}
	for _, bad := range []string{"", "product", "PRODUCT.CREATE", "product.create ", "menu.drop"} {
		if audit.IsAction(bad) {
			t.Errorf("IsAction(%q) = true", bad)
		}
	}
	if audit.IsEntityType("user") {
		t.Error("IsEntityType(user) = true")
	}
}

func stringOf(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
