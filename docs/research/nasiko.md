# Nasiko — verified findings

Researched 2026-09-20 against https://github.com/Nasiko-Labs/nasiko (Apache-2.0,
Rust control plane, ~6.6k stars, last push 2026-09-14). Read this before
writing any AgentCard, Dockerfile or deploy script. Anything marked
**UNVERIFIED** must be confirmed against a running cluster, not assumed.

---

## 1. Three traps in the official example

`agents/currency-agent/` is the template the brief tells us to copy. It has
three problems. Copy the *structure*, not the bugs.

**Trap 1 — the Dockerfile does not build.** It is, verbatim:

```dockerfile
FROM python:3.13-slim
WORKDIR /app
COPY pyproject.toml .
RUN pip install --no-cache-dir .
COPY currency-agent/main.py .
EXPOSE 8000
CMD ["python", "main.py", "--host", "0.0.0.0", "--port", "8000"]
```

There is no `pyproject.toml` at `agents/` or at `agents/currency-agent/`
anywhere in the repo. The `COPY` fails. We write our own Dockerfile per agent
(see §4) and do not inherit this.

**Trap 2 — `protocolVersion` is stale.** The example ships `"0.2.9"`. The
repo's own `docs/A2A_PROTOCOL.md` says Nasiko requires spec **v1.0** and
rejects anything else with `-32009 VersionNotSupported`. **Every BrandPulse
AgentCard uses `"protocolVersion": "1.0"`.**

**Trap 3 — the default flow guard is wider than our brief.** Defaults are
fan-out 20, depth 5. The brief specifies fan-out ≤ 8, depth 3. We set those
explicitly (§5).

## 2. AgentCard — what actually gates a deploy

The CLI's `cli/src/commands/validate.rs` is ground truth, not the docs. It
requires exactly these fields:

```
name, description, url, version, capabilities, skills,
protocolVersion, preferredTransport
```

plus the presence of both `AgentCard.json` and `Dockerfile`. Empty `skills` is
a warning, not an error — but an agent with no skills is invisible to the
orchestrator's routing, so every BrandPulse agent declares at least one.

Nasiko-specific non-spec fields seen in the example and worth setting:
`agentFramework` / `framework` (`"starlette"`), `llm_provider` (`null` for our
five deterministic agents), `tags`, `transport` (`"http"`).

`nasiko deploy` uses `name` + `version` as the image tag and rewrites `version`
back into the file (`sync_card_version`). Bump `version` per deploy or accept
the CLI's rewrite.

Discovery path: `/.well-known/agent-card.json` (also accepts legacy
`/.well-known/agent.json`). The `a2a` SDK's `create_agent_card_routes` serves
this for us.

## 3. SDK and server shape

PyPI package is **`a2a-sdk`**, pinned in the repo's working template as
`a2a-sdk[http-server]==1.1.0`. Note the import namespace is `a2a`, the package
name is `a2a-sdk`.

The server construction pattern, copied from the working example:

```python
from a2a.server.agent_execution import AgentExecutor, RequestContext
from a2a.server.events import EventQueue
from a2a.server.request_handlers import DefaultRequestHandler
from a2a.server.routes import create_agent_card_routes, create_jsonrpc_routes
from a2a.server.tasks import InMemoryTaskStore
from a2a.types import AgentCapabilities, AgentCard, AgentInterface, AgentSkill, TaskState
from a2a.helpers import (new_task_from_user_message,
                         new_text_artifact_update_event,
                         new_text_status_update_event)
from starlette.applications import Starlette

class SomethingExecutor(AgentExecutor):
    async def execute(self, context: RequestContext, event_queue: EventQueue) -> None:
        task = context.current_task or new_task_from_user_message(context.message)
        await event_queue.enqueue_event(task)
        # ... working status, then artifact, then completed status
    async def cancel(self, context, event_queue) -> None:
        pass

handler = DefaultRequestHandler(agent_executor=SomethingExecutor(),
                                task_store=InMemoryTaskStore(),
                                agent_card=agent_card)
routes = [*create_agent_card_routes(agent_card),
          *create_jsonrpc_routes(handler, rpc_url="/")]
app = Starlette(routes=routes)
```

The example emits **text** artifacts via `new_text_artifact_update_event`. We
need **JSON** artifacts (`application/json`) because DronaHQ binds to them
directly. B5 confirms the `a2a-sdk` 1.1.0 helper for a data/JSON part on first
contact with the SDK and puts the answer in `shared/bp_core/a2a.py` as a single
`emit_json_artifact(...)` helper every agent calls. **UNVERIFIED** which helper
name that is; if none exists, construct the `Artifact` with a `DataPart`
directly. Do not let nine agents each invent this.

