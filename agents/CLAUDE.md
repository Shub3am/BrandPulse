# agents

## What this module owns

Nine A2A agents in Go, one directory each, every one of them a container Nasiko
deploys. Each directory holds `main.go` (`package main`), `AgentCard.json`,
`Dockerfile` and its `_test.go` files beside the code.

## What it must not know about

Each other's internals. An agent talks to a peer through `a2a.Call` and the
typed models, never by importing it. `agents/bp-x` importing `agents/bp-y` is
a build-order mistake, not a shortcut.

## Entry points

| Agent | One sentence | LLM |
|---|---|---|
| `bp-orchestrator` | Runs the pipeline for one brand and one trigger. | no |
| `bp-onboarder` | Crawls a brand's site into a keyword set. | 1 call |
| `bp-collector` | Fetches one source's mentions for one window. | no |
| `bp-enricher` | Classifies sentiment, intent and aspects in batches. | batched |
| `bp-clusterer` | Groups mentions into topics and names them. | 1 per cluster |
| `bp-detector` | Fires the five statistical alert rules. | **no, ever** |
| `bp-responder` | Drafts a guardrailed reply. | 1 call |
| `bp-briefer` | Writes the daily and weekly brief. | 1 call |
| `bp-sov` | Counts share of voice. | no |

Per-agent input and output signatures are frozen in
[docs/CONTRACTS.md](../docs/CONTRACTS.md) §2.

## Invariants and gotchas

- **Every agent's `main()` is the same twenty lines.** Build the handler, call
  `obs.Setup`, call `a2a.Serve(card, handler)`. If your `main.go` is longer
  than that, logic has leaked out of the handler.
- **One `application/json` artifact per call**, emitted through
  `a2a.JSONArtifact`. Never hand-build an artifact.
- **Handler shape is `Handle(ctx context.Context, in XInput) (Output, error)`**
  on a type named `<Name>Handler`. `internal/a2a` adapts that to the SDK's
  executor interface, so no agent implements the SDK interface directly and an
  SDK version bump is a one-file change.
- **Errors return the normal struct with `Errors` populated**, they do not
  return a non-nil error out of the handler. A dead source must not kill a run.
  The `error` return is for malformed input only.
- **Zero values are the hazard.** Go has no field defaults. Build domain types
  with their `New*` constructor and call `Validate()` before persisting. See
  [internal/models/CLAUDE.md](../internal/models/CLAUDE.md).
- **Nasiko auto-injects OpenTelemetry for Python containers only.** Go agents
  self-instrument: `obs.Setup(serviceName)` in `main()`, deferred shutdown.
  Skip it and the agent is invisible to `nasiko observe`. Note that
  **`OTEL_EXPORTER_OTLP_ENDPOINT` is injected for nobody**, Python included, so
  B5 sets it as a vault-wide secret at deploy time. Nothing changes in this
  directory because of that.
- **Never deploy through the Nasiko dashboard zip uploader, and never through
  `nasiko upload` either.** Both post to `/api/agents/upload`, whose
  `validate_agent_zip` check requires a `main.py` and rejects a Go agent with a
  `400`. `nasiko deploy` does not go near that route: it branches on
  `AgentCard.json`, builds locally and pushes the image. **Deploy, never
  upload.** One letter apart, one fatal.
- **`AgentCard.json` uses `protocolVersion: "1.0"`.** The Nasiko example ships
  `"0.2.9"` and a real cluster rejects it with `-32009 VersionNotSupported`.
- **You write your own `AgentCard.json` and `Dockerfile`**, from the templates
  in `docs/research/nasiko.md` §2 and §4. B5 owns the templates and the deploy
  and edits neither file: a bad card comes back to you as a blocker row.
- **The card is a union of two shapes and needs both halves.** `a2a-go` v2.5.0
  marshals to A2A 1.0, which moves `url`, `protocolVersion` and
  `preferredTransport` into `supportedInterfaces[]`. Nasiko's `validate.rs`
  requires all three at the **top level**. So generating the card from the Go
  struct produces a card that fails `nasiko validate`, and a card with only the
  top-level three is invisible to an A2A 1.0 consumer. Write both. It is safe
  because the validator checks presence and `encoding/json` ignores unknown
  keys. B5 confirmed this in both directions against the real CLI.
- **`skills` is never empty** or the agent is invisible to routing. One skill
  per card with a real `id`, `description` and `examples`. Empty `skills` is
  only a warning to `validate`, which is why it is easy to ship broken.
- **`llm_provider: null` on the four agents that make no LLM call**:
  bp-collector, bp-sov, bp-detector, bp-orchestrator. It is true, and it is the
  kind of true a judge notices. The other five do call an LLM.
- **`nasiko deploy` rewrites `version` back into your card** via
  `sync_card_version`, and tags the image `name` + `version`. Expect a dirty
  tree after a deploy rather than being surprised by one. It also writes
  `.nasiko/agent.json` caching the agent id: that is gitignored, and keeping it
  locally is what makes a redeploy update the agent instead of duplicating it.
- **The Dockerfile builds with the repo root as context** so `internal/` and
  `go.mod` are available, and it is multi-stage: `golang:1.27` to build with
  `CGO_ENABLED=0`, then a distroless static run stage holding one binary. The
  example `currency-agent` Dockerfile does not build at all.
- **The LLM router ignores the `model` field in the request body.** Per-agent
  model choice is set with `nasiko llm-config`, not in code.
- **No agent constructs a peer URL.** `a2a.Call` resolves one by substituting
  `{agent}` into the `NASIKO_API_URL` template, so **a peer address is deploy
  configuration, never compiled in**. Two templates are in use: the control
  plane's `https://<host>/api/agents/{agent}` and compose's
  `http://{agent}:8000/`. Which one is wrong is only wrong at runtime, so
  neither belongs in Go.
- **Every agent listens on 8000.** `a2a.Serve` reads `PORT` and falls back to
  `8000`, Nasiko's injector defaults `PORT` to 8000, every `Dockerfile` here
  says `EXPOSE 8000` and every `AgentCard.json` url is `:8000`. Those four have
  to agree, and 8080 is the number people reach for by habit.
- **All nine come up locally as `docker compose -f docker-compose.yml -f
  docker-compose.agents.yml up -d`.** The agents file layers onto the Postgres
  compose project and does not work on its own, because `postgres` is defined
  in the other one. It sets `NASIKO_API_URL` to `http://{agent}:8000/` and sets
  no `PORT`, so the fallback in `a2a.Serve` stays the single source of 8000.
- Fan-out caps live with the orchestrator; a collector handles one source per
  call and does not fan out.

## Who calls this

Nasiko routes external traffic in. DronaHQ calls bp-onboarder, bp-briefer and
bp-responder over REST. `demo/` calls bp-orchestrator.
