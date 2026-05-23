# BrandPulse

Social listening for Indian D2C brands, built as nine A2A agents.

A brand owner sells skincare on Amazon and their own site. At 11pm a thread on
Reddit turns sour about a batch that arrived leaking. By morning it has 200
upvotes and the first 1-star reviews are landing. Nobody told them, because the
tools that would have told them start around ₹77,000 a month and are sold on
annual enterprise contracts to companies that have a social team. Sprinklr
discontinued the only self-serve tier in the market in April 2026. The working
and the sources are in [docs/PRICING.md](docs/PRICING.md).

BrandPulse watches seven public sources, groups what it finds into topics,
fires alerts on five statistical rules, drafts a guardrailed reply, and writes
a brief. It costs cents a day per brand.

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

## The sources we actually read

Seven, and we are specific about this because the brief named eight and
Anakin's Wire catalogue does not carry four of them. The full working is in
[docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md).

| Source | How we read it | Quality |
|---|---|---|
| Reddit | Anakin Wire, search, subreddit posts, post details | Solid, full thread bodies |
| YouTube | Anakin Wire, search and comment threads | Solid |
| Amazon | Anakin Wire, product search and reviews | Solid, star ratings and dates |
| News | Anakin Search API | Solid |
| Web | Anakin Search API, then the URL Scraper for bodies | Solid |
| App Store | Apple's public customer-review RSS feed | Good, documented public feed |
| Play Store | URL Scraper with `useBrowser: true` | Probe, validated before we count it |

X/Twitter is read through the Search API scoped to `site:x.com`, snippets only.
It carries no engagement metrics, so it cannot feed the influencer rule, and it
is the first source the flow guard drops. Instagram and Flipkart are **not**
read at all. They remain in the `Source` enum so the schema does not change,
but no brand has them enabled.

**No source is faked.** Synthetic data exists in exactly two places, the demo's
crisis injection and `web/`'s fictional sample brand, and both are labelled as
synthetic in the interface.

## How it is put together

Nine agents, each a container Nasiko deploys, each answering A2A calls. Four of
them make no LLM call at all. The full call graph and the reasoning behind the
split is in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

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

`bp-detector` never calls a model. An alert ships the numbers that fired it, so
a brand owner can check the arithmetic. A model that decides what counts as a
crisis is a model that cannot be audited at 11pm.

The four platforms each do a job nothing else here does:

- **Nasiko** deploys and routes the nine agents, enforces the flow-guard fan-out
  caps, proxies every agent-to-agent call, and collects the traces.
- **Anakin** is the entire data layer. There is no scraper in this repository.
- **DronaHQ** is the chat agent a brand owner talks to and the ops dashboard an
  analyst works in, including the approve/reject step for drafted replies.
- **Go** is the agents, chosen for a 14.6MB container against a Python image an
  order of magnitude larger. The reasoning is in
  [docs/decisions/001-go-for-agents.md](docs/decisions/001-go-for-agents.md).

## Run it

Full instructions, including the parts that bite at 2am, are in
[docs/SETUP.md](docs/SETUP.md). The short version:

```bash
cp .env.example .env
docker compose up -d --no-recreate postgres
docker compose ps                    # wait for (healthy)
go build ./... && go vet ./...
BP_FIXTURE_MODE=replay go test ./...
./demo/run_demo.sh                   # the 2-minute flow
```

`BP_FIXTURE_MODE=replay` is the default everywhere including CI. It reads
recorded fixtures and makes zero network calls, so a clone needs no API key to
run the tests. A test that needs a credential is a broken test.

## What we sent back upstream

We ran Nasiko's CLI hard enough to find a spec-conformance bug in it. A2A 1.0
moved `url`, `protocolVersion` and `preferredTransport` out of the AgentCard
root into `supportedInterfaces[]`, renaming the transport to `protocolBinding`.
`nasiko validate` only read the root, so a card serialised by any current A2A
SDK failed validation naming three fields that were all present. The fix is
open at [Nasiko-Labs/nasiko#176](https://github.com/Nasiko-Labs/nasiko/pull/176).

## Documentation

| Document | What it answers |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | The nine agents, the call graph, the two front ends |
| [docs/SETUP.md](docs/SETUP.md) | Getting a clone running from cold |
| [docs/CONTRACTS.md](docs/CONTRACTS.md) | The frozen wire format, per-agent signatures |
| [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md) | Which sources we ship and what we lost |
| [docs/PRICING.md](docs/PRICING.md) | Measured cost per brand-day against list prices |
| [docs/DEMO-SCRIPT.md](docs/DEMO-SCRIPT.md) | The 2-minute run, with what to say when it breaks |
| [docs/DEPLOY-NOTES.md](docs/DEPLOY-NOTES.md) | Everything we learned about Nasiko the hard way |
