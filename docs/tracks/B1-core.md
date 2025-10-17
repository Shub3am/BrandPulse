# Track B1: Shared core, database, CI

**Branch:** `track/b1-core`
**Owns:** `internal/` (every package except `cluster/`), `db/`,
`docker-compose.yml`, `.github/workflows/`
**Blocks:** every other track. Land Task 1 fast, land Phase 1 fast, then stay on
call as integrator.

Read [PLAN.md](../../PLAN.md) and [CONTRACTS.md](../CONTRACTS.md) first. You are
the owner of `CONTRACTS.md`; other tracks propose changes, you land them.

`internal/models/` is **already built and committed**. Do not rewrite
`models.go`. Task 0 puts a guard on it and Task 1 adds `agentio.go` beside it.

---

## What this track owns

The only code in the repo that talks to the network or the database, plus the
two packages that make nine `main()` functions identical: `internal/obs` and
`internal/a2a`. Nine agents import from here and do no I/O of their own.

## What it must not know about

Any agent. Nothing under `internal/` imports from `agents/`. If a helper needs
to know which agent is calling it, the helper is in the wrong place.

`internal/cluster/` is **B3's**, not yours. CONTRACTS §3 lists it in the import
surface, which is a doc bug: PLAN's file table and the ADR both put it on B3,
who builds it as their Task 1 before `bp-clusterer`. Do not create the package,
do not stub it, and correct §3 when you land the first contract fix.

---

## Task 0: The schema guard

**Files:** `internal/models/parity_test.go`

Right now nothing guards `models.go`. The Python version of this test exists at
`shared/tests/test_contract_schema_parity.py`; read it, then port it. Same job,
Go reflection instead of pydantic introspection.

- [ ] Parse `db/migrations/001_init.sql` with a `CREATE TABLE (\w+) \((.*?)\n\);`
      regexp, take the first token of every non-blank, non-`--` line in the
      body, and drop the lines opening with `UNIQUE`, `PRIMARY`, `FOREIGN`,
      `CHECK` or `CONSTRAINT`. Those are constraints, not columns.
- [ ] Reflect over each struct's fields, read the `json` tag, strip
      `,omitempty`, skip `-`. That set is the wire format.
- [ ] Table-driven case per pair, with the two exception sets from CONTRACTS
      §3b:

      | Struct | Table | Model only | SQL only |
      |---|---|---|---|
      | `BrandProfile` | `brand_profiles` | `name`, `website` | `confirmed_at`, `created_at` |
      | `Mention` | `mentions` | | `collected_at` |
      | `Enrichment` | `mention_enrichment` | | `created_at` |
      | `Topic` | `topics` | `mention_ids`, `top_examples` | `created_at` |
      | `Alert` | `alerts` | `sample_mentions` | `sample_mention_ids` |
      | `ReplyDraft` | `reply_drafts` | | `created_at` |
      | `RunRecord` | `runs` | | |

- [ ] **`ReplyDraft` diverges from the Python test and that is correct.** There
      the model-only set held `requires_human_approval`; here the struct has no
      such field, so reflection never sees it and the set is empty. Port
      `test_a_reply_draft_always_requires_human_approval` instead as: marshal a
      draft, unmarshal into a `map[string]any`, assert the key is present and
      `true`.
- [ ] **`alerts.created_at` is on both sides.** `Alert` carries `CreatedAt` on
      the wire, so it belongs in neither exception set. CONTRACTS §3b claims
      `created_at` is storage-only for *every* model, which the code
      contradicts. The code wins: narrow that §3b row to name the tables it
      actually covers, in this same commit.
- [ ] Port `test_idempotency_keys_are_required`. Go has no required-field
      metadata, so the equivalent is behavioural: `Alert{...}.Validate()` with
      an empty `DedupeKey` must return an error, same for `RunRecord` with an
      empty `TimeBucket`.
- [ ] No database, no socket, no agent import in this file. It reads one `.sql`
      file off disk and reflects. That is all.
