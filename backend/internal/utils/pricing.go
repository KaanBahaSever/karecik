package utils

import "math"

// Supported rounding modes, used by the bulk price update.
const (
	RoundNone      = "none"       // no rounding, 2 decimals
	RoundInteger   = "integer"    // to a whole number   (147.60 -> 148)
	RoundNearest5  = "nearest_5"  // to a multiple of 5  (147.60 -> 150)
	RoundNearest10 = "nearest_10" // to a multiple of 10 (147.60 -> 150)
	RoundEnds99    = "ends_99"    // ends with .99       (147.60 -> 147.99)
	RoundEnds95    = "ends_95"    // ends with .95       (147.60 -> 147.95)
	RoundEnds50    = "ends_50"    // to a multiple of .50 (147.60 -> 147.50)
)

// ValidRoundingModes is used to validate the value coming from the request.
var ValidRoundingModes = map[string]bool{
	RoundNone: true, RoundInteger: true, RoundNearest5: true,
	RoundNearest10: true, RoundEnds99: true, RoundEnds95: true, RoundEnds50: true,
}

// ApplyPercentage applies a percentage increase or discount to a price.
//
//	ApplyPercentage(100, 10)  -> 110   (10% increase)
//	ApplyPercentage(100, -15) ->  85   (15% discount)
func ApplyPercentage(price, percentage float64) float64 {
	return price * (1 + percentage/100)
}

// RoundPrice rounds a price using the selected mode. The result is never negative.
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
		// Round to the nearest integer, then drop one kuruş: 148 -> 147.99
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
		// the shared 2-decimal rounding below applies
	}

	if price < 0 {
		price = 0
	}
	return Round2(price)
}

// Round2 clears floating point noise and pins a value to 2 decimals.
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}
