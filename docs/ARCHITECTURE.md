# Architecture

Nine A2A agents in Go, deployed on Nasiko. Anakin is the whole data layer.
Two front ends, aimed at two different people.

The frozen input and output signature for every agent is in
[CONTRACTS.md](CONTRACTS.md) §2. This document is the shape, not the schema.

---

## 1. The nine agents

Each agent is one directory under `agents/`, one container, one A2A endpoint,
one job statable in a sentence without an "and".

| Agent | Job | LLM calls per run |
|---|---|---|
| `bp-orchestrator` | Runs the pipeline for one brand and one trigger | **0** |
| `bp-onboarder` | Crawls a brand's site into a keyword set | 1 |
| `bp-collector` | Fetches one source's mentions for one window | **0** |
| `bp-enricher` | Classifies sentiment, intent and aspects | batched |
| `bp-clusterer` | Groups mentions into topics and names them | 1 per cluster |
| `bp-detector` | Fires the five statistical alert rules | **0, ever** |
| `bp-responder` | Drafts a guardrailed reply | 1 per alert |
| `bp-briefer` | Writes the daily and weekly brief | 1 |
| `bp-sov` | Counts share of voice | **0** |

### The four deterministic ones, and why that matters

`bp-orchestrator`, `bp-collector`, `bp-detector` and `bp-sov` make no model
call. Their cards carry `llm_provider: null`, which is true and is the kind of
true worth being able to prove.

`bp-detector` is the one that matters most. It fires on five statistical rules
over counts and rates, and an alert ships the numbers that fired it. A brand
owner woken at 11pm can check the arithmetic. Put a model in that seat and the
answer to "why did this fire?" becomes "the model thought so", which is not an
answer a brand owner can act on and not a decision anyone can audit after the
fact.

The other three are deterministic for a cheaper reason: counting mentions,
fetching a page and sequencing eight steps are not tasks a model does better
than code, and every model call is latency, cost and a new failure mode.

---

## 2. The call graph

One external call in, at `bp-orchestrator`. Everything else is a peer call
through the Nasiko proxy.

```
                         bp-orchestrator
                                │
        ┌───────────────────────┼───────────────────────┐
        │  step 4, parallel     │  step 6, parallel     │  steps 5, 7, 8
        │  fan-out ≤ 8          │  fan-out = 3          │
        ▼                       ▼                       ▼
  bp-collector            bp-clusterer            bp-enricher
  (one per source)        bp-sov                  bp-responder (≤ 5)
        │                 bp-detector             bp-briefer
        ▼                       │                       │
     Anakin                     ▼                       ▼
                             Postgres                Postgres

  bp-onboarder runs once per brand, outside the pipeline, from DronaHQ.
```

The pipeline, in order:

1. Load the brand profile, latest confirmed version.
2. Compute the time bucket. An existing run for `(brand, kind, bucket)` is
   returned unchanged unless `Force` is set, so a retry is free.
3. Rank the brand's sources by yesterday's yield in mentions per credit, and
   truncate to the flow-guard fan-out cap. Dropped sources are recorded, not
   silently skipped.
4. **Fan out**: `bp-collector` once per surviving source, in parallel. Persist.
5. `bp-enricher` over the new mentions. Persist.
6. **Fan out**: `bp-clusterer`, `bp-sov`, `bp-detector` in parallel.
7. `bp-responder` for each alert of severity high or above, capped at 5.
8. `bp-briefer` for the day.
9. Write the run row with real credits, tokens and cost.

### The caps are enforced, not documented

Fan-out from a single orchestrator call stays at or below 8, depth at or below
3. Nasiko's flow guards enforce both and **fail closed**: exceeding a cap is a
dropped call, not a slow one. Parallel steps use `errgroup` with a bounded
`SetLimit` for exactly this reason. A goroutine per source with no limit is how
the cap gets breached by accident, and the symptom is a missing artifact rather
than an error.

### Degradation is a first-class outcome

A dead source must not kill a run. An agent that fails returns its normal
output struct with `Errors` populated rather than returning an error, and the
orchestrator records the reason in `DegradedReason` and carries on. The `error`
return exists for malformed input only. A brief with a hole in it, labelled as
having a hole, beats no brief.

---

## 3. Where each platform is load-bearing

