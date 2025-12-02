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
`agentFramework` / `framework` (the example says `"starlette"`; ours is
`"a2a-go"`), `llm_provider` (`null` for our five deterministic agents), `tags`,
`transport` (`"http"`). None of these are validated, so a wrong value is
cosmetic, not fatal.

`nasiko deploy` uses `name` + `version` as the image tag and rewrites `version`
back into the file (`sync_card_version`). Bump `version` per deploy or accept
the CLI's rewrite.

Discovery path: `/.well-known/agent-card.json` (also accepts legacy
`/.well-known/agent.json`). In Go that is `a2asrv.WellKnownAgentCardPath` served
by `a2asrv.NewStaticAgentCardHandler(card)`.

### The trap: you cannot generate this file from the Go struct

**`a2a-go` v2.5.0 marshals `a2a.AgentCard` to the A2A 1.0 shape, and Nasiko
validates the A2A 0.2.x shape.** Marshal the struct and three of the eight
required fields are simply absent, because v2 moved them into a
`supportedInterfaces[]` array:

```
$ go run ./probe      # json.MarshalIndent(&a2a.AgentCard{...})
{
  "supportedInterfaces": [
    { "url": "...", "protocolBinding": "JSONRPC", "protocolVersion": "1.0" }
  ],
  ...
}
--- Nasiko REQUIRED_CARD_FIELDS present? ---
name               true
description        true
url                false
version            true
capabilities       true
skills             true
protocolVersion    false
preferredTransport false
```

**One file satisfies both**, because `validate.rs` checks presence and
`encoding/json` ignores unknown keys. Write the union: keep
`supportedInterfaces[]` for the SDK and add `url`, `protocolVersion` and
`preferredTransport` back at the top level for Nasiko. This is the template all
nine agents copy.

```json
{
  "name": "bp-<name>",
  "description": "<one line, this is what routing reads>",
  "version": "1.0.0",
  "url": "http://bp-<name>:8000/invoke",
  "protocolVersion": "1.0",
  "preferredTransport": "JSONRPC",
  "capabilities": { "streaming": true },
  "defaultInputModes": ["application/json"],
  "defaultOutputModes": ["application/json"],
  "supportedInterfaces": [
    { "url": "http://bp-<name>:8000/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0" }
  ],
  "skills": [
    { "id": "<snake_case>", "name": "<human name>", "description": "<what it does>", "tags": ["<tag>"] }
  ]
}
```

Note `"version": "1.0.0"` and not `"1.0"`: the server rejects anything
`parse_plain_version` will not take, with no default applied
(`server/src/agents/upload.rs:475-485`).

### Verified, 2026-09-20, with the real CLI

Both directions, on a Go agent directory holding that card plus the §4
Dockerfile plus a `main.go`:

```
$ nasiko validate
Validating agent at .../agents/bp-template

  ✓ Dockerfile
  ✓ AgentCard.json
  ✓ source directory
  ✓ AgentCard.json fields
  ✓ skills (1 defined)
  ! docker-compose.yml — missing (recommended)
  ! .env.example — missing (recommended)

✓ Valid (2 warning(s))
```

And with the three legacy fields removed, which is exactly what marshalling the
Go struct gives you:

```
  ✗ AgentCard.json — missing fields: url, protocolVersion, preferredTransport

✗ 1 error(s), 2 warning(s)
Error: validation failed
```

The two warnings are noise for us. `docker-compose.yml` and `.env.example` are
looked for **inside the agent directory**; ours live at the repo root.

The round-trip is safe in the other direction too: `json.Unmarshal` of the
superset into `a2a.AgentCard` succeeds and populates `SupportedInterfaces`
correctly. The extra top-level keys are ignored, not an error.

## 3. SDK and server shape

Go module is **`github.com/a2aproject/a2a-go/v2`**, latest `v2.5.0` (tagged
2026-08-18, confirmed on `proxy.golang.org`). Two packages matter: `a2a` for
the core types and constructors, `a2asrv` for the server.

**The v2 executor is an iterator, not an event queue.** This is the single
biggest difference from every Python example and from v1, and guessing it wrong
costs an afternoon:

```go
Execute(ctx context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error]
```

You return a `func(yield func(a2a.Event, error) bool)`. There is no
`event_queue.enqueue_event`. Server construction, from the module's own
`examples/helloworld/server/jsonrpc/main.go`:

```go
executor := a2asrv.AgentExecutorFunc(func(ctx context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		// ... emit events here
	}
})

handler := a2asrv.NewHandler(executor)          // transport-agnostic
mux := http.NewServeMux()
mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(handler))
mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
http.ListenAndServe(":"+port, mux)
```

`*a2asrv.ExecutorContext` carries `Message`, `TaskID`, `ContextID`, `User` and
`Metadata`, and it **implements `a2a.TaskInfoProvider`**, so it passes straight
into the artifact constructors below.

### Open question #1, answered: the JSON artifact helper

There is no one-call helper. Two constructors, and both of the things CONTRACTS
asks for have to be set by hand afterwards:

```go
func a2a.NewDataPart(data any) *a2a.Part
func a2a.NewArtifactEvent(infoProvider a2a.TaskInfoProvider, parts ...*a2a.Part) *a2a.TaskArtifactUpdateEvent
```

