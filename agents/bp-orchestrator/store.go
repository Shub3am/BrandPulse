// The Postgres implementation of Store.
//
// Every statement in this file exists because pipeline.go needs it, and the
// column lists are written out rather than built, so a schema change breaks the
// build here instead of failing at runtime in front of a judge.
//
// This file holds no pipeline logic. It does not decide what a run means, only
// how a run is written down.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/internal/ids"
	"brandpulse/internal/models"
)

// PostgresStore is Store backed by the shared pool from internal/db.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) PostgresStore {
	return PostgresStore{pool: pool}
}

// jsonb marshals a value for a jsonb column. pgx encodes a Go slice as a
// Postgres array by default, which is the wrong type for every jsonb column in
// 001_init.sql, so the conversion is explicit everywhere.
//
// A nil slice becomes [] and a nil map becomes {}, never JSON null. Every jsonb
// column in 001_init.sql apart from the two payload columns is declared NOT
// NULL DEFAULT '[]' or '{}', and the literal null satisfies NOT NULL: Postgres
// checks for a SQL NULL, not for a JSON one. So writing null here puts a value
// in the table that the column promises cannot be there, every reader that
// unmarshals it gets a nil back, and the dashboard throws on the first .join.
// Checking v == nil is not enough because a nil []string arrives boxed in a
// non-nil interface, which is exactly how this got through the first time.
func jsonb(v any) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	switch value := reflect.ValueOf(v); value.Kind() {
	case reflect.Slice:
		if value.IsNil() {
			return []byte("[]"), nil
		}
	case reflect.Map:
		if value.IsNil() {
			return []byte("{}"), nil
		}
	}
	return json.Marshal(v)
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

// LoadProfile returns the highest-version confirmed profile. An unconfirmed
// profile is a draft bp-onboarder produced and a human has not agreed to, so a
// run must not spend credits against it.
func (s PostgresStore) LoadProfile(ctx context.Context, brandID string) (models.BrandProfile, error) {
	const query = `
		SELECT b.name, coalesce(b.website, ''),
		       p.keywords, p.hashtags, p.products, p.competitors, p.sources,
		       p.negative_keywords, p.source_handles, p.voice, p.version
		FROM brand_profiles p
		JOIN brands b ON b.id = p.brand_id
		WHERE p.brand_id = $1 AND p.confirmed_at IS NOT NULL
		ORDER BY p.version DESC
		LIMIT 1`

	var (
		profile                                   models.BrandProfile
		keywords, hashtags, products, competitors []byte
		sources, negatives, handles, voice        []byte
	)
	err := s.pool.QueryRow(ctx, query, brandID).Scan(
		&profile.Name, &profile.Website,
		&keywords, &hashtags, &products, &competitors, &sources,
		&negatives, &handles, &voice, &profile.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.BrandProfile{}, fmt.Errorf("no confirmed profile for brand %s", brandID)
	}
	if err != nil {
		return models.BrandProfile{}, err
	}

	profile.BrandID = brandID
	for _, field := range []struct {
		raw  []byte
		into any
	}{
		{keywords, &profile.Keywords},
		{hashtags, &profile.Hashtags},
		{products, &profile.Products},
		{competitors, &profile.Competitors},
		{sources, &profile.Sources},
		{negatives, &profile.NegativeKeywords},
		{handles, &profile.SourceHandles},
		{voice, &profile.Voice},
	} {
		if err := json.Unmarshal(field.raw, field.into); err != nil {
			return models.BrandProfile{}, fmt.Errorf("decoding profile for brand %s: %w", brandID, err)
		}
	}
	return profile, nil
}

func (s PostgresStore) CreditBudget(ctx context.Context, brandID string) (int, error) {
	var budget int
	err := s.pool.QueryRow(ctx, `SELECT daily_credit_budget FROM brands WHERE id = $1`, brandID).Scan(&budget)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("no brand %s", brandID)
	}
	return budget, err
}

