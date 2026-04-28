// The nine-step pipeline for one brand and one trigger, as specified in
// docs/CONTRACTS.md §2.
//
// Three things this file must never do. It must not write an agent URL: peers
// are named, and the proxy address and routing header are Nasiko's. It must not
// start a bare goroutine: every fan-out is an errgroup with an explicit
// SetLimit, because the flow guard fails closed and an accidental ninth call is
// a dropped call. And it must not let one dead source kill a run: per-source
// errors are collected and the run finishes partial.

package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"brandpulse/internal/a2a"
	"brandpulse/internal/ids"
	"brandpulse/internal/models"
	"brandpulse/internal/stats"
)

// Peer agents are named, never addressed. internal/a2a resolves a name through
// the Nasiko proxy; a hardcoded URL here breaks on the first redeploy.
const (
	agentCollector = "bp-collector"
	agentEnricher  = "bp-enricher"
	agentClusterer = "bp-clusterer"
	agentSOV       = "bp-sov"
	agentDetector  = "bp-detector"
	agentResponder = "bp-responder"
	agentBriefer   = "bp-briefer"
)

const (
	// defaultWindowHours is what RunInput.WindowHours == 0 means. The agent
	// applies the default; there is no *int in this contract.
	defaultWindowHours = 24

	// analysisFanOut and responderFanOut are steps 6 and 7. Step 4 uses
	// fanOutCap, which is the flow guard's own limit.
	analysisFanOut  = 3
	responderFanOut = 5

	// baselineDays is passed to ComputeBaseline explicitly. It has no default:
	// 0 means zero days, not fourteen.
	baselineDays = 14

	// briefPeriodDaily is the period a scheduled run writes. Weekly briefs are
	// a separate trigger and B5 renders their PDF.
	briefPeriodDaily = "daily"
)

// Store is everything this pipeline reads from and writes to Postgres.
//
// It is an interface because the pipeline has two callers that matter: the
// deployed agent, backed by pgx, and the tests, backed by a fake. Running the
// concurrency tests against a real database would make them slow and flaky,
// which is the same as not having them.
type Store interface {
	// LoadProfile returns the latest confirmed profile for a brand.
	LoadProfile(ctx context.Context, brandID string) (models.BrandProfile, error)

	// CreditBudget is the brand's hard ceiling on Anakin credits for one day.
	CreditBudget(ctx context.Context, brandID string) (int, error)

	// SourceYield is mentions per credit per source for one UTC day.
	SourceYield(ctx context.Context, brandID string, day time.Time) (map[models.Source]float64, error)

	// ClaimRun inserts the run and reports whether this call owns it. A row
	// already present for (brand_id, kind, time_bucket) is returned instead,
	// with false. The unique constraint decides, so two simultaneous triggers
	// cannot both win.
	ClaimRun(ctx context.Context, run models.RunRecord) (models.RunRecord, bool, error)

	// FinishRun writes the terminal state of a run this call owns.
	FinishRun(ctx context.Context, run models.RunRecord) error

	// RecordSourceYield writes what each source returned, which is what the
	// next run ranks against.
	RecordSourceYield(ctx context.Context, brandID string, day time.Time, runs []sourceRun) error

	// CountMentions is how many mentions a brand had in a window, used for the
	// brief's period-over-period delta.
	CountMentions(ctx context.Context, brandID string, start, end time.Time) (int, error)

	// PriorWindowCounts is the previous window's mentions per topic label,
	// which is what Topic.Trend is computed against.
	PriorWindowCounts(ctx context.Context, brandID string, start, end time.Time) (map[string]int, error)

	SaveMentions(ctx context.Context, mentions []models.Mention) error
	SaveEnrichments(ctx context.Context, enrichments []models.Enrichment) error
	SaveTopics(ctx context.Context, topics []models.Topic) error
	SaveAlerts(ctx context.Context, alerts []models.Alert) error
	SaveDrafts(ctx context.Context, drafts []models.ReplyDraft) error
	SaveBrief(ctx context.Context, period string, brief models.DailyBrief) error
}

