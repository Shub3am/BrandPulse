# BrandPulse

Social listening for Indian D2C brands, built as nine A2A agents.

A brand owner sells skincare on Amazon and their own site. At 11pm a thread on
Reddit turns sour about a batch that arrived leaking. By morning it has 200
upvotes and the first 1-star reviews are landing. Nobody told them, because the
tools that would have told them start around ₹77,000 a month and are sold on
annual enterprise contracts to companies that have a social team. Sprinklr
discontinued the only self-serve tier in the market in April 2026. The working
and the sources are in [docs/PRICING.md](docs/PRICING.md). BrandPulse watches
six public sources, groups what it finds into topics, fires alerts on five
statistical rules, drafts a guardrailed reply a human approves, and writes a
brief. It is for the brand doing ₹40 lakh a month with no social team and no
₹48,000 a month to spend on finding out.

## Two statements that are load-bearing

**Public data only.** Every source is a public API, a public review feed, or a
public web page. There is no scraping behind a login, no purchased dataset, no
private message, and no personal data beyond what a person chose to publish
under their own handle. Mention text is run through `redact.PII` before it ever
reaches a model prompt.

**Nothing auto-posts.** There is no code path in this repository that publishes
to any platform. A drafted reply is a draft. Approving one hands it to a human
in DronaHQ, who copies it and posts it themselves if they want to. This is a
deliberate design constraint, not a missing feature: an autonomous system that
can speak in a brand's voice in public is a liability nobody asked us to build.

## Architecture

Nine agents, each a container Nasiko deploys, each answering A2A 1.0 calls,
each doing one job statable in a sentence without an "and". Four of them make
no LLM call at all.

| Agent | What it does | LLM |
|---|---|---|
| `bp-orchestrator` | Runs the pipeline for one brand and one trigger | no |
| `bp-onboarder` | Crawls a brand's site into a keyword set | 1 call |
| `bp-collector` | Fetches one source's mentions for one window | no |
| `bp-enricher` | Classifies sentiment, intent and aspects in batches | batched |
| `bp-clusterer` | Groups mentions into topics and names them | 1 per cluster |
| `bp-detector` | Fires the five statistical alert rules | **no, ever** |
| `bp-responder` | Drafts a guardrailed reply | 1 call |
| `bp-briefer` | Writes the daily and weekly brief | 1 call |
| `bp-sov` | Counts share of voice | no |

One external call goes in, at `bp-orchestrator`. Everything else is a peer call
through the Nasiko proxy, resolved from an injected URL template. No agent
imports another and no agent compiles in a peer's address.

```
                          PUBLIC WEB
                               │
                          ANAKIN API        Search · Wire · URL Scraper · Crawl/Map
                               │
                               ▼
                         bp-collector       one call per source, no LLM
                               ▲
                               │ step 4, fan-out ≤ 8
                               │
  DronaHQ ──REST──▶ ┌──────────┴────────────┐ ◀── demo/run_demo.sh
  (chat + ops)      │    bp-orchestrator    │     (A2A SendMessage)
                    │  ranks sources, fans  │
  bp-onboarder ◀─── │  out, records the run │
  (once per brand)  └──────────┬────────────┘
                               │
      step 5 ──────────────────┼──── step 6, fan-out = 3 ───── steps 7 and 8
         │                     │              │                      │
         ▼                     ▼              ▼                      ▼
   bp-enricher    bp-clusterer · bp-sov · bp-detector   bp-responder ──▶ bp-briefer
         │                     │              │                      │         │
         └─────────────────────┴──────────────┴──────────────────────┴─────────┘
                                              │  every write in the system
                                              ▼
  ┌──────────────────────────────────────────────────────────────────────────────┐
  │                                  POSTGRES                                    │
  │  brands · brand_profiles · mentions · mention_enrichment · topics            │
  │  alerts · reply_drafts · briefs · fetch_cache · runs · source_yield          │
  └──────────────────────────────────────────────────────────────────────────────┘
                                              ▲
                                              │ read-only pool
                                   ┌──────────┴──────────┐
                                   │     Fastify BFF     │  read routes, plus
                                   │        :8080        │  POST /api/runs, which
                                   └──────────┬──────────┘  is one A2A call
                                              ▲
                                              │ the only backend it knows about
                                   ┌──────────┴──────────┐
                                   │    Next.js web/     │  the two-minute
                                   │        :3000        │  judge screen
                                   └─────────────────────┘
```