// SourceYield is mentions per credit. A source with credits of 0 yields 0
// rather than an infinity: a free source is not evidence that it is the best
// source, and dividing by zero here would pin it at the top of every ranking
// forever.
func (s PostgresStore) SourceYield(ctx context.Context, brandID string, day time.Time) (map[models.Source]float64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT source, mentions, credits FROM source_yield WHERE brand_id = $1 AND day = $2`,
		brandID, day.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	yields := map[models.Source]float64{}
	for rows.Next() {
		var (
			source            models.Source
			mentions, credits int
		)
		if err := rows.Scan(&source, &mentions, &credits); err != nil {
			return nil, err
		}
		if credits > 0 {
			yields[source] = float64(mentions) / float64(credits)
		}
	}
	return yields, rows.Err()
}

func (s PostgresStore) CountMentions(ctx context.Context, brandID string, start, end time.Time) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM mentions WHERE brand_id = $1 AND posted_at >= $2 AND posted_at < $3`,
		brandID, start.UTC(), end.UTC()).Scan(&count)
	return count, err
}

// EnrichedInWindow returns every mention in the window that has been
// classified, newest first.
//
// It is an inner join because an EnrichedMention is both halves and there is no
// neutral enrichment to stand in: a mention the enricher has not reached yet is
// not yet analysable, and inventing a zero sentiment for it would move every
// average the detector reads.
//
// The nullable TEXT columns are coalesced because their Go fields are plain
// strings with omitempty, so "" and NULL already mean the same thing there.
// Rating is not: it is *float64 precisely so that "no rating" stays distinct
// from a zero-star review.
func (s PostgresStore) EnrichedInWindow(ctx context.Context, brandID string, start, end time.Time) ([]models.EnrichedMention, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.brand_id, m.source, m.external_id,
		       coalesce(m.url, ''), coalesce(m.author, ''),
		       m.author_followers, m.text, m.lang, m.posted_at, m.engagement,
		       m.rating, coalesce(m.matched_keyword, ''), m.content_hash,
		       e.sentiment, e.sentiment_label, e.emotion, e.intent, e.aspects,
		       e.is_about_brand, coalesce(e.about_competitor, ''), e.model, e.cost_paise
		  FROM mentions m
		  JOIN mention_enrichment e ON e.mention_id = m.id
		 WHERE m.brand_id = $1 AND m.posted_at >= $2 AND m.posted_at < $3
		 ORDER BY m.posted_at DESC`,
		brandID, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enriched []models.EnrichedMention
	for rows.Next() {
		var (
			item       models.EnrichedMention
			source     string
			label      string
			emotion    string
			intent     string
			engagement []byte
			aspects    []byte
		)
		if err := rows.Scan(
			&item.Mention.ID, &item.Mention.BrandID, &source, &item.Mention.ExternalID,
			&item.Mention.URL, &item.Mention.Author, &item.Mention.AuthorFollowers,
			&item.Mention.Text, &item.Mention.Lang, &item.Mention.PostedAt, &engagement,
			&item.Mention.Rating, &item.Mention.MatchedKeyword, &item.Mention.ContentHash,
			&item.Enrichment.Sentiment, &label, &emotion, &intent, &aspects,
			&item.Enrichment.IsAboutBrand, &item.Enrichment.AboutCompetitor,
			&item.Enrichment.Model, &item.Enrichment.CostPaise,
		); err != nil {
			return nil, err
		}

		item.Mention.Source = models.Source(source)
		item.Enrichment.MentionID = item.Mention.ID
		item.Enrichment.SentimentLabel = models.SentimentLabel(label)
		item.Enrichment.Emotion = models.Emotion(emotion)
		item.Enrichment.Intent = models.Intent(intent)
		if err := json.Unmarshal(engagement, &item.Mention.Engagement); err != nil {
			return nil, fmt.Errorf("decoding engagement for %s: %w", item.Mention.ID, err)
		}
		if err := json.Unmarshal(aspects, &item.Enrichment.Aspects); err != nil {
			return nil, fmt.Errorf("decoding aspects for %s: %w", item.Mention.ID, err)
		}

		enriched = append(enriched, item)
	}
	return enriched, rows.Err()
}

// PriorWindowCounts sums topic sizes by label. Labels are model-written, so two
// windows only line up when the clusterer names the same theme the same way;
// a label that moved reads as a new topic with a flat trend, which is the
// honest answer rather than a fabricated one.
func (s PostgresStore) PriorWindowCounts(ctx context.Context, brandID string, start, end time.Time) (map[string]int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT label, sum(size)::int FROM topics
		 WHERE brand_id = $1 AND window_start >= $2 AND window_end <= $3
		 GROUP BY label`,
		brandID, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var (
			label string
			size  int
		)
		if err := rows.Scan(&label, &size); err != nil {
			return nil, err
		}
		counts[label] = size
	}
	return counts, rows.Err()
}