// BaselineFunc is the shape of stats.ComputeBaseline, held as a dependency so
// the pipeline tests do not need fourteen days of rows in a database.
type BaselineFunc func(ctx context.Context, brandID string, days int, now time.Time) (models.BaselineStats, error)

// OrchestratorHandler runs the pipeline. Call, Baseline and Now are injected so
// every test in this package runs with no network, no database and no clock.
type OrchestratorHandler struct {
	Store    Store
	Call     a2a.CallFunc
	Baseline BaselineFunc
	Now      func() time.Time
}

// NewOrchestratorHandler wires the real peers, the real baseline query and the
// real clock.
func NewOrchestratorHandler(store Store) OrchestratorHandler {
	return OrchestratorHandler{
		Store:    store,
		Call:     a2a.Call,
		Baseline: stats.ComputeBaseline,
		Now:      time.Now,
	}
}

// window is the span of time a run covers.
type window struct {
	start time.Time
	end   time.Time
}

// sourceRun is what one source returned, kept so the next run can rank it.
type sourceRun struct {
	Source   models.Source
	Mentions int
	Credits  int
}

// spend is what a run actually cost, summed from the child artifacts. Never an
// estimate: eval/ reads these numbers and prices a brand-day off them.
type spend struct {
	credits int
	tokens  int
	paise   float64
}

// runLog collects what went wrong without stopping anything. Its mutex is the
// reason a failing collector degrades the run instead of racing on a slice.
type runLog struct {
	mu     sync.Mutex
	errors []string
}

func (l *runLog) addf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, fmt.Sprintf(format, args...))
}

// all returns the errors sorted, so two runs over the same failures produce the
// same row and a diff of two demo runs shows only what really changed.
func (l *runLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.errors))
	copy(out, l.errors)
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Steps 1 and 2
// ---------------------------------------------------------------------------

// Handle runs the pipeline once.
//
// The error return is for malformed input and for the two failures that make a
// run impossible: no profile to run against, and no way to claim the run. Once
// the run is claimed every later failure is recorded and the run finishes
// partial, because a dead source must not kill a run.
func (h OrchestratorHandler) Handle(ctx context.Context, in models.RunInput) (models.RunRecord, error) {
	if in.BrandID == "" {
		return models.RunRecord{}, fmt.Errorf("bp-orchestrator: RunInput.brand_id is required")
	}
	if in.Trigger == "" {
		return models.RunRecord{}, fmt.Errorf("bp-orchestrator: RunInput.trigger is required, it is half of the idempotency key")
	}

	now := h.Now().UTC()

	profile, err := h.Store.LoadProfile(ctx, in.BrandID)
	if err != nil {
		return models.RunRecord{}, fmt.Errorf("bp-orchestrator: loading the profile for %s: %w", in.BrandID, err)
	}

	run, mine, err := h.claim(ctx, in, bucketFor(in.Trigger, now), now)
	if err != nil {
		return models.RunRecord{}, err
	}
	if !mine {
		// Step 2: a run already covers this bucket and Force is false. Return
		// it unchanged, call nobody, spend nothing.
		return run, nil
	}

	return h.execute(ctx, in, profile, run, windowFor(in, now)), nil
}

// bucketFor picks the idempotency grain.
//
// A scheduled run and an onboarding run happen once per day, so their bucket is
// the day. An on-demand run and a crisis replay are things a human triggers
// again while watching, so their bucket is the hour: re-triggering inside the
// same hour is the no-op that proves idempotency, and an hour later is a new
// run rather than a silent refusal.
func bucketFor(kind models.RunKind, now time.Time) string {
	switch kind {
	case models.RunScheduled, models.RunOnboard:
		return stats.DayBucket(now)
	default:
		return stats.HourBucket(now)
	}
}

func windowFor(in models.RunInput, now time.Time) window {
	hours := in.WindowHours
	if hours <= 0 {
		hours = defaultWindowHours
	}
	return window{start: now.Add(-time.Duration(hours) * time.Hour), end: now}
}

