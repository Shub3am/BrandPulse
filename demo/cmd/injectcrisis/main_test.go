package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"brandpulse/demo/crisis"
	"brandpulse/internal/models"
)

// The rules bucket by source and hour, so the shape that matters is how many
// mentions landed on each source inside one hour. That is asserted here; that
// those numbers actually fire the rule is asserted in
// agents/bp-detector/injection_test.go, against the real thresholds.
func TestTheSurgeIsSpreadOverThreeSources(t *testing.T) {
	corpus := crisis.Mentions("brd_demo", time.Now().UTC())

	perSource := map[models.Source]int{}
	for _, em := range corpus {
		perSource[em.Mention.Source]++
	}

	if len(perSource) != 3 {
		t.Fatalf("the surge landed on %d sources, want 3: %v", len(perSource), perSource)
	}
	for _, source := range crisis.Sources {
		if perSource[source] < 13 {
			t.Errorf("%s got %d mentions, too few to look like a surge", source, perSource[source])
		}
	}
}

func TestEveryMentionBelongsToTheBrandItWasInjectedFor(t *testing.T) {
	for _, em := range crisis.Mentions("brd_other", time.Now().UTC()) {
		if em.Mention.BrandID != "brd_other" {
			t.Fatalf("mention %q carries brand %q", em.Mention.ID, em.Mention.BrandID)
		}
		if em.Enrichment.MentionID != em.Mention.ID {
			t.Fatalf("enrichment points at %q, mention is %q", em.Enrichment.MentionID, em.Mention.ID)
		}
	}
}

// CI runs this path. It must not need DATABASE_URL, a container or a network.
func TestDryRunWritesTheShapeAndTouchesNothing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	if err := run(context.Background(), "brd_demo", true, false, &out); err != nil {
		t.Fatalf("dry run: %v", err)
	}

	printed := out.String()
	for _, want := range []string{
		"dry run",
		"nothing written",
		"brd_demo",
		"40 synthetic mentions",
		string(models.SourceX),
		string(models.SourceReddit),
		string(models.SourceInstagram),
		crisis.SyntheticKey,
	} {
		if !strings.Contains(printed, want) {
			t.Errorf("dry run output is missing %q:\n%s", want, printed)
		}
	}
}

// -dry-run wins over -cleanup. A dry run that deletes rows is the worst
// possible reading of the flag, and the flag is typed by hand on stage.
func TestDryRunWinsOverCleanup(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	if err := run(context.Background(), "brd_demo", true, true, &out); err != nil {
		t.Fatalf("dry run with cleanup: %v", err)
	}
	if strings.Contains(out.String(), "removed") {
		t.Errorf("a dry run reported a deletion:\n%s", out.String())
	}
}

func TestWithoutDryRunTheCommandNeedsADatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	err := run(context.Background(), "brd_demo", false, false, &out)
	if err == nil {
		t.Fatal("a real injection with no DATABASE_URL reported success")
	}
	if out.Len() != 0 {
		t.Errorf("it printed something before failing:\n%s", out.String())
	}
}