// ---------------------------------------------------------------------------
// The run row
// ---------------------------------------------------------------------------

// ClaimRun lets the unique constraint decide the winner.
//
// The insert either returns its own id, which means this caller owns the
// bucket, or conflicts, and the row already there is read back and returned
// with false. There is no check-then-insert, so two triggers arriving in the
// same millisecond cannot both believe they won.
func (s PostgresStore) ClaimRun(ctx context.Context, run models.RunRecord) (models.RunRecord, bool, error) {
	attempted, err := jsonb(run.SourcesAttempted)
	if err != nil {
		return models.RunRecord{}, false, err
	}
	skipped, err := jsonb(run.SourcesSkipped)
	if err != nil {
		return models.RunRecord{}, false, err
	}
	runErrors, err := jsonb(run.Errors)
	if err != nil {
		return models.RunRecord{}, false, err
	}

	const insert = `
		INSERT INTO runs (id, brand_id, kind, time_bucket, started_at, status,
		                  sources_attempted, sources_skipped, errors)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (brand_id, kind, time_bucket) DO NOTHING
		RETURNING id`

	var claimedID string
	err = s.pool.QueryRow(ctx, insert,
		run.ID, run.BrandID, string(run.Kind), run.TimeBucket, run.StartedAt.UTC(),
		string(run.Status), attempted, skipped, runErrors,
	).Scan(&claimedID)
	if err == nil {
		return run, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return models.RunRecord{}, false, err
	}

	existing, err := s.findRun(ctx, run.BrandID, run.Kind, run.TimeBucket)
	if err != nil {
		return models.RunRecord{}, false, err
	}
	return existing, false, nil
}

func (s PostgresStore) findRun(ctx context.Context, brandID string, kind models.RunKind, bucket string) (models.RunRecord, error) {
	const query = `
		SELECT id, brand_id, kind, time_bucket, started_at, finished_at, status,
		       credits_used, tokens_used, cost_paise,
		       sources_attempted, sources_skipped, coalesce(degraded_reason, ''),
		       mentions_collected, errors
		FROM runs
		WHERE brand_id = $1 AND kind = $2 AND time_bucket = $3`

	var (
		run                      models.RunRecord
		attempted, skipped, errs []byte
		kindText, statusText     string
	)
	err := s.pool.QueryRow(ctx, query, brandID, string(kind), bucket).Scan(
		&run.ID, &run.BrandID, &kindText, &run.TimeBucket, &run.StartedAt, &run.FinishedAt,
		&statusText, &run.CreditsUsed, &run.TokensUsed, &run.CostPaise,
		&attempted, &skipped, &run.DegradedReason, &run.MentionsCollected, &errs,
	)
	if err != nil {
		return models.RunRecord{}, err
	}
	run.Kind, run.Status = models.RunKind(kindText), models.RunStatus(statusText)

	if err := json.Unmarshal(attempted, &run.SourcesAttempted); err != nil {
		return models.RunRecord{}, err
	}
	if err := json.Unmarshal(skipped, &run.SourcesSkipped); err != nil {
		return models.RunRecord{}, err
	}
	if err := json.Unmarshal(errs, &run.Errors); err != nil {
		return models.RunRecord{}, err
	}
	return run, nil
}

func (s PostgresStore) FinishRun(ctx context.Context, run models.RunRecord) error {
	attempted, err := jsonb(run.SourcesAttempted)
	if err != nil {
		return err
	}
	skipped, err := jsonb(run.SourcesSkipped)
	if err != nil {
		return err
	}
	runErrors, err := jsonb(run.Errors)
	if err != nil {
		return err
	}

	const update = `
		UPDATE runs SET
			finished_at = $2, status = $3,
			credits_used = $4, tokens_used = $5, cost_paise = $6,
			sources_attempted = $7, sources_skipped = $8, degraded_reason = $9,
			mentions_collected = $10, errors = $11
		WHERE id = $1`

	_, err = s.pool.Exec(ctx, update,
		run.ID, run.FinishedAt, string(run.Status),
		run.CreditsUsed, run.TokensUsed, run.CostPaise,
		attempted, skipped, run.DegradedReason,
		run.MentionsCollected, runErrors,
	)
	return err
}

