package main

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"brandpulse/internal/models"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const testBrandID = "brd_test"

// testNow is fixed, so every bucket, window and finished_at in these tests is
// the same on every machine and in CI.
var testNow = time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)

func testProfile(sources ...models.Source) models.BrandProfile {
	profile := models.NewBrandProfile(testBrandID, "Testbrand")
	profile.Keywords = []string{"testbrand"}
	profile.Competitors = []string{"Rivalco"}
	profile.Sources = sources
	return profile
}

func testMention(source models.Source, id string, postedAt time.Time) models.Mention {
	return models.Mention{
		ID:          id,
		BrandID:     testBrandID,
		Source:      source,
		ExternalID:  id + "_ext",
		Text:        "the strap on my testbrand order snapped in a week",
		Lang:        "en",
		PostedAt:    postedAt,
		ContentHash: "hash_" + id,
	}
}

func testEnrichment(mentionID string, label models.SentimentLabel, score float64) models.Enrichment {
	return models.Enrichment{
		MentionID:      mentionID,
		Sentiment:      score,
		SentimentLabel: label,
		IsAboutBrand:   true,
		Model:          "test-enricher",
	}
}

// ---------------------------------------------------------------------------
// The fake store
// ---------------------------------------------------------------------------

// fakeStore is a Store in memory. Its mutex is not decoration: PriorWindowCounts
// is called from inside the step 6 group, so -race reads it from a goroutine.
type fakeStore struct {
	mu sync.Mutex

	profile     models.BrandProfile
	profileErr  error
	budget      int
	budgetErr   error
	yields      map[models.Source]float64
	yieldErr    error
	priorCount  int
	priorTopics map[string]int

	runs map[string]models.RunRecord // keyed by brand|kind|bucket

	savedMentions []models.Mention
	savedEnrich   []models.Enrichment
	savedTopics   []models.Topic
	savedAlerts   []models.Alert
	savedDrafts   []models.ReplyDraft
	savedBriefs   []models.DailyBrief
	savedYield    []sourceRun
	finished      []models.RunRecord
}

func newFakeStore(profile models.BrandProfile) *fakeStore {
	return &fakeStore{
		profile:     profile,
		budget:      160,
		yields:      tenYields(),
		priorTopics: map[string]int{},
		runs:        map[string]models.RunRecord{},
	}
}

func runKey(brandID string, kind models.RunKind, bucket string) string {
	return brandID + "|" + string(kind) + "|" + bucket
}

func (s *fakeStore) LoadProfile(context.Context, string) (models.BrandProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.profile, s.profileErr
}

func (s *fakeStore) CreditBudget(context.Context, string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.budget, s.budgetErr
}

func (s *fakeStore) SourceYield(context.Context, string, time.Time) (map[models.Source]float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.yields, s.yieldErr
}

// ClaimRun reproduces the unique constraint: the first caller for a bucket owns
// it, every later one gets the row that is already there.
func (s *fakeStore) ClaimRun(_ context.Context, run models.RunRecord) (models.RunRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := runKey(run.BrandID, run.Kind, run.TimeBucket)
	if existing, ok := s.runs[key]; ok {
		return existing, false, nil
	}
	s.runs[key] = run
	return run, true, nil
}

func (s *fakeStore) FinishRun(_ context.Context, run models.RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[runKey(run.BrandID, run.Kind, run.TimeBucket)] = run
	s.finished = append(s.finished, run)
	return nil
}

func (s *fakeStore) RecordSourceYield(_ context.Context, _ string, _ time.Time, runs []sourceRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedYield = append(s.savedYield, runs...)
	return nil
}

func (s *fakeStore) CountMentions(context.Context, string, time.Time, time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.priorCount, nil
}

func (s *fakeStore) PriorWindowCounts(context.Context, string, time.Time, time.Time) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.priorTopics, nil
}

func (s *fakeStore) SaveMentions(_ context.Context, mentions []models.Mention) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedMentions = append(s.savedMentions, mentions...)
	return nil
}

