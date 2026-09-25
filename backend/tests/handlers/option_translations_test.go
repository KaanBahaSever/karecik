package handlers_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"karecik/backend/internal/handlers"
	"karecik/backend/internal/models"
)

// optionGroup is a one-group, one-item payload whose names are given per
// language, the way ProductModal sends them.
func optionGroup(groupName string, groupTranslations models.OptionTranslations,
	itemName string, itemTranslations models.OptionTranslations) models.ProductOptions {

	return models.ProductOptions{{
		Name:         groupName,
		Translations: groupTranslations,
		Type:         "single",
		Items: []models.ProductOptionItem{
			{Name: itemName, Translations: itemTranslations, Price: 10},
		},
	}}
}

func mustSanitizeOptions(t *testing.T, in models.ProductOptions, defaultLang string) models.ProductOptions {
	t.Helper()
	out, message := handlers.SanitizeOptions(in, defaultLang)
	if message != "" {
		t.Fatalf("SanitizeOptions refused a valid payload: %q", message)
	}
	return out
}

func TestSanitizeOptionsKeepsTranslatedNames(t *testing.T) {
	out := mustSanitizeOptions(t, optionGroup(
		" Süt Tercihi ",
		models.OptionTranslations{"en": {Name: " Milk "}, "ar": {Name: "الحليب"}},
		"Yulaf Sütü",
		models.OptionTranslations{"en": {Name: "Oat milk"}, "de": {Name: "Hafermilch"}},
	), "tr")

	group := out[0]
	if group.Name != "Süt Tercihi" {
		t.Errorf("group name = %q, want %q", group.Name, "Süt Tercihi")
	}
	// The default language is stored with the others, as a copy of Name.
	wantGroup := models.OptionTranslations{
		"tr": {Name: "Süt Tercihi"}, "en": {Name: "Milk"}, "ar": {Name: "الحليب"},
	}
	if !reflect.DeepEqual(group.Translations, wantGroup) {
		t.Errorf("group translations = %#v, want %#v", group.Translations, wantGroup)
	}
	wantItem := models.OptionTranslations{
		"tr": {Name: "Yulaf Sütü"}, "en": {Name: "Oat milk"}, "de": {Name: "Hafermilch"},
	}
	if !reflect.DeepEqual(group.Items[0].Translations, wantItem) {
		t.Errorf("item translations = %#v, want %#v", group.Items[0].Translations, wantItem)
	}
}

// Nothing but the default language means nothing to store: the option keeps
// the shape it had before translations existed.
func TestSanitizeOptionsOmitsTranslationsWithoutASecondLanguage(t *testing.T) {
	for name, translations := range map[string]models.OptionTranslations{
		"nil":                 nil,
		"empty":               {},
		"only_default":        {"tr": {Name: "Boy"}},
		"blank_other":         {"en": {Name: "   "}},
		"unsupported_other":   {"it": {Name: "Taglia"}},
		"uppercase_code":      {"EN": {Name: "Size"}},
		"default_and_unknown": {"tr": {Name: "Boy"}, "xx": {Name: "?"}},
	} {
		t.Run(name, func(t *testing.T) {
			out := mustSanitizeOptions(t, optionGroup("Boy", translations, "Büyük", translations), "tr")
			if out[0].Translations != nil || out[0].Items[0].Translations != nil {
				t.Errorf("translations were stored: group %#v, item %#v",
					out[0].Translations, out[0].Items[0].Translations)
			}
			encoded, _ := json.Marshal(out)
			if strings.Contains(string(encoded), "translations") {
				t.Errorf("the stored JSON carries a translations key: %s", encoded)
			}
		})
	}
}

// Name is the default-language text. A different default-language translation
// is overwritten by it, and a blank Name is taken from that translation.
func TestSanitizeOptionsDefaultLanguageName(t *testing.T) {
	t.Run("name_wins_over_the_default_translation", func(t *testing.T) {
		out := mustSanitizeOptions(t, optionGroup("Boy",
			models.OptionTranslations{"tr": {Name: "Eski Boy"}, "en": {Name: "Size"}},
			"Büyük", nil), "tr")
		if got := out[0].Translations["tr"].Name; got != "Boy" {
			t.Errorf("the tr translation is %q, want the Name %q", got, "Boy")
		}
	})

	t.Run("blank_name_comes_from_the_default_translation", func(t *testing.T) {
		out := mustSanitizeOptions(t, optionGroup("  ",
			models.OptionTranslations{"en": {Name: "Size"}, "de": {Name: "Größe"}},
			"", models.OptionTranslations{"en": {Name: "Large"}}), "en")
		if out[0].Name != "Size" {
			t.Errorf("group name = %q, want the default-language (en) translation %q", out[0].Name, "Size")
		}
		if out[0].Items[0].Name != "Large" {
			t.Errorf("item name = %q, want %q", out[0].Items[0].Name, "Large")
		}
		// The item had no language but the default one: nothing to store.
		if out[0].Items[0].Translations != nil {
			t.Errorf("item translations = %#v, want none", out[0].Items[0].Translations)
		}
	})

	t.Run("a_name_in_another_language_only_is_refused", func(t *testing.T) {
		_, message := handlers.SanitizeOptions(optionGroup("",
			models.OptionTranslations{"en": {Name: "Size"}}, "Büyük", nil), "tr")
		if message != "Seçenek grubunun adı zorunludur." {
			t.Errorf("message = %q, want the required-name refusal", message)
		}
		_, message = handlers.SanitizeOptions(optionGroup("Boy", nil,
			"", models.OptionTranslations{"en": {Name: "Large"}}), "tr")
		if message != "Seçenek adı zorunludur." {
			t.Errorf("message = %q, want the required-item-name refusal", message)
		}
	})
}

