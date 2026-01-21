package anakin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestBudgetRefusesTheSpendThatBreachesTheCeiling is the case from the B1
// brief: ceiling 10, spend 6, spend 6. The second spend is refused whole. A
// budget that let the second call through partially would be a budget that
// does not hold, because a partial Anakin call still costs its credits.
func TestBudgetRefusesTheSpendThatBreachesTheCeiling(t *testing.T) {
	ctx := context.Background()
	b := NewBudget(nil, "brd_test", time.Now(), 10)

	if err := b.Spend(ctx, 6); err != nil {
		t.Fatalf("first spend of 6 against a ceiling of 10: %v", err)
	}

	err := b.Spend(ctx, 6)
	if err == nil {
		t.Fatal("second spend of 6 was allowed; the ceiling is 10")
	}
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("error does not wrap ErrBudgetExceeded: %v", err)
	}

	remaining, err := b.Remaining(ctx)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if remaining != 4 {
		t.Errorf("Remaining = %d, want 4: the refused spend must not be counted", remaining)
	}
}

func TestBudgetAllowsASpendThatExactlyFits(t *testing.T) {
	ctx := context.Background()
	b := NewBudget(nil, "brd_test", time.Now(), 10)

	if err := b.Spend(ctx, 10); err != nil {
		t.Fatalf("spending the whole ceiling: %v", err)
	}

	remaining, err := b.Remaining(ctx)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if remaining != 0 {
		t.Errorf("Remaining = %d, want 0", remaining)
	}

	if err := b.Spend(ctx, 1); !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("one more credit: got %v, want ErrBudgetExceeded", err)
	}
}

// TestBudgetHoldsUnderParallelSpends is the reason for the mutex. The
// orchestrator fans collectors out through errgroup, so Spend is called from
// several goroutines at once. Run with -race.
//
// Fifty goroutines each ask for 4 against a ceiling of 100, so exactly 25 must
// succeed. An unguarded counter lets more through and that is a real overspend
// of real credits.
func TestBudgetHoldsUnderParallelSpends(t *testing.T) {
	const (
		callers = 50
		each    = 4
		ceiling = 100
	)

	ctx := context.Background()
	b := NewBudget(nil, "brd_test", time.Now(), ceiling)

	var wg sync.WaitGroup
	granted := make(chan struct{}, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Spend(ctx, each); err == nil {
				granted <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(granted)

	if got, want := len(granted), ceiling/each; got != want {
		t.Errorf("%d spends of %d were granted against a ceiling of %d, want %d",
			got, each, ceiling, want)
	}

	remaining, err := b.Remaining(ctx)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if remaining != 0 {
		t.Errorf("Remaining = %d, want 0", remaining)
	}
}

// TestBudgetWithoutAPoolNeedsACeiling covers replay mode's shape. With no pool
// there is nowhere to read brands.daily_credit_budget from, so a zero ceiling
// is a configuration error rather than an unlimited budget.
func TestBudgetWithoutAPoolNeedsACeiling(t *testing.T) {
	b := NewBudget(nil, "brd_test", time.Now(), 0)

	if err := b.Spend(context.Background(), 1); err == nil {
		t.Fatal("a pool-less budget with a zero ceiling allowed a spend")
	}
}

func TestBudgetTruncatesTheDay(t *testing.T) {
	at := time.Date(2026, 9, 20, 17, 45, 3, 0, time.UTC)
	b := NewBudget(nil, "brd_test", at, 10)

	want := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	if !b.day.Equal(want) {
		t.Errorf("day = %s, want %s: the ceiling is per UTC day", b.day, want)
	}
}
