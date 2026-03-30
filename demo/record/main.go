// demo/record is the one live Anakin recording session, B2's only live spend
// of the 300 free credits.
//
// It must not run without -confirm, and the dry run must not build an Anakin
// client at all: the estimate is arithmetic over the plan, so the path a person
// runs to decide whether to spend cannot itself spend.
//
//	go run ./demo/record                                  # estimate, spends nothing
//	BP_FIXTURE_MODE=record go run ./demo/record -confirm   # the real thing, once
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources"
	"brandpulse/internal/models"
)

const (
	// freeTierCredits is the whole account, not the whole run.
	freeTierCredits = 300

	// spentBeforeRecording is what Tasks 1 to 6 already cost, from the budget
	// table in docs/research/anakin.md §6. There is no usage endpoint on the
	// Anakin API, so this number is a hand-kept ledger and the only count that
	// exists. Update it in the same commit as that table.
	spentBeforeRecording = 35

	// abortAbove is the brief's cut-scope threshold: an estimate over this
	// means the plan is wrong, not that the ceiling should be raised.
	abortAbove = 200

	// recordingWindowDays spans the corpus. The detector needs a baseline to
	// compare today against, and a window under 14 days gives it one day of
	// history and a meaningless z-score.
	recordingWindowDays = 21
)

// step is one line of the recording plan: what it fetches, how many calls that
// takes, and what each call costs.
//
// CreditsPerCall is a static table from docs/research/anakin.md §6. That is
// fine for an estimate and wrong for accounting: every Wire action carries a
// live credits_per_call field, so the number printed at the end of a real run
// comes from Client.Stats, never from here.
type step struct {
	Source         models.Source
	What           string
	Calls          int
	CreditsPerCall int
}

func (s step) credits() int { return s.Calls * s.CreditsPerCall }

func main() {
	confirm := flag.Bool("confirm", false, "actually spend credits; without it this prints the estimate and exits")
	flag.Parse()

	if err := run(*confirm, "demo/brand.json"); err != nil {
		log.Fatalf("record: %v", err)
	}
}

// run takes the brand file as an argument so a test can drive the whole
// refusal path, which is the part that has to be right.
func run(confirm bool, brandPath string) error {
	brand, competitors, err := loadBrands(brandPath)
	if err != nil {
		return err
	}

	profiles := append([]models.BrandProfile{brand}, competitors...)
	plan := planFor(profiles)
	estimate := totalCredits(plan)
	printEstimate(plan, estimate)

	if estimate > abortAbove {
		return fmt.Errorf("the estimate of %d credits is over the %d ceiling; cut scope rather than raising it", estimate, abortAbove)
	}
	if remaining := freeTierCredits - spentBeforeRecording; estimate > remaining {
		return fmt.Errorf("the estimate of %d credits is %d over the %d left on the account", estimate, estimate-remaining, remaining)
	}
	if !confirm {
		fmt.Printf("\nDry run. Nothing was spent and no Anakin client was built.\nRe-run with -confirm and BP_FIXTURE_MODE=record to record for real.\n")
		return nil
	}

	mode := anakin.Mode(os.Getenv("BP_FIXTURE_MODE"))
	if mode != anakin.ModeRecord {
		return fmt.Errorf("-confirm needs BP_FIXTURE_MODE=record, got %q; refusing to spend credits without writing fixtures", mode)
	}
	return record(context.Background(), profiles, estimate)
}

