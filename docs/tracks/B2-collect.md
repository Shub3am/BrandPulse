# Track B2: Source adapters, collector, onboarder, fixtures

**Branch:** `track/b2-collect`
**Owns:** `internal/anakin/sources/`, `agents/bp-collector/`, `agents/bp-onboarder/`, `fixtures/`
**Blocked by:** B1's signature commit to compile, B1's Phase 1 gate to run a
`replay` test against a real `anakin.Client`
**Blocks:** B3's eval work needs your fixtures.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md),
[research/anakin.md](../research/anakin.md) and
[SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md) before writing a line. Where the
brief and those two research documents disagree, the research wins.

You hold the only irreversible action in this project: spending the 300 free
Anakin credits. Read Task 7 before you spend anything.

---

## What this track owns

Turning Anakin responses into `models.Mention` values, and producing the fixture
corpus the whole team tests against.

`anakin.Client` returns `json.RawMessage`, not typed structs. The typed response
shapes live **only** inside your adapter files, unexported, one set per source.
That is deliberate: the literal Anakin response field names are not known yet,
Task 1 resolves them, and keeping the shape in one file per source means a
further surprise is a one-file fix rather than a contract change.

## What it must not know about

Sentiment, topics, alerts, briefs. The collector emits raw `Mention`s and
nothing else. **No LLM call in bp-collector**, ever. That keeps collection cost
linear in credits alone and is why the unit economics work. bp-onboarder is
allowed exactly one LLM call.

Nothing in this track implements the A2A SDK interface. You write a handler type
with one `Handle` method and `internal/a2a` adapts it.

---

## Task 1: Read the live catalogue before writing any adapter

**Files:** `docs/research/wire-schemas.md`

The largest unknown in this project is the literal field names Wire returns. The
catalogue pages render prose, not JSON. Guessing a field name produces an empty
dashboard at demo time and you will not find out until then. Everything
downstream of this task, including every typed struct you write in Task 4,
depends on the output of this one.

- [ ] With the real key, call `GET /v1/wire/catalog/reddit`, `/youtube`,
      `/amazon`. Catalogue reads are free or near-free.
- [ ] Execute **one** `rt_search` and **one** `yt_search` (2 + 1 credits) and
      paste the literal response JSON.
- [ ] Execute **one** Search API call (3 credits) and settle the UNVERIFIED
      locale and freshness parameters in
      [research/anakin.md](../research/anakin.md) §2. If they do not exist, the
      `news` and `web` adapters filter on `date` client-side and pay for results
      they discard. Write down which it is.
- [ ] Write `docs/research/wire-schemas.md` with the real parameter and response
      schemas per action. This file is the adapter spec and the source of every
      `json` tag you will write in Task 4.
- [ ] Commit it before writing an adapter. Other tracks read it too.

**Budget for this task: ≤ 10 credits.** Print the running total as you go. If
you are at 10 and a schema is still unresolved, stop and note it in
`HACKATHON_NOTES.md` rather than spending into Task 7's budget.

## Task 2: The Play Store probe

**Files:** `docs/research/playstore-probe.md`

- [ ] Three `POST /v1/url-scraper/scrape` calls with `useBrowser: true` on the
      demo brand's Play Store listing. 3 credits. `useBrowser` carries no
      surcharge, which is what makes this cheap enough to try.
- [ ] Pass = review text, star rating and date are all extractable from the
      returned markdown. Anything less is a fail.
- [ ] Write the verdict and a sample of the returned markdown to the file.
- [ ] **On fail:** remove `playstore` from the demo brand's `sources`, tell B5
      to say "six sources" in the README and pitch, and note it in
      `HACKATHON_NOTES.md`. Do not retry more than twice. Do not fake it.
- [ ] Commit.

## Task 3: Pick the demo brand

**Files:** `demo/brand.json`, `HACKATHON_NOTES.md` entry

- [ ] Pick a real Indian D2C brand with genuine public discourse across Reddit,
      YouTube and Amazon, plus two real competitors. Skincare or audio are the
      safest bets for chatter volume.
- [ ] Requirements: an active subreddit presence or frequent Reddit mentions;
      YouTube review videos with comment threads; Amazon products with ≥ 50
      reviews; an App Store app id if you want `appstore` to work.
- [ ] Record brand name, website and the two competitors, and put the Amazon
      ASIN, the App Store app id and the Play package name in
      `BrandProfile.SourceHandles`, keyed by `Source` value. That map exists for
      exactly the ids keyword search cannot supply.
- [ ] Sanity-check volume with one free-ish Reddit search before committing to
      the choice. A quiet brand makes the whole demo look broken.
- [ ] Commit.

## Task 4: Source adapters

**Files:** `internal/anakin/sources/{reddit,youtube,amazon,news,web,appstore,playstore,x}/`,
one `<source>.go` and one `<source>_test.go` in each, plus
`internal/anakin/sources/registry.go`

Every adapter is one exported function with the same signature, per CONTRACTS §4:

```go
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error)
```

