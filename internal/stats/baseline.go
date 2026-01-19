// baseline.go builds the 14-day history bp-detector compares the current hour
// against.
//
// The one rule it may never break: the window ends at the start of the hour
// containing now. A baseline that includes the current hour raises the mean by
// exactly the spike being measured, and the spike suppresses itself.
package stats

import (
	"context"
	"fmt"
	"time"

	"brandpulse/internal/db"
	"brandpulse/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ComputeBaseline reads the last days days of mentions for brandID and returns
// the per-source hourly distribution the detector compares against.
//
// It excludes the hour containing now, or a spike would raise the mean it is
// being measured against and suppress itself.
//
// days has no default. The detector passes 14 explicitly; 0 means zero days.
func ComputeBaseline(ctx context.Context, brandID string, days int, now time.Time) (models.BaselineStats, error) {
	baseline := models.BaselineStats{
		PerSourceHourlyMean: map[models.Source]float64{},
		PerSourceHourlyStd:  map[models.Source]float64{},
		Days:                days,
	}
	if days <= 0 {
		return baseline, nil
	}

	windowEnd := now.UTC().Truncate(time.Hour)
	windowStart := windowEnd.Add(-time.Duration(days) * 24 * time.Hour)
	hoursInWindow := days * 24

	pool, err := db.Pool(ctx)
	if err != nil {
		return models.BaselineStats{}, err
	}

	counts, err := hourlyCountsPerSource(ctx, pool, brandID, windowStart, windowEnd)
	if err != nil {
		return models.BaselineStats{}, err
	}
	for source, busyHours := range counts {
		mean, std := meanAndStd(padWithQuietHours(busyHours, hoursInWindow))
		baseline.PerSourceHourlyMean[source] = mean
		baseline.PerSourceHourlyStd[source] = std
	}

	shares, err := hourlyNegativeShares(ctx, pool, brandID, windowStart, windowEnd)
	if err != nil {
		return models.BaselineStats{}, err
	}
	baseline.NegativeShareMean, baseline.NegativeShareStd = meanAndStd(shares)

	baseline.MeanRating, err = meanRating(ctx, pool, brandID, windowStart, windowEnd)
	if err != nil {
		return models.BaselineStats{}, err
	}

	return baseline, nil
}

// padWithQuietHours turns the hours that had mentions into one value per hour
// in the window, with a zero for every hour that had none.
//
// Averaging only the hours a source posted in would make a source that posts
// once a fortnight look like a source that posts every hour, and nothing would
// ever spike. The quiet hours are the denominator, which is also why the hour
// each count belongs to does not matter here: a mean and a population std do
// not care about order, only about how many zeros there are.
func padWithQuietHours(busyHours []float64, hoursInWindow int) []float64 {
	if len(busyHours) >= hoursInWindow {
		return busyHours
	}
	padded := make([]float64, hoursInWindow)
	copy(padded, busyHours)
	return padded
}

func hourlyCountsPerSource(ctx context.Context, pool *pgxpool.Pool, brandID string, windowStart, windowEnd time.Time) (map[models.Source][]float64, error) {
	rows, err := pool.Query(ctx,
		`SELECT source::text, count(*)
		 FROM mentions
		 WHERE brand_id = $1 AND posted_at >= $2 AND posted_at < $3
		 GROUP BY source, date_trunc('hour', posted_at)`,
		brandID, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("stats: count mentions per source per hour for %s: %w", brandID, err)
	}
	defer rows.Close()

	counts := map[models.Source][]float64{}
	for rows.Next() {
		var source string
		var count float64
		if err := rows.Scan(&source, &count); err != nil {
			return nil, fmt.Errorf("stats: read an hourly count for %s: %w", brandID, err)
		}
		counts[models.Source(source)] = append(counts[models.Source(source)], count)
	}
	return counts, rows.Err()
}

// hourlyNegativeShares returns one share per hour that had at least one
// enriched mention.
//
// An hour with no mentions has no negative share, and counting it as 0.0 would
// drag the mean down until any negativity at all looked like a crisis. An hour
// whose mentions nobody enriched yet is equally not a measurement.
func hourlyNegativeShares(ctx context.Context, pool *pgxpool.Pool, brandID string, windowStart, windowEnd time.Time) ([]float64, error) {
	rows, err := pool.Query(ctx,
		`SELECT count(*) FILTER (WHERE e.sentiment_label = 'negative')::float8 / count(*)
		 FROM mentions m
		 JOIN mention_enrichment e ON e.mention_id = m.id
		 WHERE m.brand_id = $1 AND m.posted_at >= $2 AND m.posted_at < $3
		 GROUP BY date_trunc('hour', m.posted_at)`,
		brandID, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("stats: read negative share per hour for %s: %w", brandID, err)
	}
	defer rows.Close()

	var shares []float64
	for rows.Next() {
		var share float64
		if err := rows.Scan(&share); err != nil {
			return nil, fmt.Errorf("stats: read an hourly negative share for %s: %w", brandID, err)
		}
		shares = append(shares, share)
	}
	return shares, rows.Err()
}

// meanRating returns nil when nothing in the window carried a rating, which is
// a different fact from an average of 0.0 and the one the brief reports as
// context.
//
// "rating IS NOT NULL" is the whole review-source filter: Source.IsReviewSource
// names the three sources that set a rating, and no other adapter sets one.
// Listing those three again here would be a second copy to forget.
func meanRating(ctx context.Context, pool *pgxpool.Pool, brandID string, windowStart, windowEnd time.Time) (*float64, error) {
	var mean *float64
	err := pool.QueryRow(ctx,
		`SELECT avg(rating)::float8
		 FROM mentions
		 WHERE brand_id = $1 AND posted_at >= $2 AND posted_at < $3
		   AND rating IS NOT NULL`,
		brandID, windowStart, windowEnd).Scan(&mean)
	if err != nil {
		return nil, fmt.Errorf("stats: read the mean rating for %s: %w", brandID, err)
	}
	return mean, nil
}
