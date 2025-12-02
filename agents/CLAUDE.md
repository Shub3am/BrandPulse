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
- **`skills` is never empty** or the agent is invisible to routing.
- **The Dockerfile builds with the repo root as context** so `internal/` and
  `go.mod` are available, and it is multi-stage: `golang:1.27` to build with
  `CGO_ENABLED=0`, then a distroless static run stage holding one binary. The
  example `currency-agent` Dockerfile does not build at all.
- **The LLM router ignores the `model` field in the request body.** Per-agent
  model choice is set with `nasiko llm-config`, not in code.
- **No agent constructs a peer URL.** Fan-out caps live with the orchestrator;
  a collector handles one source per call and does not fan out.

## Who calls this

Nasiko routes external traffic in. DronaHQ calls bp-onboarder, bp-briefer and
bp-responder over REST. `demo/` calls bp-orchestrator.