> **Contract snag, resolve it before you write code.** CONTRACTS §4 puts the
> adapters at flat files `internal/anakin/sources/<source_value>.go` with one
> exported `Fetch` each. Eight files in one Go package cannot each declare
> `Fetch`; that is a redeclaration error. The signature is right, the path
> granularity is not. Post the correction in `HACKATHON_NOTES.md` and let B1
> land it on `main` per the freeze rule. Build against a directory per source in
> the meantime, because it is the only layout that compiles and it changes no
> signature.

- [ ] Write each adapter **against a hand-written fixture first**, derived from
      the real schemas in Task 1. Test passes, then the adapter meets live data.
      We do not discover a parsing bug with live credits.
- [ ] Each adapter: build queries from `p.Keywords` and `p.Hashtags`, call the
      right `anakin.Client` method, unmarshal the `json.RawMessage` into its own
      unexported response structs, map fields to `models.Mention`, set
      `MatchedKeyword`, and filter to `[start, end)`.
- [ ] `ID` comes from `ids.New("mnt")`. `ContentHash` comes from
      `hashing.ContentHash(text, source)` and nothing else computes it.
- [ ] **`Lang` has no default and `Mention.Validate()` rejects an empty one.**
      pydantic used to fill `"en"`; Go fills `""`, which silently drops the
      mention from every language-filtered query and fails validation at the
      insert. Every adapter sets `Lang` explicitly on every mention. This is the
      single most likely thing in this track to bite you.
- [ ] `PostedAt` must be a UTC `time.Time`. Every source formats dates
      differently; normalise in the adapter, never downstream. A zero
      `PostedAt` fails `Validate()` because baselines are built from it.
- [ ] `Rating` is a `*float64` and is set only by `amazon`, `appstore` and
      `playstore`. Nil, not zero: a 0.0 reads as the worst possible review and
      the `review_bomb` rule fires on it.
- [ ] `AuthorFollowers` is set where the source gives it, 0 otherwise. The
      `influencer_mention` rule depends on it, so do not invent values.
- [ ] Call `Validate()` on every mention before returning it, and drop plus
      record the ones that fail rather than returning an invalid batch.
- [ ] `registry.go` exposes `Adapters map[models.Source]FetchFunc` so the
      collector dispatches without an eight-branch switch.
- [ ] `go vet ./internal/anakin/sources/...` clean. Every exported identifier
      gets a doc comment starting with its own name.
- [ ] Commit per adapter, not all at once.

**Negative-keyword filtering happens here.** A mention whose text matches a term
in `p.NegativeKeywords` is dropped in the adapter, before it costs an enrichment
token.

**Per-source mechanisms are fixed by [SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md),
not by you.** `reddit`, `youtube` and `amazon` go through Wire. `news` and `web`
go through Search, and `web` chains Search into Scrape for bodies because Search
returns a snippet and not a page. `appstore` reads Apple's public review RSS
through Scrape. `playstore` ships only if Task 2 passed. `x` is Search scoped to
`site:x.com`, has no follower counts, cannot feed `influencer_mention`, and
ships only if those snippets prove usable. `instagram` and `flipkart` stay in
the enum and get no adapter file.

## Task 5: bp-collector

**Files:** `agents/bp-collector/main.go`, `agents/bp-collector/collector.go`,
`agents/bp-collector/collector_test.go`, `agents/bp-collector/AgentCard.json`,
`agents/bp-collector/Dockerfile`

- [ ] `package main`. Handler type `CollectorHandler` with exactly one method:
      `Handle(ctx context.Context, in models.CollectInput) (models.MentionBatch, error)`.
      `main()` is `obs.Setup` plus `a2a.Serve(card, handler)` and nothing else.
      You do not implement the SDK's executor interface; `internal/a2a` does.
- [ ] `models.CollectInput` and `models.MentionBatch` are declared by B1 in
      `internal/models/agentio.go`. Do not redeclare them, do not shadow them
      with a local struct.
- [ ] Dispatch through `sources.Adapters[in.Source]`. One source per call, the
      orchestrator owns fan-out, you do not. An unknown source is an entry in
      `Errors` and an empty batch, not a panic.
- [ ] Enforce `in.MaxCredits`: stop early and set `Truncated: true` rather than
      overspending. `anakin.ErrBudgetExceeded` is the signal, caught with
      `errors.Is`, and it is a truncation and not a failure. Test this.
- [ ] Dedupe within the batch by `ContentHash`, and skip hashes already in
      `mentions` for that brand. The returned slice has unique hashes within it.
- [ ] An adapter that returns an error must be caught, appended to `Errors`, and
      the partial batch returned. One dead source must not kill a run. Per
      CONTRACTS §1, errors populate the output struct, they do not fail the task.
- [ ] Populate `CreditsUsed` and `CacheHits` from the client's counters. The
      published `anakin.Client` interface has no accessor for them: ask B1 for
      one in `HACKATHON_NOTES.md` rather than pricing calls yourself, because
      Wire costs vary per action and the cost table belongs in `internal/anakin`.
- [ ] `AgentCard.json` with `protocolVersion: "1.0"`, non-empty `skills` and
      `llm_provider: null`.
