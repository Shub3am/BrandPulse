# Track B2 — Source adapters, collector, onboarder, fixtures

**Branch:** `track/b2-collect`
**Owns:** `shared/bp_core/sources/`, `agents/bp-collector/`, `agents/bp-onboarder/`, `fixtures/`
**Blocked by:** B1's Phase 1 gate (a working `AnakinClient` in `replay` mode)
**Blocks:** B3's eval work needs your fixtures.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md),
[research/anakin.md](../research/anakin.md) and
[SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md) before writing a line.

You hold the only irreversible action in this project: spending the 300 free
Anakin credits. Read Task 2 before you spend anything.

---

## What this track owns

Turning Anakin responses into `Mention` objects, and producing the fixture
corpus the whole team tests against.

## What it must not know about

Sentiment, topics, alerts, briefs. The collector emits raw `Mention`s and
nothing else. **No LLM call in this track's collector**, ever — that keeps
collection cost linear in credits alone and is why the unit economics work.
bp-onboarder is allowed exactly one LLM call.

---

## Task 1 — Read the live Wire schemas before writing any adapter

**Files:** `docs/research/wire-schemas.md`

The largest unknown in this project is the literal field names Wire returns.
The catalogue pages render prose, not JSON. Guessing a field name produces an
empty dashboard at demo time and you will not find out until then.

- [ ] With the real key, call `GET /v1/wire/catalog/reddit`,
      `/youtube`, `/amazon`. Catalogue reads are free or near-free.
- [ ] Execute **one** `rt_search` and **one** `yt_search` (2 + 1 credits) and
      paste the literal response JSON.
- [ ] Write `docs/research/wire-schemas.md` with the real parameter and
      response schemas per action. This file is the adapter spec.
- [ ] Commit it before writing an adapter. Other tracks read it too.

Budget for this task: **≤ 10 credits.**

## Task 2 — The Play Store probe

**Files:** `docs/research/playstore-probe.md`

- [ ] Three `POST /v1/url-scraper/scrape` calls with `useBrowser: true` on the
      demo brand's Play Store listing. 3 credits.
- [ ] Pass = review text, star rating and date are all extractable from the
      returned markdown. Anything less is a fail.
- [ ] Write the verdict and a sample of the returned markdown to the file.
- [ ] **On fail:** remove `playstore` from the demo brand's `sources`, tell B5
      to say "six sources" in the README and pitch, and note it in
      `HACKATHON_NOTES.md`. Do not retry more than twice. Do not fake it.
- [ ] Commit.

## Task 3 — Pick the demo brand

**Files:** `demo/brand.json`, `HACKATHON_NOTES.md` entry

- [ ] Pick a real Indian D2C brand with genuine public discourse across Reddit,
      YouTube and Amazon, plus two real competitors. Skincare or audio are the
      safest bets for chatter volume.
- [ ] Requirements: an active subreddit presence or frequent Reddit mentions;
      YouTube review videos with comment threads; Amazon products with ≥ 50
      reviews; an App Store app id if you want `appstore` to work.
- [ ] Record brand name, website, the two competitors, Amazon ASINs, the App
      Store app id and the Play package name in `demo/brand.json`.
- [ ] Sanity-check volume with one free-ish Reddit search before committing to
      the choice. A quiet brand makes the whole demo look broken.
- [ ] Commit.

## Task 4 — Source adapters

**Files:** `shared/bp_core/sources/{reddit,youtube,amazon,news,web,appstore,playstore,x}.py`,
`shared/bp_core/sources/__init__.py`, `shared/tests/sources/test_<source>.py`

Every adapter is one function with the same signature, per CONTRACTS §4:

```python
async def fetch(client: AnakinClient, profile: BrandProfile,
                window_start: datetime, window_end: datetime,
                budget: CreditBudget) -> list[Mention]
```

- [ ] Write each adapter **against a hand-written fixture first**, derived from
      the real schemas in Task 1. Test passes, then the adapter meets live data.
      We do not discover a parsing bug with live credits.
- [ ] Each adapter: build queries from `profile.keywords` and
      `profile.hashtags`, call the right Anakin surface, map response fields to
      `Mention`, compute `content_hash` via `bp_core.hashing`, set
      `matched_keyword`, and filter to the time window.
- [ ] `posted_at` must be timezone-aware UTC. Every source formats dates
      differently; normalise in the adapter, never downstream.
- [ ] `rating` is set only by `amazon`, `appstore`, `playstore`.
- [ ] `author_followers` is set where the source gives it, 0 otherwise. The
      `influencer_mention` rule depends on it, so do not invent values.