- [ ] `go test ./internal/models/ -v`, paste the output, commit.

## Task 1: The whole import surface, compiling, in twenty minutes

**Files:** `internal/models/agentio.go`, and one file in each of
`internal/{ids,hashing,redact,llm,db,stats,anakin,prompts,obs,a2a}/`

Five tracks are blocked until this exists. Go will not let them stub around a
missing package the way Python let them stub around a missing import, so
nothing else I do matters until this is on `main`.

- [ ] `internal/models/agentio.go`: every `*Input` and batch type from CONTRACTS
      §2, copied verbatim. `OnboardInput`, `CollectInput`, `MentionBatch`,
      `EnrichInput`, `EnrichmentBatch`, `ClusterInput`, `TopicSet`, `SOVInput`,
      `DetectInput`, `BaselineStats`, `AlertSet`, `RespondInput`, `BriefInput`,
      `RunInput`. Field names are the `json` tags; do not tidy one.
- [ ] Every signature in the CONTRACTS §3 block, written exactly as printed,
      with a `panic("not implemented")` body. `anakin.Client` is the interface
      as written plus its five `*Opt` types and `ErrBudgetExceeded`.
- [ ] Not `internal/cluster`. That is B3's.
- [ ] Every stub panics. **No stub returns a plausible zero value**, because a
      track that leans on unimplemented behaviour must fail loudly at the first
      call rather than quietly compute on an empty slice.
- [ ] `go build ./... && go vet ./...` green, commit to `main` directly, push,
      post it in `HACKATHON_NOTES.md`.

**This is the one place in the repo where a stub is correct rather than lazy.**
It is a compile target, not a fake passing test. Nothing here claims to work,
nothing here is green, and every body panics. The moment a stub returns a value
instead of panicking, it stops being a compile target and starts being a lie.

## Task 2: IDs and hashing

**Files:** `internal/ids/ids.go`, `internal/ids/ids_test.go`,
`internal/hashing/hashing.go`, `internal/hashing/hashing_test.go`

- [ ] `go get github.com/oklog/ulid/v2`.
- [ ] `ids.New(prefix string) string` returning `"<prefix>_<ulid>"`. The
      prefixes in use are `brd`, `mnt`, `top`, `alr`, `drf`, `brf`, `run`; list
      them in the package doc comment, not in a constant nobody imports.
- [ ] `hashing.ContentHash(text string, source models.Source) string`:
      lowercase, collapse whitespace runs, strip URLs and `@handles`, then
      sha256 hex. This package importing `models` is fine, `models` imports
      nothing internal so there is no cycle.
- [ ] Two copies of the same tweet with different trailing links must hash
      equal. Write that as the test, table-driven.
- [ ] Test, run it, paste, commit.

## Task 3: PII redaction

**Files:** `internal/redact/redact.go`, `internal/redact/redact_test.go`

- [ ] `redact.PII(text string) string` replacing emails with `[email]` and phone
      numbers with `[phone]`. Indian formats matter: `+91 98765 43210`,
      `9876543210`, `+91-98765-43210`.
- [ ] Must **not** redact public handles (`@brand`) or URLs. Those are the data.
      Test both directions.
- [ ] The function is `PII`, not `RedactPII`. CONTRACTS §0 says one and §3 says
      the other; §3 is the block five tracks compile against, so §3 wins and I
      fix the §0 line in this commit.
- [ ] Compile the regexps once at package level. This runs on every mention text
      in every run.
- [ ] Test, paste, commit.

## Task 4: Database layer

**Files:** `internal/db/db.go`, `internal/db/migrate.go`,
`internal/db/db_test.go`, `docker-compose.yml`, `db/CLAUDE.md`

- [ ] `docker-compose.yml` with `postgres:16` on 5432, a named volume, a
      healthcheck, and a `DATABASE_URL` default of
      `postgresql://brandpulse:brandpulse@localhost:5432/brandpulse`.