The pipeline, in order: load the confirmed profile, compute a time bucket so a
retry inside it is free, rank the brand's sources by yesterday's mentions per
credit and truncate to the flow-guard fan-out cap, fan out the collectors,
enrich, fan out clusterer/SOV/detector, draft a reply for each alert of
severity high or above capped at five, write the brief, record the run with
real credits and tokens.

**`bp-detector` never calls a model.** The five rules are `spike`, `crisis`,
`review_bomb`, `influencer_mention` and `competitor_move`, all z-scores and
rates over a 14-day baseline. Every alert carries the observed value and the
threshold that fired it, so a brand owner can check the arithmetic. A model
that decides what counts as a crisis is a model that cannot be audited at 11pm.

**Degradation is a first-class outcome.** An agent whose output is a batch
returns that struct with `Errors` populated and partial data in place rather
than failing the task, and the orchestrator records the reason in
`RunRecord.DegradedReason` and carries on. A dead source must not kill a run.

Every agent replies with exactly one `application/json` artifact carrying one
`internal/models` struct, with the media type on the part rather than the
artifact, so the body lands inline as real JSON and DronaHQ binds a field
without decoding a string first. Full call graph in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), frozen signatures in
[docs/CONTRACTS.md](docs/CONTRACTS.md).

## The sources we actually read

Six. The brief assumed eight and Anakin's Wire catalogue does not carry four of
them, so we wrote down what was lost instead of quietly shipping less. The full
working is in [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md), and the
registry that decides it is
[`internal/anakin/sources/registry.go`](internal/anakin/sources/registry.go).

| Source | How we read it |
|---|---|
| Reddit | Anakin Wire: search, subreddit posts, post details |
| YouTube | Anakin Wire: search and comment threads |
| News | Anakin Search API |
| Web | Anakin Search API, then the URL Scraper for bodies |
| App Store | Apple's public customer-review RSS feed, via the URL Scraper |
| Play Store | URL Scraper with `useBrowser: true` |

X, Instagram, Amazon and Flipkart remain in the `Source` enum so the schema
does not change, but no adapter collects them. Amazon was dropped for
correctness rather than budget: `am_product_reviews` returned empty `title` and
`text` on every review it gave back, so every Amazon mention would hash to the
same content hash and dedupe would collapse the lot to one row.

**No source is faked.** See the section on that below.

## The three platforms, and why each is load-bearing

### Nasiko

The runtime. It hosts and orchestrates all nine agents as A2A containers,
routes external traffic in, proxies every agent-to-agent call, enforces the
flow-guard fan-out caps, and collects the traces that make a run inspectable
with `nasiko observe`.

Every agent listens on port 8000 with health at `/healthz`, ships its own
`AgentCard.json` and a multi-stage Dockerfile ending in a distroless static
image, and is deployed with `nasiko deploy`. **Never `nasiko upload`**: it
posts to `/api/agents/upload`, whose validator requires a `main.py` and rejects
a Go agent with a 400. No agent hardcodes a peer address: `a2a.Call`
substitutes `{agent}` into the injected `NASIKO_API_URL` template, so peer
resolution is deploy configuration. All LLM traffic goes only through the
injected `OPENAI_BASE_URL`, with no provider SDK configured in any agent, and
per-agent model choice is set with `nasiko llm-config`. The Go agents
self-instrument OpenTelemetry via `obs.Setup` because Nasiko's auto-injection
covers Python containers only.

