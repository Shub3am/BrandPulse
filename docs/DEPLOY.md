# DEPLOY

**Still needed from the repo owner before any of this runs: the Nasiko control plane URL and a working login. Neither is recorded in this file or anywhere else in this repo.**

This is the runbook a human follows on stage to put all nine BrandPulse agents on Nasiko. Everything below was checked against the repo on 2026-09-20 and against the live Nasiko docs at `docs.nasiko.com` the same day. Where the live docs are silent I say so on the line.

**The sanctioned path is the `deploy` GitHub Actions workflow** (`.github/workflows/deploy.yml`, `workflow_dispatch`), and the root `CLAUDE.md` says to deploy that way and never `nasiko deploy` from a laptop. That workflow installs the CLI from source, sets the same secrets, validates, and runs the same `nasiko deploy` per agent. Use it. This document is the manual equivalent: what to type if the workflow is unavailable on the day, and what every step of it actually does so a human can read a failure. The two must agree; if they drift, the workflow is what runs.

## The one mistake that kills the demo

Deploy with `nasiko deploy`. Never `nasiko upload`, and never the dashboard's zip uploader.

`nasiko upload` and the dashboard uploader both post to `/api/agents/upload`, and that endpoint's `validate_agent_zip` requires a `main.py`. Every BrandPulse agent is Go. The server answers 400 and the agent never exists. `nasiko deploy` takes a different path: it branches on `AgentCard.json`, builds the image locally with Docker and pushes it to the cluster registry. One letter apart, one fatal.

So: `deploy`, not `upload`. If someone on the laptop reaches for the dashboard because the CLI looks slow, stop them.

## Five things to expect

1. **A dirty git tree after every deploy.** `nasiko deploy` tags the image from `name` plus `version` out of `AgentCard.json`, and `sync_card_version` writes `version` back into the card. Nine deploys means nine modified `AgentCard.json` files in `git status`. That is normal. Do not revert it mid-demo and do not commit it mid-demo either.

2. **`.nasiko/agent.json` must survive.** `nasiko deploy` writes it, caching the agent id. It is gitignored (`.gitignore`, last block). Keeping the local copy is exactly what makes a second deploy *update* the agent instead of creating a duplicate with the same name. Do not `rm -rf .nasiko` between deploys, and do not deploy from a fresh clone that never had it.

3. **LLM traffic goes only through the injected `OPENAI_BASE_URL`.** At deploy time the router mints a per-agent JWT and injects `OPENAI_API_KEY`, `OPENAI_BASE_URL` and `OPENAI_MODEL` into the container. `internal/llm/llm.go` is the only caller and it reads `OPENAI_BASE_URL` at line 212. No agent configures a provider SDK.

4. **Per-agent model choice is `nasiko llm-config`, not code.** The router discards the `model` field in the request body and resolves the real model server-side from the agent's llm-config and the model registry tier mapping. Setting `OPENAI_MODEL` or `BP_MODEL_SMALL` locally changes nothing once deployed. Leave them empty on the cluster.

5. **`OTEL_EXPORTER_OTLP_ENDPOINT` is injected for nobody.** `ServerState::agent_env` injects exactly the agent's own secrets, `OPENAI_API_KEY` / `OPENAI_BASE_URL` / `OPENAI_MODEL`, and `PORT`. That is the whole list, and it is the same list for Python agents. `internal/obs/obs.go` line 42 makes tracing a no-op when the variable is empty, so a Go agent without it is silent rather than broken, which is worse: it comes up healthy and is invisible to `nasiko observe`. Set it as a **vault-wide secret before the first deploy**.

   Note that `.env.example` line 48 currently claims Nasiko injects this. It does not. The code and `docs/research/nasiko.md` section 7 agree it does not. Treat `.env.example` as stale on that one line.

## Ports and health

Every agent listens on 8000. `internal/a2a/serve.go` reads `PORT` and falls back to 8000; Nasiko injects `PORT` and defaults it to 8000 too, so the two agree. Every `AgentCard.json` `url` is `http://bp-<name>:8000/`, and every Dockerfile has `EXPOSE 8000`.

The health path is `/healthz`, registered in `internal/a2a/serve.go` line 84. It is not part of A2A and **Nasiko does not probe it**. It exists for us.

The run image is `gcr.io/distroless/static-debian12:nonroot`. It has no shell, no curl and no wget. A health probe therefore has to come from outside the container. Locally that means `curl` from the host against a published port. On the cluster it means `nasiko ps`, `nasiko logs` and `nasiko chat`, not an exec into the container.

## What a human types, in order

### 0. Before you touch the cluster

```bash
cd /path/to/brandpulse
python3 scripts/check_agent_cards.py
```

Expect nine `ok` lines and exit 0. This is the half of `nasiko validate` that needs no cluster, so it fails on a laptop in two seconds instead of failing on stage.

### 1. Point the CLI at the cluster and log in

```bash
nasiko connect <control-plane-url>      # URL comes from the repo owner
nasiko auth login
```

### 2. Set the vault-wide secrets, before the first deploy

Secrets are injected at deploy and restart only, so an agent deployed before the secret exists does not pick it up until it is restarted.

```bash
nasiko secrets set OTEL_EXPORTER_OTLP_ENDPOINT <otlp-endpoint>
nasiko secrets set DATABASE_URL <postgres-url>
nasiko secrets set NASIKO_API_URL 'https://<control-plane-host>/api/agents/{agent}'
```