func (s *fakeStore) SaveEnrichments(_ context.Context, e []models.Enrichment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedEnrich = append(s.savedEnrich, e...)
	return nil
}

func (s *fakeStore) SaveTopics(_ context.Context, t []models.Topic) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedTopics = append(s.savedTopics, t...)
	return nil
}

func (s *fakeStore) SaveAlerts(_ context.Context, a []models.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedAlerts = append(s.savedAlerts, a...)
	return nil
}

func (s *fakeStore) SaveDrafts(_ context.Context, d []models.ReplyDraft) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedDrafts = append(s.savedDrafts, d...)
	return nil
}

func (s *fakeStore) SaveBrief(_ context.Context, _ string, b models.DailyBrief) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedBriefs = append(s.savedBriefs, b)
	return nil
}

// ---------------------------------------------------------------------------
// The stub peers
// ---------------------------------------------------------------------------

// stubPeers answers every a2a.Call in memory and records what was asked.
type stubPeers struct {
	mu    sync.Mutex
	calls []string

	// inFlight and peak prove the fan-out cap, which is the one thing a
	// reviewer cannot check by reading the code alone.
	inFlight atomic.Int64
	peak     atomic.Int64

	// collect answers per source. A source missing from this map returns an
	// error, which is how the dead-source test is written.
	collect map[models.Source]models.MentionBatch

	enrichment models.EnrichmentBatch
	topics     models.TopicSet
	sov        models.ShareOfVoice
	alerts     models.AlertSet
	draft      models.ReplyDraft
	brief      models.DailyBrief

	// slowCollect makes the fan-out overlap so peak concurrency is observable.
	slowCollect time.Duration
}

func (p *stubPeers) record(agent string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, agent)
}

func (p *stubPeers) callsTo(agent string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, call := range p.calls {
		if call == agent {
			n++
		}
	}
	return n
}

func (p *stubPeers) Call(_ context.Context, agent string, in any, out any) error {
	p.record(agent)

	current := p.inFlight.Add(1)
	for {
		peak := p.peak.Load()
		if current <= peak || p.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	defer p.inFlight.Add(-1)

	switch agent {
	case agentCollector:
		if p.slowCollect > 0 {
			time.Sleep(p.slowCollect)
		}
		source := in.(models.CollectInput).Source
		batch, ok := p.collect[source]
		if !ok {
			return fmt.Errorf("source %s is rate limited", source)
		}
		*out.(*models.MentionBatch) = batch
	case agentEnricher:
		*out.(*models.EnrichmentBatch) = p.enrichment
	case agentClusterer:
		*out.(*models.TopicSet) = p.topics
	case agentSOV:
		*out.(*models.ShareOfVoice) = p.sov
	case agentDetector:
		*out.(*models.AlertSet) = p.alerts
	case agentResponder:
		*out.(*models.ReplyDraft) = p.draft
	case agentBriefer:
		*out.(*models.DailyBrief) = p.brief
	default:
		return fmt.Errorf("no stub for peer %q", agent)
	}
	return nil
}

func newStubPeers() *stubPeers {
	return &stubPeers{
		collect: map[models.Source]models.MentionBatch{},
		sov:     models.NewShareOfVoice(testBrandID, testNow.Add(-24*time.Hour), testNow),
		brief:   models.NewDailyBrief(testBrandID, testNow.Add(-24*time.Hour), testNow),
	}
}

// withMentions loads a source's stub answer and the matching enrichments, so a
// test names a source once instead of wiring two peers by hand.
func (p *stubPeers) withMentions(source models.Source, count, credits int) *stubPeers {
	batch := models.MentionBatch{Source: source, CreditsUsed: credits}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("mn_%s_%d", source, i)
		batch.Mentions = append(batch.Mentions, testMention(source, id, testNow.Add(-time.Duration(i)*time.Minute)))
		p.enrichment.Enrichments = append(p.enrichment.Enrichments,
			testEnrichment(id, models.SentimentNegative, -0.7))
	}
	p.collect[source] = batch
	return p
}

