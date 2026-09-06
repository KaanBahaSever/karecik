package utils

import "testing"

// TestSlugify verifies that Turkish characters are converted to the ASCII
// equivalents a subdomain needs. If this conversion breaks, businesses end up
// with the wrong address (kahve-duragi.karecik.com).
func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Çınar Kahve Durağı", "cinar-kahve-duragi"},
		{"Kahve Durağı", "kahve-duragi"},
		{"İstanbul Şişli Lokantası", "istanbul-sisli-lokantasi"},
		{"ÇĞİIÖŞÜ", "cgiiosu"},
		{"çğıiöşü", "cgiiosu"},
		{"Öz Ürün & Şölen", "oz-urun-ve-solen"},
		{"  Boşluklu   Ad  ", "bosluklu-ad"},
		{"Çok---Fazla___Tire", "cok-fazla-tire"},
		{"", "isletme"},
		{"!!!", "isletme"},
		{"Cafe 42", "cafe-42"},
	}

	for _, c := range cases {
		if got := Slugify(c.input); got != c.want {
			t.Errorf("Slugify(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

// TestSlugifyWithFallback covers the caller-chosen fallback. A menu whose name
// slugifies to nothing must become "menu", not "isletme" — the fallback is the
// only thing that differs between a business slug and a menu slug here.
func TestSlugifyWithFallback(t *testing.T) {
	cases := []struct {
		input    string
		fallback string
		want     string
	}{
		{"", "menu", "menu"},
		{"   ", "menu", "menu"},
		{"!!!", "menu", "menu"},
		{"---", "menu", "menu"},
		{"", "isletme", "isletme"},
		{"Kahvaltı", "menu", "kahvalti"},
		{"Akşam Menüsü", "menu", "aksam-menusu"},
		{"Çınar Kahve Durağı", "menu", "cinar-kahve-duragi"},
		{"  Boşluklu   Ad  ", "menu", "bosluklu-ad"},
		{"Öz Ürün & Şölen", "menu", "oz-urun-ve-solen"},
		{"Cafe 42", "menu", "cafe-42"},
	}

	for _, c := range cases {
		if got := SlugifyWithFallback(c.input, c.fallback); got != c.want {
			t.Errorf("SlugifyWithFallback(%q, %q) = %q; want %q",
				c.input, c.fallback, got, c.want)
		}
	}

	// Slugify is SlugifyWithFallback bound to the business fallback.
	if got, want := Slugify("!!!"), SlugifyWithFallback("!!!", "isletme"); got != want {
		t.Errorf("Slugify(%q) = %q; want %q", "!!!", got, want)
	}

	// The 60-character cap survives the delegation and never trims away the
	// whole slug.
	long := "cok-uzun-bir-isletme-adi-ve-devami-daha-da-uzun-oluyor-iste-boyle-uzun"
	if got := SlugifyWithFallback(long, "menu"); len(got) > 60 || got == "menu" {
		t.Errorf("SlugifyWithFallback(long, %q) = %q (len %d); want a slug of at most 60 chars",
			"menu", got, len(got))
	}
}

// TestRoundPrice verifies the rounding modes used by the bulk price update.
func TestRoundPrice(t *testing.T) {
	cases := []struct {
		price float64
		mode  string
		want  float64
	}{
		{147.60, RoundNone, 147.60},
		{147.60, RoundInteger, 148},
		{147.60, RoundNearest5, 150},
		{147.60, RoundNearest10, 150},
		{147.60, RoundEnds50, 147.50},
		{147.60, RoundEnds95, 147.95},
		{147.60, RoundEnds99, 147.99},
		{-5, RoundNone, 0},
		{0.2, RoundEnds99, 0.99},
	}

	for _, c := range cases {
		if got := RoundPrice(c.price, c.mode); got != c.want {
			t.Errorf("RoundPrice(%v, %q) = %v; want %v", c.price, c.mode, got, c.want)
		}
	}
}

// TestApplyPercentage verifies the percentage increase / discount maths.
func TestApplyPercentage(t *testing.T) {
	if got := Round2(ApplyPercentage(100, 10)); got != 110 {
		t.Errorf("10%% increase = %v; want 110", got)
	}
	if got := Round2(ApplyPercentage(100, -15)); got != 85 {
		t.Errorf("15%% discount = %v; want 85", got)
	}
}
