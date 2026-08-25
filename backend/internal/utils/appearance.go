package utils

// Theme, musteri menusunun gorunum temasi.
type Theme struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Primary     string `json:"primary"`
	Background  string `json:"background"`
	Surface     string `json:"surface"`
	Text        string `json:"text"`
	Muted       string `json:"muted"`
	Border      string `json:"border"`
	Dark        bool   `json:"dark"`
}

// Themes, panelde secilebilen tasarim tarzlari.
// Frontend'deki src/themes/themes.js ile ayni kimlikleri (id) kullanir.
var Themes = []Theme{
	{ID: "modern-light", Label: "Modern Açık", Description: "Beyaz zemin, mavi vurgu",
		Primary: "#1d4ed8", Background: "#ffffff", Surface: "#f6f7f8", Text: "#111827",
		Muted: "#6b7280", Border: "#e5e7eb", Dark: false},
	{ID: "modern-dark", Label: "Modern Koyu", Description: "Koyu zemin, gece kullanımı",
		Primary: "#60a5fa", Background: "#0f172a", Surface: "#1e293b", Text: "#f1f5f9",
		Muted: "#94a3b8", Border: "#334155", Dark: true},
	{ID: "zarif", Label: "Zarif", Description: "Krem zemin, serif başlıklar",
		Primary: "#8a6a3c", Background: "#faf6ef", Surface: "#f2ead9", Text: "#3b2f22",
		Muted: "#8a7a66", Border: "#e3d7c1", Dark: false},
	{ID: "sicak", Label: "Sıcak", Description: "Kiremit tonları, samimi",
		Primary: "#c2410c", Background: "#fffbf6", Surface: "#ffedd5", Text: "#431407",
		Muted: "#9a6b52", Border: "#fed7aa", Dark: false},
	{ID: "minimal", Label: "Minimal", Description: "Siyah-beyaz, sade tipografi",
		Primary: "#111111", Background: "#ffffff", Surface: "#fafafa", Text: "#111111",
		Muted: "#737373", Border: "#e5e5e5", Dark: false},
	{ID: "canli", Label: "Canlı", Description: "Mor vurgu, yüksek kontrast",
		Primary: "#7c3aed", Background: "#ffffff", Surface: "#f5f3ff", Text: "#1e1b4b",
		Muted: "#6d6a8a", Border: "#ddd6fe", Dark: false},
}

// Font, menude kullanilabilen yazi tipi.
type Font struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Stack  string `json:"stack"`  // CSS font-family
	Google string `json:"google"` // Google Fonts aile adi (bos ise sistem fontu)
}

// Fonts, panelde secilebilen yazi tipleri.
// Frontend'deki src/themes/fonts.js ile ayni kimlikleri kullanir.
var Fonts = []Font{
	{ID: "inter", Label: "Inter", Stack: "'Inter', system-ui, sans-serif", Google: "Inter"},
	{ID: "poppins", Label: "Poppins", Stack: "'Poppins', system-ui, sans-serif", Google: "Poppins"},
	{ID: "montserrat", Label: "Montserrat", Stack: "'Montserrat', system-ui, sans-serif", Google: "Montserrat"},
	{ID: "nunito", Label: "Nunito", Stack: "'Nunito', system-ui, sans-serif", Google: "Nunito"},
	{ID: "playfair", Label: "Playfair Display", Stack: "'Playfair Display', Georgia, serif", Google: "Playfair+Display"},
	{ID: "lora", Label: "Lora", Stack: "'Lora', Georgia, serif", Google: "Lora"},
	{ID: "dm-serif", Label: "DM Serif Display", Stack: "'DM Serif Display', Georgia, serif", Google: "DM+Serif+Display"},
	{ID: "space-grotesk", Label: "Space Grotesk", Stack: "'Space Grotesk', system-ui, sans-serif", Google: "Space+Grotesk"},
}

// Allergen, urunlere eklenebilen uyari etiketi.
type Allergen struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// Allergens, sabit alerjen/uyari listesi.
var Allergens = []Allergen{
	{Code: "gluten", Label: "Gluten", Icon: "wheat"},
	{Code: "sut", Label: "Süt", Icon: "milk"},
	{Code: "yumurta", Label: "Yumurta", Icon: "egg"},
	{Code: "findik", Label: "Fındık / Kuruyemiş", Icon: "nut"},
	{Code: "yer_fistigi", Label: "Yer Fıstığı", Icon: "nut"},
	{Code: "soya", Label: "Soya", Icon: "bean"},
	{Code: "balik", Label: "Balık", Icon: "fish"},
	{Code: "kabuklu_deniz", Label: "Kabuklu Deniz Ürünü", Icon: "shell"},
	{Code: "susam", Label: "Susam", Icon: "seed"},
	{Code: "hardal", Label: "Hardal", Icon: "droplet"},
	{Code: "kereviz", Label: "Kereviz", Icon: "leaf"},
	{Code: "sulfit", Label: "Sülfit", Icon: "flask"},
	{Code: "aci", Label: "Acı", Icon: "flame"},
	{Code: "vejetaryen", Label: "Vejetaryen", Icon: "sprout"},
	{Code: "vegan", Label: "Vegan", Icon: "vegan"},
	{Code: "alkol", Label: "Alkol İçerir", Icon: "wine"},
	{Code: "kafein", Label: "Kafein", Icon: "coffee"},
}

// Language, desteklenen menu dilleri.
//
// Short: arayuzde gosterilen kisa kod (TR/EN/DE). Bayrak emojisi yerine bunu
// kullaniyoruz: Windows bayrak emojilerini cizemedigi icin yerlerine ulke kodu
// basiyor ve Ingilizce "GB" olarak gorunuyordu.
type Language struct {
	Code  string `json:"code"`
	Short string `json:"short"`
	Label string `json:"label"`
	Flag  string `json:"flag"`
}

// Languages, menuye eklenebilecek diller.
var Languages = []Language{
	{Code: "tr", Short: "TR", Label: "Türkçe", Flag: "🇹🇷"},
	{Code: "en", Short: "EN", Label: "English", Flag: "🇬🇧"},
	{Code: "de", Short: "DE", Label: "Deutsch", Flag: "🇩🇪"},
	{Code: "ru", Short: "RU", Label: "Русский", Flag: "🇷🇺"},
	{Code: "ar", Short: "AR", Label: "العربية", Flag: "🇸🇦"},
	{Code: "fr", Short: "FR", Label: "Français", Flag: "🇫🇷"},
}

// IsValidTheme, tema kimliginin listede olup olmadigini soyler.
func IsValidTheme(id string) bool {
	for _, t := range Themes {
		if t.ID == id {
			return true
		}
	}
	return false
}

// IsValidFont, yazi tipi kimliginin listede olup olmadigini soyler.
func IsValidFont(id string) bool {
	for _, f := range Fonts {
		if f.ID == id {
			return true
		}
	}
	return false
}

// IsValidLanguage, dil kodunun desteklenip desteklenmedigini soyler.
func IsValidLanguage(code string) bool {
	for _, l := range Languages {
		if l.Code == code {
			return true
		}
	}
	return false
}

// IsValidAllergen, alerjen kodunun listede olup olmadigini soyler.
func IsValidAllergen(code string) bool {
	for _, a := range Allergens {
		if a.Code == code {
			return true
		}
	}
	return false
}