func newTestHandler(store *fakeStore, peers *stubPeers) OrchestratorHandler {
	var baseline models.BaselineStats
	return OrchestratorHandler{
		Store: store,
		Call:  peers.Call,
		Baseline: func(context.Context, string, int, time.Time) (models.BaselineStats, error) {
			return baseline, nil
		},
		Now: func() time.Time { return testNow },
	}
}

// ---------------------------------------------------------------------------
// Idempotency
// ---------------------------------------------------------------------------

// The brief's first named test: a double trigger returns the same run and calls
// nobody the second time.
func TestADoubleTriggerReturnsTheSameRunAndCallsNoPeer(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit))
	peers := newStubPeers().withMentions(models.SourceX, 3, 4).withMentions(models.SourceReddit, 2, 3)
	handler := newTestHandler(store, peers)

	in := models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}

	first, err := handler.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	callsAfterFirst := len(peers.calls)
	if callsAfterFirst == 0 {
		t.Fatal("the first run called no peer, so the second proves nothing")
	}

	second, err := handler.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("second run id %q, want the first run's %q", second.ID, first.ID)
	}
	if len(peers.calls) != callsAfterFirst {
		t.Errorf("the second trigger made %d further peer calls, want 0: %v",
			len(peers.calls)-callsAfterFirst, peers.calls[callsAfterFirst:])
	}
	if second.Status != first.Status || second.MentionsCollected != first.MentionsCollected {
		t.Errorf("the second trigger changed the run: %+v vs %+v", second, first)
	}
}

func TestForceReRunsTheSameBucketAndKeepsTheRunID(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	peers := newStubPeers().withMentions(models.SourceX, 3, 4)
	handler := newTestHandler(store, peers)

	in := models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}
	first, err := handler.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	callsAfterFirst := len(peers.calls)

	in.Force = true
	forced, err := handler.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("forced run: %v", err)
	}

	if forced.ID != first.ID {
		t.Errorf("forced run id %q, want the existing row's %q: the unique constraint allows one row per bucket",
			forced.ID, first.ID)
	}
	if len(peers.calls) <= callsAfterFirst {
		t.Error("a forced run called no peer, so it did not re-run anything")
	}
	if forced.CreditsUsed != first.CreditsUsed {
		t.Errorf("forced run spent %d credits, want %d: totals are reset, not accumulated",
			forced.CreditsUsed, first.CreditsUsed)
	}
}

// A scheduled run buckets by day and an on-demand run by hour, so re-triggering
// on stage inside the hour is the no-op and an hour later is a fresh run.
func TestTheBucketGrainFollowsTheTrigger(t *testing.T) {
	cases := []struct {
		kind models.RunKind
		want string
	}{
		{models.RunScheduled, "2026-09-20"},
		{models.RunOnboard, "2026-09-20"},
		{models.RunOnDemand, "2026-09-20T14"},
		{models.RunCrisisReplay, "2026-09-20T14"},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			if got := bucketFor(c.kind, testNow); got != c.want {
				t.Errorf("bucketFor(%q) = %q, want %q", c.kind, got, c.want)
			}
		})
	}
}

func TestTheTimeBucketIsNeverEmpty(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	peers := newStubPeers().withMentions(models.SourceX, 1, 1)
	handler := newTestHandler(store, peers)

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.TimeBucket == "" {
		t.Fatal("TimeBucket is empty, which is a NOT NULL insert failure, not a missing nicety")
	}
}

func TestAWindowOfZeroHoursMeans24(t *testing.T) {
	in := models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}
	if got := windowFor(in, testNow); got.end.Sub(got.start) != 24*time.Hour {
		t.Errorf("window is %v, want 24h when WindowHours is 0", got.end.Sub(got.start))
	}

	in.WindowHours = 2
	if got := windowFor(in, testNow); got.end.Sub(got.start) != 2*time.Hour {
		t.Errorf("window is %v, want 2h", got.end.Sub(got.start))
	}
}