We ran the CLI hard enough to find a spec-conformance bug in it. A2A 1.0 moved
`url`, `protocolVersion` and `preferredTransport` out of the AgentCard root
into `supportedInterfaces[]`, renaming the transport to `protocolBinding`.
`nasiko validate` only read the root, so a card serialised by any current A2A
SDK failed validation naming three fields that were all present. The fix is
open at [Nasiko-Labs/nasiko#176](https://github.com/Nasiko-Labs/nasiko/pull/176).
Until it merges, every card here carries both shapes.

### Anakin

The entire data acquisition layer. There is no scraper in this repository and
no HTTP client pointed at Reddit, YouTube or a review feed.
[`internal/anakin/`](internal/anakin/) is the only code that fetches public
data, and it goes through Anakin's Search API, Wire actions, URL Scraper and
Crawl/Map. Six adapters are registered, one per source in the table above.

Responses are recorded once into [`fixtures/`](fixtures/), keyed by a sha256
over the canonical JSON of `{method, arg, opt}`, so the demo and CI run at zero
credits and a clone needs no API key to run the tests. Real spend, from the
ledger in `HACKATHON_NOTES.md`: **92 of the 300 free credits, 208 remaining**,
of which one 57-credit recording session produced the whole fixture corpus
against a 63-credit dry-run estimate.

### DronaHQ

The two human surfaces that are not the product dashboard: the chat agent a
brand owner talks to, and the ops and analyst dashboard with the approve and
reject step. Both bind to the agents' typed JSON artifacts through one REST
connector.

[`dronahq/`](dronahq/) holds the connector and both builds as checked-in
source: an OpenAPI 3.0.3 spec of the six BFF endpoints for DronaHQ's "Import
API" flow (it passes `openapi-spec-validator`), the connector setup, the
complete paste-ready Instruction for the chat agent with its four tools and its
guardrails, that agent's model and trigger config, and a build specification
for the ops app naming six screens, every query, every control and every bound
field with its exact JSON path.

The published `chat-agent.json` and `dashboard-app.json` exports are not
committed, and [`dronahq/README.md`](dronahq/README.md) says exactly why:
DronaHQ's app export is a console action whose file schema is not published,
connectors and Agents have no export format at all, and there was no console
login during the build. Inventing a JSON file and labelling it with DronaHQ's
name would be worse than not having one. Every path and field in those specs
was read out of `bff/src/routes/`, `bff/src/contracts.ts` and `bff/src/rows.ts`
rather than remembered.

The chat agent's guardrails (never promise a refund, never admit fault, never
commit to a date, always escalate a crisis to a human) run in two places:
in the agent's own rules, and again as a server-side floor in `bp-responder`,
which substring-matches 30 banned phrases from `internal/prompts/guardrails.md`
against every draft. A guardrail that lives only in a prompt is a suggestion.

Go is the fourth choice worth naming: a 14.6MB distroless container against a
Python image an order of magnitude larger, measured rather than estimated. The
reasoning is in
[docs/decisions/001-go-for-agents.md](docs/decisions/001-go-for-agents.md).

## Run it locally

Full instructions, including the parts that bite at 2am, are in
[docs/SETUP.md](docs/SETUP.md). The short version:

```bash
cp .env.example .env                # DATABASE_URL points at port 5433, not 5432
docker compose up -d --no-recreate postgres
docker compose ps                   # must show (healthy) before anything else
go build ./... && go vet ./...
BP_FIXTURE_MODE=replay go test ./...
python3 scripts/check_agent_cards.py
./demo/run_demo.sh                  # the 2-minute flow
```

Port **5433, not 5432**: `docker-compose.yml` maps it that way because another
project's Postgres commonly holds 5432 and compose then fails to start at all.
Inside the container it is still 5432.

`BP_FIXTURE_MODE` defaults to `replay` everywhere including CI and including
`run_demo.sh`, so **the demo costs zero Anakin credits and makes no network
call**. A test that needs a credential is a broken test. Pass `--live` to
`run_demo.sh` for one extra live Anakin call, which is deliberately separate so
stage wifi cannot break the eight steps before it.

The nine agents come up together with:

```bash
docker compose -f docker-compose.yml -f docker-compose.agents.yml up -d
```

