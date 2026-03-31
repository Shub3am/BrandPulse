package sources

import (
	"testing"

	"brandpulse/internal/models"
)

func TestAdaptersHoldsTheSixProbedSources(t *testing.T) {
	want := []models.Source{
		models.SourceReddit,
		models.SourceYoutube,
		models.SourceNews,
		models.SourceWeb,
		models.SourceAppstore,
		models.SourcePlaystore,
	}

	if len(Adapters) != len(want) {
		t.Fatalf("got %d adapters, want %d", len(Adapters), len(want))
	}
	for _, source := range want {
		if Adapters[source] == nil {
			t.Errorf("no adapter registered for %q", source)
		}
	}
}

func TestAdaptersExcludesTheSourcesThatFailedTheirProbe(t *testing.T) {
	// amazon returned review metadata with empty review text, and x was never
	// probed at all. Registering either would put a source in the run log, the
	// demo and the pitch that produces nothing.
	for _, source := range []models.Source{models.SourceAmazon, models.SourceX, models.SourceInstagram, models.SourceFlipkart} {
		if _, ok := For(source); ok {
			t.Errorf("%q is registered but was never proved to work", source)
		}
	}
}

func TestEveryRegisteredKeyIsAKnownSource(t *testing.T) {
	// A typo in a map key would register an adapter under a source that
	// Mention.Validate rejects, and nothing would collect it.
	for source := range Adapters {
		if !source.Valid() {
			t.Errorf("%q is not a known models.Source", source)
		}
	}
}

func TestForDistinguishesAbsentFromNil(t *testing.T) {
	if _, ok := For(models.SourceReddit); !ok {
		t.Error("For(reddit) reported no adapter")
	}
	if _, ok := For(models.SourceAmazon); ok {
		t.Error("For(amazon) reported an adapter")
	}
}
