# Agent Contracts

The frozen interface between the nine BrandPulse agents. Tracks B1–B6 build in
parallel against this document and do not need to read each other's code.

**Freeze rule:** once `main` carries this file, no track changes a signature
alone. A change is proposed in `HACKATHON_NOTES.md`, agreed, then landed on
`main` by B1 and rebased into every worktree.

Every model referenced here lives in
[internal/models/models.go](../internal/models/models.go) (domain types) and
[internal/models/agentio.go](../internal/models/agentio.go) (the per-agent
request and response envelopes, B1 Task 0). Those files are the
machine-readable version of this document; where the two disagree, the code
wins and this file gets corrected in the same session.

The language decision and what it cost is in
[docs/decisions/001-go-for-agents.md](decisions/001-go-for-agents.md).

---

## 0. Module layout

Module path is `brandpulse` (a bare path, not a GitHub URL, because the repo
has no remote yet). Before the Nasiko deploy, B5 changes it once, on `main`,
and that is **not** a one-line change: `go mod edit -module` rewrites `go.mod`
alone, so every `brandpulse/internal/...` import statement in the tree is
rewritten in the same commit, then `go mod tidy`, then
`go build ./... && go vet ./...`. It lands as its own commit with a rebase
notice in `HACKATHON_NOTES.md`, because it touches every file five tracks are
working in.

```
internal/            shared library. B1 owns it, with one exception noted below.
  models/            wire format. Domain types + agent I/O envelopes. Pure schema.
  ids/               New(prefix) -> "mnt_01J..."
  hashing/           ContentHash(text, source) -> sha256 hex
  redact/            redact.PII(text) -> text, runs before every prompt
  llm/               ChatJSON, Embed. OPENAI_BASE_URL only, no provider config.
  db/                pgxpool wrappers + migrations runner
  stats/             ZScore, ComputeBaseline, HourBucket, DayBucket
  anakin/            Client, cache, budget, fixture replay. The only HTTP to Anakin.
  cluster/           TF-IDF + agglomerative, hand-rolled. No sklearn in Go.
  prompts/           //go:embed *.md, loaded by name, never inlined
  obs/               OpenTelemetry self-instrumentation (Nasiko auto-injects Python only)
  a2a/               a2asrv wiring: one artifact envelope, one health path, one main() shape
agents/
  bp-onboarder/      package main, one directory per agent, nine of them
  bp-collector/
  bp-enricher/
  bp-clusterer/
  bp-sov/
  bp-detector/
  bp-responder/
  bp-briefer/
  bp-orchestrator/
web/                 Next.js dashboard (B6)
bff/                 Fastify backend-for-frontend (B6)
```

`internal/` is a Go visibility keyword, not a convention: nothing outside this
module can import these packages. That is intended. The agents are the only
consumers and they live in the same module.

Go 1.27. A2A via `github.com/a2aproject/a2a-go/v2` v2.5.0, server side in its
`a2asrv` package, `a2a.Version == "1.0"`.

---

## 1. Transport

Every agent is an A2A server. A request arrives as a message whose single part
is JSON matching the agent's **Input** below. Every agent replies with exactly
one artifact:

- `mimeType`: `application/json`
- `name`: the agent's output type name, e.g. `MentionBatch`
- body: `json.Marshal(output)`

`internal/a2a` provides the helper that builds that artifact, so no agent hand
-rolls the envelope and every agent's `main()` is the same twenty lines.

**Errors do not fail the task, where there is a partial answer to give.** An
agent whose output is a batch (`MentionBatch`, `EnrichmentBatch`, `TopicSet`,
`AlertSet`) returns that struct with `Errors` populated and partial data in
place, so the orchestrator can degrade instead of dying. A dead source must not
kill a run.

The four agents whose output is a bare domain type (`BrandProfile`,
`ShareOfVoice`, `ReplyDraft`, `DailyBrief`) have no `Errors` field and are not
getting one. They produce a single indivisible answer, and half a reply draft
or a brief with a hole in it is worse than an absent one: the orchestrator can
see a failed step and say so, but it cannot see that a returned draft was
built from nothing. Those four return an A2A task failure instead. The
orchestrator records it in `RunRecord.DegradedReason` and carries on.

Malformed input is always an A2A task failure, for all nine.

