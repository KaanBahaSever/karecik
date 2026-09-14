package handlers_test

import (
	"encoding/json"
	"testing"

	"karecik/backend/internal/handlers"
	"karecik/backend/internal/models"
)

// PostgreSQL refuses U+0000 in text and jsonb alike, so every user text is
// checked for it before a write is built. The HTTP suite in backend/tests
// proves the answer is a 422 end to end; these pin the checks themselves.
//
// The character is built with string(rune(0)) rather than written into the
// source, which must stay free of raw control characters.

var nul = string(rune(0))

// jsonText encodes a Go string as a JSON string literal — encoding/json writes
// U+0000 as the escape \u0000.
func jsonText(t *testing.T, s string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("could not encode %q: %v", s, err)
	}
	return raw
}

func TestDecodeStringRefusesNUL(t *testing.T) {
	if _, err := handlers.DecodeString(jsonText(t, "a"+nul+"b")); err == nil {
		t.Fatal("DecodeString accepted a text holding U+0000")
	}
	if got, err := handlers.DecodeString(jsonText(t, "Kahvenin en iyi hali")); err != nil || got != "Kahvenin en iyi hali" {
		t.Fatalf("DecodeString of a plain text = (%q, %v)", got, err)
	}
	if _, err := handlers.DecodeNullableString(jsonText(t, nul)); err == nil {
		t.Fatal("DecodeNullableString accepted a text holding U+0000")
	}
	if got, err := handlers.DecodeNullableString(json.RawMessage("null")); err != nil || got != nil {
		t.Fatalf("DecodeNullableString(null) = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestSanitizeTranslationsRefusesNUL(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   models.Translations
	}{
		{"name", models.Translations{"tr": {Name: "Tatlı" + nul}}},
		{"description", models.Translations{"tr": {Name: "Tatlı", Description: nul}}},
		{"ingredients", models.Translations{"tr": {Name: "Tatlı", Ingredients: "un" + nul}}},
		{"another_supported_language", models.Translations{"tr": {Name: "Tatlı"}, "en": {Name: "Dessert" + nul}}},
		{"reported_before_a_length_problem", models.Translations{
			"tr": {Name: "Tatlı", Description: nul},
			"en": {Name: "x", Description: longText(501)},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, message := handlers.SanitizeTranslations(tc.in, "tr", "Ürün")
			if message != "Çeviri alanı geçersiz." || out != nil {
				t.Fatalf("SanitizeTranslations = (%v, %q), want (nil, %q)", out, message, "Çeviri alanı geçersiz.")
			}
		})
	}

	// An unsupported language is dropped rather than stored, so its texts are
	// none of the check's business.
	out, message := handlers.SanitizeTranslations(models.Translations{"tr": {Name: "Tatlı"}, "xx": {Name: nul}}, "tr", "Ürün")
	if message != "" || len(out) != 1 {
		t.Fatalf("SanitizeTranslations with a NUL only in an unsupported language = (%v, %q), want the tr entry", out, message)
	}
}

func longText(n int) string {
	runes := make([]rune, n)
	for i := range runes {
		runes[i] = 'a'
	}
	return string(runes)
}

func TestSanitizeBadgesRefusesNUL(t *testing.T) {
	valid := models.Badge{ID: "b1", Text: "Yeni", BgColor: "#1d4ed8", TextColor: "#ffffff"}
	for _, tc := range []struct {
		name  string
		badge models.Badge
	}{
		{"text", models.Badge{Text: "Yeni" + nul}},
		{"id", models.Badge{ID: "b" + nul, Text: "Yeni"}},
		{"icon", models.Badge{Text: "Yeni", Icon: "star" + nul}},
		{"bg_color", models.Badge{Text: "Yeni", BgColor: nul}},
		{"text_color", models.Badge{Text: "Yeni", TextColor: nul}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, message := handlers.SanitizeBadges(models.Badges{valid, tc.badge})
			if message != "Rozet listesi geçersiz." || out != nil {
				t.Fatalf("SanitizeBadges = (%v, %q), want (nil, %q)", out, message, "Rozet listesi geçersiz.")
			}
		})
	}
}

// A form body, a query string or a header can carry bytes that are not UTF-8,
// which PostgreSQL refuses just like U+0000; the same checks refuse them. The
// bytes are built from byte values, so the source holds none of them.
func TestTextThatIsNotUTF8IsRefusedLikeNUL(t *testing.T) {
	invalid := string([]byte{'a', 0xff, 'b'})
	truncated := string([]byte{'k', 'a', 0xc3})

	for _, s := range []string{invalid, truncated, "a" + nul} {
		if !handlers.UnstorableText(s) {
			t.Errorf("UnstorableText(%q) = false, want true", s)
		}
	}
	if handlers.UnstorableText("Kahvenin en iyi hali, şimdi") {
		t.Errorf("UnstorableText refused a plain Turkish text")
	}

	if out, message := handlers.SanitizeTranslations(models.Translations{"tr": {Name: invalid}}, "tr", "Ürün"); message != "Çeviri alanı geçersiz." || out != nil {
		t.Errorf("SanitizeTranslations of invalid UTF-8 = (%v, %q)", out, message)
	}
	if out, message := handlers.SanitizeBadges(models.Badges{{Text: truncated}}); message != "Rozet listesi geçersiz." || out != nil {
		t.Errorf("SanitizeBadges of invalid UTF-8 = (%v, %q)", out, message)
	}
	options := models.ProductOptions{{Name: invalid, Type: "single", Items: []models.ProductOptionItem{{Name: "Büyük"}}}}
	if out, message := handlers.SanitizeOptions(options); message != "Seçenek listesi geçersiz." || out != nil {
		t.Errorf("SanitizeOptions of invalid UTF-8 = (%v, %q)", out, message)
	}

	if handlers.IsValidEmail("ali" + string([]byte{0xff}) + "@example.test") {
		t.Errorf("IsValidEmail accepted an address holding a byte that is not UTF-8")
	}
	if !handlers.IsValidEmail("ali@example.test") {
		t.Errorf("IsValidEmail refused a plain address")
	}
}

func TestSanitizeOptionsRefusesNUL(t *testing.T) {
	item := models.ProductOptionItem{Name: "Büyük", Price: 10}
	for _, tc := range []struct {
		name  string
		group models.ProductOptionGroup
	}{
		{"group_name", models.ProductOptionGroup{Name: "Boy" + nul, Type: "single", Items: []models.ProductOptionItem{item}}},
		{"item_name", models.ProductOptionGroup{Name: "Boy", Type: "single",
			Items: []models.ProductOptionItem{item, {Name: "Orta" + nul}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, message := handlers.SanitizeOptions(models.ProductOptions{tc.group})
			if message != "Seçenek listesi geçersiz." || out != nil {
				t.Fatalf("SanitizeOptions = (%v, %q), want (nil, %q)", out, message, "Seçenek listesi geçersiz.")
			}
		})
	}
}
