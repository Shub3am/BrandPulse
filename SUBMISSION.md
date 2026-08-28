# BrandPulse: submission form answers

Copy each block under its heading into the organisers' form.

---

## Team Name

Solo Participant

---

## Email

shubham@vshubham.com

---

## Repo

https://github.com/Shub3am/BrandPulse

---

## What problem are you solving?

Social listening for Indian D2C brands, built as nine A2A agents.

A brand owner sells skincare on Amazon and their own site. At 11pm a thread on
Reddit turns sour about a batch that arrived leaking. By morning it has 200
upvotes and the first 1-star reviews are landing. Nobody told them, because the
tools that would have told them start around Rs 77,000 a month and are sold on
annual enterprise contracts to companies that have a social team. Sprinklr
discontinued the only self-serve tier in the market in April 2026. The working
and the sources are in `docs/PRICING.md`.

BrandPulse watches six public sources, groups what it finds into topics, fires
alerts on five statistical rules, drafts a guardrailed reply, and writes a
brief. It is built to run at cents a day per brand: the target in
`docs/PRICING.md` is under Rs 15 per brand-day, against roughly Rs 48,000 a
month for the cheapest incumbent setup.

---

## How does your solution solve the problem? Briefly describe your approach and architecture.

Nine Go agents, each its own container, each answering A2A 1.0 calls and each
doing one job. `bp-orchestrator` is the only one with a public entry point. It
runs the pipeline for one brand and one trigger: load the confirmed brand
profile, compute a time bucket so a retry is free, rank the brand's sources by
yesterday's mentions-per-credit, then fan out. `bp-collector` fetches one
source for one window (no LLM, ever). `bp-enricher` classifies sentiment,
intent and aspects in batches of 50. `bp-clusterer` groups mentions into topics
with hand-rolled TF-IDF and agglomerative clustering, then spends one LLM call
per cluster to name it. `bp-sov` counts share of voice. `bp-detector` fires the
five alert rules. `bp-responder` drafts a reply for each high-severity alert,
capped at five. `bp-briefer` writes the daily brief. `bp-onboarder` crawls a
brand's site into a keyword set, once, outside the pipeline.

`bp-detector` makes no model call at all, and that is the decision I would
defend hardest. The five rules are spike, crisis, review_bomb,
influencer_mention and competitor_move, all z-scores and rates over a 14-day
baseline. Every alert ships the observed value and the threshold that fired it,
so a brand owner woken at 11pm can check the arithmetic. A model in that seat
turns "why did this fire?" into "the model thought so".

The protocol between them is A2A 1.0. Every agent replies with exactly one
`application/json` artifact carrying one `internal/models` struct, so the media
type sits on the part and DronaHQ binds a field without decoding a string
first. No agent imports another: `a2a.Call` resolves a peer by substituting
`{agent}` into a deploy-injected URL template. Degradation is a first-class
outcome, not an error path: a batch-shaped output returns with `Errors`
populated and partial data in place, and the orchestrator records the reason in
`RunRecord.DegradedReason` and carries on, because a dead source must not kill
a run.

Postgres holds the state the agents write and nothing else reads around them:
brands, brand_profiles, mentions, mention_enrichment, topics, topic_mentions,
alerts, reply_drafts, briefs, fetch_cache, runs, source_yield. The schema
mirrors `internal/models/` and a parity test fails the build if the two drift.

Two front ends, two audiences. A Fastify BFF is the only backend the Next.js
dashboard talks to: read routes over Postgres plus a single write route,
`POST /api/runs`, which is one A2A call to the orchestrator. Its pool is opened
`default_transaction_read_only=on` and it computes no figure of its own, so
every number on screen is one an agent can be held to. The dashboard is the one
screen a judge reads in two minutes. DronaHQ is the other surface, for the
brand owner and the analyst on shift.

The roster is in `agents/CLAUDE.md`, the frozen per-agent signatures in
`docs/CONTRACTS.md`, and the call graph in `docs/ARCHITECTURE.md`.

---

## Where have you used Nasiko in your project?

Nasiko hosts and orchestrates all nine Go agents as A2A containers. It is the
runtime, not a deployment detail.

Every agent listens on port 8000 with health at `/healthz`, ships its own
`AgentCard.json` and a multi-stage Dockerfile ending in a distroless static
image, and is deployed with `nasiko deploy`. Never `nasiko upload`: both the
CLI's upload command and the dashboard zip uploader post to
`/api/agents/upload`, whose validator requires a `main.py` and rejects a Go
agent with a 400. `nasiko deploy` branches on `AgentCard.json`, builds locally
and pushes the image.

