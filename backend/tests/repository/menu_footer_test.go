package repository_test

import (
	"testing"
	"time"

	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
)

// The footer names a day, and which day depends on the zone the instant is read
// in. 2026-09-13T22:30:00Z is still 13 September in UTC but already 14 September
// in Istanbul, so it separates "formatted in Europe/Istanbul" from "formatted in
// whatever zone the process runs in" — provided the process is not itself on
// Istanbul time, as it is on any machine set to Turkish time. time.Local is
// therefore pinned to UTC for the duration of the test and restored afterwards.
func TestBuildFooterFormatsThePriceDateInIstanbul(t *testing.T) {
	saved := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = saved })

	menu := &models.Menu{
		ShowPriceDate:  true,
		PriceUpdatedAt: time.Date(2026, 9, 13, 22, 30, 0, 0, time.UTC),
	}

	const want = "Fiyatlarımız 14.09.2026 tarihinden itibaren geçerlidir."
	footer := repository.BuildFooter(menu)
	if got := footer.PriceNote; got != want {
		t.Fatalf("BuildFooter price note = %q, want %q: 2026-09-13T22:30:00Z is 14.09.2026 "+
			"in Europe/Istanbul, so any other day means the date was not formatted in "+
			"Istanbul (time.Local is UTC in this test)", got, want)
	}
	// price_date carries the same day, machine-readable, for the customer page
	// to build its own sentence in the visitor's language.
	if got := footer.PriceDate; got != "2026-09-14" {
		t.Fatalf("BuildFooter price date = %q, want 2026-09-14 (the Istanbul day)", got)
	}
}

// A menu that hides its price date sends neither the sentence nor the date —
// "" in both, never a date the owner switched off.
func TestBuildFooterHidesThePriceDateWhenSwitchedOff(t *testing.T) {
	menu := &models.Menu{
		ShowPriceDate:  false,
		PriceUpdatedAt: time.Date(2026, 9, 13, 22, 30, 0, 0, time.UTC),
	}
	footer := repository.BuildFooter(menu)
	if footer.PriceDate != "" || footer.PriceNote != "" {
		t.Fatalf("show_price_date false: price_date %q and price_note %q, want both empty",
			footer.PriceDate, footer.PriceNote)
	}
	if footer.PoweredBy == "" {
		t.Fatal("the footer lost its powered_by credit")
	}
}
