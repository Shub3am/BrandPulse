// Tests for the counting rule. Hand-built EnrichedMentions rather than
// fixtures, because nothing here reads a file, a clock or a network.
//
// The cases that matter are the ones where a wrong answer still marshals: an
// unrecognised mention inflating the brand's share, and an empty window
// producing NaN.
package main

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"brandpulse/internal/models"
)

var (
	windowStart = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
)

func testProfile() models.BrandProfile {
	profile := models.NewBrandProfile("brd_01J", "Mamaearth")
	profile.Competitors = []string{"Plum", "The Derma Co"}
	return profile
}

// mention builds one EnrichedMention. Only the fields the handler reads are
// set; everything else stays zero on purpose, because a counter that depends
// on a field it does not name is a counter that breaks when that field moves.
func mention(source models.Source, aboutBrand bool, aboutCompetitor string) models.EnrichedMention {
	return models.EnrichedMention{
		Mention:    models.Mention{ID: "mn_" + aboutCompetitor + string(source), Source: source},
		Enrichment: models.Enrichment{IsAboutBrand: aboutBrand, AboutCompetitor: aboutCompetitor},
	}
}

func handle(t *testing.T, in models.SOVInput) models.ShareOfVoice {
	t.Helper()
	out, err := SOVHandler{}.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	return out
}

func TestHandleCountsBrandAgainstListedCompetitors(t *testing.T) {
	in := models.SOVInput{
		Profile:     testProfile(),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Enriched: []models.EnrichedMention{
			mention(models.SourceX, true, ""),
			mention(models.SourceX, true, ""),
			mention(models.SourceReddit, false, "Plum"),
			mention(models.SourceReddit, false, "The Derma Co"),
		},
	}

	out := handle(t, in)

	if out.TotalMentions != 4 {
		t.Errorf("TotalMentions = %d, want 4", out.TotalMentions)
	}
	if out.BrandShare != 50 {
		t.Errorf("BrandShare = %v, want 50", out.BrandShare)
	}
	if got := out.CompetitorShares["Plum"]; got != 25 {
		t.Errorf("CompetitorShares[Plum] = %v, want 25", got)
	}
	if got := out.CompetitorShares["The Derma Co"]; got != 25 {
		t.Errorf("CompetitorShares[The Derma Co] = %v, want 25", got)
	}
	if _, present := out.CompetitorShares["Mamaearth"]; present {
		t.Error("CompetitorShares contains the brand itself")
	}
	if out.BrandID != "brd_01J" {
		t.Errorf("BrandID = %q, want brd_01J", out.BrandID)
	}
	if !out.WindowStart.Equal(windowStart) || !out.WindowEnd.Equal(windowEnd) {
		t.Errorf("window = %v..%v, want %v..%v", out.WindowStart, out.WindowEnd, windowStart, windowEnd)
	}
}

// The easiest thing to get quietly wrong: a mention that is about neither the
// brand nor a listed competitor must leave the denominator, not join the
// brand's numerator.
func TestHandleExcludesUnattributableMentionsFromTheDenominator(t *testing.T) {
	cases := []struct {
		name     string
		excluded models.EnrichedMention
	}{
		{"keyword collision", mention(models.SourceNews, false, "")},
		{"unlisted competitor", mention(models.SourceNews, false, "Minimalist")},
		{"unlisted competitor while about the brand", mention(models.SourceNews, true, "Minimalist")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := models.SOVInput{
				Profile:     testProfile(),
				WindowStart: windowStart,
				WindowEnd:   windowEnd,
				Enriched: []models.EnrichedMention{
					mention(models.SourceX, true, ""),
					mention(models.SourceX, false, "Plum"),
					tc.excluded,
				},
			}

			out := handle(t, in)

			if out.TotalMentions != 2 {
				t.Errorf("TotalMentions = %d, want 2: the third mention counts for nobody", out.TotalMentions)
			}
			if out.BrandShare != 50 {
				t.Errorf("BrandShare = %v, want 50", out.BrandShare)
			}
			if _, present := out.CompetitorShares["Minimalist"]; present {
				t.Error("an unlisted competitor was added to CompetitorShares")
			}
		})
	}
}

// A comparison carries IsAboutBrand true and a competitor name. The contract
// counts the brand only when AboutCompetitor is empty, so it belongs to the
// competitor.
func TestHandleCountsAComparisonForTheCompetitor(t *testing.T) {
	in := models.SOVInput{
		Profile:     testProfile(),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Enriched:    []models.EnrichedMention{mention(models.SourceReddit, true, "Plum")},
	}

	out := handle(t, in)

	if out.BrandShare != 0 {
		t.Errorf("BrandShare = %v, want 0", out.BrandShare)
	}
	if got := out.CompetitorShares["Plum"]; got != 100 {
		t.Errorf("CompetitorShares[Plum] = %v, want 100", got)
	}
}

