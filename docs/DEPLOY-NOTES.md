# Deploy notes

**Every BrandPulse agent reaches the cluster through `nasiko deploy`. Nobody
drags a zip into the dashboard, not once, not to "just try it".**

Running log of what Nasiko actually does, written as it is discovered. Source
citations are file:line in `Nasiko-Labs/nasiko` at commit
`58cfe600559c67d58100ec2856d7b29838e2859f` (2026-09-14), read from a local
clone. Anything marked **source-read** has not yet been confirmed against a
live cluster, because there is no Nasiko credential yet. Findings 7 and 8 are
different: those were built and run locally, and the output is pasted.

---

## Status

Tasks 1, 2 and 3 cannot complete today. Two independent blockers:

1. **No Nasiko credential.** `nasiko connect <url>` and `nasiko auth login`
   have no control plane to point at. Logged in `HACKATHON_NOTES.md`.
2. **There is no agent to deploy.** `agents/` holds only `CLAUDE.md`;
   `internal/` holds only `models/models.go`. No `bp-sov`, no `internal/a2a`,
   no `internal/obs`, no `internal/models/agentio.go`.

What follows was read from Nasiko's source instead, so that the moment both
blockers clear the deploy is mechanical rather than exploratory. Everything up
to and including `nasiko validate` has since been run for real against a
template agent, which is as far as the toolchain goes without a cluster.

## Toolchain

The CLI is Rust and is not distributed as a binary. Installing it costs a Rust
toolchain first:

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal
git clone --depth 1 https://github.com/Nasiko-Labs/nasiko.git
cd nasiko && cargo install --path cli/
```

```
$ nasiko --version
nasiko 0.1.0
```

Installed at `~/.cargo/bin/nasiko`. Anyone reproducing this needs `~/.cargo/bin`
on `PATH`.

---

## Finding 1: `nasiko upload` is barred for Go agents, not only the dashboard

The B5 brief says the `main.py` gate "guards the **dashboard zip-upload path
only**" and that "`nasiko deploy` from the CLI, GitHub import and catalog import
all bypass it entirely". Half of that is right and the half that is wrong is a
CLI command sitting one letter away from the one we want.

`validate_agent_zip` (`server/src/agents/upload.rs:716-747`) requires a
`Dockerfile` with a `FROM` line **and** one of `main.py`, `src/main.py`,
`__main__.py`, `src/__main__.py`. No Python entrypoint is a `400`.

That validator guards the server route `/api/agents/upload`. The **CLI's
`nasiko upload` command posts to exactly that route**
(`cli/src/commands/upload.rs:17` — *"POST multipart to /api/agents/upload"*),
zipping the directory first if you hand it one.

`nasiko deploy` does not go near it. `deploy_with_version_flags`
(`cli/src/commands/deploy.rs:26-63`) branches on whether the directory contains
an `AgentCard.json`, then builds the image locally and pushes it to the cluster
OCI registry. No zip, no server-side source validation.

**So the rule is wider than the brief states:** `nasiko deploy` is the path,
and `nasiko upload` is as fatal as the dashboard. Do not reach for it when a
deploy fails.

## Finding 2: `nasiko validate` accepts Go, and only warns

Worth knowing before anyone panics at the word "Python".
`cli/src/commands/validate.rs:37-46` checks for `src/`, **`cmd/`, or
`main.go`** — and a miss is a *warning*, not an error. Our
`agents/bp-<name>/main.go` satisfies it outright.

Errors, from `REQUIRED_FILES` and `REQUIRED_CARD_FIELDS`
(`validate.rs:6-17`), are only: missing `Dockerfile`, missing
`AgentCard.json`, unreadable or non-JSON card, and any of these eight card
fields absent — `name`, `description`, `url`, `version`, `capabilities`,
`skills`, `protocolVersion`, `preferredTransport`. This matches
`research/nasiko.md` §2 exactly.

Warnings only: empty `skills`, no source directory, and no
`docker-compose.yml` or `.env.example` **in the agent directory**. The last two
are noise for us; ignore them.

## Finding 3: `version` in AgentCard.json must be `x.y.z`

`server/src/agents/upload.rs:475-485` rejects anything
`parse_plain_version` does not accept, with no default applied:

> `version_tag is required and must be in x.y.z format (e.g. 1.2.3)`

Resolution order is AgentCard.json → `pyproject.toml` → `Cargo.toml`
(`detect_version_from_dir`, `upload.rs:762-790`), so for us it is the card or
nothing. `"version": "1.0"` fails. Write `"1.0.0"`.

## Finding 4: OTel — nothing injects `OTEL_EXPORTER_OTLP_ENDPOINT` into a deployed agent

This is Task 2's first checkbox answered ahead of the deploy, and it is worse
than the brief assumed, in a way that is cheap to fix.

The brief expected Nasiko to inject OTel into Python containers and skip ours.
What the source shows is that the **endpoint variable is not injected for any
language**.

`ServerState::agent_env` (`server/src/state.rs:462-472`) is the one function
that builds a deployed agent's environment. It is:

```
agent secrets (resolve_agent_env)
  + OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL   (platform_fallback_env, state.rs:449-459)
  + PORT, defaulted to 8000                          (state.rs:470)
