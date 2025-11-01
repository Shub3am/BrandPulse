# Track B4: detector, responder, briefer, orchestrator, demo

**Branch:** `track/b4-pipeline`
**Owns:** `agents/bp-detector/`, `agents/bp-responder/`, `agents/bp-briefer/`,
`agents/bp-orchestrator/`, `demo/`
**Blocked by:** B1's Phase 1 gate, and specifically B1's signature commit. You
compile against `panic("not implemented")` bodies from minute twenty. Everything
else you build against the contract, not against B2 and B3's code.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md),
[internal/models/CLAUDE.md](../../internal/models/CLAUDE.md) and
[research/nasiko.md](../research/nasiko.md) §5 and §7.

You own the two moments the demo is built around: the crisis alert firing, and
the flow guard degrading gracefully. Both have to work on stage.

---

## What this track owns

The pipeline and the judgement layer. Alerting, drafting, briefing,
orchestration, and the demo commands.

## What it must not know about

How mentions are fetched or classified. You consume `models.EnrichedMention`,
`models.Topic` and `models.ShareOfVoice` per the contract and never import a
source adapter, never import `internal/anakin/sources`, and never import
another agent's package.

---

## Shape every agent in this track shares

Four directories, four identical skeletons. Get this right once in Task 1 and
copy it three times.

- `package main` in `agents/bp-<name>/main.go`.
- Handler type `DetectorHandler`, `ResponderHandler`, `BrieferHandler`,
  `OrchestratorHandler`, each with exactly one method:
  `Handle(ctx context.Context, in models.XInput) (models.XOutput, error)`.
- `main()` is the same twenty lines: `obs.Setup`, build the handler, then
  `a2a.Serve(card, handler)`. `internal/a2a` adapts your handler to the SDK's
  executor interface, so **you never implement an SDK interface directly** and
  you never hand-build an artifact envelope.
- Errors do not fail the task. Populate the output's `Errors` slice and return
  partial data. Malformed input is the one exception.
- Zero values are the hazard. Construct through the `New*` constructor where one
  exists and call `Validate()` before you persist or return. Go has no field
  defaults, and a decoded struct bypasses the constructor entirely.
- Tests are `_test.go` beside the code, standard library `testing`,
  table-driven. No testify, no ginkgo, no pytest.

---

## Task 1: bp-detector

Start here. It has no LLM, no network and no database, so it is fully testable
on day one, and it is the centre of the demo. It should end up with the densest
test coverage in the repo: a rule is a pure function over a fixture.

**Files:** `agents/bp-detector/main.go`, `agents/bp-detector/rules.go`,
`agents/bp-detector/rules_test.go`, `agents/bp-detector/AgentCard.json`,
`agents/bp-detector/Dockerfile`

- [ ] `DetectorHandler.Handle(ctx, in models.DetectInput) (models.AlertSet, error)`,
      exactly per CONTRACTS §2. `main.go` holds the handler and `a2a.Serve`, and
      nothing else. All five rules live in `rules.go`.
- [ ] Implement the five rules exactly as tabled in CONTRACTS §2: `spike`,
      `crisis`, `review_bomb`, `influencer_mention`, `competitor_move`.
      Thresholds are named `const` declarations at the top of `rules.go`, not
      literals scattered through the bodies. `RulesEvaluated` is 5, always, even
      when nothing fires.
- [ ] Each rule is a pure function with the same signature, something like
      `func ruleSpike(in models.DetectInput) []models.Alert`, so `Handle` is a
      loop over a slice of them and a test calls one rule with no server, no
      context plumbing and no mocks.
- [ ] **No LLM call in this agent, at all.** Hard rule from PLAN.md and the repo
      CLAUDE.md. `internal/llm` is not imported here, and a test greps for it if
      you want belt and braces. An alert that cannot explain itself
      statistically is worth nothing to a founder at 11pm, and "our AI decided"
      is not a defensible demo answer.
- [ ] Every alert carries one `models.AlertEvidence` per term in its rule, with
      the real observed `Value` and the real `Threshold` it was compared
      against. A crisis alert shows the z-score, the negative share and the
      count: the numbers from the brief's demo line "volume z=4.1, negative
      share 78%, 3 high-follower authors". `Alert.Validate()` rejects an empty
      `Evidence` slice, so this is enforced, not merely requested.
- [ ] `Why` is a plain English sentence naming the rule and its numbers, not a
      summary of the mentions.