- [ ] `go get github.com/jackc/pgx/v5`.
- [ ] `db.Pool(ctx context.Context) (*pgxpool.Pool, error)`: one pool, built
      lazily from `DATABASE_URL` behind a `sync.Once`, plus `db.Close()`.
- [ ] **Do not re-wrap `Query`, `QueryRow` and `Exec`.** asyncpg needed the
      wrapper; pgxpool does not, and a passthrough layer is one more place a
      context gets dropped. Callers use the pool directly and `pgx.CollectRows`
      for slices.
- [ ] `db.Migrate(ctx context.Context, src fs.FS) error` applying `*.sql` in
      filename order, tracked in a `schema_migrations(filename, applied_at)`
      table so it is idempotent. Default I picked: it takes an `fs.FS` and
      callers pass `os.DirFS("db/migrations")`, because `//go:embed` cannot
      reach outside its own package directory and `db/` stays pure SQL per
      `db/CLAUDE.md`. Nothing runs migrations at agent boot, so nothing needs
      them compiled into a binary.
- [ ] Test against the compose Postgres: migrate twice, assert no error and one
      `schema_migrations` row per migration file. Skip with `t.Skip` when
      `DATABASE_URL` is unset so a laptop without Docker still runs the rest;
      CI always sets it, so the skip never hides a CI failure.
- [ ] `db/CLAUDE.md` still says the schema mirrors `shared/bp_core/models.py`
      and that `bp_core.db` opens the pool. Both are stale. Correct them here,
      same commit, per repo rule 12.
- [ ] Paste the output, commit.

## Task 5: Credit budget

**Files:** `internal/anakin/budget.go`, `internal/anakin/budget_test.go`

CONTRACTS puts the budget inside the anakin package, because `ErrBudgetExceeded`
is an anakin-package sentinel and nothing else spends a credit.

- [ ] `NewBudget(pool *pgxpool.Pool, brandID string, day time.Time, ceiling int)`
      with `Spend(ctx, n int) error` and `Remaining(ctx) (int, error)`.
- [ ] `ErrBudgetExceeded` is a package-level sentinel from `errors.New`, wrapped
      with `%w` at the return site and compared with `errors.Is`. Never compared
      by string.
- [ ] Backed by `runs.credits_used` summed for the brand-day so it survives a
      process restart, with the ceiling defaulting to
      `brands.daily_credit_budget`. Cache the running total in memory, re-read
      it before returning `ErrBudgetExceeded`, so a stale cache cannot block a
      run that actually has headroom.
- [ ] **The cached total is behind a `sync.Mutex`.** The Python version assumed
      one asyncio task; the orchestrator now calls collectors in parallel
      through `errgroup`, and an unguarded counter is how we overspend.
- [ ] Test: ceiling 10, spend 6, spend 6 returns an error satisfying
      `errors.Is(err, ErrBudgetExceeded)`, `Remaining()` is 4. Run it with
      `-race`.
- [ ] Paste, commit.

## Task 6: Anakin client

**Files:** `internal/anakin/anakin.go`, `internal/anakin/http.go`,
`internal/anakin/replay.go`, `internal/anakin/anakin_test.go`,
`fixtures/_canned/*.json`

This is the highest-value file in the repo. One package knows Anakin's wire
shape; everything else consumes `json.RawMessage` and parses it in `sources/`.

- [ ] Read [docs/research/anakin.md](../research/anakin.md) for the real
      endpoints, auth header and response shapes. **Do not guess.** Where the
      research marks something UNVERIFIED, put it behind a small `parse*`
      function with a comment saying it is unverified, so the fix is one
      function.
- [ ] `Client` is the interface from CONTRACTS §3, exactly. Five methods, each
      returning `json.RawMessage`. **Do not add typed response structs here.**
      The literal Anakin field names are still unknown; B2 Task 1 reads them
      live for ≤ 10 credits and the typed shapes land in
      `internal/anakin/sources/`. A guess in a frozen contract is worse than a
      `RawMessage`.
