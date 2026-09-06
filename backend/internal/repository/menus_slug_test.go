package repository

import (
	"fmt"
	"strings"
	"testing"

	"karecik/backend/internal/utils"
)

// The slug-length guarantee of EnsureUniqueMenuSlug lives in two pure helpers,
// trimSlug and menuSlugFamily, so it can be proved here without a database.
// The query half of EnsureUniqueMenuSlug (which family is collected, which
// suffix wins) belongs to the integration suite.

func TestTrimSlugCutsAndDropsTrailingDash(t *testing.T) {
	cases := []struct {
		name  string
		slug  string
		limit int
		want  string
	}{
		{"shorter than the limit is untouched", "kahvalti", 60, "kahvalti"},
		{"exactly at the limit is untouched", "kahvalti", 8, "kahvalti"},
		{"a plain cut", "kahvalti-menusu", 8, "kahvalti"},
		{"a cut landing on a dash loses it", "kahvalti-menusu", 9, "kahvalti"},
		{"a cut landing after a dash loses it", "kahve--", 6, "kahve"},
		{"a zero limit empties the slug", "kahvalti", 0, ""},
		{"a negative limit empties the slug", "kahvalti", -3, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := trimSlug(tc.slug, tc.limit); got != tc.want {
				t.Fatalf("trimSlug(%q, %d) = %q, want %q",
					tc.slug, tc.limit, got, tc.want)
			}
		})
	}
}

// A base long enough to leave no room for the suffix is the whole defect: a
// 59-character base used to produce a 61-character "-2" slug that CreateMenu
// stored and utils.IsValidSlug then rejected.
func TestMenuSlugFamilyLeavesRoomForEverySuffix(t *testing.T) {
	longest := len(fmt.Sprintf("-%d", maxMenuSlugSuffix))

	for _, length := range []int{2, 10, 54, 55, 56, 59, 60} {
		base := strings.Repeat("a", length)
		family := menuSlugFamily(base)

		if len(family)+longest > menuSlugMaxLength {
			t.Fatalf("menuSlugFamily(%d chars) = %d chars, no room for %q",
				length, len(family), fmt.Sprintf("-%d", maxMenuSlugSuffix))
		}
		if !strings.HasPrefix(base, family) {
			t.Fatalf("menuSlugFamily(%d chars) = %q, not a prefix of the base",
				length, family)
		}
		for _, suffix := range []int{2, 9, 10, 99, 100, 999, maxMenuSlugSuffix} {
			candidate := fmt.Sprintf("%s-%d", family, suffix)
			if !utils.IsValidSlug(candidate) {
				t.Fatalf("candidate %q (%d chars) from a %d-character base is not a valid slug",
					candidate, len(candidate), length)
			}
		}
	}
}

// The cut must not leave a trailing dash behind, or the candidate becomes
// "base--2" and the bare family a slug utils.IsValidSlug refuses.
func TestMenuSlugFamilyNeverEndsInADash(t *testing.T) {
	// 54 characters of "a", a dash, then filler: the cut lands right on the
	// dash at the 55-character room boundary.
	base := strings.Repeat("a", 54) + "-" + strings.Repeat("b", 5)

	family := menuSlugFamily(base)
	if strings.HasSuffix(family, "-") {
		t.Fatalf("menuSlugFamily(%q) = %q, ends in a dash", base, family)
	}
	if candidate := family + "-2"; !utils.IsValidSlug(candidate) {
		t.Fatalf("candidate %q is not a valid slug", candidate)
	}
}

// Nothing usable left after the cut still has to come back a valid slug rather
// than an empty one; Slugify cannot produce such a base, but the fallback is
// what keeps that unreachable rather than merely unlikely.
func TestMenuSlugFamilyFallsBackWhenNothingSurvives(t *testing.T) {
	if got := menuSlugFamily(""); got != "menu" {
		t.Fatalf("menuSlugFamily(\"\") = %q, want %q", got, "menu")
	}
	if got := menuSlugFamily(strings.Repeat("-", 80)); got != "menu" {
		t.Fatalf("menuSlugFamily(all dashes) = %q, want %q", got, "menu")
	}
}