// RecordSourceYield accumulates within the day, because a brand can run more
// than once in a day and the ranking wants the day's total, not the last run's.
func (s PostgresStore) RecordSourceYield(ctx context.Context, brandID string, day time.Time, runs []sourceRun) error {
	if len(runs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, run := range runs {
		batch.Queue(`
			INSERT INTO source_yield (brand_id, source, day, mentions, credits)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (brand_id, source, day) DO UPDATE SET
				mentions = source_yield.mentions + EXCLUDED.mentions,
				credits  = source_yield.credits  + EXCLUDED.credits`,
			brandID, string(run.Source), day.UTC().Truncate(24*time.Hour), run.Mentions, run.Credits)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// ---------------------------------------------------------------------------
// Writes from the pipeline steps
// ---------------------------------------------------------------------------

// SaveMentions dedupes on content_hash, which is what makes a re-run of the
// same window free of duplicate rows even though every collected mention
// carries a fresh id.
func (s PostgresStore) SaveMentions(ctx context.Context, mentions []models.Mention) error {
	batch := &pgx.Batch{}
	for _, mention := range mentions {
		engagement, err := jsonb(mention.Engagement)
		if err != nil {
			return err
		}
		raw, err := jsonb(mention.Raw)
		if err != nil {
			return err
		}
		batch.Queue(`
			INSERT INTO mentions (id, brand_id, source, external_id, url, author,
			                      author_followers, text, lang, posted_at, engagement,
			                      rating, matched_keyword, content_hash, raw)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT (brand_id, content_hash) DO NOTHING`,
			mention.ID, mention.BrandID, string(mention.Source), mention.ExternalID,
			mention.URL, mention.Author, mention.AuthorFollowers, mention.Text,
			mention.Lang, mention.PostedAt.UTC(), engagement, mention.Rating,
			mention.MatchedKeyword, mention.ContentHash, raw)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// SaveEnrichments skips a mention that is not in the table.
//
// That happens when the same content was already collected under an earlier id:
// SaveMentions dropped the duplicate row, so the foreign key has nothing to
// point at. Without the WHERE EXISTS the whole batch fails on one repeat.
func (s PostgresStore) SaveEnrichments(ctx context.Context, enrichments []models.Enrichment) error {
	batch := &pgx.Batch{}
	for _, e := range enrichments {
		aspects, err := jsonb(e.Aspects)
		if err != nil {
			return err
		}
		batch.Queue(`
			INSERT INTO mention_enrichment (mention_id, sentiment, sentiment_label, emotion,
			                                intent, aspects, is_about_brand, about_competitor,
			                                model, cost_paise)
			SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
			WHERE EXISTS (SELECT 1 FROM mentions WHERE id = $1)
			ON CONFLICT (mention_id) DO NOTHING`,
			e.MentionID, e.Sentiment, string(e.SentimentLabel), string(e.Emotion),
			string(e.Intent), aspects, e.IsAboutBrand, e.AboutCompetitor,
			e.Model, e.CostPaise)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

func (s PostgresStore) SaveTopics(ctx context.Context, topics []models.Topic) error {
	batch := &pgx.Batch{}
	for _, topic := range topics {
		mix, err := jsonb(topic.SentimentMix)
		if err != nil {
			return err
		}
		batch.Queue(`
			INSERT INTO topics (id, brand_id, window_start, window_end, label, summary,
			                    size, sentiment_mix, trend)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO NOTHING`,
			topic.ID, topic.BrandID, topic.WindowStart.UTC(), topic.WindowEnd.UTC(),
			topic.Label, topic.Summary, topic.Size, mix, topic.Trend)

		for _, mentionID := range topic.MentionIDs {
			batch.Queue(`
				INSERT INTO topic_mentions (topic_id, mention_id)
				SELECT $1, $2
				WHERE EXISTS (SELECT 1 FROM mentions WHERE id = $2)
				ON CONFLICT DO NOTHING`,
				topic.ID, mentionID)
		}
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// SaveAlerts stores sample mention ids while the wire carries whole mentions.
// That divergence is CONTRACTS §3b: DronaHQ renders an alert without a second
// fetch, and the table does not duplicate the mentions it already holds.
//
// It returns the alerts carrying the ids the table ended up with, which is not
// always the ids it was handed. See storedIDs.
func (s PostgresStore) SaveAlerts(ctx context.Context, alerts []models.Alert) ([]models.Alert, error) {
	if len(alerts) == 0 {
		return nil, nil
	}
	batch := &pgx.Batch{}
	for _, alert := range alerts {
		evidence, err := jsonb(alert.Evidence)
		if err != nil {
			return nil, err
		}
		sampleIDs := make([]string, 0, len(alert.SampleMentions))
		for _, mention := range alert.SampleMentions {
			sampleIDs = append(sampleIDs, mention.ID)
		}
		samples, err := jsonb(sampleIDs)
		if err != nil {
			return nil, err
		}
		batch.Queue(`
			INSERT INTO alerts (id, brand_id, kind, severity, title, why, evidence,
			                    sample_mention_ids, status, dedupe_key, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (brand_id, dedupe_key) DO NOTHING`,
			alert.ID, alert.BrandID, string(alert.Kind), string(alert.Severity),
			alert.Title, alert.Why, evidence, samples, string(alert.Status),
			alert.DedupeKey, alert.CreatedAt.UTC())
	}
	if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
		return nil, err
	}
	return s.storedIDs(ctx, alerts)
}

// storedIDs swaps each alert's id for the id of the row that actually holds its
// dedupe key.
//
// bp-detector mints a fresh id every run while the dedupe key is stable for the
// hour, so the second run in an hour loses the insert to ON CONFLICT DO NOTHING
// and its id belongs to no row. A reply draft written against that id is
// silently discarded by the foreign key guard in SaveDrafts, and a brief citing
// it points the dashboard at nothing. One run covers one brand, so the lookup
// keys off the brand the first alert names.
func (s PostgresStore) storedIDs(ctx context.Context, alerts []models.Alert) ([]models.Alert, error) {
	keys := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		keys = append(keys, alert.DedupeKey)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT dedupe_key, id FROM alerts WHERE brand_id = $1 AND dedupe_key = ANY($2)`,
		alerts[0].BrandID, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idByKey := make(map[string]string, len(alerts))
	for rows.Next() {
		var key, id string
		if err := rows.Scan(&key, &id); err != nil {
			return nil, err
		}
		idByKey[key] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	stored := make([]models.Alert, 0, len(alerts))
	for _, alert := range alerts {
		if id, ok := idByKey[alert.DedupeKey]; ok {
			alert.ID = id
		}
		stored = append(stored, alert)
	}
	return stored, nil
}

// SaveDrafts writes a draft only when its alert is in the table, for the same
// reason SaveEnrichments checks: a deduped alert leaves the foreign key with
// nothing to point at.
//
// Every draft is written with status 'draft'. Nothing in this repo sends one.
//
// $2 is cast because it is the one parameter Postgres cannot type on its own:
// `$2 IS NULL` accepts any type and so resolves it to unknown, while the same
// parameter in the target list and in `id = $2` resolves to text, and the
// planner rejects the contradiction with 42P08 before the batch ever runs.
func (s PostgresStore) SaveDrafts(ctx context.Context, drafts []models.ReplyDraft) error {
	batch := &pgx.Batch{}
	for _, draft := range drafts {
		doNotSay, err := jsonb(draft.DoNotSay)
		if err != nil {
			return err
		}
		batch.Queue(`
			INSERT INTO reply_drafts (id, alert_id, mention_id, channel, text, tone,
			                          do_not_say, status)
			SELECT $1, $2::text, $3, $4, $5, $6, $7, $8
			WHERE $2::text IS NULL OR EXISTS (SELECT 1 FROM alerts WHERE id = $2::text)
			ON CONFLICT (id) DO NOTHING`,
			draft.ID, nullable(draft.AlertID), nullable(draft.MentionID),
			string(draft.Channel), draft.Text, draft.Tone, doNotSay, string(draft.Status))
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// SaveBrief upserts on (brand_id, period, period_start) so a re-run replaces
// the brief for that period instead of stacking a second one the dashboard
// would have to choose between.
func (s PostgresStore) SaveBrief(ctx context.Context, period string, brief models.DailyBrief) error {
	payload, err := jsonb(brief)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO briefs (id, brand_id, period, period_start, period_end, payload)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (brand_id, period, period_start) DO UPDATE SET
			period_end = EXCLUDED.period_end,
			payload    = EXCLUDED.payload`,
		ids.New("brf"), brief.BrandID, period,
		brief.PeriodStart.UTC(), brief.PeriodEnd.UTC(), payload)
	return err
}

// nullable turns an omitempty string into a SQL NULL. reply_drafts.alert_id and
// mention_id are nullable foreign keys and an empty string is not a key.
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