func TestTheWindowReachesTheCollectors(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	peers := newStubPeers().withMentions(models.SourceX, 1, 1)

	var got models.CollectInput
	handler := newTestHandler(store, peers)
	inner := handler.Call
	handler.Call = func(ctx context.Context, agent string, in any, out any) error {
		if agent == agentCollector {
			got = in.(models.CollectInput)
		}
		return inner(ctx, agent, in, out)
	}

	_, err := handler.Handle(context.Background(), models.RunInput{
		BrandID: testBrandID, Trigger: models.RunOnDemand, WindowHours: 2,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if got.WindowEnd.Sub(got.WindowStart) != 2*time.Hour {
		t.Errorf("collector window is %v, want 2h", got.WindowEnd.Sub(got.WindowStart))
	}
	// 160 credits over 1 source.
	if got.MaxCredits != 160 {
		t.Errorf("collector ceiling is %d credits, want 160", got.MaxCredits)
	}
}

// ---------------------------------------------------------------------------
// Bounded, non-cancelling fan-out
// ---------------------------------------------------------------------------

// The brief's second named test: a dead source degrades the run instead of
// killing it, and the other sources' mentions still arrive.
func TestADeadSourceDegradesTheRunInsteadOfKillingIt(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit, models.SourceNews))
	// reddit has no stub answer, so its collector call errors.
	peers := newStubPeers().withMentions(models.SourceX, 4, 5).withMentions(models.SourceNews, 3, 4)
	handler := newTestHandler(store, peers)

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("a dead source returned an error out of the handler: %v", err)
	}

	if run.Status != models.RunStatusPartial {
		t.Errorf("status is %q, want %q", run.Status, models.RunStatusPartial)
	}
	if run.MentionsCollected != 7 {
		t.Errorf("collected %d mentions, want 7 from the two live sources", run.MentionsCollected)
	}
	if run.CreditsUsed != 9 {
		t.Errorf("spent %d credits, want 9 from the two live sources", run.CreditsUsed)
	}

	found := false
	for _, e := range run.Errors {
		if strings.Contains(e, "reddit") && strings.Contains(e, agentCollector) {
			found = true
		}
	}
	if !found {
		t.Errorf("the reddit failure is not in the run's errors: %v", run.Errors)
	}

	// The siblings were not cancelled, so both live sources answered.
	if peers.callsTo(agentCollector) != 3 {
		t.Errorf("%d collector calls, want 3: errgroup.WithContext would have cancelled the siblings",
			peers.callsTo(agentCollector))
	}
	if peers.callsTo(agentBriefer) != 1 {
		t.Error("the run stopped before the brief, so one dead source killed it")
	}
}

func TestEverySourceFailingIsAFailedRun(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit))
	peers := newStubPeers() // no stub answers at all
	handler := newTestHandler(store, peers)

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != models.RunStatusFailed {
		t.Errorf("status is %q, want %q when no source answered", run.Status, models.RunStatusFailed)
	}
}

// The flow guard fails closed, so a ninth concurrent call is a dropped call.
// This test watches the peak, because the cap cannot be read off the code.
func TestFanOutNeverExceedsTheCap(t *testing.T) {
	store := newFakeStore(tenSources())
	peers := newStubPeers()
	peers.slowCollect = 3 * time.Millisecond
	for source := range tenYields() {
		peers.withMentions(source, 2, 2)
	}
	handler := newTestHandler(store, peers)

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	peak := peers.peak.Load()
	if peak > fanOutCap {
		t.Errorf("peak concurrent peer calls was %d, over the cap of %d", peak, fanOutCap)
	}
	// Without overlap the cap assertion above would pass on serial calls and
	// prove nothing, so the test also checks the fan-out really fanned out.
	if peak < 2 {
		t.Errorf("peak concurrent peer calls was %d, so nothing ran in parallel and the cap was never exercised", peak)
	}
	if peers.callsTo(agentCollector) != fanOutCap {
		t.Errorf("%d collector calls, want %d: the cap is applied before any call is made",
			peers.callsTo(agentCollector), fanOutCap)
	}
	if len(run.SourcesSkipped) != 2 {
		t.Errorf("skipped %v, want the two lowest-yielding sources", run.SourcesSkipped)
	}
	if run.DegradedReason == "" {
		t.Error("the cap bit and DegradedReason is empty, so the dashboard shows a smaller number with no explanation")
	}
	if run.Status != models.RunStatusPartial {
		t.Errorf("status is %q, want %q: a run that dropped sources is partial", run.Status, models.RunStatusPartial)
	}
}