The agents file layers onto the Postgres compose project and does not work on
its own. It publishes `bp-orchestrator` on `127.0.0.1:8000` and sets
`NASIKO_API_URL` to `http://{agent}:8000/`.

The two front ends run on localhost:

```bash
cd bff && npm install && npm run dev            # http://localhost:8080
cd web && BFF_BASE_URL=http://localhost:8080 npm run dev   # http://localhost:3000
```

There is no hosted deployment. `web/` and `bff/` run locally against the Docker
Postgres on 5433, with no Vercel and no hosted database.

## What is real and what is synthetic

Judges should be able to check this, so here is exactly where the line is.

**Nothing posts anywhere.** There is no posting code path in this repository
and none is to be added. `bp-responder` writes a `reply_drafts` row with
`status` `'draft'`, `ReplyDraft.MarshalJSON` emits `requires_human_approval`
unconditionally with no struct field that could ever say otherwise, the BFF's
pool is opened `default_transaction_read_only=on`, and DronaHQ's send button
copies the text for a person. `run_demo.sh` prints the draft row with its
status so you can read it yourself.

**The recorded Anakin responses in `fixtures/` are real Anakin output**,
captured once in a single 57-credit session and replayed since. They are not
mock data. Replay is byte-for-byte what Anakin returned.

**The crisis is synthetic and says so.** `demo/cmd/injectcrisis` drops a
negative surge into Postgres so the detector has something to fire on during a
two-minute demo. Every injected row carries `raw: {"synthetic": true}`, the
injector's cleanup deletes by that marker and nothing else, and
`web/components/DataSourceBadge.tsx` counts those rows and badges the panel as
injected demo data. A row without the marker is never treated as synthetic.

**The demo brand is fictional.** "Suncoast" is not a real company.
`web/lib/demoData.ts` holds synthetic sample artifacts and nothing under
`web/app/` imports it: when the backend is down the dashboard throws and says
so rather than silently substituting.

**Corpus size, stated honestly.** Replaying `fixtures/` yields 181 mentions
across the demo brand and its two competitor profiles. A real single-brand run
collects roughly 60. Do not read 181 as a per-brand figure, because it is not
one.

**Cost per brand-day is not measured yet.** The target in
[docs/PRICING.md](docs/PRICING.md) is under ₹15 per brand-day and the
incumbent list prices it is measured against are sourced there. Until the
measurement runs, that number is a target and is labelled as one.

## Where to read more

Each module carries its own `CLAUDE.md` with what it owns, what it must not
know about, and the gotchas that are not visible from the code.

| Module | Doc |
|---|---|
| The nine agents | [agents/CLAUDE.md](agents/CLAUDE.md) |
| The frozen wire format | [internal/models/CLAUDE.md](internal/models/CLAUDE.md) |
| Postgres schema | [db/CLAUDE.md](db/CLAUDE.md) |
| DronaHQ chat agent and dashboard | [dronahq/CLAUDE.md](dronahq/CLAUDE.md) |
| Next.js dashboard | [web/CLAUDE.md](web/CLAUDE.md) |
| Fastify BFF | [bff/CLAUDE.md](bff/CLAUDE.md) |
| The 2-minute demo | [demo/CLAUDE.md](demo/CLAUDE.md) |
| Clustering | [internal/cluster/CLAUDE.md](internal/cluster/CLAUDE.md) |

| Document | What it answers |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | The nine agents, the call graph, the two front ends |
| [docs/SETUP.md](docs/SETUP.md) | Getting a clone running from cold |
| [docs/CONTRACTS.md](docs/CONTRACTS.md) | The frozen wire format, per-agent signatures |
| [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md) | Which sources we ship and what we lost |
| [docs/PRICING.md](docs/PRICING.md) | The cost target and the incumbent list prices |
| [docs/DEMO-SCRIPT.md](docs/DEMO-SCRIPT.md) | The 2-minute run, with what to say when it breaks |
| [docs/DEPLOY-NOTES.md](docs/DEPLOY-NOTES.md) | Everything we learned about Nasiko the hard way |
| [SUBMISSION.md](SUBMISSION.md) | The hackathon submission answers |
