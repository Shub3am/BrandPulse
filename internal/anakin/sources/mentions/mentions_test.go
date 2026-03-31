package mentions

import (
	"testing"
	"time"

	"brandpulse/internal/models"
)

// window is the collection window every case in this file is filtered against.
var (
	windowStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
)

func draft(text string, postedAt time.Time) models.Mention {
	return models.Mention{
		Source:     models.SourceReddit,
		ExternalID: "t3_abc",
		Text:       text,
		Lang:       "en",
		PostedAt:   postedAt,
	}
}

func TestFilterWindowIsHalfOpen(t *testing.T) {
	cases := []struct {
		name     string
		postedAt time.Time
		kept     bool
	}{
		{"before the window", windowStart.Add(-time.Second), false},
		{"exactly at start is inside", windowStart, true},
		{"inside", windowStart.Add(72 * time.Hour), true},
		{"exactly at end is outside", windowEnd, false},
		{"after the window", windowEnd.Add(time.Second), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, dropped := Filter([]models.Mention{draft("boAt Airdopes died in a week", tc.postedAt)},
				models.BrandProfile{}, windowStart, windowEnd)

			if got := len(kept) == 1; got != tc.kept {
				t.Fatalf("kept=%v, want %v (dropped=%+v)", got, tc.kept, dropped)
			}
			if !tc.kept && dropped.OutsideWindow != 1 {
				t.Fatalf("OutsideWindow=%d, want 1", dropped.OutsideWindow)
			}
		})
	}
}

func TestFilterNegativeKeywords(t *testing.T) {
	profile := models.BrandProfile{
		NegativeKeywords: []string{"Trent Boult", "cricket", "ODI", "boat ride"},
	}
	inside := windowStart.Add(time.Hour)

	cases := []struct {
		name string
		text string
		kept bool
	}{
		{"clean mention survives", "boAt Airdopes Loop sound quality is fine", true},
		{"multi-word phrase drops", "Trent Boult bowled a great spell", false},
		{"case is ignored", "watched the CRICKET last night with my boAt", false},
		{"phrase with a space drops", "took a boat ride wearing these", false},
		{"short term does not match inside a word", "the melodic mids are good", true},
		{"short term matches standalone", "third ODI highlights", false},
		{"substring of a longer word survives", "cricketer is not the term", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, dropped := Filter([]models.Mention{draft(tc.text, inside)}, profile, windowStart, windowEnd)

			if got := len(kept) == 1; got != tc.kept {
				t.Fatalf("kept=%v, want %v for %q", got, tc.kept, tc.text)
			}
			if !tc.kept && dropped.NegativeKeyword != 1 {
				t.Fatalf("NegativeKeyword=%d, want 1", dropped.NegativeKeyword)
			}
		})
	}
}

// An empty or whitespace-only negative keyword must not compile to a pattern
// that matches every mention. A profile edited by hand in DronaHQ can contain
// one, and the failure would be a source that silently collects nothing.
func TestFilterIgnoresEmptyNegativeKeywords(t *testing.T) {
	profile := models.BrandProfile{NegativeKeywords: []string{"", "   "}}
	inside := windowStart.Add(time.Hour)

	kept, dropped := Filter([]models.Mention{draft("boAt Rockerz review", inside)}, profile, windowStart, windowEnd)
	if len(kept) != 1 {
		t.Fatalf("kept %d mentions, want 1 (dropped=%+v)", len(kept), dropped)
	}
}

func TestFilterCountsEachReasonSeparately(t *testing.T) {
	profile := models.BrandProfile{NegativeKeywords: []string{"cricket"}}
	inside := windowStart.Add(time.Hour)

	kept, dropped := Filter([]models.Mention{
		draft("good earbuds", inside),
		draft("good earbuds but old", windowStart.Add(-time.Hour)),
		draft("cricket highlights", inside),
		draft("also good", inside),
	}, profile, windowStart, windowEnd)

	if len(kept) != 2 {
		t.Fatalf("kept %d, want 2", len(kept))
	}
	if dropped.OutsideWindow != 1 || dropped.NegativeKeyword != 1 {
		t.Fatalf("dropped=%+v, want 1 window and 1 negative", dropped)
	}
	if dropped.Total() != 2 {
		t.Fatalf("Total()=%d, want 2", dropped.Total())
	}
}

func TestFilterOnEmptyInput(t *testing.T) {
	kept, dropped := Filter(nil, models.BrandProfile{}, windowStart, windowEnd)
	if len(kept) != 0 || dropped.Total() != 0 {
		t.Fatalf("kept=%d dropped=%+v, want both empty", len(kept), dropped)
	}
}
