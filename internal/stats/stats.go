// Package stats holds the arithmetic the detector, the cache keys and the run
// idempotency keys all share.
//
// This package belongs to B1 (their Task 8). The three pure functions are
// implemented here because track B4's detector tests are arithmetic over them
// and cannot run against a panic. ComputeBaseline needs the database and stays
// a panicking stub: B1 owns that query, including the rule that a baseline
// excludes the current hour or a spike suppresses itself.
package stats

import (
	"context"
	"math"
	"time"

	"brandpulse/internal/models"
)

// ZScore reports how many standard deviations value sits from mean.
//
// It returns 0.0 when std is zero rather than dividing by it. Go does not raise
// on a float division by zero: the result is +Inf or NaN, +Inf compares
// >= 3.0 as true and NaN compares as false, so a missing guard here is a silent
// wrong alert on a quiet brand rather than a crash.
func ZScore(value, mean, std float64) float64 {
	if std == 0 || math.IsNaN(std) {
		return 0.0
	}
	return (value - mean) / std
}

// HourBucket is the UTC hour an instant falls in, as "2006-01-02T15".
//
// One implementation, three consumers: Anakin cache keys, run idempotency and
// alert dedupe. They must agree or a sustained crisis alerts once per run
// instead of once per hour.
func HourBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02T15")
}

// DayBucket is the UTC day an instant falls in, as "2006-01-02".
func DayBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// ComputeBaseline reads the brand's last days days out of Postgres and returns
// the behaviour the current window is compared against.
//
// days has no default: the detector passes 14 explicitly and 0 means zero days,
// not fourteen.
func ComputeBaseline(ctx context.Context, brandID string, days int, now time.Time) (models.BaselineStats, error) {
	panic("not implemented: B1 Task 8 owns stats.ComputeBaseline")
}
