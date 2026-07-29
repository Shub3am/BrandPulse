// Live-Postgres tests for store.go. They are skipped unless BP_LIVE_DB names a
// database, so the replay suite and CI stay offline and zero-credit.
//
// These cover the two failures a fake store cannot reproduce: a NULL in a
// nullable column, and a parameter Postgres cannot type. Neither reaches the
// pipeline tests, because fakeStore never speaks SQL.
//
// Each test owns its fixture brand and deletes it, so it does not depend on
// demo data and does not leave any behind.

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/internal/models"
)

const liveFixtureBrandID = "brd_storetest"

func livePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BP_LIVE_DB")
	if dsn == "" {
		t.Skip("BP_LIVE_DB is unset; this test needs a real Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// liveFixture inserts one brand and returns it clean. Deleting the brand
// cascades to every row these tests write.
func liveFixture(t *testing.T, pool *pgxpool.Pool) context.Context {
	t.Helper()
	ctx := context.Background()

	drop := func() {
		if _, err := pool.Exec(ctx, `DELETE FROM brands WHERE id = $1`, liveFixtureBrandID); err != nil {
			t.Fatalf("cleaning fixture brand: %v", err)
		}
	}
	drop()
	t.Cleanup(drop)

	if _, err := pool.Exec(ctx,
		`INSERT INTO brands (id, name) VALUES ($1, 'store_test')`, liveFixtureBrandID); err != nil {
		t.Fatalf("inserting fixture brand: %v", err)
	}
	return ctx
}

// TestEnrichedInWindowReadsNullColumns is the regression for the read that
// dropped every collected mention whose url was NULL.
func TestEnrichedInWindowReadsNullColumns(t *testing.T) {
	pool := livePool(t)
	ctx := liveFixture(t, pool)
	postedAt := time.Now().UTC().Add(-time.Hour)

	if _, err := pool.Exec(ctx, `
		INSERT INTO mentions (id, brand_id, source, external_id, text, posted_at, content_hash,
		                      url, author, rating, matched_keyword)
		VALUES ('mnt_storetest', $1, 'x', 'ext-1', 'a post with no url', $2, 'hash-storetest',
		        NULL, NULL, NULL, NULL)`, liveFixtureBrandID, postedAt); err != nil {
		t.Fatalf("inserting fixture mention: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mention_enrichment (mention_id, sentiment, sentiment_label, about_competitor)
		VALUES ('mnt_storetest', -0.4, 'negative', NULL)`); err != nil {
		t.Fatalf("inserting fixture enrichment: %v", err)
	}

	store := NewPostgresStore(pool)
	enriched, err := store.EnrichedInWindow(ctx, liveFixtureBrandID,
		postedAt.Add(-time.Hour), postedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("EnrichedInWindow: %v", err)
	}
	if len(enriched) != 1 {
		t.Fatalf("read back %d mentions, want 1", len(enriched))
	}

	item := enriched[0]
	for _, field := range []struct {
		name string
		got  string
	}{
		{"url", item.Mention.URL},
		{"author", item.Mention.Author},
		{"matched_keyword", item.Mention.MatchedKeyword},
		{"about_competitor", item.Enrichment.AboutCompetitor},
	} {
		if field.got != "" {
			t.Errorf("%s read back as %q, want the empty string", field.name, field.got)
		}
	}
	// Rating stays a pointer: no rating must not read as a zero-star review.
	if item.Mention.Rating != nil {
		t.Errorf("rating read back as %v, want nil", *item.Mention.Rating)
	}
}

// TestSaveDraftsWritesBothShapes is the regression for the batch Postgres
// refused to prepare because it could not type the alert_id parameter.
func TestSaveDraftsWritesBothShapes(t *testing.T) {
	pool := livePool(t)
	ctx := liveFixture(t, pool)

	if _, err := pool.Exec(ctx, `
		INSERT INTO mentions (id, brand_id, source, external_id, text, posted_at, content_hash)
		VALUES ('mnt_storetest', $1, 'x', 'ext-1', 'a post', now(), 'hash-storetest')`,
		liveFixtureBrandID); err != nil {
		t.Fatalf("inserting fixture mention: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO alerts (id, brand_id, kind, severity, title, why, dedupe_key)
		VALUES ('alr_storetest', $1, 'spike', 'high', 'a title', 'a reason', 'key-storetest')`,
		liveFixtureBrandID); err != nil {
		t.Fatalf("inserting fixture alert: %v", err)
	}

	onAlert := models.NewReplyDraft("rpl_storetest_alert", models.ChannelStatement)
	onAlert.AlertID = "alr_storetest"
	onAlert.Text = "we are looking into it"

	onMention := models.NewReplyDraft("rpl_storetest_mention", models.ChannelEmail)
	onMention.MentionID = "mnt_storetest"
	onMention.Text = "thanks for flagging this"

	orphan := models.NewReplyDraft("rpl_storetest_orphan", models.ChannelEmail)
	orphan.AlertID = "alr_does_not_exist"
	orphan.Text = "this alert lost its insert to the dedupe key"

	store := NewPostgresStore(pool)
	if err := store.SaveDrafts(ctx, []models.ReplyDraft{onAlert, onMention, orphan}); err != nil {
		t.Fatalf("SaveDrafts: %v", err)
	}

	var saved int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM reply_drafts WHERE id LIKE 'rpl_storetest%'`).Scan(&saved); err != nil {
		t.Fatalf("counting drafts: %v", err)
	}
	if saved != 2 {
		t.Fatalf("stored %d drafts, want 2: the alert-backed one and the mention-backed one", saved)
	}
}

// TestSaveTopicsPreparesAgainstRealPostgres covers the other batch writer built
// on INSERT ... SELECT with a guard, which no run has reached yet: topics is
// empty in the demo database, so nothing has ever proved Postgres can type its
// parameters.
func TestSaveTopicsPreparesAgainstRealPostgres(t *testing.T) {
	pool := livePool(t)
	ctx := liveFixture(t, pool)

	if _, err := pool.Exec(ctx, `
		INSERT INTO mentions (id, brand_id, source, external_id, text, posted_at, content_hash)
		VALUES ('mnt_storetest', $1, 'x', 'ext-1', 'a post', now(), 'hash-storetest')`,
		liveFixtureBrandID); err != nil {
		t.Fatalf("inserting fixture mention: %v", err)
	}

	topic := models.NewTopic("tpc_storetest", liveFixtureBrandID)
	topic.WindowStart = time.Now().UTC().Add(-time.Hour)
	topic.WindowEnd = time.Now().UTC()
	topic.Label = "delivery delays"
	topic.Size = 1
	topic.MentionIDs = []string{"mnt_storetest", "mnt_does_not_exist"}

	if err := NewPostgresStore(pool).SaveTopics(ctx, []models.Topic{topic}); err != nil {
		t.Fatalf("SaveTopics: %v", err)
	}

	var links int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM topic_mentions WHERE topic_id = 'tpc_storetest'`).Scan(&links); err != nil {
		t.Fatalf("counting topic_mentions: %v", err)
	}
	if links != 1 {
		t.Fatalf("linked %d mentions, want 1: the mention that exists", links)
	}
}

// TestEnrichedInWindowOverTheDemoBrand reads whatever the demo brand holds. It
// proves the read survives the real mix of NULLs rather than a crafted one, and
// it asserts nothing about the count because the demo data moves.
func TestEnrichedInWindowOverTheDemoBrand(t *testing.T) {
	pool := livePool(t)
	ctx := context.Background()

	var mentions, nullURLs int
	err := pool.QueryRow(ctx,
		`SELECT count(*), count(*) - count(url) FROM mentions WHERE brand_id = 'brd_demo'`).
		Scan(&mentions, &nullURLs)
	if err != nil {
		t.Fatalf("counting demo mentions: %v", err)
	}
	if mentions == 0 {
		t.Skip("no brd_demo mentions in this database")
	}

	end := time.Now().UTC().Add(24 * time.Hour)
	enriched, err := NewPostgresStore(pool).EnrichedInWindow(ctx, "brd_demo", end.AddDate(0, 0, -31), end)
	if err != nil {
		t.Fatalf("EnrichedInWindow: %v", err)
	}
	t.Logf("brd_demo holds %d mentions, %d with a NULL url; EnrichedInWindow read back %d",
		mentions, nullURLs, len(enriched))
	if len(enriched) == 0 {
		t.Fatal("read back nothing from a database that holds mentions")
	}
}