Datetimes are `time.Time`, marshalled RFC 3339 with an explicit UTC offset. IDs
are strings, generated as `ids.New(prefix)` giving `"<prefix>_<ulid>"`.

**Zero values are the hazard.** Go has no field defaults. Construct every
domain type through its `New*` constructor and call `Validate()` before
persisting. A decoded struct bypasses both, which is why `Validate` exists
separately from the constructor. See the package header in `models.go`.

---

## 2. Per-agent signatures

Types written below in Go are declared in `internal/models/agentio.go`. Field
names are the `json` tags, not the Go identifiers.

### bp-onboarder

```go
type OnboardInput struct {
    BrandID     string   `json:"brand_id"`
    Name        string   `json:"name"`
    Website     string   `json:"website,omitempty"`
    Competitors []string `json:"competitors"`
    MaxPages    int      `json:"max_pages"` // 0 means 25; the agent applies the default
}
// Output: models.BrandProfile
```

Crawls the website with Anakin Map + Crawl, extracts product names, asks the
LLM for a keyword set (one call), returns an **unconfirmed** profile with
`Version = 1`. The human confirms in DronaHQ; confirmation writes
`brand_profiles.confirmed_at` and does not re-run the agent.

Budget: ≤ 30 Anakin credits, ≤ 1 LLM call.

Every `int` input whose Python default was non-zero is documented like
`MaxPages` above: **the agent applies the default, the caller may omit it.**
This is the pattern for all nine agents. Do not add a `*int`.

---

### bp-collector

```go
type CollectInput struct {
    Profile     models.BrandProfile `json:"profile"`
    Source      models.Source       `json:"source"`
    WindowStart time.Time           `json:"window_start"`
    WindowEnd   time.Time           `json:"window_end"`
    MaxCredits  int                 `json:"max_credits"`
    RunID       string              `json:"run_id"`
}

type MentionBatch struct {
    Mentions    []models.Mention `json:"mentions"`
    CreditsUsed int              `json:"credits_used"`
    CacheHits   int              `json:"cache_hits"`
    Source      models.Source    `json:"source"`
    Truncated   bool             `json:"truncated"`
    Errors      []string         `json:"errors"`
}
```

One call handles **one source**. The orchestrator fans out across sources and
is the only component that decides how many run. The collector never exceeds
`MaxCredits`; if it would, it stops early and sets `Truncated: true`.

Dedupe is the collector's job: the returned slice has unique `ContentHash`
within it, and the collector skips hashes already in `mentions` for that brand.

No LLM calls. Ever. This keeps collection cost linear in credits alone.

---

### bp-enricher

```go
type EnrichInput struct {
    Mentions  []models.Mention    `json:"mentions"`
    Profile   models.BrandProfile `json:"profile"`
    BatchSize int                 `json:"batch_size"` // 0 means 50
}

type EnrichmentBatch struct {
    Enrichments []models.Enrichment `json:"enrichments"`
    TokensUsed  int                 `json:"tokens_used"`
    CostPaise   float64             `json:"cost_paise"`
    CacheHits   int                 `json:"cache_hits"`
    Errors      []string            `json:"errors"`
}
```

Batches ≤ 50 mentions per LLM call with a JSON-schema-constrained prompt.
Results are cached by `Mention.ContentHash`, so a re-run over the same corpus
costs zero tokens. Order of `Enrichments` is not guaranteed; join on
`MentionID`.

`IsAboutBrand == false` mentions still get returned. The clusterer and SOV
agents filter them out; the collector does not delete them.

`redact.PII` runs on every `Mention.Text` before it reaches the prompt. This is
not optional and it is not the model's job.

---

### bp-clusterer

```go
type ClusterInput struct {
    Enriched           []models.EnrichedMention `json:"enriched"`
    BrandID            string                   `json:"brand_id"`
    WindowStart        time.Time                `json:"window_start"`
    WindowEnd          time.Time                `json:"window_end"`
    PriorWindowCounts  map[string]int           `json:"prior_window_counts"`
    MinClusterSize     int                      `json:"min_cluster_size"` // 0 means 3
}

type TopicSet struct {
    Topics      []models.Topic `json:"topics"`
    Unclustered []string       `json:"unclustered"`
    TokensUsed  int            `json:"tokens_used"`
    CostPaise   float64        `json:"cost_paise"`
    Errors      []string       `json:"errors"`
}
```

