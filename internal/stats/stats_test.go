// stats_test.go splits along the line the package does: ZScore and the two
// buckets are pure and table-driven, and ComputeBaseline needs Postgres and
// skips without DATABASE_URL, the same way internal/db does.
//
// The database tests write into a brand of their own, "brd_stats_test_<pid>",
// and delete it afterwards. The compose volume is shared by all seven
// worktrees, so nothing here may touch a row it did not create.
package stats

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"brandpulse/internal/db"
	"brandpulse/internal/models"
)

func TestZScore(t *testing.T) {
	cases := []struct {
		name             string
		value, mean, std float64
		want             float64
	}{
		{"three standard deviations above", 50, 20, 10, 3},
		{"below the mean", 5, 20, 10, -1.5},
		{"on the mean", 20, 20, 10, 0},
		// The guard that keeps a quiet brand out of the alert list. A source
		// that posted the same count every hour for fourteen days has std 0,
		// and dividing by it would be +Inf on every rule at once.
		{"a quiet brand has no variance", 50, 20, 0, 0},
		{"a quiet brand that did not move", 20, 20, 0, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ZScore(c.value, c.mean, c.std)
			if got != c.want {
				t.Errorf("ZScore(%v, %v, %v) = %v, want %v", c.value, c.mean, c.std, got, c.want)
			}
			if math.IsInf(got, 0) || math.IsNaN(got) {
				t.Errorf("ZScore(%v, %v, %v) = %v, which no threshold comparison survives", c.value, c.mean, c.std, got)
			}
		})
	}
}

func TestBuckets(t *testing.T) {
	// Deliberately not UTC: every consumer formats a local timestamp at some
	// point, and two worktrees in two timezones must agree on the key.
	istMidnight := time.Date(2026, 9, 20, 0, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))

	cases := []struct {
		name string
		at   time.Time
		hour string
		day  string
	}{
		{
			name: "a UTC afternoon",
			at:   time.Date(2026, 9, 20, 14, 59, 59, 0, time.UTC),
			hour: "2026-09-20T14",
			day:  "2026-09-20",
		},
		{
			name: "the first second of a UTC hour",
			at:   time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC),
			hour: "2026-09-20T15",
			day:  "2026-09-20",
		},
		{
			// 00:30 IST is 19:00 the previous day in UTC, so the day bucket
			// steps back. A dedupe key that used local time would let the same
			// alert fire twice across this boundary.
			name: "half past midnight in India is the previous UTC day",
			at:   istMidnight,
			hour: "2026-09-19T19",
			day:  "2026-09-19",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HourBucket(c.at); got != c.hour {
				t.Errorf("HourBucket = %q, want %q", got, c.hour)
			}
			if got := DayBucket(c.at); got != c.day {
				t.Errorf("DayBucket = %q, want %q", got, c.day)
			}
		})
	}
}

func TestMeanAndStd(t *testing.T) {
	cases := []struct {
		name      string
		values    []float64
		mean, std float64
	}{
		{"an empty window", nil, 0, 0},
		{"one hour", []float64{7}, 7, 0},
		{"no variance", []float64{4, 4, 4, 4}, 4, 0},
		// Population std of 2,4,4,4,5,5,7,9 is exactly 2.
		{"the textbook set", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 5, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mean, std := meanAndStd(c.values)
			if !closeEnough(mean, c.mean) || !closeEnough(std, c.std) {
				t.Errorf("meanAndStd = %v, %v, want %v, %v", mean, std, c.mean, c.std)
			}
		})
	}
}

func TestQuietHoursAreTheDenominator(t *testing.T) {
	// One source posted 24 mentions in a single hour of a 14-day window and
	// nothing in the other 335. Averaging only the hour it posted in would
	// report a mean of 24, and the next 24-mention hour would score z = 0.
	mean, std := meanAndStd(padWithQuietHours([]float64{24}, 14*24))

	if mean >= 1 {
		t.Errorf("mean = %v over 336 hours with 24 mentions in one of them; the quiet hours were not counted", mean)
	}
	if std == 0 {
		t.Error("std = 0 for a source that posted in exactly one hour; ZScore would then return 0 for every spike")
	}
	if z := ZScore(24, mean, std); z < 3 {
		t.Errorf("a repeat of the busiest hour scores z = %v, which would never fire the spike rule", z)
	}
}

// ---------------------------------------------------------------------------
// ComputeBaseline. Needs the compose Postgres.
// ---------------------------------------------------------------------------