```

`OTEL_EXPORTER_OTLP_ENDPOINT` is not in it. Grepping every caller of
`agent_env` (`catalog/import.rs`, `agents/deployments.rs`, `agents/update.rs`,
`agents/build_worker.rs`, `agents/upload.rs`) finds no later insertion of it
either. The only place the platform sets it on a container is
`server/src/seed.rs:133-134`, and that is the **seed path** for Nasiko's own
built-in demo agents, not the deploy path.

What Python *does* get is different and does not help us: the build pipeline
patches an OTel bootstrap script into the Dockerfile via `PYTHONSTARTUP`
(`upload.rs:858-920`). That script installs instrumentation and then reads
`OTEL_EXPORTER_OTLP_ENDPOINT` at line 891 — the same variable nothing sets. So
Python gets the libraries wired up and still needs the endpoint supplied.

**What this changes for us.** Nothing about B1's `internal/obs`: Go still
self-instruments, exactly as the ADR says, and `obs.Setup` reading
`OTEL_EXPORTER_OTLP_ENDPOINT` is still correct. What changes is that **B5 must
supply the variable**, and it is not a code fix:

```bash
nasiko secrets set OTEL_EXPORTER_OTLP_ENDPOINT <collector-url>   # vault-wide
```

`nasiko deploy` also accepts `--env-file` and `-e KEY=VALUE`
(`cli/src/commands/deploy.rs:141-167`), so it can go in inline, but vault-wide
is right for nine agents.

**Still unknown and needs the live cluster:** the collector's actual address as
seen from inside an agent container. Source-read cannot give that.

This is not a deploy blocker in the way the brief feared — it does not need
B1 to change anything — but forgetting it means nine agents deploy silent and
`nasiko observe` has no spans, which is the "₹11 for today's run" beat gone.

## Finding 5: agent-to-agent calls — there is no injected proxy env var

This answers Open question #2, which the brief hands to B5 Task 1 for B1 to
implement. `research/nasiko.md` §7 guessed at a `NASIKO_PROXY_URL`. **No such
variable exists.**

`agent_env` (above) injects the OpenAI trio and `PORT`. That is the whole list.
Nothing carries a proxy base URL and nothing carries a trust credential.

How the platform actually works (`docs/A2A_PROTOCOL.md:660-740`):

- The **server is the sole ingress**. Agent containers are internal and not
  directly reachable.
- The caller reaches a peer at `POST /api/agents/{agent_id}` (or
  `/api/agents/{id}/{*rest}`), and the server resolves the container endpoint,
  proxies to the agent's root path, and injects the trust headers `x-user-id`,
  `x-username`, `x-is-superuser`, plus `A2A-Version: 1.0` and `traceparent`.
- The only agent-facing credential is `x-nasiko-agent-token`, a delegation JWT
  the server **mints and sends inbound** to the agent
  (`server/src/router/a2a_dispatch.rs:796-811`), proving "I am this agent id,
  acting for this user". It is minted only when the server has `JWT_SECRET`,
  and is best-effort otherwise.
- All nine Nasiko-shipped example agents were grepped for a peer call. **None
  of them calls another agent.** There is no first-party example to copy.

**Consequence for `a2a.Call` (B1's, per CONTRACTS §3).** It needs two things
neither of which arrives for free:

1. A control-plane base URL. Nothing injects one, so we set it ourselves as a
   vault-wide secret. Suggest `NASIKO_API_URL`, chosen by us, not by Nasiko.
2. A credential. The sanest available shape is for `bp-orchestrator` to read
   the inbound `x-nasiko-agent-token` off the request it is already serving and
   replay it on the outbound peer call. That is a per-request value, so it has
   to thread through `a2a.Call`'s `ctx`, not sit in a package-level client.

Point 2 is an inference from how the server mints and consumes the token, not
something the docs state. **Flag it to B1 as a design constraint, and confirm
it against a live two-agent call before the fleet deploys.**

## Finding 6: flow guard defaults and the Redis fail-closed, confirmed

`flow/src/guard.rs:29-33`, verbatim:

| Var | Default |
|---|---|
| `NASIKO_FLOW_MAX_DEPTH` | 5 |
| `NASIKO_FLOW_MAX_FAN_OUT` | 20 |
| `NASIKO_FLOW_MAX_TOKENS` | 100000 |
| `NASIKO_FLOW_TIMEOUT_SECS` | 120 |
| `NASIKO_FLOW_STATE_TTL_SECS` | 300 |

The brief's four settings are all tighter than the defaults, so all four must
be set explicitly or we silently run at fan-out 20 and depth 5. There is a
fifth, `NASIKO_FLOW_STATE_TTL_SECS`, the brief does not mention; the 300s
default outlives our 180s flow timeout, so leave it alone.

These are read with `std::env::var` in the **flow service's own process**
(`guard.rs:38-43`), not in an agent container. They are control-plane
configuration, not a deploy flag.

**Redis is the sole backing store** for depth, cycle, fan-out and token state
(`guard.rs:105-113`), and an unreachable Redis returns `GuardUnavailable` —
*"flow guard unavailable (redis unreachable) — failing closed"*
(`guard.rs:67-98`). **Confirm Redis is up before blaming an agent.** A fleet
where every call is rejected is a Redis problem, not nine bugs.

## Finding 7: an AgentCard marshalled from the Go struct fails `nasiko validate`

This one would have cost the first deploy, and it is the first thing on this
page that is **not** source-read: it was run.

`a2a-go` v2.5.0 marshals `a2a.AgentCard` to the A2A **1.0** shape, which moves
`url`, `protocolVersion` and `preferredTransport` into a
`supportedInterfaces[]` array. Nasiko's `validate.rs` requires all three at the
**top level**. So the obvious move, generate the card from the struct, produces
a card missing three of the eight required fields.

Confirmed in both directions on a Go agent directory. With the union card:

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

With exactly those three fields deleted, which is what the struct gives you:

```
  ✗ AgentCard.json — missing fields: url, protocolVersion, preferredTransport