Clusters with `internal/cluster`: TF-IDF vectors, cosine distance, average
-linkage agglomerative merging to a distance cutoff. **Hand-rolled, roughly 240
–320 lines.** Go has no sklearn and this is the one place the language choice
costs real work. It is priced in the ADR and it is B3's Task 1.

Then spends **one** LLM call per cluster to produce `Label` and `Summary`.
`Unclustered` holds mention ids that fell into noise.

`Trend` is `size / PriorWindowCounts[label]`, defaulting to **1.0** when there
is no prior window. `Topic.Validate()` rejects `Trend <= 0`, because a zero
trend means "nobody set the field", not "this topic vanished".

Embedding through `llm.Embed` is optional and currently unused: TF-IDF is
enough at the corpus size we ship (the Phase 2 gate is 300 mentions) and costs
no tokens. Do not add embeddings to hit a quality bar nobody has measured, and
do not add a `BP_VECTORISER` switch: one implementation ships, and an unused
branch is a branch nobody tests.

---

### bp-sov

```go
type SOVInput struct {
    Enriched    []models.EnrichedMention `json:"enriched"`
    Profile     models.BrandProfile      `json:"profile"`
    WindowStart time.Time                `json:"window_start"`
    WindowEnd   time.Time                `json:"window_end"`
}
// Output: models.ShareOfVoice
```

Pure counting, no LLM, no network. A mention counts for the brand when
`IsAboutBrand` is true and `AboutCompetitor` is empty; it counts for a
competitor when `AboutCompetitor` matches a name in `Profile.Competitors`.
Mentions matching neither are excluded from the denominator.

---

### bp-detector

```go
type DetectInput struct {
    BrandID  string                   `json:"brand_id"`
    Enriched []models.EnrichedMention `json:"enriched"`
    Baseline BaselineStats            `json:"baseline"`
    Now      time.Time                `json:"now"`
}

type BaselineStats struct {
    PerSourceHourlyMean map[models.Source]float64 `json:"per_source_hourly_mean"`
    PerSourceHourlyStd  map[models.Source]float64 `json:"per_source_hourly_std"`
    NegativeShareMean   float64                   `json:"negative_share_mean"`
    NegativeShareStd    float64                   `json:"negative_share_std"`
    MeanRating          *float64                  `json:"mean_rating"` // nil = no review source in the window
    Days                int                       `json:"days"`
}

type AlertSet struct {
    Alerts         []models.Alert `json:"alerts"`
    RulesEvaluated int            `json:"rules_evaluated"`
    Errors         []string       `json:"errors"`
}
```

`MeanRating` is a pointer because "no review source ran" and "average rating
was 0.0" are different facts, and the review-bomb rule fires on the second.
`models.Mention.Rating` is a pointer for exactly the same reason and carries
exactly the same nil-deref hazard. The other pointers in the contract
(`RunRecord.FinishedAt`, `RespondInput.Alert`, `RespondInput.Mention`) mean
"not yet" and "not this one" rather than "absent versus zero".

**Ruling on which mean `review_bomb` compares:** the observed hour's mean,
computed from the non-nil `Mention.Rating` values in that hour's review-source
mentions. `BaselineStats.MeanRating` is context for the brief and is not the
comparison operand. If fewer than five of the hour's mentions carry a rating,
the rule does not fire, because the threshold is five reviews and a mention
without a rating is not a review.

**Deterministic. No LLM call in this agent, at all.** The five rules:

| Rule | Fires when | Severity |
|---|---|---|
| `spike` | hourly volume z-score ≥ 3.0 on any source | z≥3 medium, z≥5 high |
| `crisis` | z ≥ 3.0 **and** negative share ≥ 0.6 in the same hour | high, critical at ≥0.8 |
| `review_bomb` | ≥ 5 reviews in 1h on a review source with mean rating ≤ 2.0 | high |
| `influencer_mention` | any mention with `author_followers` ≥ 50 000 | medium, high if negative |
| `competitor_move` | competitor mention volume z ≥ 3.0 | low |

Every alert carries one `AlertEvidence` per term in its rule, with the real
number and the real threshold. `DedupeKey` is `"<kind>:<source>:<hour_bucket>"`
so a sustained crisis produces one alert per hour, not one per run.

Baselines come from `stats.ComputeBaseline(ctx, brandID, 14, now)`, which B1
owns. `ZScore` returns 0.0 when `std == 0` rather than dividing by zero: that
guard is the difference between a working detector and a demo that panics on a
quiet brand.

