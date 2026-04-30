package main

import (
	"testing"
	"time"
)

var (
	corpusEnd = time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	replayNow = time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
)

func TestTheCorpusEndLandsExactlyOnNow(t *testing.T) {
	got := shift(corpusEnd, corpusEnd, replayNow)
	if !got.Equal(replayNow) {
		t.Errorf("the last mention landed at %s, want %s", got, replayNow)
	}
}

// The baseline is a count per source per hour over fourteen days. If the shift
// changed the gaps, every one of those counts would change with it and the
// z-score on stage would be a different number from the one in the fixture.
func TestTheGapsBetweenMentionsSurviveTheShift(t *testing.T) {
	posted := []time.Time{
		corpusEnd.Add(-14 * 24 * time.Hour),
		corpusEnd.Add(-38 * time.Hour),
		corpusEnd.Add(-97 * time.Minute),
		corpusEnd.Add(-45 * time.Second),
		corpusEnd,
	}

	for i := 1; i < len(posted); i++ {
		before := posted[i].Sub(posted[i-1])
		after := shift(posted[i], corpusEnd, replayNow).Sub(shift(posted[i-1], corpusEnd, replayNow))
		if before != after {
			t.Errorf("gap %d was %s, became %s", i, before, after)
		}
	}
}

// Nothing may land in the future. A mention dated after now reads as a clock
// bug to anyone watching and would sit outside every window the pipeline asks
// the database for.
func TestNothingIsShiftedPastNow(t *testing.T) {
	for _, age := range []time.Duration{
		14 * 24 * time.Hour, 24 * time.Hour, time.Hour, time.Minute, 0,
	} {
		got := shift(corpusEnd.Add(-age), corpusEnd, replayNow)
		if got.After(replayNow) {
			t.Errorf("a mention %s old landed at %s, after now (%s)", age, got, replayNow)
		}
	}
}

// Same inputs, same output, every run. This is the property the whole rehearsal
// rests on.
func TestTheShiftIsDeterministic(t *testing.T) {
	posted := corpusEnd.Add(-73 * time.Hour)
	first := shift(posted, corpusEnd, replayNow)
	for i := 0; i < 100; i++ {
		if again := shift(posted, corpusEnd, replayNow); !again.Equal(first) {
			t.Fatalf("run %d gave %s, the first run gave %s", i, again, first)
		}
	}
}

// The input timestamps arrive from Postgres in whatever zone the driver chose,
// and a corpus half in IST and half in UTC is a demo where the baseline window
// is off by five and a half hours.
func TestTheOutputIsAlwaysUTC(t *testing.T) {
	ist := time.FixedZone("IST", 5*60*60+30*60)
	got := shift(corpusEnd.In(ist), corpusEnd.In(ist), replayNow.In(ist))

	if got.Location() != time.UTC {
		t.Errorf("shift returned %s in %s, want UTC", got, got.Location())
	}
	if !got.Equal(replayNow) {
		t.Errorf("the zone changed the instant: got %s, want %s", got, replayNow)
	}
}

// Replaying a corpus that already ends now is the second run of run_demo.sh.
// It must be a no-op, not a nudge.
func TestReplayingAnAlreadyCurrentCorpusChangesNothing(t *testing.T) {
	posted := replayNow.Add(-6 * time.Hour)
	if got := shift(posted, replayNow, replayNow); !got.Equal(posted) {
		t.Errorf("a current corpus moved: %s became %s", posted, got)
	}
}

func TestLatestIsTheNewestMentionWhateverTheOrder(t *testing.T) {
	newest := corpusEnd
	corpus := []corpusRow{
		{id: "mn_2", postedAt: corpusEnd.Add(-2 * time.Hour)},
		{id: "mn_4", postedAt: newest},
		{id: "mn_1", postedAt: corpusEnd.Add(-9 * time.Hour)},
		{id: "mn_3", postedAt: corpusEnd.Add(-30 * time.Minute)},
	}

	if got := latest(corpus); !got.Equal(newest) {
		t.Errorf("latest = %s, want %s", got, newest)
	}
}

func TestLatestOfASingleMention(t *testing.T) {
	corpus := []corpusRow{{id: "mn_1", postedAt: corpusEnd}}
	if got := latest(corpus); !got.Equal(corpusEnd) {
		t.Errorf("latest = %s, want %s", got, corpusEnd)
	}
}