`NASIKO_API_URL` is our name, not Nasiko's. `internal/a2a/call.go` reads it as a template and substitutes the literal `{agent}` with the peer's name, so a value without `{agent}` is refused rather than silently resolving every peer to one address. Keep the single quotes so the shell does not eat the braces.

### 3. Set the two per-agent secrets

Only `bp-onboarder` and `bp-collector` read `ANAKIN_API_KEY`. Nothing else does, so nothing else gets it.

```bash
nasiko secrets set ANAKIN_API_KEY <key> --agent bp-onboarder
nasiko secrets set ANAKIN_API_KEY <key> --agent bp-collector
```

Precedence, highest first: inline `-e` on the deploy, agent-specific secrets, vault-wide secrets.

### 4. Validate each agent directory

```bash
for a in bp-orchestrator bp-onboarder bp-collector bp-enricher bp-clusterer \
         bp-detector bp-responder bp-briefer bp-sov; do
  nasiko validate agents/$a
done
```

### 5. Deploy the nine

Order matters only in that `bp-orchestrator` calls the other eight, so deploy it last and it finds live peers on its first call.

```bash
for a in bp-onboarder bp-collector bp-enricher bp-clusterer \
         bp-detector bp-responder bp-briefer bp-sov bp-orchestrator; do
  nasiko deploy agents/$a --name $a --port 8000
done
```

Flags confirmed live at `docs.nasiko.com/cli/overview.md` and `docs.nasiko.com/adlc/deploy.md`: `nasiko deploy <image-or-dir> [--name <name>] [--port <port>] [--env-file <file>] [-e KEY=VALUE]`. Add `--env-file .env` only if you deliberately want local values to beat the vault, which on stage you do not.

**This is the step most likely to break, and it is worth reading before you run it.** Each Dockerfile builds from the **repo root** as context, not from the agent directory, because every agent needs `internal/` and `go.mod`. `COPY internal/` cannot reach outside the build context, so a build whose context is `agents/bp-<name>/` fails outright.

`.github/workflows/deploy.yml` line 152 runs `( cd "$dir" && nasiko deploy . --name "$name" )`, which hands `nasiko deploy` the agent directory. If the CLI passes that straight to `docker build` as the context, all nine builds fail on `COPY internal/`. I have not been able to run the CLI to settle it, so treat this as the first thing to check when a deploy fails, and have the fallback below ready:

```bash
docker build -f agents/$a/Dockerfile -t $a:0.1.0 .
nasiko deploy $a:0.1.0 --name $a --port 8000
```

### 6. Attach an LLM config to the five agents that call a model

Four agents make no LLM call and get nothing: `bp-collector`, `bp-sov`, `bp-detector`, `bp-orchestrator`. The other five do.

```bash
nasiko llm-config providers
nasiko llm-config create --name bp-default --provider openai --model gpt-4o-mini
for a in bp-onboarder bp-enricher bp-clusterer bp-responder bp-briefer; do
  nasiko llm-config attach --name bp-default --agent $a
done
```

The `--agent` spelling on `attach` is the one flag here I could not confirm verbatim in the live docs; the docs list `nasiko llm-config providers/create/attach/get` without expanding `attach`'s flags. Run `nasiko llm-config attach --help` on the day.

### 7. Check each agent came up

```bash
nasiko ps
```

Expect nine rows, all nine running, all on port 8000. Then, per agent:

```bash
for a in bp-orchestrator bp-onboarder bp-collector bp-enricher bp-clusterer \
         bp-detector bp-responder bp-briefer bp-sov; do
  echo "== $a"; nasiko logs $a -n 20
done
```

What a healthy start looks like: the agent binds `:8000` and stays up. What a dead start looks like: a restart loop. The usual cause is a card whose `supportedInterfaces[]` declares `preferredTransport` instead of `protocolBinding`, because JSONRPC is the only transport `a2a.Serve` mounts and an unrecognised key unmarshals to an empty binding. `scripts/check_agent_cards.py` catches exactly that before you leave the laptop, which is why step 0 is step 0.

Then prove one round trip end to end:

```bash
nasiko chat bp-detector '{"brand_id":"brd_demo","window_hours":24}'
```

And confirm the traces are arriving, which is the check that the OTLP secret in step 2 actually took:

```bash
nasiko observe sessions
```

If `nasiko observe sessions` is empty while `nasiko ps` shows nine healthy agents, the OTLP endpoint is missing or was set after the deploy. Set it and restart the agents; secrets are injected at deploy and restart only.

### 8. Local health probe, if you need one

The run image has no curl and no wget, so probe from the host:

```bash
docker run -d -e PORT=8000 -p 18000:8000 --name bp-probe bp-detector:0.1.0
curl -s http://127.0.0.1:18000/healthz
docker rm -f bp-probe
```

## Redeploying mid-demo

```bash
nasiko deploy agents/bp-briefer --name bp-briefer --port 8000
```

Same name updates the existing agent, because agent names are unique per cluster and `.nasiko/agent.json` holds the id. Expect `AgentCard.json` to come back modified again. Check `nasiko logs bp-briefer -f` until it binds, then re-run the call that failed.

## Rollback

There is no rollback command in this runbook. `nasiko stop`, `start`, `restart`, `scale <n>` and `rm --name <agent>` exist, and `nasiko registry` browses the artifact registry, but I have not run any of them against this cluster, so I am not going to write a rollback procedure I have not tested. If an agent is wedged on stage, `nasiko restart` it and move to the next demo beat.