---

### bp-responder

```go
type RespondInput struct {
    Alert   *models.Alert       `json:"alert"`
    Mention *models.Mention     `json:"mention"`
    Profile models.BrandProfile `json:"profile"`
    Channel models.Channel      `json:"channel"`
}
// Output: models.ReplyDraft
```

Exactly one of `Alert` / `Mention` is non-nil; both nil or both set is an input
error. One LLM call. The prompt injects `Profile.Voice.DoNotSay` plus the
global guardrail list from `prompts.Guardrails` and the output is checked
against both before returning; a draft containing a banned phrase is
regenerated once, then returned with the phrase removed and a note in `Tone`.

`requires_human_approval` is always `true` on the wire and **there is no such
field on the struct**. `ReplyDraft.MarshalJSON` emits it unconditionally, so no
serialisation of this type can claim otherwise, and it does not round-trip:
`Unmarshal` has nowhere to put it and drops it. That asymmetry is deliberate.
Do not add the field back.

Nothing in this repo posts to any platform. There is no posting code path to
audit and none is to be added.

---

### bp-briefer

```go
type BriefInput struct {
    BrandID     string               `json:"brand_id"`
    Profile     models.BrandProfile  `json:"profile"`
    Period      string               `json:"period"` // "daily" | "weekly"
    PeriodStart time.Time            `json:"period_start"`
    PeriodEnd   time.Time            `json:"period_end"`
    Topics      []models.Topic       `json:"topics"`
    Alerts      []models.Alert       `json:"alerts"`
    SOV         models.ShareOfVoice  `json:"sov"`
    Numbers     models.BriefNumbers  `json:"numbers"`
}
// Output: models.DailyBrief
```

One LLM call producing `Headline`, `SuggestedActions` and the narrative.
`Markdown` is the full brief; `WhatsappShort` is ≤ `models.WhatsappShortLimit`
(600) characters with no tables and no markdown links. The briefer does not
query the database: the orchestrator hands it everything.

---

### bp-orchestrator

```go
type RunInput struct {
    BrandID     string         `json:"brand_id"`
    Trigger     models.RunKind `json:"trigger"`
    WindowHours int            `json:"window_hours"` // 0 means 24
    Force       bool           `json:"force"`
}
// Output: models.RunRecord
```

The pipeline:

1. Load `BrandProfile` (latest confirmed version).
2. Compute `TimeBucket`. If a `runs` row exists for
   `(brand_id, kind, time_bucket)` and `Force` is false, return it unchanged.
3. Choose sources: `Profile.Sources`, ranked by yesterday's `source_yield`
   (mentions per credit, descending), truncated to the flow-guard fan-out cap.
   Dropped sources go in `SourcesSkipped` with `DegradedReason` set.
4. Split the remaining credit budget across chosen sources, call bp-collector
   once per source **in parallel**, persist mentions.
5. bp-enricher over new mentions, persist.
6. bp-clusterer, bp-sov, bp-detector in parallel.
7. bp-responder for each alert with severity ≥ high.
8. bp-briefer for the day.
9. Write the `runs` row with real credits, tokens and cost.

Fan-out from a single orchestrator call must stay ≤ 8 and depth ≤ 3. Step 4 is
the only wide fan-out; steps 6 and 7 are capped at 3 and 5 respectively. Nasiko
flow guards enforce this and **fail closed**, so exceeding the cap is a dropped
call, not a slow one.

Parallel steps use `golang.org/x/sync/errgroup` with a bounded `SetLimit`. A
goroutine per source with no limit is how the fan-out cap gets breached by
accident.

---

## 3. Shared library surface (B1 owns, everyone imports)

```go
import (
    "brandpulse/internal/models"   // every type in §2, plus the domain types
    "brandpulse/internal/ids"      // ids.New("mnt") -> "mnt_01J..."
    "brandpulse/internal/hashing"  // hashing.ContentHash(text, source) -> string
    "brandpulse/internal/redact"   // redact.PII(text) -> string
    "brandpulse/internal/llm"      // llm.ChatJSON, llm.Embed
    "brandpulse/internal/db"       // db.Pool, db.Migrate
    "brandpulse/internal/stats"    // stats.ComputeBaseline, stats.ZScore, stats.HourBucket
    "brandpulse/internal/anakin"   // anakin.Client, anakin.ErrBudgetExceeded
    "brandpulse/internal/prompts"  // prompts.Load(name), prompts.Guardrails
    "brandpulse/internal/obs"      // obs.Setup(serviceName) -> shutdown func
    "brandpulse/internal/a2a"      // a2a.Serve(card, handler), a2a.JSONArtifact
)
```