- [ ] `HTTPClient` implements it, constructed as
      `NewHTTPClient(apiKey string, mode Mode, b *Budget, pool *pgxpool.Pool, hc *http.Client)`.
      The `*http.Client` argument is not decoration: it is how CI enforces
      no-network, because Go has no `pytest-socket`.
- [ ] Each method: build a stable `query_hash` from its arguments, compute
      `time_bucket` (hour for Wire and Search, day for Map and Crawl), return
      the cached `fetch_cache` payload on a hit; on a miss `b.Spend(cost)`, HTTP
      call with retry on 429 and 5xx (3 attempts, exponential backoff), write
      the cache.
- [ ] The three modes from CONTRACTS §3. `replay` reads
      `fixtures/<source>/<query_hash>.json` and returns an error **naming the
      missing path** when it is absent. A missing fixture must be obvious, not
      an empty list.
- [ ] `record` writes every response to `fixtures/` after the live call.
- [ ] Export `anakin.NoNetwork() *http.Client`, whose `Transport` returns an
      error on every call. One implementation, injected by every track's tests
      and by CI, instead of five copies of the same eight lines. This is an
      addition to §3; land it in CONTRACTS in the same commit and post it.
- [ ] Tests run in `replay` against `fixtures/_canned/` with `NoNetwork()`
      injected. Assert: a cache hit spends zero credits, an exceeded budget
      returns `ErrBudgetExceeded`, a missing fixture names the path, and every
      one of the five methods returns a canned response. That last one is the
      Phase 1 gate; make it one named test.
- [ ] Paste, commit.

## Task 7: LLM router client

**Files:** `internal/llm/llm.go`, `internal/llm/cost.go`,
`internal/llm/llm_test.go`

- [ ] `go get github.com/openai/openai-go/v3`, pointed at `OPENAI_BASE_URL`. No
      provider configuration anywhere else in the repo.
- [ ] `ChatJSON(ctx, prompt string, schema any, opt Opt) (json.RawMessage, Usage, error)`
      using a strict JSON-schema response format built from `schema`. CONTRACTS
      names `llm.Opt` without its fields, so I define
      `type Opt struct { Model string; MaxTokens int }` and land it in §3.
- [ ] `Usage` is `struct { PromptTokens, CompletionTokens int; CostPaise float64 }`,
      exactly as §3 prints it.
- [ ] The cost table lives in `cost.go`, one map, so `eval/cost` has a single
      source. **Key it on the model the router reports back in the response, not
      the one requested**: per `agents/CLAUDE.md` the Nasiko router ignores the
      `model` field in the request body and per-agent choice is set with
      `nasiko llm-config`. Costing the requested model would report a number we
      never paid.
- [ ] `Embed(ctx, texts []string, model string) ([][]float32, error)`, batched
      at 100. It is on no critical path: the router lists chat models only and
      bp-clusterer uses TF-IDF. Implement the signature, do not polish it.
- [ ] Both functions retry once on a JSON parse failure with a "return only
      valid JSON" nudge, then return the error.
- [ ] Tests inject an `*http.Client` returning a canned completion body. No live
      call in CI, and `OPENAI_API_KEY` is never set there.
- [ ] Paste, commit.

## Task 8: Stats

**Files:** `internal/stats/stats.go`, `internal/stats/baseline.go`,
`internal/stats/stats_test.go`

- [ ] `ZScore(value, mean, std float64) float64`, returning 0.0 when `std == 0`
      rather than dividing by zero. This guard is the difference between a
      working detector and a demo that panics on a quiet brand.
- [ ] `ComputeBaseline(ctx, brandID string, days int, now time.Time) (models.BaselineStats, error)`
      producing exactly the shape in CONTRACTS §2 under bp-detector. `days` is a
      Go argument and has no default; the detector passes 14 explicitly and 0
      means zero days, not fourteen.