Not one of these is decoration. Remove any and something stops working.

**Nasiko** is the runtime. It deploys the nine containers, routes external
traffic in, proxies every agent-to-agent call (agents are never publicly
exposed), enforces the flow-guard caps described above, and collects traces
that make a run inspectable with `nasiko observe`. The A2A protocol is what
lets nine separate containers behave like one system without any of them
importing another.

**Anakin** is the entire data layer. There is no scraper in this repository and
no HTTP client pointed at Reddit. `bp-collector` calls Anakin's Wire actions,
the Search API and the URL Scraper, and that is the whole ingestion story.
Which sources exist and which we lost is in
[SOURCE-STRATEGY.md](SOURCE-STRATEGY.md).

**DronaHQ** is both human surfaces that are not the product dashboard: the chat
agent a brand owner talks to, and the ops dashboard with the approve/reject
step. The human-in-the-loop guardrail lives here.

**Go** is the agent language. A 14.6MB distroless container against a Python
image an order of magnitude larger, with no runtime to cold-start. Measured,
not estimated. The decision is written up at
[decisions/001-go-for-agents.md](decisions/001-go-for-agents.md).

### The AgentCard carries two shapes on purpose

`a2a-go` v2.5.0 serialises to A2A 1.0, which moved `url`, `protocolVersion` and
the transport into `supportedInterfaces[]`. Nasiko's `validate.rs` requires all
three at the card root. So a card generated from the Go struct fails
validation, and a card carrying only the root three is incomplete to any 1.0
consumer. Every card here carries both halves, which is safe because the
validator checks presence and `encoding/json` ignores unknown keys.

We also fixed it upstream:
[Nasiko-Labs/nasiko#176](https://github.com/Nasiko-Labs/nasiko/pull/176). Until
that merges, the union card is what deploys.

---

## 4. Two front ends, two audiences

This split is a decision, not an accident of having two tools available. They
serve different people doing different things, and neither one would be
improved by absorbing the other.

| | `dronahq/` | `web/` |
|---|---|---|
| **Who** | The brand owner, and the analyst on shift | A judge, a prospect, anyone being shown the product |
| **What they do** | Ask a question, read a brief, approve or reject a draft | Look at one screen and understand the system |
| **When** | Every day, on a phone, often at an awkward hour | Once, for two minutes |
| **Shape** | Conversation plus an ops table | A single composed dashboard |
| **Backend** | Deployed agent URLs, over REST, binding the typed artifacts | The Fastify BFF, and nothing else |
| **Built with** | DronaHQ's console, exported to JSON and committed | Next.js 16, React 19, plain CSS |

**The chat agent** runs on DronaHQ's Chat trigger, and `bp-detector` pushes
alerts in through a Webhook trigger. Its guardrails are written into the
agent's own "Rules & Guardrails" instructions and enforced again server-side by
`bp-responder`: never promise a refund, never admit fault, never commit to a
date, always escalate a crisis to a human. Both places, not either, because a
guardrail that lives only in a prompt is a suggestion.

WhatsApp is out of the MVP. Both triggers above need no external account.
WhatsApp, Slack and email are post-MVP channels that would read the same
`alerts` rows.

**The product dashboard** knows about no agent, no database and no Anakin call.
Its only backend is the BFF. No business rule runs in the browser, because a
rule that runs in the browser is a rule invisible to `nasiko observe`.

Neither surface can post anything. Approving a draft is a handoff to a person.

---

## 5. What the layers may not know about each other

These are the boundaries that keep the thing buildable by seven people at once.

- An agent never imports another agent. Peers are reached through `a2a.Call`
  and the typed models. `agents/bp-x` importing `agents/bp-y` is a build-order
  mistake wearing a shortcut's clothes.
- No agent constructs a peer URL. Addressing is the proxy's job.
- Every agent emits exactly one `application/json` artifact per call, through
  `a2a.JSONArtifact`. DronaHQ and the BFF bind to those structs directly, which
  is why `internal/models/` is frozen.
- Every `main()` is the same twenty lines: build the handler, `obs.Setup`,
  `a2a.Serve`. A longer `main.go` means logic leaked out of the handler.
- `internal/a2a` is the only thing that touches the SDK's executor interface,
  so an SDK version bump is a one-file change.
