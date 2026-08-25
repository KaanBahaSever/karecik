package utils

// Currency, desteklenen bir para birimini tanimlar.
type Currency struct {
	Code     string `json:"code"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Position string `json:"position"` // "suffix" (100 ₺) veya "prefix" ($100)
}

// Currencies, panelde secilebilen para birimleri. Varsayilan: TRY.
var Currencies = map[string]Currency{
	"TRY": {Code: "TRY", Symbol: "₺", Name: "Türk Lirası", Position: "suffix"},
	"USD": {Code: "USD", Symbol: "$", Name: "Amerikan Doları", Position: "prefix"},
	"EUR": {Code: "EUR", Symbol: "€", Name: "Euro", Position: "prefix"},
	"GBP": {Code: "GBP", Symbol: "£", Name: "İngiliz Sterlini", Position: "prefix"},
	"AZN": {Code: "AZN", Symbol: "₼", Name: "Azerbaycan Manatı", Position: "suffix"},
	"RUB": {Code: "RUB", Symbol: "₽", Name: "Rus Rublesi", Position: "suffix"},
	"SAR": {Code: "SAR", Symbol: "﷼", Name: "Suudi Riyali", Position: "suffix"},
	"AED": {Code: "AED", Symbol: "د.إ", Name: "BAE Dirhemi", Position: "suffix"},
}

// CurrencySymbol, para birimi kodunun simgesini dondurur. Bilinmeyen kodda "₺".
func CurrencySymbol(code string) string {
	if cur, ok := Currencies[code]; ok {
		return cur.Symbol
	}
	return "₺"
}

// IsValidCurrency, kodun desteklenip desteklenmedigini soyler.
func IsValidCurrency(code string) bool {
	_, ok := Currencies[code]
	return ok
}