- [ ] `MeanRating` is `*float64` and stays nil when no review source ran in the
      window. It is the only pointer in the whole contract and the review-bomb
      rule reads it. Test nil and 0.0 as separate cases.
- [ ] Baseline **excludes the current hour**, or a spike suppresses itself.
      Write that as an explicit named test.
- [ ] `HourBucket(t time.Time) string` and `DayBucket(t time.Time) string`, UTC,
      used by cache keys, run idempotency and alert dedupe. One implementation,
      three consumers.
- [ ] `ZScore` and the buckets are pure and table-driven with no database.
      `ComputeBaseline` needs the pool and skips without `DATABASE_URL`, same as
      Task 4.
- [ ] Paste, commit.

## Task 9: Prompts and guardrails

**Files:** `internal/prompts/prompts.go`, `internal/prompts/guardrails.md`

- [ ] `//go:embed *.md` with `Load(name string) string` reading `<name>.md` from
      the embedded FS. The signature returns no error, so an unknown name
      panics. That is right: a missing prompt is a build mistake, not a runtime
      condition.
- [ ] **A `//go:embed *.md` pattern that matches nothing is a compile error**,
      and B1 owns no agent prompt, so `guardrails.md` is the file that keeps the
      package building from Task 1 onward.
- [ ] `var Guardrails []string` parsed from `guardrails.md`'s bullet lines at
      init, not duplicated as a Go literal. One source of truth, and the text
      stays editable by whoever tunes the responder.
- [ ] Seed it with: no refunds promised, no admission of fault or liability, no
      medical or safety claims, no naming an individual employee, no commitment
      to a date, no legal characterisation ("we were negligent").
- [ ] Every agent's prompt `.md` lands in this package and is owned by that
      agent's track, not by me. Note in the package doc that adding one is a
      recompile, because the loader reads the embed and not the disk.
- [ ] Commit.

## Task 10: OpenTelemetry self-instrumentation

**Files:** `internal/obs/obs.go`, `internal/obs/obs_test.go`

New work the Python plan never had. Nasiko auto-injects OTel only into
Dockerfiles with a `FROM python` base, so Go agents instrument themselves. The
research put this at ~150 lines per agent; it is ~150 lines **once**, here, and
nine copies would be the mistake.

- [ ] `obs.Setup(serviceName string) (shutdown func(context.Context) error, err error)`:
      OTLP trace exporter to `OTEL_EXPORTER_OTLP_ENDPOINT`, a resource carrying
      `service.name = serviceName`, a batch span processor,
      `otel.SetTracerProvider` and the W3C propagator.