// loadBrands reads demo/brand.json and derives one profile per competitor.
//
// A competitor profile carries the competitor's name as its only keyword and
// this brand's sources. Share of voice needs the competitor's mentions in the
// same corpus and the same window, not a second recording session.
func loadBrands(path string) (models.BrandProfile, []models.BrandProfile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.BrandProfile{}, nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var brand models.BrandProfile
	if err := json.Unmarshal(raw, &brand); err != nil {
		return models.BrandProfile{}, nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	if err := brand.Validate(); err != nil {
		return models.BrandProfile{}, nil, fmt.Errorf("%s: %w", path, err)
	}

	competitors := make([]models.BrandProfile, 0, len(brand.Competitors))
	for _, name := range brand.Competitors {
		rival := models.NewBrandProfile(name, name)
		rival.Keywords = []string{name}
		// The competitor is searched for on the sources that carry keyword
		// search. appstore and playstore are keyed by an app id we do not hold
		// for a rival, and their adapters reject a profile without one.
		rival.Sources = keywordSearchable(brand.Sources)
		rival.NegativeKeywords = brand.NegativeKeywords
		competitors = append(competitors, rival)
	}
	return brand, competitors, nil
}

// keywordSearchable drops the sources that need a per-brand handle.
func keywordSearchable(all []models.Source) []models.Source {
	out := []models.Source{}
	for _, source := range all {
		if source == models.SourceAppstore || source == models.SourcePlaystore {
			continue
		}
		out = append(out, source)
	}
	return out
}

// planFor builds the recording plan from the brands themselves, so the printed
// estimate moves when demo/brand.json does. It takes the brand and its
// competitors as one list, the same list record() spends against.
func planFor(profiles []models.BrandProfile) []step {
	// Per-source cost and call shape, from docs/research/anakin.md §6.
	// keywordsUsed caps how many of a profile's keywords become queries: the
	// demo brand carries 9 and searching all of them triples the bill for
	// mentions that overlap heavily.
	const keywordsUsed = 3

	var plan []step
	for _, profile := range profiles {
		queries := min(len(profile.Keywords), keywordsUsed)
		for _, source := range profile.Sources {
			switch source {
			case models.SourceReddit:
				plan = append(plan, step{source, profile.Name + " rt_search", queries, 2})
			case models.SourceYoutube:
				// One search for the videos, then comments on the top two.
				plan = append(plan, step{source, profile.Name + " yt_search", 1, 1})
				plan = append(plan, step{source, profile.Name + " yt_comments", 2, 3})
			case models.SourceNews, models.SourceWeb:
				plan = append(plan, step{source, profile.Name + " Search API", queries, 3})
			case models.SourceAppstore:
				plan = append(plan, step{source, profile.Name + " review feed", 1, 1})
			case models.SourcePlaystore:
				plan = append(plan, step{source, profile.Name + " listing", 1, 1})
			}
		}
	}
	return plan
}

func totalCredits(plan []step) int {
	total := 0
	for _, s := range plan {
		total += s.credits()
	}
	return total
}

func printEstimate(plan []step, estimate int) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\nRECORDING PLAN, %d-day window\n\n", recordingWindowDays)
	fmt.Fprintln(w, "SOURCE\tWHAT\tCALLS\tPER CALL\tCREDITS")
	for _, s := range plan {
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n", s.Source, s.What, s.Calls, s.CreditsPerCall, s.credits())
	}
	fmt.Fprintf(w, "\t\t\tESTIMATE\t%d\n", estimate)
	w.Flush()

	remaining := freeTierCredits - spentBeforeRecording
	fmt.Printf("\nAlready spent (docs/research/anakin.md §6): %d of %d\n", spentBeforeRecording, freeTierCredits)
	fmt.Printf("Remaining before this run:                  %d\n", remaining)
	fmt.Printf("This run would leave:                       %d\n", remaining-estimate)
}

// record runs every adapter once per brand and lets the client write fixtures.
//
// The ceiling passed to the client is the estimate, not the remaining balance:
// if the plan is wrong, the run must stop near the number a person approved
// rather than eat the rest of the account.
func record(ctx context.Context, profiles []models.BrandProfile, estimate int) error {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -recordingWindowDays)

	client, err := anakin.NewHTTPClient(anakin.Config{
		APIKey:     os.Getenv("ANAKIN_API_KEY"),
		Mode:       anakin.ModeRecord,
		MaxCredits: estimate,
		BrandID:    profiles[0].BrandID,
		Day:        end,
	})
	if err != nil {
		return fmt.Errorf("building the anakin client: %w", err)
	}

	recorded := 0
	for _, profile := range profiles {
		for _, source := range profile.Sources {
			fetch, ok := sources.Adapters[source]
			if !ok {
				log.Printf("no adapter for source %q, skipping", source)
				continue
			}
			mentions, err := fetch(ctx, client, profile, start, end)
			// A dead source does not stop the session. It is recorded as
			// absent, and the repo rule is that a source which failed its probe
			// does not appear in a fixture, a count or a sentence, so the count
			// is only logged when the fetch actually returned one.
			if err != nil {
				log.Printf("%s/%s: %v", profile.BrandID, source, err)
				continue
			}
			log.Printf("%s/%s: %d mentions", profile.BrandID, source, len(mentions))
			recorded += len(mentions)
		}
	}

	spent := client.Stats()
	fmt.Printf("\nRecorded %d mentions over %d days.\n", recorded, recordingWindowDays)
	fmt.Printf("Credits actually spent: %d (estimate was %d, cache hits %d).\n", spent.CreditsUsed, estimate, spent.CacheHits)
	fmt.Printf("Update the ledger in docs/research/anakin.md §6 to %d of %d.\n", spentBeforeRecording+spent.CreditsUsed, freeTierCredits)
	return nil
}
