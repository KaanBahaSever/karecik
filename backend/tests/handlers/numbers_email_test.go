package handlers_test

import (
	"math"
	"testing"

	"karecik/backend/internal/handlers"
	"karecik/backend/internal/utils"
)

// CheckPrice and NormalizeEmail are the checks that keep a number or an address
// PostgreSQL cannot hold, or would hold as something else, away from a write.
// The HTTP suites in backend/tests prove the answers end to end; these pin the
// checks themselves.

func TestCheckPrice(t *testing.T) {
	const negative, notANumber, tooLarge = "negative", "not a number", "too large"

	for _, tc := range []struct {
		name  string
		price float64
		want  string
	}{
		{"zero", 0, ""},
		{"negative_zero", math.Copysign(0, -1), ""},
		{"a_plain_price", 145.5, ""},
		{"exactly_the_maximum", 9999999999.99, ""},
		{"just_below_zero", -0.01, negative},
		{"minus_infinity", math.Inf(-1), negative},
		{"just_above_the_maximum", 9999999999.991, tooLarge},
		{"ten_billion", 1e10, tooLarge},
		{"1e300", 1e300, tooLarge},
		{"plus_infinity", math.Inf(1), tooLarge},
		{"nan", math.NaN(), notANumber},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := handlers.CheckPrice(tc.price, negative, notANumber, tooLarge); got != tc.want {
				t.Fatalf("CheckPrice(%v) = %q, want %q", tc.price, got, tc.want)
			}
		})
	}

	// A price that passes is one the column holds once it is rounded.
	if rounded := utils.Round2(9999999999.99); rounded > utils.MaxPrice {
		t.Fatalf("the largest accepted price rounds to %v, above utils.MaxPrice", rounded)
	}
}

func TestPriceMessagesNameTheLimit(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{handlers.MsgPriceTooLarge, "Fiyat en fazla 9.999.999.999,99 olabilir."},
		{handlers.MsgComparePriceTooLarge, "Karşılaştırma fiyatı en fazla 9.999.999.999,99 olabilir."},
		{handlers.MsgBulkPriceTooLarge, "Bu değişiklik bazı fiyatları izin verilen en yüksek değerin (9.999.999.999,99) üzerine çıkarıyor."},
	} {
		if tc.got != tc.want {
			t.Errorf("message %q, want %q", tc.got, tc.want)
		}
	}
}

func TestNormalizeEmailRefusesAnUnstorableAddressBeforeLowercasingIt(t *testing.T) {
	replacement := string(rune(0xFFFD))

	for _, tc := range []struct {
		name, raw, want string
		storable        bool
	}{
		{"trimmed_and_lowercased", "  Ali@Example.TEST ", "ali@example.test", true},
		{"an_address_that_really_holds_the_replacement_character", replacement + "@example.test",
			replacement + "@example.test", true},
		{"byte_ff", string([]byte{0xff}) + "@example.test", "", false},
		{"byte_fe_among_capitals", "ALI" + string([]byte{0xfe}) + "@EXAMPLE.TEST", "", false},
		{"truncated_sequence", "ali@example.tes" + string([]byte{0xc3}), "", false},
		{"nul", "ali" + nul + "@example.test", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, storable := handlers.NormalizeEmail(tc.raw)
			if got != tc.want || storable != tc.storable {
				t.Fatalf("NormalizeEmail(%q) = (%q, %t), want (%q, %t)", tc.raw, got, storable, tc.want, tc.storable)
			}
		})
	}
}
