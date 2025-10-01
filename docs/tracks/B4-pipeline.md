# Track B4 — Detector, responder, briefer, orchestrator, demo

**Branch:** `track/b4-pipeline`
**Owns:** `agents/bp-detector/`, `agents/bp-responder/`, `agents/bp-briefer/`,
`agents/bp-orchestrator/`, `demo/`
**Blocked by:** B1's Phase 1 gate. Everything else you build against the
contract, not against B2 and B3's code.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md) and
[research/nasiko.md](../research/nasiko.md) §5 and §7.

You own the two moments the demo is built around: the crisis alert firing, and
the flow guard degrading gracefully. Both have to work on stage.

---

## What this track owns

The pipeline and the judgement layer. Alerting, drafting, briefing,
orchestration, and the demo scripts.

## What it must not know about

How mentions are fetched or classified. You consume `EnrichedMention`,
`Topic`, `ShareOfVoice` per the contract and never import a source adapter.

---

## Task 1 — bp-detector

Start here. It has no LLM and no network, so it is fully testable today, and it
is the centre of the demo.

**Files:** `agents/bp-detector/main.py`, `AgentCard.json`, `Dockerfile`,
`agents/bp-detector/tests/test_rules.py`

- [ ] Implement the five rules exactly as tabled in CONTRACTS §2. Thresholds
      are constants at the top of the module, named, not scattered literals.
- [ ] **No LLM call in this agent.** This is a hard rule from PLAN.md. An
      alert that cannot explain itself statistically is worth nothing to a
      founder at 11pm, and "our AI decided" is not a defensible demo answer.
- [ ] Every alert carries one `AlertEvidence` per term in its rule, with the
      real observed value and the real threshold. A crisis alert shows the
      z-score, the negative share, and the count — the numbers from the brief's
      demo line "volume z=4.1, negative share 78%, 3 high-follower authors".
- [ ] `why` is a plain English sentence naming the rule, not a summary of the
      mentions.
- [ ] `dedupe_key = f"{kind}:{source}:{hour_bucket}"` so a sustained crisis
      produces one alert per hour, not one per run. Test a double-run.
- [ ] Use `bp_core.stats.zscore`, which returns 0.0 when std is 0. A brand with
      a flat baseline must not divide by zero — that is the single most likely
      way this agent breaks on stage.
- [ ] Test each rule in isolation with a hand-built `BaselineStats` and a
      hand-built mention list. Five rules, at least ten tests, including the
      near-miss cases that must **not** fire.
- [ ] Commit.

## Task 2 — bp-responder

**Files:** `agents/bp-responder/main.py`, `AgentCard.json`, `Dockerfile`,
`shared/bp_core/prompts/responder.md`, `agents/bp-responder/tests/`

- [ ] One LLM call. Input is exactly one of `alert` or `mention`, validated.
- [ ] The prompt injects `profile.voice` plus `profile.voice.do_not_say` plus
      `bp_core.prompts.GUARDRAILS`.
- [ ] **Check the output against both lists before returning.** A draft
      containing a banned phrase regenerates once, then returns with the phrase
      removed and a note in `tone`. Test that a model output containing "we
      will refund everyone" does not escape.
- [ ] `requires_human_approval` is always `true`. There is no code path in this
      repo that posts to any platform — do not add one, not even behind a flag.
      A flag is a thing a judge will ask about.
- [ ] Templates differ per `intent`: a complaint gets acknowledgement plus a
      route to support, a comparison gets a factual correction, a crisis gets a
      holding statement that commits to nothing.
- [ ] Commit.

## Task 3 — bp-briefer

**Files:** `agents/bp-briefer/main.py`, `AgentCard.json`, `Dockerfile`,
`shared/bp_core/prompts/briefer.md`, `agents/bp-briefer/tests/`

- [ ] One LLM call producing `headline` and `suggested_actions`; the numbers
      are formatted deterministically, not generated. A model must never be
      asked to restate a percentage — it will get one wrong eventually and it
      will be on stage.