// claim takes ownership of the bucket, or reports who already has it.
//
// The insert carries the unique constraint, so this is a claim and not a check
// followed by a hope. Force re-runs against the row that already exists, which
// is the only thing runs.UNIQUE (brand_id, kind, time_bucket) allows: the run
// keeps its id and is written again.
func (h OrchestratorHandler) claim(ctx context.Context, in models.RunInput, bucket string, now time.Time) (models.RunRecord, bool, error) {
	run := models.NewRunRecord(ids.New("run"), in.BrandID, in.Trigger, bucket, now)
	if err := run.Validate(); err != nil {
		return models.RunRecord{}, false, fmt.Errorf("bp-orchestrator: %w", err)
	}

	claimed, mine, err := h.Store.ClaimRun(ctx, run)
	if err != nil {
		return models.RunRecord{}, false, fmt.Errorf("bp-orchestrator: claiming run bucket %s: %w", bucket, err)
	}
	if mine || !in.Force {
		return claimed, mine, nil
	}

	// Forced: adopt the existing row and run it again from the top.
	claimed.Status = models.RunStatusRunning
	claimed.StartedAt = now
	claimed.FinishedAt = nil
	claimed.Errors = []string{}
	claimed.SourcesAttempted = []models.Source{}
	claimed.SourcesSkipped = []models.Source{}
	claimed.DegradedReason = ""
	claimed.CreditsUsed, claimed.TokensUsed, claimed.CostPaise = 0, 0, 0
	claimed.MentionsCollected = 0
	return claimed, true, nil
}

// ---------------------------------------------------------------------------
// Steps 3 to 9
// ---------------------------------------------------------------------------

// execute runs the claimed pipeline and always returns a written run record.
// From here on nothing returns an error: every failure lands in the run's
// Errors and the status says how much of the pipeline survived.
func (h OrchestratorHandler) execute(ctx context.Context, in models.RunInput, profile models.BrandProfile, run models.RunRecord, w window) models.RunRecord {
	log := &runLog{}
	var total spend

	// Step 3: rank and cap the sources.
	yields, err := h.Store.SourceYield(ctx, in.BrandID, w.end.AddDate(0, 0, -1))
	if err != nil {
		log.addf("source yield unavailable, sources ranked in profile order: %v", err)
		yields = map[models.Source]float64{}
	}
	chosen, skipped := chooseSources(profile, yields, fanOutCap)
	run.SourcesAttempted = chosen
	run.SourcesSkipped = skipped
	run.DegradedReason = degradedReason(skipped, fanOutCap)

	budget, err := h.Store.CreditBudget(ctx, in.BrandID)
	if err != nil {
		log.addf("credit budget unavailable, collectors ran with no per-source ceiling: %v", err)
	}

	// Step 4: collect in parallel, persist.
	mentions, perSource, failed := h.collect(ctx, profile, chosen, w, run.ID, splitCredits(budget, len(chosen)), log, &total)
	run.MentionsCollected = len(mentions)
	if len(mentions) > 0 {
		if err := h.Store.SaveMentions(ctx, mentions); err != nil {
			log.addf("persisting mentions: %v", err)
		}
	}

	// Step 5: enrich, persist.
	enriched := h.enrich(ctx, profile, mentions, log, &total)

	// Step 6: cluster, count share of voice and detect, in parallel.
	topics, sov, alerts := h.analyse(ctx, in.BrandID, profile, enriched, w, log, &total)

	// Step 7: draft a reply for every alert a human would have to answer today.
	h.respond(ctx, profile, alerts, log)

	// Step 8: write the brief.
	h.writeBrief(ctx, in.BrandID, profile, topics, alerts, sov, enriched, w, log)

	// Step 9: close the run with what it really spent.
	run.CreditsUsed, run.TokensUsed, run.CostPaise = total.credits, total.tokens, total.paise
	run.Errors = log.all()
	run.Status = finalStatus(run, len(chosen), failed)
	finished := h.Now().UTC()
	run.FinishedAt = &finished

	if err := h.Store.RecordSourceYield(ctx, in.BrandID, w.end, perSource); err != nil {
		run.Errors = append(run.Errors, fmt.Sprintf("recording source yield: %v", err))
	}
	if err := h.Store.FinishRun(ctx, run); err != nil {
		run.Errors = append(run.Errors, fmt.Sprintf("writing the runs row: %v", err))
	}
	return run
}

