// Package stats is the arithmetic behind alerting and idempotency.
//
// bp-detector is deterministic by design: it calls nothing in this repository
// that talks to a model, and every alert it raises carries the value and the
// threshold that fired it. Those numbers come from here.
//
// The time buckets live here too, rather than in three places, because cache
// keys, run idempotency and alert dedupe must agree on what "this hour" means
// or a re-run silently duplicates work.
//
// STUB: signatures only, bodies panic. B1 Task 8 implements this.
package stats

import (
	"context"
	"time"

	"brandpulse/internal/models"
)

// ZScore returns how many standard deviations value sits from mean.
//
// It returns 0.0 when std is 0 rather than dividing by zero. A brand with no
// variance is quiet, not in crisis, and this guard is the difference between a
// working detector and a demo that panics on a quiet brand.
func ZScore(value, mean, std float64) float64 {
	panic("not implemented")
}

// ComputeBaseline reads the last days days of mentions for brandID and returns
// the per-source hourly distribution the detector compares against.
//
// It excludes the hour containing now, or a spike would raise the mean it is
// being measured against and suppress itself.
//
// days has no default. The detector passes 14 explicitly; 0 means zero days.
func ComputeBaseline(ctx context.Context, brandID string, days int, now time.Time) (models.BaselineStats, error) {
	panic("not implemented")
}

// HourBucket formats t as the UTC hour it falls in. Used by fetch_cache keys,
// run idempotency and alert dedupe keys.
func HourBucket(t time.Time) string {
	panic("not implemented")
}

// DayBucket formats t as the UTC day it falls in.
func DayBucket(t time.Time) string {
	panic("not implemented")
}
