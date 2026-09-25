package handlers_test

// The pure helpers of the analytics endpoints: how a visitor is keyed, how a
// user agent is made storable, and how the summary's from/to become a window.

import (
	"strings"
	"testing"
	"time"

	"karecik/backend/internal/handlers"
	"karecik/backend/internal/utils"
)

func TestVisitorKey(t *testing.T) {
	if got := handlers.VisitorKey("abc_123", "203.0.113.9", "UA"); got != "v:abc_123" {
		t.Errorf("a sent visitor id keys as %q, want v:abc_123", got)
	}

	hashed := handlers.VisitorKey("", "203.0.113.9", "Mozilla/5.0")
	if !strings.HasPrefix(hashed, "h:") || len(hashed) != 2+32 {
		t.Fatalf("an anonymous visitor keys as %q, want h: and 32 hex digits", hashed)
	}
	if strings.Contains(hashed, "203.0.113.9") {
		t.Error("the key carries the address in clear")
	}
	if again := handlers.VisitorKey("", "203.0.113.9", "Mozilla/5.0"); again != hashed {
		t.Error("the same address and agent key differently twice")
	}
	for _, other := range []string{
		handlers.VisitorKey("", "203.0.113.10", "Mozilla/5.0"),
		handlers.VisitorKey("", "203.0.113.9", "Mozilla/5.1"),
	} {
		if other == hashed {
			t.Error("a different address or agent keys the same")
		}
	}
	// The separator keeps "ab"+"c" and "a"+"bc" apart.
	if handlers.VisitorKey("", "ab", "c") == handlers.VisitorKey("", "a", "bc") {
		t.Error("address and agent run together in the hash")
	}
}

func TestCleanUserAgent(t *testing.T) {
	if got := handlers.CleanUserAgent("  Mozilla/5.0 (iPhone)  "); got != "Mozilla/5.0 (iPhone)" {
		t.Errorf("trim: got %q", got)
	}
	if got := handlers.CleanUserAgent("a\x00b\nc\x7fd\xff\xfee"); got != "abcde" {
		t.Errorf("control characters and invalid UTF-8: got %q, want abcde", got)
	}
	long := strings.Repeat("ş", 1000)
	if got := handlers.CleanUserAgent(long); len([]rune(got)) != 300 {
		t.Errorf("a 1000-rune agent is kept as %d runes, want 300", len([]rune(got)))
	}
	if got := handlers.CleanUserAgent(""); got != "" {
		t.Errorf("an absent agent reads %q", got)
	}
}

func TestSummaryWindow(t *testing.T) {
	// 21:30 UTC on 24 September is already 25 September in Istanbul.
	now := time.Date(2026, 9, 24, 21, 30, 0, 0, time.UTC)

	window, message := handlers.SummaryWindow("", "", now)
	if message != "" {
		t.Fatalf("the default window was refused: %s", message)
	}
	if window.FromDay != "2026-08-27" || window.ToDay != "2026-09-25" {
		t.Errorf("default window %s..%s, want the last 30 Istanbul days 2026-08-27..2026-09-25",
			window.FromDay, window.ToDay)
	}
	if !window.Start.Equal(time.Date(2026, 8, 26, 21, 0, 0, 0, time.UTC)) {
		t.Errorf("the window starts at %s, want Istanbul midnight of 27 August (21:00 UTC the day before)", window.Start.UTC())
	}
	if !window.End.Equal(time.Date(2026, 9, 25, 21, 0, 0, 0, time.UTC)) {
		t.Errorf("the window ends at %s, want Istanbul midnight after 25 September", window.End.UTC())
	}

	window, message = handlers.SummaryWindow("2026-09-01", "2026-09-01", now)
	if message != "" || window.FromDay != "2026-09-01" || window.ToDay != "2026-09-01" {
		t.Errorf("a one-day window: %+v %q", window, message)
	}
	if got := window.End.Sub(window.Start); got != 24*time.Hour {
		t.Errorf("one Istanbul day spans %s", got)
	}

	window, message = handlers.SummaryWindow("", "2026-09-10", now)
	if message != "" || window.FromDay != "2026-08-12" {
		t.Errorf("only to: from is %s (%q), want 30 days back, 2026-08-12", window.FromDay, message)
	}
	window, message = handlers.SummaryWindow("2026-09-20", "", now)
	if message != "" || window.ToDay != "2026-09-25" {
		t.Errorf("only from: to is %s (%q), want today, 2026-09-25", window.ToDay, message)
	}

	for _, tc := range []struct{ from, to string }{
		{"2026-09-10", "2026-09-01"}, // reversed
		{"2025-01-01", "2026-09-01"}, // longer than 366 days
		{"01.09.2026", ""},           // wrong format
		{"", "2026-13-01"},           // no such month
		{"2026-09-01'; DROP TABLE menu_events; --", "2026-09-02"},
	} {
		if _, message := handlers.SummaryWindow(tc.from, tc.to, now); message == "" {
			t.Errorf("from=%q to=%q was accepted", tc.from, tc.to)
		}
	}
	// Exactly 366 days is the longest window.
	if _, message := handlers.SummaryWindow("2025-09-01", "2026-09-01", now); message != "" {
		t.Errorf("a 366-day window was refused: %s", message)
	}
	if utils.Istanbul == nil {
		t.Fatal("utils.Istanbul is nil")
	}
}