func TestComputeBaselineNeedsNoDatabaseForZeroDays(t *testing.T) {
	// days has no default, and 0 means zero days. It must not reach Postgres
	// and must not quietly become 14.
	baseline, err := ComputeBaseline(context.Background(), "brd_never_inserted", 0, time.Now())
	if err != nil {
		t.Fatalf("ComputeBaseline with 0 days: %v", err)
	}
	if baseline.Days != 0 {
		t.Errorf("Days = %d, want 0", baseline.Days)
	}
	if len(baseline.PerSourceHourlyMean) != 0 {
		t.Errorf("PerSourceHourlyMean = %v, want empty", baseline.PerSourceHourlyMean)
	}
	if baseline.MeanRating != nil {
		t.Errorf("MeanRating = %v, want nil", *baseline.MeanRating)
	}
}

func TestComputeBaselineExcludesTheCurrentHour(t *testing.T) {
	ctx, brandID := seedBrand(t)

	// now is 14:30. The spike lands at 14:05, inside the current hour, and one
	// mention lands at 12:05, inside the window. A baseline that counted the
	// current hour would report a mean of 51/336 instead of 1/336, and the
	// spike would be measuring itself.
	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	insertMentions(t, ctx, brandID, models.SourceX, now.Add(-2*time.Hour), 1)
	insertMentions(t, ctx, brandID, models.SourceX, now.Add(-25*time.Minute), 50)

	baseline, err := ComputeBaseline(ctx, brandID, 14, now)
	if err != nil {
		t.Fatalf("ComputeBaseline: %v", err)
	}

	mean := baseline.PerSourceHourlyMean[models.SourceX]
	if want := 1.0 / float64(14*24); !closeEnough(mean, want) {
		t.Fatalf("PerSourceHourlyMean[x] = %v, want %v; the current hour's 50 mentions were counted", mean, want)
	}
	if z := ZScore(50, mean, baseline.PerSourceHourlyStd[models.SourceX]); z < 3 {
		t.Errorf("the spike scores z = %v against its own baseline, so it suppressed itself", z)
	}
}

func TestComputeBaselineSplitsPerSource(t *testing.T) {
	ctx, brandID := seedBrand(t)

	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	insertMentions(t, ctx, brandID, models.SourceX, now.Add(-2*time.Hour), 10)
	insertMentions(t, ctx, brandID, models.SourceReddit, now.Add(-3*time.Hour), 2)

	baseline, err := ComputeBaseline(ctx, brandID, 14, now)
	if err != nil {
		t.Fatalf("ComputeBaseline: %v", err)
	}

	if len(baseline.PerSourceHourlyMean) != 2 {
		t.Fatalf("PerSourceHourlyMean = %v, want exactly x and reddit", baseline.PerSourceHourlyMean)
	}
	if x, reddit := baseline.PerSourceHourlyMean[models.SourceX], baseline.PerSourceHourlyMean[models.SourceReddit]; x <= reddit {
		t.Errorf("x mean %v is not above reddit mean %v, so the sources were pooled", x, reddit)
	}
	if baseline.Days != 14 {
		t.Errorf("Days = %d, want the 14 that was passed", baseline.Days)
	}
}

// TestMeanRatingIsNilWhenNoReviewSourceRan and the 0.0 case are separate tests
// on purpose. They are the two facts the pointer exists to tell apart, and a
// single table-driven test would let one of them be quietly deleted.

func TestMeanRatingIsNilWhenNoReviewSourceRan(t *testing.T) {
	ctx, brandID := seedBrand(t)

	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	insertMentions(t, ctx, brandID, models.SourceX, now.Add(-2*time.Hour), 5)

	baseline, err := ComputeBaseline(ctx, brandID, 14, now)
	if err != nil {
		t.Fatalf("ComputeBaseline: %v", err)
	}
	if baseline.MeanRating != nil {
		t.Fatalf("MeanRating = %v, want nil; x carries no rating", *baseline.MeanRating)
	}
}

func TestMeanRatingIsZeroWhenEveryReviewRatedZero(t *testing.T) {
	ctx, brandID := seedBrand(t)

	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	insertRatedMentions(t, ctx, brandID, models.SourceAmazon, now.Add(-2*time.Hour), 4, 0)

	baseline, err := ComputeBaseline(ctx, brandID, 14, now)
	if err != nil {
		t.Fatalf("ComputeBaseline: %v", err)
	}
	if baseline.MeanRating == nil {
		t.Fatal("MeanRating = nil for four one-star-floor reviews; nil means no review source ran, not an average of zero")
	}
	if *baseline.MeanRating != 0 {
		t.Errorf("MeanRating = %v, want 0", *baseline.MeanRating)
	}
}

