package models_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"karecik/backend/internal/models"
)

// The name of an option group or item in one language: its translation, then
// the menu's default language, then Name — the same order the product texts
// follow in Translations.Resolve.
func TestOptionResolveNameFallsBackThroughTheDefaultLanguageToName(t *testing.T) {
	item := models.ProductOptionItem{
		Name: "Büyük",
		Translations: models.OptionTranslations{
			"tr": {Name: "Büyük"},
			"en": {Name: "Large"},
			"ar": {Name: "كبير"},
		},
		Price: 15,
	}

	cases := []struct {
		lang, fallback, want string
	}{
		{"en", "tr", "Large"},
		{"ar", "tr", "كبير"},
		{"tr", "tr", "Büyük"},
		// No German name: the default language answers.
		{"de", "tr", "Büyük"},
		// After the menu's default language became English, a language without
		// a name of its own reads the English one, as the product name does.
		{"de", "en", "Large"},
		{"fr", "xx", "Büyük"},
	}
	for _, tc := range cases {
		if got := item.ResolveName(tc.lang, tc.fallback); got != tc.want {
			t.Errorf("ResolveName(%q, %q) = %q, want %q", tc.lang, tc.fallback, got, tc.want)
		}
	}

	// An option stored before translations existed has nothing but its name.
	legacy := models.ProductOptionGroup{Name: "Boy"}
	for _, lang := range []string{"tr", "en", "ar"} {
		if got := legacy.ResolveName(lang, "tr"); got != "Boy" {
			t.Errorf("legacy group ResolveName(%q) = %q, want %q", lang, got, "Boy")
		}
	}

	// A blank translation is no translation.
	blank := models.ProductOptionItem{Name: "Orta", Translations: models.OptionTranslations{"en": {Name: ""}}}
	if got := blank.ResolveName("en", "tr"); got != "Orta" {
		t.Errorf("a blank English name resolved to %q, want the default name %q", got, "Orta")
	}

	// A row with no Name at all never comes out of SanitizeOptions, but reading
	// one must still be deterministic: the first language code in order wins.
	nameless := models.ProductOptionItem{Translations: models.OptionTranslations{
		"ru": {Name: "Большой"}, "de": {Name: "Groß"}, "fr": {Name: "Grand"},
	}}
	for i := 0; i < 20; i++ {
		if got := nameless.ResolveName("tr", "tr"); got != "Groß" {
			t.Fatalf("a nameless item resolved to %q, want %q (the first code in order, de)", got, "Groß")
		}
	}
}

// The customer payload carries resolved names and no translations map, and
// resolving leaves the stored value untouched.
func TestProductOptionsResolveStripsTheTranslations(t *testing.T) {
	stored := models.ProductOptions{{
		Name:         "Süt Tercihi",
		Translations: models.OptionTranslations{"tr": {Name: "Süt Tercihi"}, "en": {Name: "Milk"}},
		Type:         models.OptionTypeMultiple,
		Required:     true,
		Items: []models.ProductOptionItem{
			{Name: "Yulaf Sütü", Translations: models.OptionTranslations{"tr": {Name: "Yulaf Sütü"}, "en": {Name: "Oat milk"}}, Price: 25},
			{Name: "Laktozsuz", Price: 10},
		},
	}}
	before, _ := json.Marshal(stored)

	resolved := stored.Resolve("en", "tr")
	want := models.ProductOptions{{
		Name:     "Milk",
		Type:     models.OptionTypeMultiple,
		Required: true,
		Items: []models.ProductOptionItem{
			{Name: "Oat milk", Price: 25},
			{Name: "Laktozsuz", Price: 10},
		},
	}}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("Resolve(en) =\n  %+v\nwant\n  %+v", resolved, want)
	}

	payload, _ := json.Marshal(resolved)
	if strings.Contains(string(payload), "translations") {
		t.Errorf("the resolved options still carry a translations key: %s", payload)
	}
	if after, _ := json.Marshal(stored); string(after) != string(before) {
		t.Errorf("Resolve modified the stored options:\n  before %s\n  after  %s", before, after)
	}

	// The payload carries [] and never null.
	for _, options := range []models.ProductOptions{nil, {}} {
		empty := options.Resolve("en", "tr")
		if empty == nil {
			t.Errorf("Resolve of %#v returned nil, want an empty list", options)
		}
		if encoded, _ := json.Marshal(empty); string(encoded) != "[]" {
			t.Errorf("Resolve of %#v encodes as %s, want []", options, encoded)
		}
	}
}

// Normalize trims translated names, drops the blank ones and removes a map left
// empty, so an untranslated option keeps the exact JSON it always had.
func TestNormalizeCleansOptionTranslations(t *testing.T) {
	options := models.ProductOptions{{
		Name:         " Boy ",
		Translations: models.OptionTranslations{"en": {Name: "  Size "}, "de": {Name: "   "}},
		Type:         "single",
		Items: []models.ProductOptionItem{
			{Name: "Küçük", Translations: models.OptionTranslations{"en": {Name: ""}}, Price: 0},
			{Name: "Büyük", Translations: models.OptionTranslations{}, Price: 10},
		},
	}}

	normalized := options.Normalize()
	if got := normalized[0].Translations; !reflect.DeepEqual(got, models.OptionTranslations{"en": {Name: "Size"}}) {
		t.Errorf("group translations = %#v, want only en: Size", got)
	}
	for i, item := range normalized[0].Items {
		if item.Translations != nil {
			t.Errorf("item %d kept an empty translations map: %#v", i, item.Translations)
		}
	}

	encoded, _ := json.Marshal(normalized[0].Items)
	if want := `[{"name":"Küçük","price":0},{"name":"Büyük","price":10}]`; string(encoded) != want {
		t.Errorf("untranslated items encode as %s, want %s", encoded, want)
	}
}

// An option stored before this change — a bare name and price — reads and
// writes back byte for byte. That is what keeps every stored row valid, and
// what keeps an audit diff of a product whose options nobody touched empty.
func TestLegacyOptionsRoundTripUnchanged(t *testing.T) {
	const stored = `[{"name":"Boy","type":"single","required":true,"items":[{"name":"Küçük","price":0},{"name":"Büyük","price":12.5}]}]`

	var options models.ProductOptions
	if err := json.Unmarshal([]byte(stored), &options); err != nil {
		t.Fatalf("a legacy options value did not decode: %v", err)
	}
	encoded, _ := json.Marshal(options.Normalize())
	if string(encoded) != stored {
		t.Errorf("a legacy options value changed on the way through:\n  stored %s\n  now    %s", stored, encoded)
	}
}