- [ ] `__init__.py` exposes `ADAPTERS: dict[Source, Callable]` so the collector
      dispatches without a nine-branch if-chain.
- [ ] Commit per adapter, not all at once.

**Negative-keyword filtering happens here.** A mention whose text matches a
term in `profile.negative_keywords` is dropped in the adapter, before it costs
an enrichment token.

## Task 5 — bp-collector

**Files:** `agents/bp-collector/main.py`, `agents/bp-collector/AgentCard.json`,
`agents/bp-collector/Dockerfile`, `agents/bp-collector/tests/test_collector.py`

- [ ] `CollectorExecutor` per the A2A pattern in
      [research/nasiko.md](../research/nasiko.md) §3. Use `bp_core.a2a.emit_json_artifact`.
- [ ] Input and `MentionBatch` output exactly per CONTRACTS §2.
- [ ] Dispatch to `ADAPTERS[source]`. One source per call — the orchestrator
      owns fan-out, you do not.
- [ ] Enforce `max_credits`: stop early and set `truncated: true` rather than
      overspending. Test this.
- [ ] Dedupe within the batch by `content_hash`, and skip hashes already in
      `mentions` for that brand.
- [ ] An adapter that raises must be caught, recorded in `errors`, and return
      a partial batch. One dead source must not kill a run.
- [ ] `AgentCard.json` with `protocolVersion: "1.0"` and `llm_provider: null`.
- [ ] Tests run in `replay` mode with sockets disabled.
- [ ] Commit.

## Task 6 — bp-onboarder

**Files:** `agents/bp-onboarder/main.py`, `AgentCard.json`, `Dockerfile`,
`shared/bp_core/prompts/onboarder.md`, `agents/bp-onboarder/tests/`

- [ ] Map the site first (`POST /v1/map`, 1 credit), filter the link list to
      product/about/shop pages, **then** crawl only those with `maxPages ≤ 20`.
      Blanket-crawling costs 1 credit per page and wastes the budget.
- [ ] One LLM call: site text + competitor names in, a keyword set out.
      Constrain the output to the `BrandProfile` fields with a JSON schema.
- [ ] Prompt lives in `prompts/onboarder.md`, not inline.
- [ ] Ask for 15–20 keywords: brand name variants and common misspellings,
      product names, category terms, and a `negative_keywords` list for
      name collisions. Misspellings matter — "mama earth" vs "Mamaearth".
- [ ] Returns `version: 1`, unconfirmed. Confirmation is DronaHQ's job.
- [ ] Budget: ≤ 30 credits, ≤ 1 LLM call. Test the cap.
- [ ] Commit.

## Task 7 — The recording session

**Files:** `fixtures/**`, `demo/record_fixtures.py`

This is the irreversible step. Everything above must be committed and green
first.

- [ ] `demo/record_fixtures.py` that **prints a dry-run credit estimate and
      exits** unless `--confirm` is passed. Per the budget table in
      [research/anakin.md](../research/anakin.md) §6, the target is ~136 credits.
- [ ] Run the dry run. If the estimate exceeds 200, cut scope before spending.
- [ ] Run with `BP_FIXTURE_MODE=record --confirm`. Record demo brand plus both
      competitors across every source that passed its probe.
- [ ] Verify: ≥ 300 mentions, ≥ 7 sources (or 6 if the Play probe failed),
      spread over ≥ 14 days so the detector has a real baseline.
- [ ] Print actual credits spent and paste it into the PR. Not an estimate.
- [ ] Commit fixtures. They are the team's shared test corpus from here on.

**If a source comes back thin, do not top it up by re-running blindly.** Check
the query first. A bad keyword burns credits at the same rate as a good one.

## Task 8 — Hand the labelling set to B3

- [ ] Write `fixtures/labelled/mentions.jsonl` with 300 mentions sampled
      across sources and sentiment, one JSON object per line, `label` field
      left empty. B3 fills the labels; you provide the sample.
- [ ] Sample deliberately: roughly balanced across sources, and deliberately
      including brand-name collisions so `is_about_brand` gets tested.
- [ ] Post in `HACKATHON_NOTES.md` that fixtures are ready. B3 is waiting.
- [ ] Commit.

---

## Definition of done

```bash
BP_FIXTURE_MODE=replay pytest agents/bp-collector agents/bp-onboarder shared/tests/sources -q \
  --disable-socket --allow-unix-socket
```

Green, output pasted in the PR, plus the real credit spend from Task 7 and the
fixture counts per source.