**`internal/cluster` is the one exception and it is B3's, not B1's.** It is the
only package under `internal/` that nobody but `bp-clusterer` imports, it is
240 to 320 lines of algorithm rather than shared plumbing, and B3 builds it as
their Task 1 before the agent that uses it. B1 does not create it and does not
stub it, because a `panic("not implemented")` TF-IDF that compiles is worse
than a missing package: it fails at run time on stage instead of at build time
on day one.

Signatures B1 must publish exactly as written, because five tracks compile
against them before B1's implementation exists:

```go
func ids.New(prefix string) string
func hashing.ContentHash(text string, source models.Source) string
func redact.PII(text string) string

func llm.ChatJSON(ctx context.Context, prompt string, schema any, opt llm.Opt) (json.RawMessage, llm.Usage, error)
func llm.Embed(ctx context.Context, texts []string, model string) ([][]float32, error)
type llm.Usage struct { PromptTokens, CompletionTokens int; CostPaise float64 }
type llm.Opt   struct { Model string; MaxTokens int }   // both optional; zero Model
                                                        // defers to the router's
                                                        // per-agent llm-config

func stats.ZScore(value, mean, std float64) float64
func stats.ComputeBaseline(ctx context.Context, brandID string, days int, now time.Time) (models.BaselineStats, error)
func stats.HourBucket(t time.Time) string
func stats.DayBucket(t time.Time) string

func prompts.Load(name string) string
var  prompts.Guardrails []string

func obs.Setup(serviceName string) (shutdown func(context.Context) error, err error)

// Serve wires a handler to the A2A SDK. Every agent's main() ends in this.
// The type parameters are inferred from h, so the call site stays exactly
// a2a.Serve(card, handler) and no agent writes a type argument.
type a2a.Handler[In, Out any] interface { Handle(context.Context, In) (Out, error) }
func a2a.Serve[In, Out any](card a2a.Card, h a2a.Handler[In, Out]) error
func a2a.JSONArtifact(name string, v any) (a2a.Artifact, error)

// LoadCard reads an agent's AgentCard.json and rejects a protocolVersion other
// than "1.0". No agent builds a Card by hand, which is what lets internal/a2a
// reconcile a2a.Card with the SDK's own card type without changing a main().
func a2a.LoadCard(path string) (a2a.Card, error)

// Call reaches a peer agent through the Nasiko proxy. bp-orchestrator is the
// only caller. No agent constructs a peer URL: the proxy address and the
// routing header are Nasiko's, and hardcoding one breaks on redeploy.
func a2a.Call(ctx context.Context, agent string, in any, out any) error
```

B1 also adds the two domain constructors the existing `models.go` lacks and
which B4 needs, `models.NewAlert` and `models.NewDailyBrief`, matching the
shape of the six already there. The repo rule is "construct through the `New*`
constructor", and right now that rule is unsatisfiable for exactly the two
types B4 produces.

```go
func models.NewAlert(id, brandID string, kind AlertKind, severity Severity,
    dedupeKey string, createdAt time.Time) Alert
func models.NewDailyBrief(brandID string, periodStart, periodEnd time.Time) DailyBrief
```

Neither reads the clock, for the same reason `DetectInput` carries `Now`: a
replayed run must produce the alert it produced live. `NewAlert` leaves
`Evidence` empty and `Validate` rejects it that way, because an alert that
cannot show the numbers that fired it is the one thing this detector may not
emit.

### anakin.Client

An **interface**, not a struct, because every agent's test needs a stub and
nobody should need a live key or a fixture directory to compile:

```go
type Client interface {
    Search(ctx context.Context, query string, opt SearchOpt) (json.RawMessage, error)
    Wire(ctx context.Context, platform, query string, opt WireOpt) (json.RawMessage, error)
    Scrape(ctx context.Context, url string, opt ScrapeOpt) (json.RawMessage, error)
    Map(ctx context.Context, url string, opt MapOpt) (json.RawMessage, error)
    Crawl(ctx context.Context, url string, opt CrawlOpt) (json.RawMessage, error)

    // Stats reports what this client has spent since it was built. The
    // collector reads it once at the end to fill MentionBatch.CreditsUsed
    // and CacheHits. Per-action Wire costs vary, so the collector must not
    // keep its own cost table: that would be a second source of truth for
    // the number the whole budget story rests on.
    Stats() Stats
}

type Stats struct {
    CreditsUsed int `json:"credits_used"`
    CacheHits   int `json:"cache_hits"`
}
```

**How `CollectInput.MaxCredits` reaches the client.** The adapter signature
takes no budget, on purpose. The collector builds a client bound to its own
ceiling and hands that to the adapter:

```go
func anakin.NewHTTPClient(cfg Config) (Client, error)  // cfg.MaxCredits is the ceiling
```

Once the ceiling is reached every method returns `ErrBudgetExceeded` without
calling out. The adapter returns what it has, and the collector treats
`errors.Is(err, anakin.ErrBudgetExceeded)` as `Truncated: true` rather than as
a failure. That way "stop at the ceiling" is enforced in the one place that
knows the real per-action cost, and no adapter can spend past it by
forgetting a check.

**Fetch errors carry a reason.** Anakin distinguishes a blocked page, a CAPTCHA
wall, a timeout and a DNS failure, and `RunRecord.DegradedReason` is supposed to
say which. Plain `errors.New` throws that away:

```go
type FetchError struct {
    Reason FetchReason // blocked | captcha | timeout | dns | tls | ratelimited | upstream
    Source models.Source
    Err    error
}
func (e *FetchError) Error() string { ... }
func (e *FetchError) Unwrap() error { return e.Err }
```

`json.RawMessage` rather than a typed struct on purpose: **the literal Anakin
response field names are not yet known.** B2 Task 1 resolves them with one live
catalogue read costing ≤ 10 credits, and the typed structs land then, inside
the source adapters. Guessing them now would put a wrong shape in a frozen
contract.

The concrete `anakin.HTTPClient` implementing it: checks `fetch_cache` first,
decrements the credit budget, returns `ErrBudgetExceeded` rather than
overspending, records the raw payload, and in `BP_FIXTURE_MODE=replay` reads
from `fixtures/` instead of the network.

**The three modes, set by `BP_FIXTURE_MODE`:**

| Mode | Behaviour |
|---|---|
| `replay` | Never touches the network. Reads `fixtures/<source>/<query_hash>.json`. **CI default.** |
| `record` | Calls Anakin for real, writes the response to `fixtures/`. Used once, by B2, on the demo brand. |
| `live` | Calls Anakin, caches to Postgres, writes no fixtures. Stage demo only. |

CI runs with zero credits and zero API keys. Any test that needs the network is
a broken test. Go has no `pytest-socket`; B1 enforces it instead by making
`HTTPClient` take an `*http.Client` and CI injecting one whose `Transport`
returns an error on every call. That client is published here so there is one
implementation rather than the same eight lines in six worktrees:

```go
func anakin.NoNetwork() *http.Client
```

Every track's tests inject it. It is the whole of the no-network guarantee, so
a test that does not use it is a test that can spend a credit.

---

## 3b. Where the models and the schema deliberately differ

`internal/models/models.go` is the wire format and `db/migrations/001_init.sql`
is storage. They are close but not identical, on purpose. These divergences are
intended: do not "fix" one and do not add a field to close one.

| Model | Table | Difference | Why |
|---|---|---|---|
| `Alert.SampleMentions` | `alerts.sample_mention_ids` | objects on the wire, ids in storage | DronaHQ renders the alert without a second fetch; Postgres does not duplicate mention rows. |
| `Topic.MentionIDs`, `.TopExamples` | `topic_mentions` join table | slice on the wire, rows in storage | The join is queryable; the artifact is self-contained. |
| `BrandProfile.Name`, `.Website` | on `brands`, not `brand_profiles` | denormalised onto the wire | A profile artifact must be readable alone. Profiles are versioned, brand identity is not. |
| every model | `created_at`, `collected_at`, `confirmed_at` | storage-only | Set by Postgres defaults. No agent writes them. |
| `ReplyDraft` requires_human_approval | no column, no struct field | constant `true`, emitted by `MarshalJSON` | It is an invariant, not state. Storing it would imply it could be false. |