No agent hardcodes a peer address. `internal/a2a/call.go` substitutes `{agent}`
into the injected `NASIKO_API_URL` template, so peer resolution is deploy
configuration: `https://<control-plane>/api/agents/{agent}` deployed,
`http://{agent}:8000/` under compose. The call also replays the inbound
`x-nasiko-agent-token` delegation JWT, so the credential belongs to the request
being served rather than to the process.

All LLM traffic goes only through the injected `OPENAI_BASE_URL`.
`internal/llm` is the single path to a model, no agent configures a provider
SDK or holds a key, and per-agent model choice is set with `nasiko llm-config`
rather than in code. The router ignores the model named in the request body, so
cost is priced from the model the router reports back, not the one asked for.
All nine cards declare `llm_provider`, and the four agents that make no model
call declare it `null`: bp-orchestrator, bp-collector, bp-detector and bp-sov.
The other five name `nasiko-router`. That is true, and it is the kind of true a
judge can check in one grep.

The Go agents self-instrument OpenTelemetry through `obs.Setup` in every
`main()`, because Nasiko's auto-injection covers Python containers only. Skip it
and the agent is invisible to `nasiko observe`.

We also found and fixed a spec-conformance bug in Nasiko while doing this. A2A
1.0 moved `url`, `protocolVersion` and `preferredTransport` out of the
AgentCard root into `supportedInterfaces[]`, renaming the transport to
`protocolBinding`. `nasiko validate` only read the root, so a card serialised
by a current A2A SDK failed validation naming three fields that were all
present. The fix is open at Nasiko-Labs/nasiko#176. Until it merges, every card
here carries both shapes.

---

## Where have you used Anakin in your project?

Anakin is the entire data acquisition layer. There is no scraper in this
repository and no HTTP client pointed at Reddit, YouTube or a review feed.
`internal/anakin` is the only code that fetches public data, and everything
else consumes what it returns.

Six source adapters are registered in `internal/anakin/sources/registry.go`:
reddit, youtube, news, web, appstore and playstore. They go through Anakin's
Search API, Wire actions, URL Scraper and Crawl/Map. Reddit and YouTube use
Wire actions. News and web go Search first, then the URL Scraper for bodies,
because Search returns snippets rather than page content. App Store reads
Apple's public customer-review RSS feed through the Scraper. Play Store uses
the Scraper with `useBrowser: true`. `bp-onboarder` maps a brand's site, filters
the link list itself and scrapes what it keeps.

Responses are recorded once into `fixtures/`, keyed by a sha256 over the
canonical JSON of `{method, arg, opt}`, so the demo and CI run at zero credits.
`BP_FIXTURE_MODE=replay` is the default everywhere including CI, and in replay
the client makes no network call at all: a clone needs no API key to run the
tests.

Real credit spend, from the ledger in `HACKATHON_NOTES.md`: **92 of the 300
free credits, 208 remaining.** That is 35 across schema reads, the Play Store
probe and picking the demo brand, plus one 57-credit recording session against
a 63-credit dry-run estimate. The recording is one-shot and is not run again.

Four sources the brief assumed did not survive contact with the catalogue, and
that is written down rather than papered over. Instagram, Flipkart reviews,
Google Play reviews and App Store reviews are not Wire actions. We replaced two
of them with the public RSS feed and the browser scraper and dropped the other
two. Amazon was dropped for correctness, not budget: `am_product_reviews`
returned empty `title` and `text` on every review, which would collapse every
Amazon mention to one row on content hash. `docs/SOURCE-STRATEGY.md` has the
full working.

---

## Where have you used DronaHQ in your project?

DronaHQ is the human surface: the chat agent a brand owner talks to, and the
ops and analyst dashboard, both binding to the agents' typed JSON artifacts
through one REST connector.

What is committed in `dronahq/` today, and I am being exact about this:

- `connectors/brandpulse-openapi.json`, an OpenAPI 3.0.3 spec of the six BFF
  endpoints DronaHQ calls, pinned to 3.0 because that is what DronaHQ's
  "Import API" for custom API connectors documents accepting. It parses, every
  `$ref` resolves and it passes `openapi-spec-validator`. It is a description
  of our API in a standard format, transcribed from `bff/src/routes/`, not a
  DronaHQ export.
- `connectors/rest-connector.md`, the connector setup: both the OpenAPI import
  route and the by-hand table, with per-endpoint curl.