// finalStatus reads the run, not the exceptions. A run that dropped a source to
// the cap is partial even when nothing errored, because the number on the
// dashboard is smaller than the brand's sources would have produced and a
// founder is entitled to know why.
func finalStatus(run models.RunRecord, attempted, failed int) models.RunStatus {
	if attempted > 0 && failed == attempted {
		return models.RunStatusFailed
	}
	if len(run.Errors) > 0 || run.DegradedReason != "" {
		return models.RunStatusPartial
	}
	return models.RunStatusOK
}

// ---------------------------------------------------------------------------
// Step 4
// ---------------------------------------------------------------------------

// collect calls bp-collector once per source, in parallel, bounded.
//
// It deliberately does not use errgroup.WithContext. That cancels every sibling
// on the first error, which here would mean one rate-limited source killing the
// other seven. Every goroutine returns nil; failures go in the log and the
// group always finishes.
func (h OrchestratorHandler) collect(
	ctx context.Context,
	profile models.BrandProfile,
	sources []models.Source,
	w window,
	runID string,
	perSourceCredits int,
	log *runLog,
	total *spend,
) (mentions []models.Mention, perSource []sourceRun, failed int) {
	perSource = make([]sourceRun, 0, len(sources))
	if len(sources) == 0 {
		return nil, perSource, 0
	}

	var mu sync.Mutex
	g := new(errgroup.Group)
	g.SetLimit(fanOutCap)

	for _, source := range sources {
		g.Go(func() error {
			in := models.CollectInput{
				Profile:     profile,
				Source:      source,
				WindowStart: w.start,
				WindowEnd:   w.end,
				MaxCredits:  perSourceCredits,
				RunID:       runID,
			}

			var batch models.MentionBatch
			err := h.Call(ctx, agentCollector, in, &batch)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failed++
				log.addf("%s(%s): %v", agentCollector, source, err)
				return nil
			}
			for _, e := range batch.Errors {
				log.addf("%s(%s): %s", agentCollector, source, e)
			}
			if batch.Truncated {
				log.addf("%s(%s): stopped at the credit ceiling of %d", agentCollector, source, perSourceCredits)
			}

			mentions = append(mentions, batch.Mentions...)
			total.credits += batch.CreditsUsed
			perSource = append(perSource, sourceRun{
				Source:   source,
				Mentions: len(batch.Mentions),
				Credits:  batch.CreditsUsed,
			})
			return nil
		})
	}
	// No goroutine above returns an error, so Wait cannot either.
	_ = g.Wait()

	sortMentions(mentions)
	sort.Slice(perSource, func(i, j int) bool { return perSource[i].Source < perSource[j].Source })
	return mentions, perSource, failed
}

// sortMentions puts the collected set in a fixed order. Goroutines finish in
// whatever order the network allows, and an unsorted slice makes every
// downstream artifact reorder between runs, which is a demo that cannot be
// rehearsed.
func sortMentions(mentions []models.Mention) {
	sort.SliceStable(mentions, func(i, j int) bool {
		if !mentions[i].PostedAt.Equal(mentions[j].PostedAt) {
			return mentions[i].PostedAt.Before(mentions[j].PostedAt)
		}
		return mentions[i].ID < mentions[j].ID
	})
}

// ---------------------------------------------------------------------------
// Step 5
// ---------------------------------------------------------------------------

