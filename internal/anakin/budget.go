// budget.go is the credit ceiling. It lives in this package because
// ErrBudgetExceeded is an anakin sentinel and nothing outside this package
// spends a credit.
//
// The 300 free credits are the whole data layer's fuel and there is no second
// allocation, so this file is the thing standing between a runaway fan-out and
// a dead demo.
package anakin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Budget is a credit ceiling for one brand-day, backed by the summed
// runs.credits_used so it survives a process restart.
//
// The running total is cached in memory behind a mutex: the orchestrator calls
// collectors in parallel through errgroup, and an unguarded counter is how a
// budget gets overspent.
//
// The total is two numbers, not one. persisted is what Postgres says earlier
// runs cost and can be re-read; inProcess is what this run has spent and has
// not written to a run record yet. Collapsing them into one counter means a
// re-read silently forgets this run's own spending, which is the same bug as
// having no budget at all.
type Budget struct {
	pool    *pgxpool.Pool
	brandID string
	day     time.Time
	ceiling int

	mu        sync.Mutex
	persisted int
	inProcess int
	loaded    bool
}

// NewBudget returns a budget for one brand-day. A ceiling of 0 means the
// brand's own brands.daily_credit_budget applies.
//
// pool may be nil, which is replay mode: nothing is persisted and nothing is
// read back, so the ceiling must then be given explicitly. day is truncated to
// its UTC day.
func NewBudget(pool *pgxpool.Pool, brandID string, day time.Time, ceiling int) *Budget {
	return &Budget{
		pool:    pool,
		brandID: brandID,
		day:     day.UTC().Truncate(24 * time.Hour),
		ceiling: ceiling,
	}
}

// Spend records n credits, returning an error wrapping ErrBudgetExceeded when
// they would breach the ceiling. It re-reads the persisted total before
// refusing, so a stale cache cannot block a run that has headroom.
func (b *Budget) Spend(ctx context.Context, n int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.load(ctx); err != nil {
		return err
	}

	if b.persisted+b.inProcess+n > b.ceiling {
		// The persisted figure was read when this budget was created and
		// another process may have finished a run since. Refusing on a number
		// that old would kill a run that has headroom, so the truth is read
		// once more before saying no.
		if err := b.readPersisted(ctx); err != nil {
			return err
		}
		if b.persisted+b.inProcess+n > b.ceiling {
			return fmt.Errorf("anakin: spending %d would take brand %s to %d of %d credits today: %w",
				n, b.brandID, b.persisted+b.inProcess+n, b.ceiling, ErrBudgetExceeded)
		}
	}

	b.inProcess += n
	return nil
}

// Remaining reports how many credits are left in the ceiling.
func (b *Budget) Remaining(ctx context.Context) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.load(ctx); err != nil {
		return 0, err
	}

	left := b.ceiling - b.persisted - b.inProcess
	if left < 0 {
		return 0, nil
	}
	return left, nil
}

// load fills the ceiling and the persisted total on first use. Callers hold
// the mutex.
func (b *Budget) load(ctx context.Context) error {
	if b.loaded {
		return nil
	}
	if b.pool == nil {
		if b.ceiling <= 0 {
			return errors.New("anakin: a budget with no pool needs an explicit ceiling")
		}
		b.loaded = true
		return nil
	}

	if b.ceiling <= 0 {
		err := b.pool.QueryRow(ctx,
			`SELECT daily_credit_budget FROM brands WHERE id = $1`, b.brandID).Scan(&b.ceiling)
		if err != nil {
			return fmt.Errorf("anakin: read the credit ceiling for %s: %w", b.brandID, err)
		}
	}
	if err := b.readPersisted(ctx); err != nil {
		return err
	}

	b.loaded = true
	return nil
}

// readPersisted refreshes what Postgres says this brand-day has already cost.
// Callers hold the mutex.
func (b *Budget) readPersisted(ctx context.Context) error {
	if b.pool == nil {
		return nil
	}

	// runs.credits_used is the persisted total, which is why a restarted
	// process does not get a fresh allowance.
	err := b.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(credits_used), 0) FROM runs
		 WHERE brand_id = $1 AND started_at >= $2 AND started_at < $3`,
		b.brandID, b.day, b.day.Add(24*time.Hour)).Scan(&b.persisted)
	if err != nil {
		return fmt.Errorf("anakin: read today's credit spend for %s: %w", b.brandID, err)
	}
	return nil
}
