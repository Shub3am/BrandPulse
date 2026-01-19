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
// It must not decide anything. No threshold, no severity and no rule lives in
// this package; those are bp-detector's, and a threshold hidden behind a mean
// is a threshold nobody can put in an alert's evidence. ComputeBaseline is in
// baseline.go.
package stats

import (
	"math"
	"time"
)

// ZScore returns how many standard deviations value sits from mean.
//
// It returns 0.0 when std is 0 rather than dividing by zero. A brand with no
// variance is quiet, not in crisis, and this guard is the difference between a
// working detector and a demo that panics on a quiet brand.
func ZScore(value, mean, std float64) float64 {
	if std == 0 {
		return 0
	}
	return (value - mean) / std
}

// HourBucket formats t as the UTC hour it falls in. Used by fetch_cache keys,
// run idempotency and alert dedupe keys.
func HourBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02T15")
}

// DayBucket formats t as the UTC day it falls in.
func DayBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// meanAndStd returns the population mean and standard deviation of values.
//
// Population, not sample: the window is every hour in the baseline, so there
// is nothing outside it to infer about. Both are 0 for an empty slice, which
// is the quiet-brand case ZScore already guards.
func meanAndStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	var total float64
	for _, value := range values {
		total += value
	}
	mean := total / float64(len(values))

	var squaredError float64
	for _, value := range values {
		squaredError += (value - mean) * (value - mean)
	}
	return mean, math.Sqrt(squaredError / float64(len(values)))
}