`NewDataPart` leaves `MediaType` empty and `NewArtifactEvent` leaves
`Artifact.Name` empty. CONTRACTS requires one `application/json` artifact named
for its `internal/models` struct, so both need assigning:

```go
part := a2a.NewDataPart(report)
part.MediaType = "application/json"
ev := a2a.NewArtifactEvent(ec, part)
ev.Artifact.Name = "SovReport"
```

**That is exactly why `internal/a2a` has a `JSONArtifact` helper.** Four lines
that are easy to get three-quarters right, times nine agents, is how a
dashboard ends up bound to an artifact with no name.

### The wire shape DronaHQ binds to

Marshalled from the real types, not transcribed from a spec:

```
--- naive NewDataPart, no MediaType, no Name ---
{
  "artifact": {
    "artifactId": "01a0be25-f06f-7505-9ed9-b81f9f0c47f3",
    "parts": [ { "data": { "brand_id": "acme", "share": { "acme": 0.42 } } } ]
  },
  "contextId": "ctx-456",
  "taskId": "task-123"
}

--- with MediaType and Name set by hand ---
{
  "artifact": {
    "artifactId": "01a0be25-f070-7153-afb8-0df865890698",
    "name": "SovReport",
    "parts": [
      {
        "data": { "brand_id": "acme", "share": { "acme": 0.42 } },
        "mediaType": "application/json"
      }
    ]
  },
  "contextId": "ctx-456",
  "taskId": "task-123"
}
```

**Two things for whoever writes a binding.** The payload is at
`artifact.parts[0].data`, and there is **no `kind` discriminator** on the part:
v2 discriminates by which content key is present, so a binding that looks for
`"kind": "data"` finds nothing. The `Part.Content` field's `json:"content"` tag
never appears on the wire either; `Part` has a custom marshaller.

## 4. Our Dockerfile (replaces the broken one)

**This is the shared template all nine agents copy.** CONTRACTS §4 puts the
per-agent `Dockerfile` in the agent track's hands and this template in B5's, so
change it here and the change is everyone's. It was Python until 2026-09-20 and
is now Go, built and run by B5 before being written down. Output below.

Build context is the repo root, because every agent imports
`brandpulse/internal`.

```dockerfile
# Build context is the repo root, not this directory:
#   docker build -f agents/bp-<name>/Dockerfile -t bp-<name>:1.0.0 .
# Every agent imports brandpulse/internal, so the context has to see go.mod.

FROM golang:1.27.1 AS build
WORKDIR /src

# go.mod and go.sum first so dependency download caches across source edits.
COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY agents/bp-<name>/ ./agents/bp-<name>/

# CGO_ENABLED=0 is load-bearing: the run stage has no libc to link against.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/agent ./agents/bp-<name>

# static:nonroot, not scratch. It carries the CA bundle that outbound TLS to
# Anakin and the LLM router needs, plus /etc/passwd for the nonroot user.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/agent /agent
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/agent"]
```

**Four things in there are load-bearing and one is not.**

- `CGO_ENABLED=0`. Distroless static has no libc. A cgo-linked binary exits
  immediately with a loader error that looks nothing like a Go panic.
- `gcr.io/distroless/static:nonroot`, not `scratch`. Anakin and the LLM router
  are both HTTPS, and `scratch` has no CA bundle, so every outbound call fails
  with `x509: certificate signed by unknown authority`.
- Repo root as context. `COPY internal/` cannot reach outside the context, so
  building from inside `agents/bp-<name>/` cannot work at all.
- `go.mod`/`go.sum` copied before the source. Without that split, every source
  edit re-downloads the module graph.
- `-ldflags="-s -w"` is the one that is not load-bearing. It strips the symbol
  table. Drop it if you want a readable stack trace more than you want ~20%
  off the binary.

**The binary must read `PORT`.** Nasiko injects it and defaults it to 8000
(`server/src/state.rs:470`), so bind `":"+os.Getenv("PORT")` with 8000 as the
fallback, not a hardcoded 8000.

### Verified, 2026-09-20

Built from a staged copy of this repo's `go.mod` and `internal/models/`, plus a
template `main.go` importing both `brandpulse/internal/models` and
`github.com/a2aproject/a2a-go/v2/a2a`:

```
$ docker build -f agents/bp-template/Dockerfile -t bp-template:2.0.0 .
#13 [build 7/7] RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w"     -o /out/agent ./agents/bp-template
#13 DONE 3.0s
#15 naming to docker.io/library/bp-template:2.0.0 done

$ docker images --format '{{.Repository}}:{{.Tag}}\t{{.Size}}' | grep bp-template
bp-template:2.0.0	14.6MB

$ docker run -d -e PORT=8000 -p 18124:8000 bp-template:2.0.0
$ curl -s http://127.0.0.1:18124/health
{"ok":true}

$ docker inspect bp-template:2.0.0 --format 'User={{.Config.User}} Entrypoint={{.Config.Entrypoint}}'
User=nonroot:nonroot Entrypoint=[/agent]
```

**~15MB per agent, nine agents.** That is the number for the pitch, and it is
measured, not estimated. A real agent with a database driver and an OTel
exporter will be larger; this is the floor.

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