- `chat-agent/system-prompt.md`, the complete paste-ready Instruction for the
  DronaHQ Agent, with its four tools (`getBrandPulse`, `getAlerts`,
  `getMentions`, `startRun`), its "every number you say came back from a tool
  call in this conversation" rule, and its guardrails: never promise a refund,
  never admit fault, never commit to a date, always escalate a crisis to a
  human.
- `chat-agent/agent-config.md`, the model, tools, triggers, cost and test
  script for that agent.
- `dashboard/build-spec.md`, a 400-line build specification for the ops app:
  six screens, every query, every control and every bound field with its exact
  JSON path into a real response, plus the four properties of our API that look
  like bugs in the binding editor.
- `README.md`, which states plainly what is verified and how, and what still
  needs the console.

What is not committed: the published `chat-agent.json` and `dashboard-app.json`
exports. Two honest reasons. DronaHQ's app export is a console action whose file
schema is not published anywhere in its docs, and connectors and Agents have no
export format at all, so writing those files from this repo would mean
inventing JSON and labelling it with DronaHQ's name. And I had no console login
during the build, so nothing that happens inside DronaHQ has been clicked
through by me. The specifications are written to the precision where guessing
would otherwise start, every path read out of the BFF source rather than
remembered, so each one becomes the thing its export is checked against the
moment the console is available.

The guardrails run in two places on purpose. They are in the agent's own rules,
and `bp-responder` enforces a server-side floor under them by substring-matching
30 banned phrases from `internal/prompts/guardrails.md` against every draft. A
guardrail that lives only in a prompt is a suggestion. The approve/reject step
is the human-in-the-loop control, and approving a draft copies it for a person.
Nothing in DronaHQ posts anywhere, because nothing in this repo does.

---

## Is there a live hosted version?

No. BrandPulse runs locally with docker compose, and the demo is one command:

```bash
cp .env.example .env && docker compose up -d --no-recreate postgres && ./demo/run_demo.sh
```

That is deliberate for the demo rather than an omission. `web/` and `bff/` run
on localhost against the Docker Postgres on port 5433, with no Vercel and no
hosted Postgres. The demo runs at zero Anakin credits and makes no network call,
so stage wifi cannot break it. The agents build and run as nine containers via
`docker compose -f docker-compose.yml -f docker-compose.agents.yml up -d`, and
the Nasiko deploy workflow is committed and runs by `workflow_dispatch`.

---

## Social Media Post

**Pending.** Not yet posted. Drafts below are ready to go.

### LinkedIn

> A two-person skincare brand finds out about a batch of leaking caps four days
> late, from a customer who called to ask why nobody had replied to her Reddit
> thread.
>
> The tools that would have caught it start around Rs 77,000 a month on an
> annual enterprise contract. Sprinklr discontinued the only self-serve tier in
> the market this April. So brands that size use nothing.
>
> So I built BrandPulse over a weekend: nine A2A agents in Go that read six
> public sources, group mentions into topics, fire alerts on five statistical
> rules, and draft a reply a human approves.
>
> Three decisions I would defend outside a hackathon:
>
> The agent that decides whether something is a crisis makes no model call. It
> ships the numbers that fired it, so you can check its arithmetic at 11pm.
>
> It drafts replies and cannot post them. There is no posting code path in the
> repository. Approve hands it to a person.
>
> Four of the sources the brief assumed do not exist in the data catalogue. I
> shipped six real ones and wrote down exactly what was lost and why.
>
> Built on Nasiko (https://www.linkedin.com/company/nasikolabs) for the agent
> runtime and A2A routing, Anakin (https://www.linkedin.com/company/anakintech)
> for the entire data layer, and DronaHQ
> (https://www.linkedin.com/company/deltecs-infotech) for the chat agent and
> the ops dashboard.
>
> Repo: https://github.com/Shub3am/BrandPulse

### X

> Built BrandPulse: social listening for Indian D2C brands, as 9 A2A agents in Go.
>
> @nasikolabs runs them. @anakinHQ is the whole data layer. @DronaHQ is the chat and ops UI.
>
> The crisis detector makes zero LLM calls. It ships the numbers that fired it.

---

## Demo video

**Pending.** Not yet recorded. The script is in `docs/DEMO-SCRIPT.md` and the
run it narrates is `./demo/run_demo.sh`, which takes about two minutes: seed
the demo brand, replay the recorded corpus, run the pipeline, inject a labelled
crisis, re-run so the detector sees the surge, then show the alert with its
evidence, the unsent draft and the brief straight out of Postgres.
