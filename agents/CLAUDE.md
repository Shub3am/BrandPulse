# agents

## What this module owns

Nine A2A agents, one directory each, every one of them a container Nasiko
deploys. Each directory holds `main.py`, `AgentCard.json`, `Dockerfile` and
`tests/`.

## What it must not know about

Each other's internals. An agent talks to a peer through
`bp_core.a2a.call_agent` and the typed models, never by importing it.

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

- **One `application/json` artifact per call**, emitted through
  `bp_core.a2a.emit_json_artifact`. Never hand-build an artifact.
- **Errors return the normal model with `errors` populated**, they do not raise
  out of the executor. A dead source must not kill a run.
- **`AgentCard.json` uses `protocolVersion: "1.0"`.** The Nasiko example ships
  `"0.2.9"` and a real cluster rejects it with `-32009 VersionNotSupported`.
- **`skills` is never empty** or the agent is invisible to routing.
- **The Dockerfile builds with the repo root as context** so `shared/` is
  available. The example `currency-agent` Dockerfile does not build at all; ours
  is in [docs/research/nasiko.md](../docs/research/nasiko.md) §4.
- **The LLM router ignores the `model` field in the request body.** Per-agent
  model choice is set with `nasiko llm-config`, not in code.
- **No agent constructs a peer URL.** Fan-out caps live with the orchestrator;
  a collector handles one source per call and does not fan out.

## Who calls this

Nasiko routes external traffic in. DronaHQ calls bp-onboarder, bp-briefer and
bp-responder over REST. `demo/` calls bp-orchestrator.