// enrich classifies the window in one call and joins the results back onto the
// mentions. The enricher batches internally; the orchestrator does not fan out
// here, because one agent handling fifty mentions is one call and fifty
// collectors would be fifty.
func (h OrchestratorHandler) enrich(ctx context.Context, profile models.BrandProfile, mentions []models.Mention, log *runLog, total *spend) []models.EnrichedMention {
	if len(mentions) == 0 {
		return nil
	}

	var batch models.EnrichmentBatch
	if err := h.Call(ctx, agentEnricher, models.EnrichInput{Mentions: mentions, Profile: profile}, &batch); err != nil {
		log.addf("%s: %v", agentEnricher, err)
		return nil
	}
	for _, e := range batch.Errors {
		log.addf("%s: %s", agentEnricher, e)
	}
	total.tokens += batch.TokensUsed
	total.paise += batch.CostPaise

	if len(batch.Enrichments) > 0 {
		if err := h.Store.SaveEnrichments(ctx, batch.Enrichments); err != nil {
			log.addf("persisting enrichments: %v", err)
		}
	}

	// Order is not guaranteed, so join on MentionID. A mention with no
	// enrichment is dropped rather than carried with a zero-valued one: a zero
	// Enrichment reads as neutral sentiment about the brand, which is a fact
	// nobody established.
	byID := make(map[string]models.Enrichment, len(batch.Enrichments))
	for _, e := range batch.Enrichments {
		byID[e.MentionID] = e
	}

	enriched := make([]models.EnrichedMention, 0, len(mentions))
	for _, mention := range mentions {
		enrichment, ok := byID[mention.ID]
		if !ok {
			log.addf("%s: no enrichment for mention %s", agentEnricher, mention.ID)
			continue
		}
		enriched = append(enriched, models.EnrichedMention{Mention: mention, Enrichment: enrichment})
	}
	return enriched
}

// ---------------------------------------------------------------------------
// Step 6
// ---------------------------------------------------------------------------

// analyse runs the clusterer, the share-of-voice count and the detector at the
// same time. They share an input and nothing else, so each writes its own
// result and only the spend needs a lock.
func (h OrchestratorHandler) analyse(
	ctx context.Context,
	brandID string,
	profile models.BrandProfile,
	enriched []models.EnrichedMention,
	w window,
	log *runLog,
	total *spend,
) ([]models.Topic, models.ShareOfVoice, []models.Alert) {
	sov := models.NewShareOfVoice(brandID, w.start, w.end)
	if len(enriched) == 0 {
		return nil, sov, nil
	}

	var (
		mu     sync.Mutex
		topics []models.Topic
		alerts []models.Alert
	)

	g := new(errgroup.Group)
	g.SetLimit(analysisFanOut)

	g.Go(func() error {
		prior, err := h.Store.PriorWindowCounts(ctx, brandID, w.start.Add(-w.end.Sub(w.start)), w.start)
		if err != nil {
			log.addf("prior window counts unavailable, topic trends read flat: %v", err)
			prior = map[string]int{}
		}

		var set models.TopicSet
		if err := h.Call(ctx, agentClusterer, models.ClusterInput{
			Enriched:          enriched,
			BrandID:           brandID,
			WindowStart:       w.start,
			WindowEnd:         w.end,
			PriorWindowCounts: prior,
		}, &set); err != nil {
			log.addf("%s: %v", agentClusterer, err)
			return nil
		}
		for _, e := range set.Errors {
			log.addf("%s: %s", agentClusterer, e)
		}

		mu.Lock()
		topics = set.Topics
		total.tokens += set.TokensUsed
		total.paise += set.CostPaise
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		var counted models.ShareOfVoice
		if err := h.Call(ctx, agentSOV, models.SOVInput{
			Enriched:    enriched,
			Profile:     profile,
			WindowStart: w.start,
			WindowEnd:   w.end,
		}, &counted); err != nil {
			log.addf("%s: %v", agentSOV, err)
			return nil
		}

		mu.Lock()
		sov = counted
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		baseline, err := h.Baseline(ctx, brandID, baselineDays, w.end)
		if err != nil {
			log.addf("baseline unavailable, no alert rule could run: %v", err)
			return nil
		}

		var set models.AlertSet
		if err := h.Call(ctx, agentDetector, models.DetectInput{
			BrandID:  brandID,
			Enriched: enriched,
			Baseline: baseline,
			Now:      w.end,
		}, &set); err != nil {
			log.addf("%s: %v", agentDetector, err)
			return nil
		}
		for _, e := range set.Errors {
			log.addf("%s: %s", agentDetector, e)
		}

		mu.Lock()
		alerts = set.Alerts
		mu.Unlock()
		return nil
	})

	_ = g.Wait()

	if len(topics) > 0 {
		if err := h.Store.SaveTopics(ctx, topics); err != nil {
			log.addf("persisting topics: %v", err)
		}
	}
	if len(alerts) > 0 {
		if err := h.Store.SaveAlerts(ctx, alerts); err != nil {
			log.addf("persisting alerts: %v", err)
		}
	}
	return topics, sov, alerts
}