- [ ] `DedupeKey` is `fmt.Sprintf("%s:%s:%s", kind, source, stats.HourBucket(t))`
      so a sustained crisis produces one alert per hour, not one per run. It
      backs `alerts UNIQUE (brand_id, dedupe_key)` and the column is `NOT NULL`.
      Test a double-run over the same input: same keys, no new alerts.
- [ ] **`Baseline.MeanRating` is a `*float64` and it is the most likely nil
      deref in this track.** `nil` means no review source ran in the baseline
      window; `0.0` means the average rating was genuinely zero. The
      `review_bomb` rule must fire on the second and must not panic on the
      first. Write that as two separate table rows, one with `nil`, one with
      `ptr(0.0)`, and assert the alert count on each.
- [ ] `models.Mention.Rating` is also a `*float64`. Skip the nils when you
      average an hour's reviews; do not treat a missing rating as zero, which
      would read as the worst possible review and fire the rule on a quiet
      source.
- [ ] Use `stats.ZScore`, which returns `0.0` when `std == 0`. Go does not raise
      on division by zero here: you get `+Inf` or `NaN`, which compares
      `>= 3.0` as true for `+Inf` and false for `NaN`. A missing guard is
      therefore a silent wrong alert, not a crash. Write the quiet-brand case:
      a source with `std == 0` and a non-zero count must produce no `spike`.
- [ ] `models.Source.IsReviewSource()` gates `review_bomb`. Do not hand-roll the
      source list a second time.
- [ ] Table-driven tests, one table per rule, with the near-miss rows that must
      **not** fire: z = 2.9, negative share 0.59, four reviews, 49 999
      followers. Five rules, at least ten cases, plus the two nil cases and the
      zero-std case above.
- [ ] `go vet ./agents/bp-detector/...` clean. Commit.

## Task 2: bp-responder

**Files:** `agents/bp-responder/main.go`, `agents/bp-responder/guardrails.go`,
`agents/bp-responder/guardrails_test.go`, `agents/bp-responder/AgentCard.json`,
`agents/bp-responder/Dockerfile`, `internal/prompts/responder.md`

- [ ] `ResponderHandler.Handle(ctx, in models.RespondInput) (models.ReplyDraft, error)`.
      One LLM call through `llm.ChatJSON`, never a provider SDK.
- [ ] Exactly one of `in.Alert` and `in.Mention` is non-nil. Both nil or both
      set is an input error, and it is the one case in this agent that returns a
      real `error` rather than a draft with `Errors` filled. Test both bad
      shapes.
- [ ] The prompt is `internal/prompts/responder.md`, `//go:embed`ed and loaded
      with `prompts.Load("responder")`. No prompt string literal in `main.go`.
- [ ] The prompt injects `in.Profile.Voice`, `in.Profile.Voice.DoNotSay` and the
      global `prompts.Guardrails` list.
- [ ] **Check the output against both lists before returning.** A draft
      containing a banned phrase regenerates **once**; if the second draft still
      hits, strip the phrase and note it in `Tone`. That is the contract's
      wording and it is three states, not two: clean, regenerated clean,
      stripped. Test that a stubbed model returning "we will refund everyone"
      never escapes the handler with that string in `Text`.
- [ ] The guardrail check is a pure function over `(text string, banned
      []string) []string` in `guardrails.go`, so its test needs no LLM stub at
      all. The regenerate-once loop is the only part that needs one.
- [ ] `redact.PII` runs on every mention text before it reaches the prompt.
- [ ] **`ReplyDraft` has no `RequiresHumanApproval` field and you are not adding
      one.** `MarshalJSON` emits `requires_human_approval: true`
      unconditionally. The consequence you will actually trip over: the type
      does **not** round-trip, so a test that marshals a draft and unmarshals it
      back has no field to assert on. Assert on the marshalled **bytes**
      instead, for example that `json.Marshal(draft)` contains
      `"requires_human_approval":true`. Do not write
      `Unmarshal(...); if got.RequiresHumanApproval` because it will not
      compile, and do not "fix" that by adding the field.
- [ ] Nothing in this repo posts to any platform. There is no posting code path
      to audit and none is to be added, not even behind a flag. A flag is a
      thing a judge will ask about.
- [ ] Build the draft with `models.NewReplyDraft(ids.New("drf"), channel)` so
      `Status` and `DoNotSay` are not zero values, then `Validate()` before
      returning. `Validate` rejects a draft referencing neither an alert nor a
      mention.
- [ ] Templates differ per `Intent`: a complaint gets acknowledgement plus a
      route to support, a comparison gets a factual correction, a crisis gets a
      holding statement that commits to nothing.