- [ ] Tests are stdlib `testing`, table-driven, in `collector_test.go` beside
      the code. Run in `BP_FIXTURE_MODE=replay` with the network blocked by
      injecting an `*http.Client` whose `Transport` returns an error on every
      call. A stub `anakin.Client` covers the adapter dispatch cases; the
      interface exists so your test needs no key and no fixture directory.
- [ ] Commit.

## Task 6: bp-onboarder

**Files:** `agents/bp-onboarder/main.go`, `agents/bp-onboarder/onboarder.go`,
`agents/bp-onboarder/onboarder_test.go`, `agents/bp-onboarder/AgentCard.json`,
`agents/bp-onboarder/Dockerfile`, `internal/prompts/onboarder.md`

- [ ] Handler type `OnboarderHandler`, one method
      `Handle(ctx context.Context, in models.OnboardInput) (models.BrandProfile, error)`.
- [ ] Map the site first (`Client.Map`, 1 credit per job), filter the returned
      link list to product, about and shop pages, **then** `Client.Crawl` only
      those. Blanket-crawling costs 1 credit per page and wastes the budget.
- [ ] `in.MaxPages == 0` means 25 and **the agent applies that default**, per
      CONTRACTS §2. Do not change the type to a `*int`. Note that
      [research/anakin.md](../research/anakin.md) §5 recommends capping at 20 and
      the recording budget table assumes 20; pass 20 explicitly in Task 7 rather
      than editing the contract default.
- [ ] One LLM call: site text plus competitor names in, a keyword set out,
      constrained to the `BrandProfile` fields with a JSON schema through
      `llm.ChatJSON`. `redact.PII` runs on the crawled text before it reaches
      the prompt.
- [ ] Prompt lives in `internal/prompts/onboarder.md`, `//go:embed`ed and loaded
      with `prompts.Load("onboarder")`. Never inlined in `main.go`.
- [ ] Ask for 15 to 20 keywords: brand name variants and common misspellings,
      product names, category terms, and a `NegativeKeywords` list for name
      collisions. Misspellings matter: "mama earth" against "Mamaearth".
- [ ] Build the result with `models.NewBrandProfile`, which sets `Version: 1`.
      A decoded or hand-built profile gets `Version: 0`, which means "never
      onboarded" in the DB and fails `Validate()`. Returns unconfirmed;
      confirmation is DronaHQ's job and does not re-run the agent.
- [ ] Budget: ≤ 30 Anakin credits, ≤ 1 LLM call. Test the cap.
- [ ] Commit.

## Task 7: The recording session

**Files:** `fixtures/**`, `demo/record/main.go`

This is the irreversible step: the one and only live spend of the 300 free
credits, in `record` mode, on the demo brand plus two competitors. Everything
above must be committed and green first.

- [ ] `demo/record/main.go`, `package main`, that **prints a dry-run credit
      estimate and exits** unless `-confirm` is passed. Per the budget table in
      [research/anakin.md](../research/anakin.md) §6, the target is ~136 credits.
- [ ] Run the dry run: `go run ./demo/record`. If the estimate exceeds 200, cut
      scope before spending.
- [ ] Run `BP_FIXTURE_MODE=record go run ./demo/record -confirm`. Record the
      demo brand plus both competitors across every source that passed its probe.
      Fixtures land at `fixtures/<source>/<query_hash>.json`.
- [ ] Verify: ≥ 300 mentions, ≥ 7 sources (or 6 if the Play probe failed),
      spread over ≥ 14 days so the detector has a real baseline.
- [ ] Print actual credits spent and paste it into the PR. Not an estimate.
- [ ] Re-run the whole suite in `replay` against the recorded fixtures before
      committing them. A fixture that only the recorder can read is not a
      fixture.
- [ ] Commit fixtures. They are the team's shared test corpus from here on.

**If a source comes back thin, do not top it up by re-running blindly.** Check
the query first. A bad keyword burns credits at the same rate as a good one.

## Task 8: Hand the labelling set to B3

**Files:** `fixtures/labelled/mentions.jsonl`

- [ ] Write `fixtures/labelled/mentions.jsonl` with 300 mentions sampled across
      sources and sentiment, one JSON object per line, `label` field left empty.
      B3 fills the labels; you provide the sample.
- [ ] Sample deliberately: roughly balanced across sources, and deliberately
      including brand-name collisions so `is_about_brand` gets tested.
- [ ] Post in `HACKATHON_NOTES.md` that fixtures are ready. B3 is waiting.
- [ ] Commit.

---

## Definition of done

```bash
docker compose up -d --no-recreate postgres   # shared with every other track
go build ./... && go vet ./...
BP_FIXTURE_MODE=replay go test ./internal/anakin/sources/... \
  ./agents/bp-collector/... ./agents/bp-onboarder/... -v
```

Green, output pasted in the PR, plus the real credit spend from Task 7 and the
fixture counts per source. `go vet` is part of done, not optional. Zero network
calls in that run, and the injected erroring `Transport` is what proves it.
