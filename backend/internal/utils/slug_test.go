package utils

import "testing"

// TestSlugify, Turkce karakterlerin subdomain'e uygun ASCII karsiliklarina
// dogru cevrildigini dogrular. Bu donusum bozulursa isletmelerin menu adresi
// (kahve-duragi.karecik.com) yanlis uretilir.
func TestSlugify(t *testing.T) {
	durumlar := []struct {
		girdi  string
		beklenen string
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

	for _, d := range durumlar {
		if sonuc := Slugify(d.girdi); sonuc != d.beklenen {
			t.Errorf("Slugify(%q) = %q; beklenen %q", d.girdi, sonuc, d.beklenen)
		}
	}
}

// TestRoundPrice, toplu fiyat guncellemesindeki yuvarlama modlarini dogrular.
func TestRoundPrice(t *testing.T) {
	durumlar := []struct {
		fiyat    float64
		mod      string
		beklenen float64
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

	for _, d := range durumlar {
		if sonuc := RoundPrice(d.fiyat, d.mod); sonuc != d.beklenen {
			t.Errorf("RoundPrice(%v, %q) = %v; beklenen %v", d.fiyat, d.mod, sonuc, d.beklenen)
		}
	}
}

// TestApplyPercentage, yuzde bazli zam/indirim hesabini dogrular.
func TestApplyPercentage(t *testing.T) {
	if got := Round2(ApplyPercentage(100, 10)); got != 110 {
		t.Errorf("%%10 zam = %v; beklenen 110", got)
	}
	if got := Round2(ApplyPercentage(100, -15)); got != 85 {
		t.Errorf("%%15 indirim = %v; beklenen 85", got)
	}
}