## 4. Our Dockerfile (replaces the broken one)

Build context is the repo root, because every agent needs `shared/bp_core`.

```dockerfile
FROM python:3.13-slim
WORKDIR /app
COPY shared/ /app/shared/
RUN pip install --no-cache-dir /app/shared
COPY agents/bp-<name>/main.py /app/main.py
EXPOSE 8000
CMD ["python", "main.py", "--host", "0.0.0.0", "--port", "8000"]
```

Built with `docker build -f agents/bp-<name>/Dockerfile .` from the repo root.

## 5. Flow guards — env vars on the control plane

Configured purely by environment variables read in `flow/src/guard.rs`
(`FlowConfig::from_env`). There is no config file. State lives in Redis, keyed
`flow:{flow_id}`; if Redis is down the guard **fails closed** and rejects every
call with `GuardUnavailable`. Worth knowing before the demo.

| Env var | Default | BrandPulse value | Why |
|---|---|---|---|
| `NASIKO_FLOW_MAX_DEPTH` | 5 | **3** | orchestrator → collector → (nothing deeper) |
| `NASIKO_FLOW_MAX_FAN_OUT` | 20 | **8** | the brief's source cap; this is what makes the degradation demo fire |
| `NASIKO_FLOW_MAX_TOKENS` | 100000 | **60000** | per-brand-day token ceiling |
| `NASIKO_FLOW_TIMEOUT_SECS` | 120 | **180** | a 7-source run with clustering needs headroom |
| `NASIKO_FLOW_STATE_TTL_SECS` | 300 | default | — |

The guard also detects cycles independently of depth (`CycleDetected`).

**This is the demo moment.** A brand with more enabled sources than the fan-out
cap forces the orchestrator to drop the lowest-yield sources and record
`degraded_reason`. Set the cap to 8 and enable 10 sources on the demo brand to
make it fire on cue.

## 6. LLM router — two findings that change our design

At deploy time `llm-router/src/inject.rs` mints a 1-year agent-identity JWT and
injects `OPENAI_BASE_URL={LLM_GATEWAY_BASE_URL}/v1` and
`OPENAI_API_KEY=<that JWT>`. The stock `openai` Python SDK is used unmodified:

```python
AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY"),
            base_url=os.getenv("OPENAI_BASE_URL"))
```

Spend is metered server-side into a Postgres `token_usage` table keyed by
`agent_id` / `owner_id` from the JWT, with cost computed by a DB trigger
against `model_pricing`. `nasiko observe` surfaces it. **This is the per-brand
cost number we show in the demo** — we do not have to build it, we read it.

**Finding A — the `model` field is ignored.** The router discards the
request's `model` and resolves the real model server-side from the agent's
`llm_config` and the `model_registry` tier mapping. Consequence:
`BP_MODEL_SMALL` cannot be honoured by passing `model=` in the call. We still
send it (harmless, and it works against a plain OpenAI endpoint in local dev),
but per-agent model selection on Nasiko is configured with
`nasiko llm-config` per agent, not in our code. B5 owns that configuration.

Catalog at `GET /v1/models` lists `openai/gpt-4o`, `openai/gpt-4o-mini`,
`anthropic/claude-3-5-sonnet-20241022`, `gemini/gemini-1.5-pro`. An OpenRouter
provider was merged 2026-09-14 and may not be listed yet.

**Finding B — no embeddings endpoint is documented.** The router's catalog and
handlers cover chat completions. Nothing verifies that `/v1/embeddings` is
proxied. This directly threatens bp-clusterer as specified.

**Decision:** bp-clusterer's default vectoriser is **local TF-IDF**
(`sklearn.feature_extraction.text.TfidfVectorizer`) with cosine distance and
agglomerative clustering. No network, no tokens, deterministic, and it runs in
CI for free. `bp_core.llm.embed` is implemented behind a feature flag
(`BP_VECTORISER=tfidf|embeddings`) and we switch to router embeddings only
after someone confirms the endpoint exists on a live cluster. The LLM is still
used for cluster *labelling*, which is the part that actually needs it.

This is a better default anyway: 300 mentions is far too small a corpus for
embedding quality to beat TF-IDF on topic separation, and it removes a network
dependency from the demo path.

## 7. Agent-to-agent calls

Agents are not publicly reachable and never call each other directly. Every
hop is proxied by `nasiko-server`:

- Direct: `POST /api/agents/{agent_id}` — server resolves the container
  endpoint, injects trust headers, `A2A-Version: 1.0` and `traceparent`.
- LLM-routed: `POST /api/orchestrator/a2a` — the control plane picks the target.
  We do **not** use this; our orchestration is explicit, which is the point of
  a deterministic pipeline.

