package utils

// Currency describes one supported currency.
//
// NOTE: Name is shown in the dashboard and is therefore Turkish on purpose.
type Currency struct {
	Code     string `json:"code"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Position string `json:"position"` // "suffix" (100 ₺) or "prefix" ($100)
}

// Currencies lists the currencies selectable in the dashboard. Default: TRY.
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

// CurrencySymbol returns the symbol for a currency code, falling back to "₺".
func CurrencySymbol(code string) string {
	if currency, ok := Currencies[code]; ok {
		return currency.Symbol
	}
	return "₺"
}

// IsValidCurrency reports whether a currency code is supported.
func IsValidCurrency(code string) bool {
	_, ok := Currencies[code]
	return ok
}