// ---------------------------------------------------------------------------
// Step 7
// ---------------------------------------------------------------------------

// respond drafts a reply for every alert a human has to answer today.
//
// It drafts. Nothing in this repo posts, not behind a flag. Each draft is
// written with requires_human_approval on the wire and a person sends it or
// does not.
func (h OrchestratorHandler) respond(ctx context.Context, profile models.BrandProfile, alerts []models.Alert, log *runLog) {
	urgent := make([]models.Alert, 0, len(alerts))
	for _, alert := range alerts {
		if alert.Severity == models.SeverityHigh || alert.Severity == models.SeverityCritical {
			urgent = append(urgent, alert)
		}
	}
	if len(urgent) == 0 {
		return
	}

	var (
		mu     sync.Mutex
		drafts []models.ReplyDraft
	)

	g := new(errgroup.Group)
	g.SetLimit(responderFanOut)

	for _, alert := range urgent {
		g.Go(func() error {
			var draft models.ReplyDraft
			if err := h.Call(ctx, agentResponder, models.RespondInput{
				Alert:   &alert,
				Profile: profile,
				Channel: models.ChannelStatement,
			}, &draft); err != nil {
				log.addf("%s(%s): %v", agentResponder, alert.ID, err)
				return nil
			}

			mu.Lock()
			drafts = append(drafts, draft)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	if len(drafts) == 0 {
		return
	}
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].AlertID < drafts[j].AlertID })
	if err := h.Store.SaveDrafts(ctx, drafts); err != nil {
		log.addf("persisting reply drafts: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Step 8
// ---------------------------------------------------------------------------

// writeBrief hands bp-briefer every figure it needs. The briefer does not query
// the database, which is what makes it pure, and that only holds if the numbers
// are computed here.
func (h OrchestratorHandler) writeBrief(
	ctx context.Context,
	brandID string,
	profile models.BrandProfile,
	topics []models.Topic,
	alerts []models.Alert,
	sov models.ShareOfVoice,
	enriched []models.EnrichedMention,
	w window,
	log *runLog,
) {
	span := w.end.Sub(w.start)
	prior, err := h.Store.CountMentions(ctx, brandID, w.start.Add(-span), w.start)
	if err != nil {
		log.addf("prior mention count unavailable, the brief's delta reads flat: %v", err)
	}

	var brief models.DailyBrief
	if err := h.Call(ctx, agentBriefer, models.BriefInput{
		BrandID:     brandID,
		Profile:     profile,
		Period:      briefPeriodDaily,
		PeriodStart: w.start,
		PeriodEnd:   w.end,
		Topics:      topics,
		Alerts:      alerts,
		SOV:         sov,
		Numbers:     briefNumbers(enriched, prior, sov),
	}, &brief); err != nil {
		log.addf("%s: %v", agentBriefer, err)
		return
	}
	if err := h.Store.SaveBrief(ctx, briefPeriodDaily, brief); err != nil {
		log.addf("persisting the brief: %v", err)
	}
}

// briefNumbers is every figure in the brief, computed once, here.
//
// MentionsDeltaPct is 0 when the prior window had none: a rise from nothing is
// not a percentage, and "up 100%" from one mention to two is a sentence that
// makes a founder distrust the whole page.
func briefNumbers(enriched []models.EnrichedMention, priorCount int, sov models.ShareOfVoice) models.BriefNumbers {
	numbers := models.BriefNumbers{
		Mentions:     len(enriched),
		ShareOfVoice: sov.BrandShare,
	}
	if priorCount > 0 {
		numbers.MentionsDeltaPct = (float64(len(enriched)) - float64(priorCount)) / float64(priorCount) * 100
	}
	if len(enriched) == 0 {
		return numbers
	}

	var sentimentTotal float64
	negative := 0
	for _, e := range enriched {
		sentimentTotal += e.Enrichment.Sentiment
		if e.Enrichment.SentimentLabel == models.SentimentNegative {
			negative++
		}
	}
	numbers.SentimentAvg = sentimentTotal / float64(len(enriched))
	numbers.NegativeShare = float64(negative) / float64(len(enriched)) * 100
	return numbers
}