// ---------------------------------------------------------------------------
// Cost accounting
// ---------------------------------------------------------------------------

// eval/ prices a brand-day off these three numbers, so an estimate here becomes
// a lie in the pitch.
func TestCostsAreSummedFromTheChildArtifacts(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit))
	peers := newStubPeers().withMentions(models.SourceX, 2, 7).withMentions(models.SourceReddit, 3, 11)
	peers.enrichment.TokensUsed = 1200
	peers.enrichment.CostPaise = 42.5
	peers.topics.TokensUsed = 800
	peers.topics.CostPaise = 17.5
	handler := newTestHandler(store, peers)

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if run.CreditsUsed != 18 {
		t.Errorf("credits %d, want 18 (7 + 11 from the two MentionBatches)", run.CreditsUsed)
	}
	if run.TokensUsed != 2000 {
		t.Errorf("tokens %d, want 2000 (1200 enricher + 800 clusterer)", run.TokensUsed)
	}
	if run.CostPaise != 60 {
		t.Errorf("cost %v paise, want 60 (42.5 + 17.5)", run.CostPaise)
	}
}

func TestWhatEachSourceReturnedIsRecordedForTheNextRun(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit))
	peers := newStubPeers().withMentions(models.SourceX, 4, 5).withMentions(models.SourceReddit, 1, 5)
	handler := newTestHandler(store, peers)

	if _, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(store.savedYield) != 2 {
		t.Fatalf("recorded %d source yields, want 2: %+v", len(store.savedYield), store.savedYield)
	}
	for _, yield := range store.savedYield {
		if yield.Source == models.SourceX && (yield.Mentions != 4 || yield.Credits != 5) {
			t.Errorf("x yield is %+v, want 4 mentions for 5 credits", yield)
		}
	}
}

// ---------------------------------------------------------------------------
// The steps after collection
// ---------------------------------------------------------------------------

func TestAMentionWithNoEnrichmentIsDroppedRatherThanZeroed(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	peers := newStubPeers().withMentions(models.SourceX, 3, 4)
	// The enricher answered for two of the three.
	peers.enrichment.Enrichments = peers.enrichment.Enrichments[:2]

	var detect models.DetectInput
	handler := newTestHandler(store, peers)
	inner := handler.Call
	handler.Call = func(ctx context.Context, agent string, in any, out any) error {
		if agent == agentDetector {
			detect = in.(models.DetectInput)
		}
		return inner(ctx, agent, in, out)
	}

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(detect.Enriched) != 2 {
		t.Errorf("the detector saw %d enriched mentions, want 2: a zero-valued Enrichment reads as neutral about the brand",
			len(detect.Enriched))
	}
	if run.MentionsCollected != 3 {
		t.Errorf("collected %d, want 3: a dropped enrichment is not a dropped mention", run.MentionsCollected)
	}
}

// Only an alert a human has to answer today gets a draft. Nothing sends one.
func TestOnlyHighAndCriticalAlertsGetADraft(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	peers := newStubPeers().withMentions(models.SourceX, 3, 4)
	peers.alerts.Alerts = []models.Alert{
		models.NewAlert("alt_1", testBrandID, models.AlertSpike, models.SeverityLow),
		models.NewAlert("alt_2", testBrandID, models.AlertCrisis, models.SeverityCritical),
		models.NewAlert("alt_3", testBrandID, models.AlertReviewBomb, models.SeverityHigh),
		models.NewAlert("alt_4", testBrandID, models.AlertCompetitorMove, models.SeverityMedium),
	}
	peers.draft = models.NewReplyDraft("rd_1", models.ChannelStatement)
	handler := newTestHandler(store, peers)

	if _, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := peers.callsTo(agentResponder); got != 2 {
		t.Errorf("%d responder calls, want 2 (one critical, one high)", got)
	}
	for _, draft := range store.savedDrafts {
		if draft.Status != models.DraftStatusDraft {
			t.Errorf("draft %q saved with status %q, want %q: nothing in this repo advances a draft",
				draft.ID, draft.Status, models.DraftStatusDraft)
		}
	}
}

