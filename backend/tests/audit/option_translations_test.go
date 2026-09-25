package audit_test

// The audit trail of per-language option names. Options are diffed as one
// field, so a name edit in any language has to show up as a change of
// "options", and a save that stores the same options — the product dialog
// re-sends them on every save — has to record nothing.

import (
	"encoding/json"
	"reflect"
	"testing"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/handlers"
	"karecik/backend/internal/models"
)

// sanitized runs options through the handler exactly as a save does.
func sanitized(t *testing.T, options models.ProductOptions) models.ProductOptions {
	t.Helper()
	out, message := handlers.SanitizeOptions(options, "tr")
	if message != "" {
		t.Fatalf("SanitizeOptions refused the fixture: %q", message)
	}
	return out
}

func translatedMilk() models.ProductOptions {
	return models.ProductOptions{{
		Name:         "Süt Tercihi",
		Translations: models.OptionTranslations{"en": {Name: "Milk"}},
		Type:         "single",
		Items: []models.ProductOptionItem{
			{Name: "Yulaf Sütü", Translations: models.OptionTranslations{"en": {Name: "Oat milk"}}, Price: 25},
		},
	}}
}

func TestDiffProductRecordsAnOptionNameEditInAnyLanguage(t *testing.T) {
	before := product()
	before.Options = sanitized(t, translatedMilk())

	edited := translatedMilk()
	edited[0].Items[0].Translations["en"] = models.OptionTranslation{Name: "Oat drink"}
	edited[0].Items[0].Translations["ar"] = models.OptionTranslation{Name: "حليب الشوفان"}
	after := product()
	after.Options = sanitized(t, edited)

	changes := audit.DiffProduct(before, after)
	if got, want := changes.Keys(), []string{"options"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("changed fields %q, want %q", got, want)
	}
	// The row carries the whole list on both sides, translations included, so
	// the trail shows which language moved.
	encoded, _ := json.Marshal(changes["options"].New)
	var logged []map[string]any
	if err := json.Unmarshal(encoded, &logged); err != nil || len(logged) != 1 {
		t.Fatalf("the new options were not logged as a list: %s", encoded)
	}
	items, _ := logged[0]["items"].([]any)
	item, _ := items[0].(map[string]any)
	translations, _ := item["translations"].(map[string]any)
	if ar, _ := translations["ar"].(map[string]any); ar["name"] != "حليب الشوفان" {
		t.Errorf("the logged options lost the Arabic name: %s", encoded)
	}
}

// What the owner read back, saved again, is not a change — neither for a
// translated option nor for one stored before translations existed.
func TestDiffProductRecordsNothingForAnUnchangedOptionsSave(t *testing.T) {
	legacy := models.ProductOptions{{
		Name: "Boy", Type: "single", Required: true,
		Items: []models.ProductOptionItem{{Name: "Küçük", Price: 0}, {Name: "Büyük", Price: 12.5}},
	}}

	for name, stored := range map[string]models.ProductOptions{
		"translated": sanitized(t, translatedMilk()),
		"legacy":     legacy,
	} {
		t.Run(name, func(t *testing.T) {
			// The dialog echoes the stored JSON back; the handler sanitizes it.
			encoded, _ := json.Marshal(stored)
			var echoed models.ProductOptions
			if err := json.Unmarshal(encoded, &echoed); err != nil {
				t.Fatalf("the stored options did not decode: %v", err)
			}

			before, after := product(), product()
			before.Options = stored
			after.Options = sanitized(t, echoed)

			if changes := audit.DiffProduct(before, after); len(changes) != 0 {
				t.Errorf("an unchanged options save recorded %q", changes.Keys())
			}
		})
	}
}