**ANSWERED 2026-09-20 by B5, from source at commit `58cfe60`: there is no such
env var.** This section previously guessed at a `NASIKO_PROXY_URL`. It does not
exist and never did.

`ServerState::agent_env` (`server/src/state.rs:462-472`) is the one function
that builds a deployed container's environment, and it injects exactly: the
agent's own secrets, `OPENAI_API_KEY` / `OPENAI_BASE_URL` / `OPENAI_MODEL`, and
`PORT` defaulted to 8000. No base URL, no trust credential. All nine
Nasiko-shipped example agents were grepped and **none of them calls another
agent**, which is why no example exists to copy.

The only agent-facing credential is `x-nasiko-agent-token`, a delegation JWT
the server mints and sends **inbound** to the agent
(`server/src/router/a2a_dispatch.rs:796-811`), minted only when the server has
`JWT_SECRET` and best-effort otherwise.

**So `a2a.Call` (CONTRACTS §3) supplies both halves itself:**

1. **Base URL.** We set one, as a vault-wide secret. Suggest `NASIKO_API_URL`.
   That name is ours, not Nasiko's, so do not go looking for it in their docs.
2. **Credential.** Replay the inbound `x-nasiko-agent-token` off the request the
   caller is already serving. That is a per-request value, so it threads through
   `a2a.Call`'s `ctx`. It cannot live in a package-level client.

Point 2 is inference from how the server mints and consumes the token, not a
documented contract, and it is confirmed on the first live two-agent call.
Detail in [DEPLOY-NOTES.md](../DEPLOY-NOTES.md) Finding 5.

Client side uses `A2ACardResolver` + `A2AClient` from `a2a.client`.

## 8. CLI

```bash
cargo install --path cli/          # from a clone of the nasiko repo
nasiko connect <control-plane-url>
nasiko auth login
nasiko validate                    # checks AgentCard + Dockerfile before deploy
nasiko deploy . --name bp-collector --env-file .env
nasiko secrets set ANAKIN_API_KEY <key> --agent bp-collector
nasiko logs bp-collector -f
nasiko ps --json
nasiko observe                     # traces, spans, FinOps — the cost demo
nasiko llm-config                  # per-agent model selection (see §6 Finding A)
```

Secret precedence, highest first: inline `-e` on deploy, agent-specific
secrets, vault-wide secrets. Secrets are AES-256-GCM at rest and injected only
at deploy/restart. `nasiko deploy` writes `.nasiko/agent.json` caching the
agent id — **gitignore it**, and keep it so a redeploy updates rather than
duplicates the agent.

## 9. Contributing the PR

`agents/` is a flat directory of standalone folders. There is **no registry or
index file to update** when adding an agent. Touch `agents/THIRD_PARTY_LICENSES.md`
only if a Dockerfile installs a third-party binary at build time.

Repo PR conventions: one logical change per PR, clear what/why. The public
mirror shows no PR-triggered test workflow (commits are `[synced-from-private]`),
so CI gating is **UNVERIFIED**.

Our PR adds `agents/brandpulse-*/` plus `agents/brandpulse/README.md` linking
the product repo and explaining the topology.

---

## Confidence

**Solid** (read from source): repo identity, the three example files verbatim,
`validate.rs` required fields, `guard.rs` env keys and defaults, `inject.rs`
injection behaviour, stock-OpenAI-SDK usage, CLI commands, contributing flow.

**Shaky, verify on a live cluster before the demo:** the JSON-artifact helper
name in `a2a-sdk` 1.1.0; whether `/v1/embeddings` is proxied at all; whether PR
CI exists; the OTLP collector address reachable from inside a container;
whether replaying `x-nasiko-agent-token` authenticates a peer call (§7).

The agent-to-agent proxy env var came off this list on 2026-09-20 by being
answered, not verified: it does not exist. See §7.

Open these: the [repo](https://github.com/Nasiko-Labs/nasiko),
[A2A_PROTOCOL.md](https://github.com/Nasiko-Labs/nasiko/blob/main/docs/A2A_PROTOCOL.md),
[AGENT_LIFECYCLE.md](https://github.com/Nasiko-Labs/nasiko/blob/main/docs/AGENT_LIFECYCLE.md),
[validate.rs](https://github.com/Nasiko-Labs/nasiko/blob/main/cli/src/commands/validate.rs),
[guard.rs](https://github.com/Nasiko-Labs/nasiko/blob/main/flow/src/guard.rs),
[inject.rs](https://github.com/Nasiko-Labs/nasiko/blob/main/llm-router/src/inject.rs),
and https://docs.nasiko.com.