func TestTheBriefGetsItsNumbersFromTheOrchestrator(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	store.priorCount = 10
	peers := newStubPeers().withMentions(models.SourceX, 4, 5)
	// Three of the four are negative, one is positive.
	peers.enrichment.Enrichments[3] = testEnrichment("mn_x_3", models.SentimentPositive, 0.8)
	peers.sov.BrandShare = 61.5

	var brief models.BriefInput
	handler := newTestHandler(store, peers)
	inner := handler.Call
	handler.Call = func(ctx context.Context, agent string, in any, out any) error {
		if agent == agentBriefer {
			brief = in.(models.BriefInput)
		}
		return inner(ctx, agent, in, out)
	}

	if _, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if brief.Numbers.Mentions != 4 {
		t.Errorf("Mentions = %d, want 4", brief.Numbers.Mentions)
	}
	if brief.Numbers.MentionsDeltaPct != -60 {
		t.Errorf("MentionsDeltaPct = %v, want -60 (4 against a prior 10)", brief.Numbers.MentionsDeltaPct)
	}
	if brief.Numbers.NegativeShare != 75 {
		t.Errorf("NegativeShare = %v, want 75 (3 of 4)", brief.Numbers.NegativeShare)
	}
	if brief.Numbers.ShareOfVoice != 61.5 {
		t.Errorf("ShareOfVoice = %v, want 61.5 from the SOV artifact", brief.Numbers.ShareOfVoice)
	}
	if brief.Period != briefPeriodDaily {
		t.Errorf("Period = %q, want %q", brief.Period, briefPeriodDaily)
	}
}

// A first window has no prior one, and "up 100%" from nothing is a sentence
// that makes a founder distrust the whole page.
func TestNoPriorWindowMeansNoDelta(t *testing.T) {
	numbers := briefNumbers(
		[]models.EnrichedMention{{Enrichment: testEnrichment("mn_1", models.SentimentNeutral, 0)}},
		0,
		models.NewShareOfVoice(testBrandID, testNow, testNow),
	)
	if numbers.MentionsDeltaPct != 0 {
		t.Errorf("MentionsDeltaPct = %v, want 0 when the prior window held nothing", numbers.MentionsDeltaPct)
	}
}

// ---------------------------------------------------------------------------
// Determinism and input validation
// ---------------------------------------------------------------------------

// Goroutines finish in whatever order the network allows, so the collected set
// is sorted before anything downstream sees it.
func TestTheCollectedSetIsInAFixedOrder(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit, models.SourceNews))
	peers := newStubPeers()
	peers.slowCollect = time.Millisecond
	peers.withMentions(models.SourceX, 3, 3).
		withMentions(models.SourceReddit, 3, 3).
		withMentions(models.SourceNews, 3, 3)
	handler := newTestHandler(store, peers)

	var first []string
	for run := 0; run < 5; run++ {
		store.savedMentions = nil
		store.runs = map[string]models.RunRecord{}

		if _, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}

		order := make([]string, 0, len(store.savedMentions))
		for _, mention := range store.savedMentions {
			order = append(order, mention.ID)
		}
		if run == 0 {
			first = order
			continue
		}
		for i := range first {
			if order[i] != first[i] {
				t.Fatalf("run %d position %d: got %q, run 0 gave %q", run, i, order[i], first[i])
			}
		}
	}
}

func TestTheRunErrorsAreSorted(t *testing.T) {
	log := &runLog{}
	log.addf("zebra")
	log.addf("alpha")
	log.addf("mango")

	got := log.all()
	if got[0] != "alpha" || got[2] != "zebra" {
		t.Errorf("errors are %v, want them sorted so two runs over the same failures diff cleanly", got)
	}
}