func TestHandleMatchesCompetitorsCaseInsensitively(t *testing.T) {
	in := models.SOVInput{
		Profile:     testProfile(),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Enriched: []models.EnrichedMention{
			mention(models.SourceX, false, "plum"),
			mention(models.SourceX, false, "PLUM"),
			mention(models.SourceX, false, "  the derma co  "),
		},
	}

	out := handle(t, in)

	if got := out.CompetitorShares["Plum"]; math.Abs(got-200.0/3) > 1e-9 {
		t.Errorf("CompetitorShares[Plum] = %v, want 66.67: casing must fold to the profile's spelling", got)
	}
	if got := out.CompetitorShares["The Derma Co"]; math.Abs(got-100.0/3) > 1e-9 {
		t.Errorf("CompetitorShares[The Derma Co] = %v, want 33.33", got)
	}
	if len(out.CompetitorShares) != 2 {
		t.Errorf("CompetitorShares has %d keys, want 2: casings must not become separate competitors", len(out.CompetitorShares))
	}
}

func TestHandleBreaksSharesDownPerSource(t *testing.T) {
	in := models.SOVInput{
		Profile:     testProfile(),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Enriched: []models.EnrichedMention{
			mention(models.SourceX, true, ""),
			mention(models.SourceX, false, "Plum"),
			mention(models.SourceReddit, true, ""),
			mention(models.SourceReddit, true, ""),
			mention(models.SourceNews, false, ""),
		},
	}

	out := handle(t, in)

	if len(out.BySource) != 2 {
		t.Fatalf("BySource has %d sources, want 2: a source with no countable mention has no row", len(out.BySource))
	}
	if got := out.BySource[models.SourceX]["Mamaearth"]; got != 50 {
		t.Errorf("BySource[x][Mamaearth] = %v, want 50", got)
	}
	if got := out.BySource[models.SourceX]["Plum"]; got != 50 {
		t.Errorf("BySource[x][Plum] = %v, want 50", got)
	}
	if got := out.BySource[models.SourceReddit]["Mamaearth"]; got != 100 {
		t.Errorf("BySource[reddit][Mamaearth] = %v, want 100", got)
	}
}

// Zero countable mentions must give the zeroed struct, not NaN. NaN survives
// the handler and fails several agents downstream as "json: unsupported
// value", so the assertion is the marshal, not just the number.
func TestHandleEmptyWindowProducesNoNaN(t *testing.T) {
	cases := []struct {
		name     string
		enriched []models.EnrichedMention
	}{
		{"no mentions at all", nil},
		{"no countable mentions", []models.EnrichedMention{mention(models.SourceWeb, false, "")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := models.SOVInput{
				Profile:     testProfile(),
				WindowStart: windowStart,
				WindowEnd:   windowEnd,
				Enriched:    tc.enriched,
			}

			out := handle(t, in)

			if out.TotalMentions != 0 {
				t.Errorf("TotalMentions = %d, want 0", out.TotalMentions)
			}
			if math.IsNaN(out.BrandShare) {
				t.Fatal("BrandShare is NaN")
			}
			if out.BrandShare != 0 {
				t.Errorf("BrandShare = %v, want 0", out.BrandShare)
			}
			if out.CompetitorShares == nil || out.BySource == nil {
				t.Fatal("NewShareOfVoice's maps must survive: a nil map panics on the first write downstream")
			}
			if len(out.CompetitorShares) != 0 || len(out.BySource) != 0 {
				t.Errorf("empty window produced shares: %v / %v", out.CompetitorShares, out.BySource)
			}
			if _, err := json.Marshal(out); err != nil {
				t.Fatalf("marshal failed, which is how a NaN actually surfaces: %v", err)
			}
		})
	}
}

func TestHandleSharesSumToOneHundred(t *testing.T) {
	in := models.SOVInput{
		Profile:     testProfile(),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Enriched: []models.EnrichedMention{
			mention(models.SourceX, true, ""),
			mention(models.SourceReddit, true, ""),
			mention(models.SourceNews, false, "Plum"),
			mention(models.SourceAmazon, false, "The Derma Co"),
			mention(models.SourceWeb, false, "Minimalist"),
			mention(models.SourceWeb, false, ""),
			mention(models.SourceYoutube, true, ""),
		},
	}

	out := handle(t, in)

	sum := out.BrandShare
	for _, share := range out.CompetitorShares {
		sum += share
	}
	if math.Abs(sum-100) > 1e-9 {
		t.Errorf("shares sum to %v, want 100", sum)
	}
}