- [ ] `markdown` is the full brief. `whatsapp_short` is ≤ 600 characters, no
      tables, no markdown links. Test the length cap.
- [ ] The briefer does **not** query the database. The orchestrator hands it
      everything. Keeps it pure and testable.
- [ ] Weekly uses the same path with `period="weekly"`; B5 renders the PDF.
- [ ] Commit.

## Task 4 — bp-orchestrator

**Files:** `agents/bp-orchestrator/main.py`, `AgentCard.json`, `Dockerfile`,
`agents/bp-orchestrator/tests/test_orchestration.py`

The nine-step pipeline is specified in CONTRACTS §2. Three things matter more
than the rest:

- [ ] **Idempotency.** A `runs` row for `(brand_id, kind, time_bucket)` means
      return it unchanged unless `force`. The DB has a unique constraint;
      handle the conflict, do not race on it. Test a double-trigger.
- [ ] **Flow-guard-aware source selection.** Rank `profile.sources` by
      yesterday's `source_yield` (mentions per credit, descending), truncate to
      the fan-out cap of 8, record dropped sources in `sources_skipped` with
      `degraded_reason`. This is a demo beat: a keyword-heavy brand hits the
      cap and degrades visibly instead of failing. Test it with 10 sources and
      a cap of 8.
- [ ] **Partial failure is not failure.** One source erroring means
      `status="partial"` with the error recorded, not a dead run. Test with one
      adapter raising.
- [ ] Fan out step 4 in parallel with `asyncio.gather`, bounded by the cap.
      Steps 6 and 7 are capped at 3 and 5 concurrent.
- [ ] Peer calls go through `bp_core.a2a.call_agent` only. Do not write an
      agent URL anywhere in this file — see research/nasiko.md §7 for why that
      indirection exists.
- [ ] Write the `runs` row with **real** credits, tokens and cost summed from
      the child artifacts.
- [ ] Commit.

## Task 5 — demo/inject_crisis.py

**Files:** `demo/inject_crisis.py`

- [ ] Insert 40 synthetic negative mentions about a plausible product issue,
      timestamped across the previous 10 minutes, spread over 2–3 sources,
      three of them with `author_followers > 50000` so the influencer rule
      fires alongside the crisis rule.
- [ ] They must trip `crisis` on the real thresholds. Do not special-case the
      detector — if the injection does not fire the real rule, the injection is
      wrong, not the rule.
- [ ] **Mark them.** Every injected mention carries `raw: {"synthetic": true}`
      and the dashboard shows a visible "injected demo data" badge. We are not
      going to imply scraped data is synthetic or the reverse.
- [ ] `--dry-run` prints what would be inserted. `--cleanup` removes them.
- [ ] Commit.

## Task 6 — demo/replay.py and run_demo.sh

**Files:** `demo/replay.py`, `demo/run_demo.sh`, `demo/seed.py`

- [ ] `seed.py` loads `demo/brand.json` and B2's fixtures into Postgres.
- [ ] `replay.py` shifts fixture timestamps so the corpus ends "now", giving a
      realistic 14-day baseline and a live-looking stream. Deterministic: same
      input, same output, every time.
- [ ] `run_demo.sh` runs the full 2-minute flow end to end: seed, replay,
      orchestrate, inject crisis, show the alert, draft a reply, print the
      brief. Each step announces itself.
- [ ] It must be re-runnable. `--reset` drops and reseeds. You will run this
      thirty times before the pitch.
- [ ] The one live Anakin call is a **separate, optional** step
      (`run_demo.sh --live`), so stage wifi cannot break the demo.
- [ ] Commit.

---

## Definition of done

```bash
BP_FIXTURE_MODE=replay pytest agents/bp-detector agents/bp-responder \
  agents/bp-briefer agents/bp-orchestrator -q --disable-socket --allow-unix-socket
./demo/run_demo.sh --reset
```

Green, plus `run_demo.sh` producing a crisis alert with real evidence numbers.
Paste the alert JSON into the PR.