Everything else must match, and `internal/models/parity_test.go` (B1 Task 0)
proves it by parsing `001_init.sql` and comparing column names against `json`
tags via reflection. That test is the schema guard. It fails the build when a
struct field and a column drift apart.

Two fields exist in the models **only** because the schema requires them, and
both are idempotency keys. Omit either and the insert fails against a `NOT NULL`
unique constraint:

- `Alert.DedupeKey` backs `alerts UNIQUE (brand_id, dedupe_key)`.
- `RunRecord.TimeBucket` backs `runs UNIQUE (brand_id, kind, time_bucket)`.

---

## 4. Naming rules that prevent merge pain

- Agent directories: `agents/bp-<name>/`. Entry file: `main.go`, `package main`.
- Every agent's handler type: `<Name>Handler`, e.g. `CollectorHandler`, with
  one method `Handle(ctx context.Context, in <Name>Input) (<Output>, error)`.
  `internal/a2a` adapts that to the A2A executor interface, so no agent
  implements the SDK's interface directly.
- Source adapters: package `internal/anakin/sources`, one file per source named
  `<source_value>.go`. Each file declares one **unexported** adapter named for
  its source, and `registry.go` maps the enum to it:

  ```go
  type Adapter func(ctx context.Context, c anakin.Client, p models.BrandProfile,
      start, end time.Time) ([]models.Mention, error)

  // Registry is the only dispatch point. A source with no entry is a config
  // error the orchestrator reports, not a nil call at collection time.
  var Registry = map[models.Source]Adapter{
      models.SourceReddit:  fetchReddit,   // reddit.go
      models.SourceYoutube: fetchYoutube,  // youtube.go
      // ...one line per shipped source
  }

  func Get(s models.Source) (Adapter, bool)
  ```

  One exported `Fetch` per file would be a redeclaration error: these are files
  in one package, not modules. The registry is written out by hand rather than
  populated from `init()` so the shipped source list is greppable in one place.
- Fixtures: `fixtures/<source>/<query_hash>.json`, plus
  `fixtures/labelled/mentions.jsonl` for the eval set.
- Prompts: `internal/prompts/<agent>.md`, `//go:embed`ed, loaded by name, never
  inlined in `main.go`.
- Tests: `_test.go` beside the code, standard library `testing`. No testify, no
  ginkgo. Table-driven where there is more than one case.
- Every exported identifier gets a doc comment starting with its own name. `go
  vet` is part of the definition of done for every track.

**Who owns `AgentCard.json` and `Dockerfile`.** The track that writes the agent
owns both, and commits them with the agent. B5 owns neither file and edits
neither: B5 owns the shared template they are copied from
(`docs/research/nasiko.md` §4), the `nasiko.yaml`, and the deploy. If a card is
wrong, B5 files it in `HACKATHON_NOTES.md` under "Open blockers" and the owning
track fixes it. Nine agents in five worktrees each editing eighteen files that
a sixth worktree also edits is a guaranteed conflict on merge day, and the
Dockerfile is identical across all nine but for the binary name, so there is
nothing for B5 to tune per agent anyway.

## 5. Environment variables

| Var | Who sets it | Used by |
|---|---|---|
| `ANAKIN_API_KEY` | Nasiko secret | B1 anakin client |
| `OPENAI_BASE_URL` | injected by Nasiko LLM router | B1 llm client |
| `OPENAI_API_KEY` | injected by Nasiko | B1 llm client |
| `DATABASE_URL` | Nasiko secret / compose | B1 db |
| `BP_FIXTURE_MODE` | `replay` in CI, `live` on stage | B1 anakin client |
| `BP_MODEL_SMALL` | compose default | enricher, responder, briefer |
| `BP_MODEL_EMBED` | compose default | clusterer (unused today) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | injected by Nasiko | B1 `internal/obs` |
| `PORT` | injected by Nasiko | every agent |

No agent reads an API key directly. Everything goes through `internal/`.

## 6. What the language change did not change

The wire format. Field names, enum values, the five detector rules, the
divergence table and the budget rules are identical to the Python freeze. A
DronaHQ binding or a fixture recorded against the old contract still works.
What changed is the implementation language and therefore the import surface in
§3, the naming rules in §4, and the addition of `internal/obs` and
`internal/cluster` as work items that pydantic and sklearn used to cover.