- [ ] **When `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, return a no-op shutdown and
      a nil error.** Local dev and CI have no collector, and an agent that
      refuses to start without one is an agent that never runs in a test. Make
      that an explicit test, and assert the returned shutdown is safe to call
      twice.
- [ ] Build the provider here and nothing else. HTTP handler instrumentation is
      `otelhttp`, wired once in Task 11, so no agent wires it either.
- [ ] The ADR calls this `obs.Init` and assigns it to B5. CONTRACTS §3 and
      PLAN's file table both say `obs.Setup` and B1. Code follows CONTRACTS;
      correct the ADR's two lines in this commit.
- [ ] Paste, commit.

## Task 11: A2A server wiring

**Files:** `internal/a2a/serve.go`, `internal/a2a/artifact.go`,
`internal/a2a/serve_test.go`

Also new. The point is that every agent's `main()` is the same twenty lines and
nobody hand-rolls the artifact envelope.

- [ ] `go get github.com/a2aproject/a2a-go/v2@v2.5.0` and **read the `a2asrv`
      package before writing a line.** If the module path, the version or the
      server surface differs from what CONTRACTS records, that is a contract
      change: land it on `main`, tell every track, do not quietly adapt around
      it inside this package.
- [ ] `JSONArtifact(name string, v any)` building the one envelope from
      CONTRACTS §1: `mimeType` `application/json`, `name` the output type name
      such as `MentionBatch`, body `json.Marshal(v)`.
- [ ] `Serve(card, handler)` reading `PORT`, registering the health path,
      wrapping the mux in `otelhttp`, and blocking.
- [ ] Adapting §4's `Handle(ctx context.Context, in <Name>Input) (<Output>, error)`
      to the SDK executor needs generics:
      `type Handler[In, Out any] interface { Handle(context.Context, In) (Out, error) }`
      and `func Serve[In, Out any](card a2asrv.AgentCard, h Handler[In, Out]) error`.
      Inference makes the call site `a2a.Serve(card, handler)` exactly as
      CONTRACTS prints it, so no agent's `main()` changes shape. Land the type
      parameters in §3.
- [ ] State the error rule here, once, so no agent reinvents it: a malformed
      input part is an A2A task failure, and a non-nil error out of `Handle` is
      also a task failure. Agents that hit trouble return their normal output
      struct with `Errors` populated and a **nil** error. A dead source must not
      kill a run.
- [ ] The card loader reads `AgentCard.json`. `protocolVersion` is `"1.0"`; the
      Nasiko example ships `"0.2.9"` and a real cluster rejects it with
      `-32009 VersionNotSupported`. B5 writes the nine cards, I provide the
      loader and that check.
- [ ] Test with `httptest`: a handler returning a struct produces exactly one
      artifact with the right mimeType and name, and a malformed input part
      produces a task failure.
- [ ] Paste, commit.

## Task 12: CI

**Files:** `.github/workflows/ci.yml`

- [ ] Postgres 16 service container, Go 1.27, `go build ./...`, `go vet ./...`,
      apply migrations, then `BP_FIXTURE_MODE=replay go test ./... -race` with
      **no secrets configured**. The job must be green on a fork with access to
      no key. That is the point.
- [ ] `go vet` is a failing step, not advisory. CONTRACTS §4 makes it part of
      the definition of done for every track.
- [ ] `-race` because the budget's cached total and the orchestrator's
      `errgroup` fan-out are the two places a data race hides until stage.
- [ ] **There is no `pytest-socket` in Go**, so no-network is enforced by
      construction instead: every test injects `anakin.NoNetwork()`, CI sets no
      `ANAKIN_API_KEY` and no `OPENAI_API_KEY`, and `BP_FIXTURE_MODE=replay` is
      the default. Add one grep step failing the build if any `_test.go` file
      names `http.DefaultClient`, which is the closest thing to the socket ban
      we had.
- [ ] Commit.

---

## Integrator duties (after Phase 1)

- Review and merge every track PR into `main`.
- Own conflict resolution in `internal/` and `db/`.
- Land any agreed contract change on `main` yourself, then tell every track to
  rebase. Post it in `HACKATHON_NOTES.md`.
- Migrations after Phase 1 are **additive only**: a new file, never an edit to
  `001_init.sql`. Task 0's parity test tells the track whether a new struct
  field owes a migration.
- The doc corrections this track owes, all of them because the code contradicts
  the doc: CONTRACTS §0 `RedactPII`, CONTRACTS §3 listing `internal/cluster`,
  CONTRACTS §3b on `created_at`, the ADR's `obs.Init` and its B5 assignment,
  `db/CLAUDE.md`, and `agents/CLAUDE.md`, which still describes `main.py` and
  `bp_core.a2a.call_agent`. That last one belongs to the agent tracks; flag it
  in `HACKATHON_NOTES.md` rather than editing it under them.

## Definition of done

```bash
docker compose up -d postgres
export DATABASE_URL=postgresql://brandpulse:brandpulse@localhost:5432/brandpulse
go build ./... && go vet ./...
BP_FIXTURE_MODE=replay go test ./internal/... -race -count=1 -v
```

Green, with the output pasted into the PR. No pasted output, no claim of
passing. Every other track can import `brandpulse/internal/...`, get a working
`anakin.Client` in replay mode for all five methods, and stand up an agent with
`obs.Setup` plus `a2a.Serve`.