func TestNegativeShareSkipsHoursWithNothingInThem(t *testing.T) {
	ctx, brandID := seedBrand(t)

	// Two enriched hours: one all-negative, one all-positive. 333 empty hours
	// counted as 0.0 would put the mean at 0.003 and make any negativity at
	// all a crisis; skipping them puts it at 0.5.
	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)
	insertEnrichedMentions(t, ctx, brandID, now.Add(-2*time.Hour), 4, models.SentimentNegative)
	insertEnrichedMentions(t, ctx, brandID, now.Add(-5*time.Hour), 4, models.SentimentPositive)

	baseline, err := ComputeBaseline(ctx, brandID, 14, now)
	if err != nil {
		t.Fatalf("ComputeBaseline: %v", err)
	}
	if !closeEnough(baseline.NegativeShareMean, 0.5) {
		t.Errorf("NegativeShareMean = %v, want 0.5; empty hours were averaged in", baseline.NegativeShareMean)
	}
	if !closeEnough(baseline.NegativeShareStd, 0.5) {
		t.Errorf("NegativeShareStd = %v, want 0.5 across one 1.0 hour and one 0.0 hour", baseline.NegativeShareStd)
	}
}

// ---------------------------------------------------------------------------
// Fixture helpers.
// ---------------------------------------------------------------------------

func requireDatabase(t *testing.T) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is unset; start the compose Postgres and export it")
	}
}

// seedBrand creates a brand nothing else in the repo uses and deletes it, and
// everything cascading off it, when the test ends.
func seedBrand(t *testing.T) (context.Context, string) {
	t.Helper()
	requireDatabase(t)

	ctx := context.Background()
	pool, err := db.Pool(ctx)
	if err != nil {
		t.Fatalf("open the pool: %v", err)
	}

	brandID := fmt.Sprintf("brd_stats_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO brands (id, name) VALUES ($1, $2)`, brandID, "stats test"); err != nil {
		t.Fatalf("insert the test brand: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM brands WHERE id = $1`, brandID); err != nil {
			t.Errorf("delete the test brand %s: %v", brandID, err)
		}
	})

	return ctx, brandID
}

func insertMentions(t *testing.T, ctx context.Context, brandID string, source models.Source, at time.Time, count int) {
	t.Helper()
	insertRatedMentions(t, ctx, brandID, source, at, count, -1)
}

// insertRatedMentions writes count mentions into the hour containing at. A
// rating below zero means NULL, which is what every non-review source stores.
func insertRatedMentions(t *testing.T, ctx context.Context, brandID string, source models.Source, at time.Time, count int, rating float64) {
	t.Helper()

	pool, err := db.Pool(ctx)
	if err != nil {
		t.Fatalf("open the pool: %v", err)
	}

	var stored *float64
	if rating >= 0 {
		stored = &rating
	}
	for i := range count {
		id := fmt.Sprintf("%s_%s_%d_%d", brandID, source, at.Unix(), i)
		_, err := pool.Exec(ctx,
			`INSERT INTO mentions (id, brand_id, source, external_id, text, posted_at, rating, content_hash)
			 VALUES ($1, $2, $3::source, $4, $5, $6, $7, $8)`,
			id, brandID, string(source), id, "stats test mention", at, stored, id)
		if err != nil {
			t.Fatalf("insert mention %s: %v", id, err)
		}
	}
}

func insertEnrichedMentions(t *testing.T, ctx context.Context, brandID string, at time.Time, count int, label models.SentimentLabel) {
	t.Helper()

	insertMentions(t, ctx, brandID, models.SourceX, at, count)

	pool, err := db.Pool(ctx)
	if err != nil {
		t.Fatalf("open the pool: %v", err)
	}
	for i := range count {
		id := fmt.Sprintf("%s_%s_%d_%d", brandID, models.SourceX, at.Unix(), i)
		_, err := pool.Exec(ctx,
			`INSERT INTO mention_enrichment (mention_id, sentiment, sentiment_label)
			 VALUES ($1, $2, $3::sentiment_label)`,
			id, 0.0, string(label))
		if err != nil {
			t.Fatalf("enrich mention %s: %v", id, err)
		}
	}
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}