func TestSanitizeOptionsRefusesAnOverlongTranslation(t *testing.T) {
	long := strings.Repeat("ş", models.MaxOptionNameRunes+1)
	exact := strings.Repeat("ş", models.MaxOptionNameRunes)

	if _, message := handlers.SanitizeOptions(optionGroup("Boy",
		models.OptionTranslations{"en": {Name: exact}}, "Büyük", nil), "tr"); message != "" {
		t.Errorf("a %d-rune translation was refused: %q", models.MaxOptionNameRunes, message)
	}

	_, message := handlers.SanitizeOptions(optionGroup("Boy",
		models.OptionTranslations{"en": {Name: long}}, "Büyük", nil), "tr")
	if message != "Seçenek grubunun adı (EN) en fazla 60 karakter olabilir." {
		t.Errorf("group message = %q", message)
	}
	_, message = handlers.SanitizeOptions(optionGroup("Boy", nil,
		"Büyük", models.OptionTranslations{"ar": {Name: long}}), "tr")
	if message != "Seçenek adı (AR) en fazla 60 karakter olabilir." {
		t.Errorf("item message = %q", message)
	}
	// A language that is dropped is not measured either.
	if _, message := handlers.SanitizeOptions(optionGroup("Boy",
		models.OptionTranslations{"it": {Name: long}}, "Büyük", nil), "tr"); message != "" {
		t.Errorf("an unsupported language's translation was measured: %q", message)
	}
}

func TestSanitizeOptionsRefusesAnUnstorableTranslation(t *testing.T) {
	const nul = "\x00"
	invalid := string([]byte{'a', 0xff})

	for name, options := range map[string]models.ProductOptions{
		"group_nul":     optionGroup("Boy", models.OptionTranslations{"en": {Name: "Size" + nul}}, "Büyük", nil),
		"item_nul":      optionGroup("Boy", nil, "Büyük", models.OptionTranslations{"de": {Name: "Groß" + nul}}),
		"group_invalid": optionGroup("Boy", models.OptionTranslations{"ru": {Name: invalid}}, "Büyük", nil),
		// Even the default-language entry, which Name then replaces.
		"default_nul": optionGroup("Boy", models.OptionTranslations{"tr": {Name: nul}}, "Büyük", nil),
	} {
		t.Run(name, func(t *testing.T) {
			out, message := handlers.SanitizeOptions(options, "tr")
			if message != "Seçenek listesi geçersiz." || out != nil {
				t.Fatalf("SanitizeOptions = (%v, %q), want (nil, %q)", out, message, "Seçenek listesi geçersiz.")
			}
		})
	}

	// A language that is dropped unread cannot refuse the list.
	if _, message := handlers.SanitizeOptions(optionGroup("Boy",
		models.OptionTranslations{"xx": {Name: nul}}, "Büyük", nil), "tr"); message != "" {
		t.Errorf("an unsupported language's text refused the list: %q", message)
	}
}

// What the server returns, sent back unchanged — the dashboard re-saving a
// product — stores exactly the same options. The price-date trigger and the
// audit diff both compare the stored value, so neither sees a change.
func TestSanitizeOptionsIsIdempotent(t *testing.T) {
	first := mustSanitizeOptions(t, optionGroup("Boy",
		models.OptionTranslations{"en": {Name: "Size"}, "tr": {Name: "x"}},
		"Büyük", models.OptionTranslations{"ar": {Name: "كبير"}}), "tr")

	encoded, _ := json.Marshal(first)
	var echoed models.ProductOptions
	if err := json.Unmarshal(encoded, &echoed); err != nil {
		t.Fatalf("the stored options did not decode: %v", err)
	}
	second := mustSanitizeOptions(t, echoed, "tr")

	if again, _ := json.Marshal(second); string(again) != string(encoded) {
		t.Errorf("a re-save changed the options:\n  first  %s\n  second %s", encoded, again)
	}
}

// The caller's value is not modified: UpdateProduct keeps nothing else, but a
// sanitizer that writes through the shared item array is a trap.
func TestSanitizeOptionsLeavesItsInputAlone(t *testing.T) {
	in := optionGroup(" Boy ", models.OptionTranslations{"en": {Name: " Size "}},
		" Büyük ", models.OptionTranslations{"en": {Name: " Large "}})
	before, _ := json.Marshal(in)
	mustSanitizeOptions(t, in, "tr")
	if after, _ := json.Marshal(in); string(after) != string(before) {
		t.Errorf("SanitizeOptions modified its input:\n  before %s\n  after  %s", before, after)
	}
}