func TestHandleRejectsInputItCannotBucket(t *testing.T) {
	handler := newTestHandler(newFakeStore(testProfile(models.SourceX)), newStubPeers())

	cases := []struct {
		name string
		in   models.RunInput
	}{
		{"no brand", models.RunInput{Trigger: models.RunScheduled}},
		{"no trigger", models.RunInput{BrandID: testBrandID}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := handler.Handle(context.Background(), c.in); err == nil {
				t.Error("want an error, got none")
			}
		})
	}
}

func TestAMissingProfileStopsTheRunBeforeItSpendsAnything(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX))
	store.profileErr = fmt.Errorf("no confirmed profile")
	peers := newStubPeers().withMentions(models.SourceX, 3, 4)
	handler := newTestHandler(store, peers)

	if _, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled}); err == nil {
		t.Fatal("want an error when there is no profile to run against")
	}
	if len(peers.calls) != 0 {
		t.Errorf("called %v without a profile", peers.calls)
	}
}

// A brand with no source yield history still runs, in profile order.
func TestAMissingYieldHistoryDegradesRatherThanStops(t *testing.T) {
	store := newFakeStore(testProfile(models.SourceX, models.SourceReddit))
	store.yieldErr = fmt.Errorf("source_yield unavailable")
	peers := newStubPeers().withMentions(models.SourceX, 2, 2).withMentions(models.SourceReddit, 2, 2)
	handler := newTestHandler(store, peers)

	run, err := handler.Handle(context.Background(), models.RunInput{BrandID: testBrandID, Trigger: models.RunScheduled})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.MentionsCollected != 4 {
		t.Errorf("collected %d, want 4: a missing ranking is not a reason to collect nothing", run.MentionsCollected)
	}
	if run.Status != models.RunStatusPartial {
		t.Errorf("status is %q, want %q with the yield failure recorded", run.Status, models.RunStatusPartial)
	}
}

// ---------------------------------------------------------------------------
// The two things this agent must not contain
// ---------------------------------------------------------------------------

// Peer calls go through internal/a2a, which resolves a name through the Nasiko
// proxy. A URL written here breaks on the first redeploy and bypasses the flow
// guard, so this test reads the package source rather than trusting review.
func TestNoAgentURLIsWrittenInThisPackage(t *testing.T) {
	forEachSourceLine(t, func(file string, line int, text string) {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
			t.Errorf("%s:%d writes a URL: %s", file, line, strings.TrimSpace(text))
		}
	})
}

// Nothing auto-posts, not behind a flag. There is no posting code path in this
// repo and this test is what keeps one from arriving quietly.
func TestNoOutboundHTTPClientAndNoPostingPath(t *testing.T) {
	forbidden := map[string]string{
		"net/http":                     "an agent that can post is an agent that will",
		"github.com/go-resty/resty/v2": "an HTTP client here is a posting path",
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing the package: %v", err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, imported := range file.Imports {
				path := strings.Trim(imported.Path.Value, `"`)
				if why, banned := forbidden[path]; banned {
					t.Errorf("%s imports %q: %s", name, path, why)
				}
			}
		}
	}
}

// bp-orchestrator is statistics, buckets and caps. It has no prompt and no
// model call of its own; every LLM cost on the runs row came from a child.
func TestTheOrchestratorUsesNoLanguageModel(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing the package: %v", err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, imported := range file.Imports {
				if path := strings.Trim(imported.Path.Value, `"`); path == "brandpulse/internal/llm" || path == "brandpulse/internal/prompts" {
					t.Errorf("%s imports %q, so the orchestrator is making a model call", name, path)
				}
			}
		}
	}
}

// forEachSourceLine walks the package's non-test Go files line by line.
func forEachSourceLine(t *testing.T, check func(file string, line int, text string)) {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.PackageClauseOnly)
	if err != nil {
		t.Fatalf("parsing the package: %v", err)
	}
	for _, pkg := range pkgs {
		for name := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			for i, line := range strings.Split(string(raw), "\n") {
				check(name, i+1, line)
			}
		}
	}
}