- [ ] `go vet` clean. Commit.

## Task 3: bp-briefer

**Files:** `agents/bp-briefer/main.go`, `agents/bp-briefer/render.go`,
`agents/bp-briefer/render_test.go`, `agents/bp-briefer/AgentCard.json`,
`agents/bp-briefer/Dockerfile`, `internal/prompts/briefer.md`

- [ ] `BrieferHandler.Handle(ctx, in models.BriefInput) (models.DailyBrief, error)`.
- [ ] One LLM call producing `Headline`, `SuggestedActions` and the narrative.
      The numbers in `models.BriefNumbers` are formatted deterministically in
      `render.go`, never generated. A model must never be asked to restate a
      percentage: it will get one wrong eventually and it will be on stage.
- [ ] `Markdown` is the full brief. `WhatsappShort` is at most
      `models.WhatsappShortLimit` (600) characters, with no tables and no
      markdown links. Use the constant, do not retype 600.
- [ ] `DailyBrief.Validate()` already fails over the limit. Do not rely on it as
      the only check: truncate at a sentence boundary in `render.go` and test
      the boundary with an input long enough to force it. A brief that fails
      validation on stage is a brief nobody reads.
- [ ] The WhatsApp renderer is a pure function over `(models.DailyBrief) string`
      so its length and formatting tests need no LLM stub. Test: no `|`, no
      `](`, length ≤ `models.WhatsappShortLimit`.
- [ ] The briefer does **not** query the database. `internal/db` is not imported
      here. The orchestrator hands it every topic, alert, SOV figure and number
      it needs, which is what makes it pure and testable.
- [ ] Weekly uses the same path with `Period == "weekly"`; B5 renders the PDF.
- [ ] `go vet` clean. Commit.

## Task 4: bp-orchestrator

**Files:** `agents/bp-orchestrator/main.go`,
`agents/bp-orchestrator/pipeline.go`, `agents/bp-orchestrator/sources.go`,
`agents/bp-orchestrator/pipeline_test.go`,
`agents/bp-orchestrator/sources_test.go`,
`agents/bp-orchestrator/AgentCard.json`, `agents/bp-orchestrator/Dockerfile`

`OrchestratorHandler.Handle(ctx, in models.RunInput) (models.RunRecord, error)`.
The nine-step pipeline is specified in CONTRACTS §2. Four things matter more
than the rest.

- [ ] **Idempotency.** Compute `TimeBucket` with `stats.HourBucket` or
      `stats.DayBucket` per trigger. If a `runs` row exists for
      `(brand_id, kind, time_bucket)` and `in.Force` is false, return that row
      unchanged and run nothing. The DB has the unique constraint; handle the
      conflict, do not race on it. Test a double-trigger returns the same
      `RunRecord.ID` and does not call a single peer the second time.
      `in.WindowHours == 0` means 24, and the agent applies that default. There
      is no `*int` in this contract.
- [ ] **Flow-guard-aware source selection**, in `sources.go` as a pure function
      over `(profile models.BrandProfile, yields map[models.Source]float64, cap int)`
      returning chosen and skipped slices. Rank `Profile.Sources` by yesterday's
      `source_yield` (mentions per credit, descending), truncate to the fan-out
      cap of 8, record dropped sources in `RunRecord.SourcesSkipped` with
      `DegradedReason` set. This is a demo beat: a keyword-heavy brand hits the
      cap and degrades visibly instead of failing. Test it with 10 sources and a
      cap of 8, and assert the two dropped are the two lowest-yield ones.
- [ ] **Parallelism is bounded, always.** Step 4 fans out to bp-collector with
      `golang.org/x/sync/errgroup` and an explicit `g.SetLimit(n)`. Steps 6 and
      7 use their own groups capped at 3 and 5. A bare `go func()` per source is
      exactly how the cap gets breached by accident, so there is no bare
      goroutine anywhere in this agent. Nasiko flow guards cap fan-out at 8 and
      depth at 3 and they **fail closed**: exceeding the cap is a dropped call
      that returns an error, not a call that runs slowly. See
      [research/nasiko.md](../research/nasiko.md) §5.
- [ ] `errgroup.WithContext` cancels siblings on the first error, which is the
      wrong behaviour here. A dead source must not kill a run. Collect
      per-source errors into a mutex-guarded slice, return `nil` from the
      goroutine, and let the group finish. Test with one stub collector
      returning an error and assert `Status == models.RunStatusPartial` with the
      error recorded and the other sources' mentions still present.
