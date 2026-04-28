package main

import (
	"strings"
	"testing"

	"brandpulse/internal/models"
)

// tenSources is the demo brand's shape: more sources than the flow guard
// allows, on purpose, so the cap fires where an audience can see it.
func tenSources() models.BrandProfile {
	return models.BrandProfile{
		BrandID: "brd_test",
		Name:    "Testbrand",
		Sources: []models.Source{
			models.SourceX, models.SourceReddit, models.SourceYoutube,
			models.SourceNews, models.SourcePlaystore, models.SourceAppstore,
			models.SourceAmazon, models.SourceFlipkart, models.SourceInstagram,
			models.SourceWeb,
		},
	}
}

// tenYields ranks every source distinctly, so the two losers are unambiguous.
// web is the worst at 0.1 and instagram the second worst at 0.2.
func tenYields() map[models.Source]float64 {
	return map[models.Source]float64{
		models.SourceX:         3.4,
		models.SourceReddit:    2.9,
		models.SourceYoutube:   2.1,
		models.SourceNews:      1.8,
		models.SourcePlaystore: 1.5,
		models.SourceAppstore:  1.1,
		models.SourceAmazon:    0.9,
		models.SourceFlipkart:  0.6,
		models.SourceInstagram: 0.2,
		models.SourceWeb:       0.1,
	}
}

func TestTenSourcesLoseTheTwoLowestYielding(t *testing.T) {
	chosen, skipped := chooseSources(tenSources(), tenYields(), fanOutCap)

	if len(chosen) != 8 {
		t.Fatalf("chose %d sources, want 8: %v", len(chosen), chosen)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped %d sources, want 2: %v", len(skipped), skipped)
	}

	// Ordered worst-last, so instagram (0.2) precedes web (0.1).
	want := []models.Source{models.SourceInstagram, models.SourceWeb}
	for i, source := range want {
		if skipped[i] != source {
			t.Errorf("skipped[%d] = %q, want %q (full: %v)", i, skipped[i], source, skipped)
		}
	}

	for _, source := range skipped {
		for _, kept := range chosen {
			if kept == source {
				t.Errorf("%q is both chosen and skipped", source)
			}
		}
	}
}

func TestSourcesAreRankedByYieldDescending(t *testing.T) {
	chosen, _ := chooseSources(tenSources(), tenYields(), fanOutCap)

	yields := tenYields()
	for i := 1; i < len(chosen); i++ {
		if yields[chosen[i-1]] < yields[chosen[i]] {
			t.Fatalf("%q (%v) ranked above %q (%v)",
				chosen[i-1], yields[chosen[i-1]], chosen[i], yields[chosen[i]])
		}
	}
	if chosen[0] != models.SourceX {
		t.Errorf("top source is %q, want x, the highest yielding", chosen[0])
	}
}

// A ranking that reorders between runs is a demo that cannot be rehearsed, so
// the tie-break is pinned rather than left to Go's randomised map iteration.
func TestRankingIsStableAcrossRuns(t *testing.T) {
	profile := tenSources()
	noYields := map[models.Source]float64{}

	first, _ := chooseSources(profile, noYields, fanOutCap)
	for i := 0; i < 50; i++ {
		again, _ := chooseSources(profile, noYields, fanOutCap)
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("run %d position %d: got %q, first run gave %q", i, j, again[j], first[j])
			}
		}
	}
}

func TestATieBreaksOnTheSourceName(t *testing.T) {
	profile := models.BrandProfile{Sources: []models.Source{
		models.SourceReddit, models.SourceAmazon, models.SourceNews,
	}}
	// All three unranked, so all three yield 0 and only the name separates them.
	chosen, _ := chooseSources(profile, map[models.Source]float64{}, fanOutCap)

	want := []models.Source{models.SourceAmazon, models.SourceNews, models.SourceReddit}
	for i := range want {
		if chosen[i] != want[i] {
			t.Errorf("chosen[%d] = %q, want %q", i, chosen[i], want[i])
		}
	}
}

func TestFewerSourcesThanTheCapSkipsNothing(t *testing.T) {
	profile := models.BrandProfile{Sources: []models.Source{models.SourceX, models.SourceReddit}}

	chosen, skipped := chooseSources(profile, tenYields(), fanOutCap)
	if len(chosen) != 2 {
		t.Errorf("chose %d, want 2", len(chosen))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped %v, want nothing", skipped)
	}
	if reason := degradedReason(skipped, fanOutCap); reason != "" {
		t.Errorf("degradedReason = %q, want empty when nothing was dropped", reason)
	}
}

func TestARepeatedSourceIsCalledOnce(t *testing.T) {
	profile := models.BrandProfile{Sources: []models.Source{
		models.SourceX, models.SourceX, models.SourceReddit, models.SourceX,
	}}

	chosen, _ := chooseSources(profile, tenYields(), fanOutCap)
	if len(chosen) != 2 {
		t.Fatalf("chose %v, want x and reddit once each", chosen)
	}
}

// "degraded" on its own tells a founder nothing and tells a judge less, so the
// reason names the cap and every source that lost.
func TestDegradedReasonNamesTheCapAndTheLosers(t *testing.T) {
	_, skipped := chooseSources(tenSources(), tenYields(), fanOutCap)
	reason := degradedReason(skipped, fanOutCap)

	for _, want := range []string{"8", "instagram", "web"} {
		if !strings.Contains(reason, want) {
			t.Errorf("degradedReason = %q, missing %q", reason, want)
		}
	}
}

func TestSplitCredits(t *testing.T) {
	cases := []struct {
		name    string
		budget  int
		sources int
		want    int
	}{
		{"even split", 160, 8, 20},
		{"remainder is dropped, not given to the first source", 150, 8, 18},
		{"no sources means no ceiling to compute", 150, 0, 0},
		{"no budget means no credits", 0, 8, 0},
		{"a budget smaller than the source count gives nobody anything", 3, 8, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := splitCredits(c.budget, c.sources); got != c.want {
				t.Errorf("splitCredits(%d, %d) = %d, want %d", c.budget, c.sources, got, c.want)
			}
		})
	}
}
