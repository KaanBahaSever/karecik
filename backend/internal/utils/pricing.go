package utils

import "math"

// Desteklenen yuvarlama modlari (toplu fiyat guncellemesinde kullanilir).
const (
	RoundNone     = "none"      // yuvarlama yok, 2 ondalik
	RoundInteger  = "integer"   // tam sayiya  (147,60 -> 148)
	RoundNearest5 = "nearest_5" // 5'in katina (147,60 -> 150)
	RoundNearest10 = "nearest_10" // 10'un katina (147,60 -> 150)
	RoundEnds99   = "ends_99"   // ...,99 ile bitir (147,60 -> 147,99)
	RoundEnds95   = "ends_95"   // ...,95 ile bitir (147,60 -> 147,95)
	RoundEnds50   = "ends_50"   // 0,50'nin katina (147,60 -> 147,50)
)

// ValidRoundingModes, istekten gelen degerin dogrulanmasi icin.
var ValidRoundingModes = map[string]bool{
	RoundNone: true, RoundInteger: true, RoundNearest5: true,
	RoundNearest10: true, RoundEnds99: true, RoundEnds95: true, RoundEnds50: true,
}

// ApplyPercentage, fiyata yuzde bazli artis/indirim uygular.
//
//	ApplyPercentage(100, 10)  -> 110   (%10 zam)
//	ApplyPercentage(100, -15) ->  85   (%15 indirim)
func ApplyPercentage(price, percentage float64) float64 {
	return price * (1 + percentage/100)
}

// RoundPrice, fiyati secilen moda gore yuvarlar. Sonuc asla negatif olmaz.
func RoundPrice(price float64, mode string) float64 {
	if price < 0 {
		price = 0
	}

	switch mode {
	case RoundInteger:
		price = math.Round(price)

	case RoundNearest5:
		price = math.Round(price/5) * 5

	case RoundNearest10:
		price = math.Round(price/10) * 10

	case RoundEnds50:
		price = math.Round(price*2) / 2

	case RoundEnds99:
		// En yakin tam sayiya git, sonra 1 kurus dus: 148 -> 147,99
		base := math.Round(price)
		if base < 1 {
			base = 1
		}
		price = base - 0.01

	case RoundEnds95:
		base := math.Round(price)
		if base < 1 {
			base = 1
		}
		price = base - 0.05

	default: // RoundNone
		// asagida ortak 2 ondalik yuvarlama yapiliyor
	}

	if price < 0 {
		price = 0
	}
	return Round2(price)
}

// Round2, kayan nokta artiklarini temizleyip 2 ondalige sabitler.
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}
