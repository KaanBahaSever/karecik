package config_test

// The analytics settings as config.Load reads them, and the retention sweep's
// cutoff. The one outcome that must never happen is a setting meant as "keep
// the events for a long time" producing a cutoff that deletes all of them —
// which is what a day count large enough to wrap time.AddDate used to do.

import (
	"math"
	"strings"
	"testing"
	"time"

	"karecik/backend/internal/config"
)

func TestRetentionDays(t *testing.T) {
	for _, tc := range []struct {
		raw     int64
		want    int
		warning bool
	}{
		{0, 0, false},
		{1, 1, false},
		{90, 90, false},
		{config.MaxAnalyticsRetentionDays, config.MaxAnalyticsRetentionDays, false},
		{-1, 90, true},
		{math.MinInt64, 90, true},
		{config.MaxAnalyticsRetentionDays + 1, config.MaxAnalyticsRetentionDays, true},
		{100_000_000, config.MaxAnalyticsRetentionDays, true},
		{1 << 60, config.MaxAnalyticsRetentionDays, true},
		{math.MaxInt64, config.MaxAnalyticsRetentionDays, true},
	} {
		got, warning := config.RetentionDays(tc.raw)
		if got != tc.want || (warning != "") != tc.warning {
			t.Errorf("RetentionDays(%d) = %d, %q — want %d and a warning: %v", tc.raw, got, warning, tc.want, tc.warning)
		}
		if tc.warning && !strings.Contains(warning, "ANALYTICS_RETENTION_DAYS") {
			t.Errorf("the warning for %d does not name the setting: %q", tc.raw, warning)
		}
	}
}

func TestDailyEventCap(t *testing.T) {
	for _, tc := range []struct {
		raw     int64
		want    int
		warning bool
	}{
		{0, 0, false},
		{1, 1, false},
		{10000, 10000, false},
		{-5, 10000, true},
		{math.MaxInt64, math.MaxInt, false},
	} {
		got, warning := config.DailyEventCap(tc.raw)
		if got != tc.want || (warning != "") != tc.warning {
			t.Errorf("DailyEventCap(%d) = %d, %q — want %d and a warning: %v", tc.raw, got, warning, tc.want, tc.warning)
		}
	}
}

func TestAnalyticsCutoffIsAlwaysSafelyInThePast(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	for _, days := range []int{1, 90, config.MaxAnalyticsRetentionDays} {
		cutoff, ok := config.AnalyticsCutoff(now, days)
		if !ok {
			t.Errorf("%d days gave no cutoff", days)
			continue
		}
		if want := now.AddDate(0, 0, -days); !cutoff.Equal(want) {
			t.Errorf("%d days gave %s, want %s", days, cutoff, want)
		}
	}

	// Every one of these must refuse rather than compute: the largest are the
	// values whose arithmetic wraps round to "now" or lands before any date the
	// database can hold.
	for _, days := range []int{0, -1, config.MaxAnalyticsRetentionDays + 1, 100_000_000, 1 << 60, math.MaxInt} {
		if cutoff, ok := config.AnalyticsCutoff(now, days); ok {
			t.Errorf("%d days gave the cutoff %s, want none", days, cutoff)
		}
	}
}

// ANALYTICS_EXCLUDED_IPS: every separator people paste works, each entry is
// read by the panel's rules, and every entry that is skipped is named — value
// and reason — because the start-up log is the only place the list can be
// checked: no endpoint ever shows it.
func TestExcludedIPs(t *testing.T) {
	prefixes, warnings := config.ExcludedIPs(
		" 198.18.139.87,192.0.2.0/24 ; 2001:DB8:77::/48\n\tnot-an-ip 10.0.0.0/8, ::ffff:198.18.139.87 ,1.2.3.4/33")

	want := []string{"198.18.139.87/32", "192.0.2.0/24", "2001:db8:77::/48"}
	if len(prefixes) != len(want) {
		t.Fatalf("ExcludedIPs kept %v, want %v", prefixes, want)
	}
	for i, prefix := range prefixes {
		if prefix.String() != want[i] {
			t.Errorf("entry %d = %s, want %s", i, prefix, want[i])
		}
	}

	if len(warnings) != 3 {
		t.Fatalf("ExcludedIPs warned %q, want one line for each of the three skipped entries", warnings)
	}
	for i, tc := range []struct{ value, reason string }{
		{`"not-an-ip"`, "not an IP address"},
		{`"10.0.0.0/8"`, "broader than /16"},
		{`"1.2.3.4/33"`, "not an IP address"},
	} {
		line := warnings[i]
		if !strings.Contains(line, "ANALYTICS_EXCLUDED_IPS") || !strings.Contains(line, tc.value) ||
			!strings.Contains(line, tc.reason) {
			t.Errorf("warning %d = %q, want it to name the setting, %s and %q", i, line, tc.value, tc.reason)
		}
	}

	for _, raw := range []string{"", "   ", ",;"} {
		if prefixes, warnings := config.ExcludedIPs(raw); len(prefixes) != 0 || len(warnings) != 0 {
			t.Errorf("ExcludedIPs(%q) = %v, %q — want nothing", raw, prefixes, warnings)
		}
	}
}
