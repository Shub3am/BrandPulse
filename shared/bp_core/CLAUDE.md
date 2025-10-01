# bp_core

## What this module owns

The shared substrate every agent imports: the wire format, the Anakin client,
the LLM router client, the database pool, statistics, PII redaction, credit
budgeting, and the A2A helpers.

## What it must not know about

Any agent. Nothing here imports from `agents/`. `models.py` additionally imports
nothing that touches the network or the database — it is pure schema, and it
must stay importable with no environment at all.

## Entry points

| File | What it is |
|---|---|
| `models.py` | Every type that crosses an agent boundary. |
| `anakin.py` | The only code in the repo that speaks to Anakin. |
| `llm.py` | `chat_json`, `embed`, `Usage`. The only code that speaks to a model. |
| `a2a.py` | `emit_json_artifact`, `call_agent`. The only code that knows how agents address each other. |
| `db.py` | asyncpg pool and query helpers. |
| `stats.py` | `zscore`, `hour_bucket`, `day_bucket`, baseline windows. |
| `redact.py` | `redact_pii`. |
| `budget.py` | `CreditBudget`. |
| `prompts/` | One markdown prompt per LLM-using agent, plus `GUARDRAILS`. |
| `sources/` | Its own module. See [sources/CLAUDE.md](sources/CLAUDE.md). |

## Invariants and gotchas

- **`models.py` is a frozen contract.** A field rename breaks every agent and
  every DronaHQ binding at once. It changes on `main`, through B1, with
  `docs/CONTRACTS.md` updated in the same commit.
- **`anakin.py` is the blast radius for Anakin being different from what we
  assumed.** Adapters consume normalised dicts, never raw Anakin JSON, so a
  wrong guess is a one-file fix. Keep it that way.
- **Three fixture modes**: `replay` (the default, and what CI uses), `record`
  (once, by B2), `live` (stage only). A missing fixture in `replay` must raise
  with the path it looked for, not fall through to the network.
- **`a2a.py` is the blast radius for Nasiko's addressing.** No agent writes a
  peer URL or constructs an artifact by hand.
- **`zscore` returns 0.0 when std is 0.** A brand with a flat baseline is the
  most likely way the detector dies on stage.
- **Baseline windows exclude the current hour.** Including it damps the very
  spike we are trying to detect.
- **`redact_pii` strips emails and Indian phone formats, and must not touch
  handles or URLs** — those are public and the responder needs them.

## Who calls this

Every agent, `demo/`, and `eval/`. Nothing calls it from outside the repo.