✗ 1 error(s), 2 warning(s)
Error: validation failed
```

**The fix is one file, not two.** `validate.rs` checks presence and
`encoding/json` ignores unknown keys, so a card carrying both shapes satisfies
Nasiko and unmarshals into `a2a.AgentCard` with `SupportedInterfaces` populated.
Template in `research/nasiko.md` §2.

## Finding 8: the Go image is 14.6MB, measured

The §4 Dockerfile template was Python until today. It is Go now, and it was
built and run before being written down: `golang:1.27.1` build stage with
`CGO_ENABLED=0`, `gcr.io/distroless/static:nonroot` run stage, repo root as
build context.

```
$ docker images --format '{{.Repository}}:{{.Tag}}\t{{.Size}}' | grep bp-template
bp-template:2.0.0	14.6MB

$ docker run -d -e PORT=8000 -p 18124:8000 bp-template:2.0.0
$ curl -s http://127.0.0.1:18124/health
{"ok":true}

$ docker inspect bp-template:2.0.0 --format 'User={{.Config.User}} Entrypoint={{.Config.Entrypoint}}'
User=nonroot:nonroot Entrypoint=[/agent]
```

That is a template agent importing `brandpulse/internal/models` and
`github.com/a2aproject/a2a-go/v2/a2a`. A real agent adds a database driver and
an OTel exporter, so treat 14.6MB as the floor, not the number for nine.

**`static:nonroot`, not `scratch`.** Anakin and the LLM router are both HTTPS
and scratch ships no CA bundle, so every outbound call would fail with `x509:
certificate signed by unknown authority` after a clean build and a green test
suite.

---

## Open, needs a live cluster

- The OTLP collector address reachable from inside an agent container.
- Whether `x-nasiko-agent-token` replay actually authenticates a peer call.
- Whether `nasiko observe` shows a span from a self-instrumented Go container.
- `nasiko push`, `nasiko deploy`, `nasiko secrets`, `nasiko ps`, `nasiko logs`.
  Everything up to and including `nasiko validate` is now verified locally;
  nothing past it is.
- The real `bp-sov` image size. Finding 8 measures a template, not the agent.
