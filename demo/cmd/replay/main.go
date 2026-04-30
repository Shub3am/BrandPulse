// replay slides the seeded corpus forward so it ends "now".
//
// A fixture recorded last week looks dead on stage: the baseline window is
// empty and the live stream has nothing in it. Shifting every timestamp by one
// offset keeps the corpus's own shape, which is what makes the 14-day baseline
// and the last hour both look real.
//
// The shift is a pure function and this file is the only caller of it. It must
// stay pure: no clock, no randomness, no map iteration. A demo that differs
// between runs cannot be rehearsed, and this will be run thirty times before
// the pitch.
//
// It never moves an injected mention, and never measures the corpus end from
// one. Those are anchored to the current hour by demo/crisis.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/demo/crisis"
	"brandpulse/internal/db"
)

func main() {
	brandID := flag.String("brand", "brd_demo", "brand whose corpus is shifted")
	dryRun := flag.Bool("dry-run", false, "report the offset and change nothing")
	flag.Parse()

	if err := run(context.Background(), *brandID, *dryRun, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "replay:", err)
		os.Exit(1)
	}
}

// corpusRow is one mention's timestamp. Nothing else is read: replay has no
// business seeing the text.
type corpusRow struct {
	id       string
	postedAt time.Time
}

func run(ctx context.Context, brandID string, dryRun bool, out io.Writer) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	corpus, err := loadCorpus(ctx, pool, brandID)
	if err != nil {
		return err
	}
	if len(corpus) == 0 {
		fmt.Fprintf(out, "nothing to replay: %s has no seeded mentions\n", brandID)
		return nil
	}

	now := time.Now().UTC()
	corpusEnd := latest(corpus)
	offset := now.Sub(corpusEnd)

	fmt.Fprintf(out, "corpus of %d mentions ends %s, shifting it forward by %s\n",
		len(corpus), corpusEnd.Format(time.RFC3339), offset.Round(time.Second))
	if dryRun {
		fmt.Fprintln(out, "dry run: nothing written")
		return nil
	}

	if err := applyShift(ctx, pool, corpus, corpusEnd, now); err != nil {
		return err
	}
	fmt.Fprintf(out, "corpus now ends %s\n", now.Format(time.RFC3339))
	return nil
}

// shift is the whole of replay's logic. Every mention moves by the same offset,
// so the gaps between them, and therefore every per-hour count the detector
// baselines on, survive the move untouched.
//
// Because postedAt is never after corpusEnd, no shifted mention lands in the
// future.
func shift(postedAt, corpusEnd, now time.Time) time.Time {
	return postedAt.Add(now.Sub(corpusEnd)).UTC()
}

// latest is the corpus end. It scans rather than relying on the query's order,
// which is why that query has no ORDER BY to keep in step with it.
func latest(corpus []corpusRow) time.Time {
	end := corpus[0].postedAt
	for _, row := range corpus[1:] {
		if row.postedAt.After(end) {
			end = row.postedAt
		}
	}
	return end
}

func loadCorpus(ctx context.Context, pool *pgxpool.Pool, brandID string) ([]corpusRow, error) {
	// Injected mentions are excluded because they are anchored to the current
	// hour: counting one as the corpus end would make the offset zero and the
	// seeded corpus would silently never move.
	rows, err := pool.Query(ctx, `
		SELECT id, posted_at
		FROM mentions
		WHERE brand_id = $1 AND raw ->> $2 IS DISTINCT FROM 'true'`,
		brandID, crisis.SyntheticKey)
	if err != nil {
		return nil, fmt.Errorf("reading the corpus: %w", err)
	}
	defer rows.Close()

	var corpus []corpusRow
	for rows.Next() {
		var row corpusRow
		if err := rows.Scan(&row.id, &row.postedAt); err != nil {
			return nil, fmt.Errorf("scanning the corpus: %w", err)
		}
		row.postedAt = row.postedAt.UTC()
		corpus = append(corpus, row)
	}
	return corpus, rows.Err()
}

// applyShift writes every new timestamp in one transaction. A half-shifted
// corpus has two baselines in it and would make the detector fire on the seam.
func applyShift(ctx context.Context, pool *pgxpool.Pool, corpus []corpusRow, corpusEnd, now time.Time) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning the shift: %w", err)
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, row := range corpus {
		batch.Queue(`UPDATE mentions SET posted_at = $2 WHERE id = $1`,
			row.id, shift(row.postedAt, corpusEnd, now))
	}

	results := tx.SendBatch(ctx, batch)
	for range corpus {
		if _, err := results.Exec(); err != nil {
			results.Close()
			return fmt.Errorf("shifting the corpus: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("closing the shift batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing the shift: %w", err)
	}
	return nil
}
