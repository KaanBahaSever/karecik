package repository_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// The preview reads its rows lock-free and ApplyPrices reads them in id order
// under its locks; both sort them with SortPriceRows before they answer, so the
// two list the same rows in the same order.
func TestSortPriceRowsFollowsTheMenuEditor(t *testing.T) {
	id := func(last byte) uuid.UUID {
		var u uuid.UUID
		u[15] = last
		return u
	}
	categoryA, categoryB := id(0x0a), id(0x0b)

	rows := []repository.PriceRow{
		{ID: id(1), CategoryPosition: 1, CategoryID: categoryA, Position: 0},
		{ID: id(2), CategoryPosition: 0, CategoryID: categoryB, Position: 1},
		{ID: id(3), CategoryPosition: 0, CategoryID: categoryB, Position: 0},
		{ID: id(4), CategoryPosition: 0, CategoryID: categoryA, Position: 5},
		{ID: id(6), CategoryPosition: 0, CategoryID: categoryB, Position: 1},
		{ID: id(5), CategoryPosition: 0, CategoryID: categoryB, Position: 1},
	}
	repository.SortPriceRows(rows)

	// Category position, then the category id for categories that share a
	// position, then the product position, then the product id.
	want := []uuid.UUID{id(4), id(3), id(2), id(5), id(6), id(1)}
	for i, row := range rows {
		if row.ID != want[i] {
			got := make([]string, 0, len(rows))
			for _, r := range rows {
				got = append(got, r.ID.String()[34:])
			}
			t.Fatalf("SortPriceRows order %s, want the ids ending in 04, 03, 02, 05, 06, 01", strings.Join(got, " "))
		}
	}
}

func TestPlanPriceChangesPricesEveryRowInItsOrder(t *testing.T) {
	rows := []repository.PriceRow{
		{ID: uuid.New(), Price: 100},
		{ID: uuid.New(), Price: 0},
		{ID: uuid.New(), Price: 147.6},
	}

	changes := repository.PlanPriceChanges(rows, 10, utils.RoundNone)
	if len(changes) != len(rows) {
		t.Fatalf("PlanPriceChanges returned %d rows, want %d", len(changes), len(rows))
	}
	for i, change := range changes {
		if change.ID != rows[i].ID {
			t.Fatalf("row #%d is %s, want %s: the order has to be kept", i, change.ID, rows[i].ID)
		}
	}
	if changes[0].NewPrice != 110 || !changes[0].Changed() {
		t.Errorf("100 +10%% = (%.2f, changed %t), want (110.00, true)", changes[0].NewPrice, changes[0].Changed())
	}
	if changes[1].NewPrice != 0 || changes[1].Changed() {
		t.Errorf("0 +10%% = (%.2f, changed %t), want (0.00, false)", changes[1].NewPrice, changes[1].Changed())
	}
	if changes[2].NewPrice != 162.36 || !changes[2].Changed() {
		t.Errorf("147.60 +10%% = (%.2f, changed %t), want (162.36, true)", changes[2].NewPrice, changes[2].Changed())
	}

	// A rounding that lands on the price it started from writes nothing.
	unchanged := repository.PlanPriceChanges([]repository.PriceRow{{ID: uuid.New(), Price: 150}}, 1, utils.RoundNearest10)
	if unchanged[0].NewPrice != 150 || unchanged[0].Changed() {
		t.Errorf("150 +1%% to the nearest 10 = (%.2f, changed %t), want (150.00, false)",
			unchanged[0].NewPrice, unchanged[0].Changed())
	}
}

// A plan is over the limit when one planned price is larger than the largest
// price NUMERIC(12,2) holds; a price exactly at the limit is not.
func TestPriceLimitExceeded(t *testing.T) {
	if utils.MaxPrice != 9999999999.99 {
		t.Fatalf("utils.MaxPrice is %v, want 9999999999.99, the largest NUMERIC(12,2)", utils.MaxPrice)
	}

	atLimit := repository.PlanPriceChanges([]repository.PriceRow{{ID: uuid.New(), Price: 100}, {ID: uuid.New(), Price: 9999999999.99}},
		0, utils.RoundNone)
	if repository.PriceLimitExceeded(atLimit) {
		t.Errorf("a plan whose largest price is exactly 9999999999.99 is reported over the limit: %+v", atLimit)
	}

	for _, tc := range []struct {
		name       string
		price      float64
		percentage float64
		rounding   string
	}{
		{"a_stored_maximum_raised_by_one_percent", 9999999999.99, 1, utils.RoundNone},
		{"a_stored_9999999999_raised_by_ten_percent", 9999999999, 10, utils.RoundNone},
		{"the_maximum_rounded_up_to_the_next_ten", 9999999999.99, 0, utils.RoundNearest10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changes := repository.PlanPriceChanges([]repository.PriceRow{{ID: uuid.New(), Price: 10}, {ID: uuid.New(), Price: tc.price}},
				tc.percentage, tc.rounding)
			if !repository.PriceLimitExceeded(changes) {
				t.Errorf("%.2f with %v%% and rounding %s plans %.2f, which is not reported over the limit",
					tc.price, tc.percentage, tc.rounding, changes[1].NewPrice)
			}
		})
	}

	if repository.PriceLimitExceeded(nil) {
		t.Errorf("an empty plan is reported over the limit")
	}
}

func TestUnstorableText(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{"plain", "kahvalti", false},
		{"turkish", "Kahvaltı Menüsü", false},
		{"nul", "kahve" + string(rune(0)), true},
		{"byte_ff", string([]byte{'k', 0xff}), true},
		{"truncated_sequence", string([]byte{'k', 'a', 0xc3}), true},
	} {
		if got := repository.UnstorableText(tc.text); got != tc.want {
			t.Errorf("UnstorableText(%s) = %t, want %t", tc.name, got, tc.want)
		}
	}
}