- [ ] Build the record with
      `models.NewRunRecord(ids.New("run"), brandID, kind, bucket, startedAt)` so
      the slices and `RunStatusRunning` are set, then `Validate()` before the
      insert. An empty `TimeBucket` is a `NOT NULL` insert failure, not a
      missing nicety.
- [ ] Peer calls go through `internal/a2a` only. **Do not write an agent URL
      anywhere in this file.** See [research/nasiko.md](../research/nasiko.md)
      §7 for why that indirection exists and what about it is still unverified.
- [ ] Write the `runs` row with **real** credits, tokens and cost summed from
      the child artifacts' `CreditsUsed`, `TokensUsed` and `CostPaise`. `eval/`
      reads these, so an estimate here becomes a lie in the pitch.
- [ ] `go vet` clean, and run the orchestrator tests with `-race`. This is the
      only concurrent code in the track and it is the only place a data race can
      hide. Commit.

## Task 5: demo crisis injection

**Files:** `demo/cmd/injectcrisis/main.go`,
`demo/cmd/injectcrisis/main_test.go`

- [ ] Insert 40 synthetic negative mentions about a plausible product issue,
      timestamped across the previous 10 minutes, spread over 2 to 3 sources,
      three of them with `AuthorFollowers > 50000` so the `influencer_mention`
      rule fires alongside `crisis`.
- [ ] They must trip `crisis` on the real thresholds. Do not special-case the
      detector: if the injection does not fire the real rule, the injection is
      wrong, not the rule. The test for this task builds the mentions, runs them
      through `DetectorHandler.Handle` with a realistic baseline, and asserts a
      `crisis` alert comes out. That test needs no database.
- [ ] **Mark them.** Every injected mention carries `Raw: map[string]any{"synthetic": true}`
      and the dashboard shows a visible "injected demo data" badge. We are not
      going to imply scraped data is synthetic or the reverse.
- [ ] Set `Lang` explicitly on every injected mention. It has no default in Go
      and `Mention.Validate()` rejects an empty one, which is exactly the trap
      this data would otherwise fall into.
- [ ] Flags: `-dry-run` prints what would be inserted and touches nothing,
      `-cleanup` removes them by the `synthetic` marker. CI runs the `-dry-run`
      path.
- [ ] Commit.

## Task 6: demo seed, replay and run_demo.sh

**Files:** `demo/cmd/seed/main.go`, `demo/cmd/replay/main.go`,
`demo/cmd/replay/main_test.go`, `demo/run_demo.sh`, `demo/brand.json`

- [ ] `go run ./demo/cmd/seed` loads `demo/brand.json` and B2's fixtures into
      Postgres. Enable 10 sources on the demo brand so the fan-out cap of 8
      fires on cue.
- [ ] `go run ./demo/cmd/replay` shifts fixture timestamps so the corpus ends
      "now", giving a realistic 14-day baseline and a live-looking stream.
      **Deterministic:** same fixtures in, same offsets out, every time. The
      shift is a pure function over `(postedAt, corpusEnd, now time.Time) time.Time`
      and that function is what the test pins. A demo that differs between runs
      cannot be rehearsed.
- [ ] `demo/run_demo.sh` runs the full 2-minute flow end to end: seed, replay,
      orchestrate, inject crisis, show the alert, draft a reply, print the
      brief. Each step announces itself and the script is `set -euo pipefail`.
- [ ] It must be re-runnable. `--reset` drops and reseeds. You will run this
      thirty times before the pitch.
- [ ] The one live Anakin call is a **separate, optional** step
      (`run_demo.sh --live`), so stage wifi cannot break the demo. Default is
      `BP_FIXTURE_MODE=replay` and no network.
- [ ] Commit.

---

## Definition of done

```bash
docker compose up -d --no-recreate postgres   # shared with every other track
go build ./... && go vet ./agents/... ./demo/...
BP_FIXTURE_MODE=replay go test ./agents/bp-detector/... ./agents/bp-responder/... \
  ./agents/bp-briefer/... ./agents/bp-orchestrator/... ./demo/... -v
BP_FIXTURE_MODE=replay go test ./agents/bp-orchestrator/... -race
./demo/run_demo.sh --reset
```

Green, with zero network calls and no `ANAKIN_API_KEY` set, plus `run_demo.sh`
producing a crisis alert with real evidence numbers.

**Paste the full `go test -v` output and the crisis alert JSON into the PR.**
Not a summary of them, the output. A claim of "tests pass" without the lines
underneath it does not close this track.
