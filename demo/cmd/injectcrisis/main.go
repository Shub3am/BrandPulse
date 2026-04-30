// injectcrisis drops the synthetic negative surge into Postgres so the demo
// can show a crisis alert firing live.
//
// The corpus itself lives in demo/crisis, which is what bp-detector's test
// imports to prove the real rules fire on it. This command only moves that
// corpus into and out of the database.
//
// It must not touch any agent and it must not special-case the detector. It
// writes only mentions and their enrichment, only for one brand. The synthetic
// marker is the only thing it ever deletes, whether from -cleanup or from the
// clear an injection does before it writes, so a scraped mention is never at
// risk from a demo command.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/demo/crisis"
	"brandpulse/internal/db"
	"brandpulse/internal/models"
)

func main() {
	brandID := flag.String("brand", "brd_demo", "brand the surge is injected against")
	dryRun := flag.Bool("dry-run", false, "print what would be inserted and touch nothing")
	doCleanup := flag.Bool("cleanup", false, "delete the injected mentions by their synthetic marker")
	flag.Parse()

	if err := run(context.Background(), *brandID, *dryRun, *doCleanup, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "injectcrisis:", err)
		os.Exit(1)
	}
}

// run is separate from main so the -dry-run path is testable without a
// database, which is the path CI runs.
func run(ctx context.Context, brandID string, dryRun, doCleanup bool, out io.Writer) error {
	if dryRun {
		describe(out, brandID, crisis.Mentions(brandID, time.Now().UTC()))
		return nil
	}

	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	if doCleanup {
		removed, err := cleanup(ctx, pool, brandID)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "removed %d injected mentions from %s\n", removed, brandID)
		return nil
	}

	inserted, err := insert(ctx, pool, brandID, crisis.Mentions(brandID, time.Now().UTC()))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "injected %d synthetic mentions into %s across %d sources\n",
		inserted, brandID, len(crisis.Sources))
	return nil
}

// describe prints the surge without opening a connection. It prints the
// per-source counts and the window rather than forty lines of text, because
// what a reader needs to check is the shape the rules bucket on.
func describe(out io.Writer, brandID string, corpus []models.EnrichedMention) {
	perSource := map[models.Source]int{}
	for _, em := range corpus {
		perSource[em.Mention.Source]++
	}

	fmt.Fprintf(out, "dry run: %d synthetic mentions for %s, nothing written\n", len(corpus), brandID)
	fmt.Fprintf(out, "window: %s to %s\n",
		corpus[0].Mention.PostedAt.Format(time.RFC3339),
		corpus[len(corpus)-1].Mention.PostedAt.Format(time.RFC3339))
	for _, source := range crisis.Sources {
		fmt.Fprintf(out, "  %-10s %d mentions\n", source, perSource[source])
	}
	fmt.Fprintf(out, "influencers: %d authors over the detector's follower threshold\n", crisis.Influencers)
	fmt.Fprintf(out, "marker: every row carries raw.%s = true\n", crisis.SyntheticKey)
}

// insert replaces the brand's last injection with this one, in one
// transaction, so a half-injected crisis never reaches the stage.
//
// It clears first because the demo is rehearsed: the corpus is anchored to the
// current hour, so leaving yesterday's rows in place would leave the surge in
// an hour bucket the detector no longer looks at while the command reported a
// successful injection. The count returned is what Postgres actually wrote.
func insert(ctx context.Context, pool *pgxpool.Pool, brandID string, corpus []models.EnrichedMention) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning the injection: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := cleanup(ctx, tx, brandID); err != nil {
		return 0, err
	}

	batch := &pgx.Batch{}
	for _, em := range corpus {
		if err := em.Mention.Validate(); err != nil {
			return 0, fmt.Errorf("mention %q: %w", em.Mention.ID, err)
		}
		engagement, err := json.Marshal(em.Mention.Engagement)
		if err != nil {
			return 0, fmt.Errorf("mention %q engagement: %w", em.Mention.ID, err)
		}
		raw, err := json.Marshal(em.Mention.Raw)
		if err != nil {
			return 0, fmt.Errorf("mention %q raw: %w", em.Mention.ID, err)
		}
		aspects, err := json.Marshal(em.Enrichment.Aspects)
		if err != nil {
			return 0, fmt.Errorf("mention %q aspects: %w", em.Mention.ID, err)
		}

		// The clear above frees the synthetic ids, so the only row that can
		// still lose here is one whose text a scraped mention already carries.
		batch.Queue(`
			INSERT INTO mentions (
				id, brand_id, source, external_id, author, author_followers,
				text, lang, posted_at, engagement, matched_keyword, content_hash, raw
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (brand_id, content_hash) DO NOTHING`,
			em.Mention.ID, em.Mention.BrandID, em.Mention.Source, em.Mention.ExternalID,
			em.Mention.Author, em.Mention.AuthorFollowers, em.Mention.Text, em.Mention.Lang,
			em.Mention.PostedAt, engagement, em.Mention.MatchedKeyword, em.Mention.ContentHash, raw,
		)

		// WHERE EXISTS, not a bare insert: if the mention lost to the
		// content_hash constraint its id is not in mentions and the foreign key
		// would fail the whole transaction.
		batch.Queue(`
			INSERT INTO mention_enrichment (
				mention_id, sentiment, sentiment_label, emotion, intent,
				aspects, is_about_brand, model
			)
			SELECT $1,$2,$3,$4,$5,$6,$7,$8
			WHERE EXISTS (SELECT 1 FROM mentions WHERE id = $1)`,
			em.Enrichment.MentionID, em.Enrichment.Sentiment, em.Enrichment.SentimentLabel,
			em.Enrichment.Emotion, em.Enrichment.Intent, aspects,
			em.Enrichment.IsAboutBrand, em.Enrichment.Model,
		)
	}

	results := tx.SendBatch(ctx, batch)
	inserted := 0
	for _, em := range corpus {
		tag, err := results.Exec()
		if err != nil {
			results.Close()
			return 0, fmt.Errorf("inserting mention %q: %w", em.Mention.ID, err)
		}
		inserted += int(tag.RowsAffected())

		if _, err := results.Exec(); err != nil {
			results.Close()
			return 0, fmt.Errorf("inserting enrichment for %q: %w", em.Mention.ID, err)
		}
	}
	if err := results.Close(); err != nil {
		return 0, fmt.Errorf("closing the injection batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing the injection: %w", err)
	}
	return inserted, nil
}

// execer is the one method cleanup needs, so -cleanup can run it on the pool
// and insert can run the same statement inside its transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// cleanup deletes by the synthetic marker and nothing else, so a scraped
// mention can never be removed by a demo command. The enrichment goes with it
// through ON DELETE CASCADE.
func cleanup(ctx context.Context, db execer, brandID string) (int64, error) {
	tag, err := db.Exec(ctx, `
		DELETE FROM mentions
		WHERE brand_id = $1 AND raw ->> $2 = 'true'`,
		brandID, crisis.SyntheticKey)
	if err != nil {
		return 0, fmt.Errorf("removing the injected mentions: %w", err)
	}
	return tag.RowsAffected(), nil
}
