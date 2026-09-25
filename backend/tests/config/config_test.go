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
