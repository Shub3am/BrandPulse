// main_test.go guards the one property that makes this command safe to run:
// nothing spends a credit without -confirm and BP_FIXTURE_MODE=record.
//
// Every test here drives the real demo/brand.json. A plan built from a fixture
// brand would pass while the real one overspends.
package main

import (
	"os"
	"strings"
	"testing"

	"brandpulse/internal/models"
)

const brandPath = "../brand.json"

func loadForTest(t *testing.T) (models.BrandProfile, []models.BrandProfile) {
	t.Helper()
	brand, competitors, err := loadBrands(brandPath)
	if err != nil {
		t.Fatalf("loadBrands: %v", err)
	}
	return brand, competitors
}

// planForTest mirrors what run() passes planFor: the brand first, then its
// competitors, as one list.
func planForTest(brand models.BrandProfile, competitors []models.BrandProfile) []step {
	return planFor(append([]models.BrandProfile{brand}, competitors...))
}

// anakin.NewHTTPClient panics as a B1 stub, so a dry run that built a client
// would panic rather than print. When B1 lands it would spend instead, which
// is the failure this test exists for.
func TestDryRunBuildsNoClient(t *testing.T) {
	if err := run(false, brandPath); err != nil {
		t.Fatalf("the dry run failed: %v", err)
	}
}

// -confirm in replay mode would spend credits and write nothing, which is the
// worst of both. The refusal has to come before the client is built.
func TestConfirmWithoutRecordModeIsRefused(t *testing.T) {
	for _, mode := range []string{"", "replay", "live"} {
		t.Run("mode="+mode, func(t *testing.T) {
			t.Setenv("BP_FIXTURE_MODE", mode)

			err := run(true, brandPath)
			if err == nil {
				t.Fatal("-confirm was accepted outside record mode")
			}
			if !strings.Contains(err.Error(), "record") {
				t.Errorf("the refusal does not name the mode it needs: %v", err)
			}
		})
	}
}

// The brief's rule: over this, cut scope rather than raise the ceiling.
func TestTheRealPlanFitsTheBudget(t *testing.T) {
	brand, competitors := loadForTest(t)
	estimate := totalCredits(planForTest(brand, competitors))

	if estimate > abortAbove {
		t.Errorf("the plan estimates %d credits, over the %d abort threshold", estimate, abortAbove)
	}
	if remaining := freeTierCredits - spentBeforeRecording; estimate > remaining {
		t.Errorf("the plan estimates %d credits and only %d are left", estimate, remaining)
	}
	if estimate <= 0 {
		t.Error("the plan estimates nothing, so it would record nothing")
	}
}

// Share of voice compares the brand against its competitors in one corpus, so
// a competitor missing from the plan is a missing dashboard number.
func TestEveryCompetitorIsRecorded(t *testing.T) {
	brand, competitors := loadForTest(t)
	if len(competitors) != len(brand.Competitors) {
		t.Fatalf("got %d competitor profiles, want %d", len(competitors), len(brand.Competitors))
	}

	plan := planForTest(brand, competitors)
	for _, rival := range brand.Competitors {
		found := false
		for _, s := range plan {
			if strings.HasPrefix(s.What, rival+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the plan never fetches anything for %q", rival)
		}
	}
}

// The appstore feed is keyed by an app id and the Play listing by a package
// name. We hold neither for a rival, so planning those calls buys two
// guaranteed adapter errors.
func TestCompetitorsSkipTheHandleKeyedSources(t *testing.T) {
	_, competitors := loadForTest(t)

	for _, rival := range competitors {
		for _, source := range rival.Sources {
			if source == models.SourceAppstore || source == models.SourcePlaystore {
				t.Errorf("%s is set to collect %s without a handle for it", rival.Name, source)
			}
		}
		if rival.Version != 1 {
			t.Errorf("%s has Version %d, which means 'never onboarded' in the DB", rival.Name, rival.Version)
		}
	}
}

// A window under 14 days gives the detector one day of history and a z-score
// computed against nothing.
func TestTheWindowGivesTheDetectorABaseline(t *testing.T) {
	if recordingWindowDays < 14 {
		t.Errorf("the recording window is %d days, the detector needs at least 14", recordingWindowDays)
	}
}

// The ledger is hand-kept because the Anakin API has no usage endpoint, so it
// is the kind of constant that goes stale silently.
func TestTheLedgerIsInsideTheFreeTier(t *testing.T) {
	if spentBeforeRecording >= freeTierCredits {
		t.Errorf("the ledger says %d of %d are already spent", spentBeforeRecording, freeTierCredits)
	}
}

func TestBrandFileIsTheOneTheDemoUses(t *testing.T) {
	if _, err := os.Stat(brandPath); err != nil {
		t.Fatalf("demo/brand.json is missing: %v", err)
	}
	brand, _ := loadForTest(t)
	if len(brand.Sources) == 0 {
		t.Error("the demo brand collects no sources, so the recording would be empty")
	}
}
