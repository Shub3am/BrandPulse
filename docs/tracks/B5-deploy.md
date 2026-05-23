# Track B5: Nasiko deployment, DronaHQ front ends, docs, pitch

**Branch:** `track/b5-deploy`
**Owns:** `AgentCard.json` + `Dockerfile` for all nine agents, the module path
change, Nasiko deploy, flow guards, OTel trace verification, the Nasiko fork PR,
`dronahq/`, `docs/`, pitch materials
**Blocked by:** B2–B4 for a full deploy. **Not blocked** for Tasks 0–3.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md),
[research/nasiko.md](../research/nasiko.md) and
[decisions/001-go-for-agents.md](../decisions/001-go-for-agents.md) in full. The
research file contains traps that will cost you an hour each if you skip them,
and the ADR names two costs that land squarely on this track.

Note that `research/nasiko.md` §3, §4 and §7 were written against the Python
stack. The SDK shape, the Dockerfile and the `bp_core.a2a.*` helper names in
those sections are superseded by this brief and by CONTRACTS §0–§1. The parts
that still hold verbatim are §1 (the three traps), §2 (AgentCard gating), §5
(flow guards), §6 (LLM router), §8 (CLI) and §9 (contributing).

---

## What this track owns

Everything between working code and a judge seeing it work.

Concretely: the nine `AgentCard.json` and nine `Dockerfile`, the one-time module
path change, every `nasiko` command that touches the cluster, the flow guard
configuration and the proof it fires, the OTel trace verification, the fork PR,
both DronaHQ surfaces, `docs/` and the pitch.

## What it must not know about

- **Agent business logic.** You do not edit `agents/bp-*/main.go`, ever. If an
  agent is wrong, it is B2, B3 or B4's fix. You own the two files beside it.
- **`internal/`.** B1 owns all of it, including `internal/obs`. You **verify**
  that traces arrive; B1 **writes** the code that sends them. The ADR's line
  about B5 owning `internal/obs` is loose phrasing: PLAN.md's file-structure
  table and CONTRACTS §3 both put it on B1, and they win.
- **`docs/CONTRACTS.md`.** B1 owns that one file. You own the rest of `docs/`.
- **`web/` and `bff/`. Those are B6's.** This matters more than it sounds,
  because the overlap invites two people building the same screen twice. The
  DronaHQ dashboard is the **ops and analyst view**: mention stream, alert log,
  approve/snooze, cost panel, raw JSON artifacts. `web/` is the **product
  view**: the polished thing a judge looks at. Two surfaces, two audiences, two
  owners. If you find yourself styling a chart for aesthetics, you are in B6's
  lane.

## The three things that sink this track

Deployment discovered late, traces that never arrive, and a front end built
against an API that is not up yet. Task 1 exists to kill the first. Task 2 kills
the second before it can multiply by nine. Tasks 7 and 8 are ordered so the
third cannot happen.

There is a fourth, smaller one: whether a DronaHQ Agent exports to a file at
all. That is a **day-one** console check inside Task 8, not a demo-morning
discovery. It used to be WhatsApp outbound send mechanics; the scope change of
2026-09-20 removed that probe along with the channel.

---

## Hard constraint: never use the dashboard uploader

`validate_agent_zip` in Nasiko's `server/src/agents/upload.rs` hard-rejects any
zip that does not contain `main.py`, `src/main.py` or `__main__.py`. Our agents
are Go binaries. There is no `main.py` anywhere in this repo and there is not
going to be one.

That gate has exactly one caller and it guards the **dashboard zip-upload path
only**. `nasiko deploy` from the CLI, GitHub import and catalog import all
bypass it entirely.

So: **every BrandPulse agent reaches the cluster through `nasiko deploy`. Nobody
drags a zip into the dashboard, not once, not to "just try it".** It will reject
the fleet on a filename and you will spend twenty minutes reading a Rust
validator to find out why. Put this sentence in `docs/DEPLOY-NOTES.md` on the
first line.

Related repo rule: deploy through the automated pipeline. No deploying over SSH
unless you are explicitly told to, per instance.

---

## Task 0: Fix the module path, once, on `main`

**Files:** `go.mod`, every `.go` file with a `brandpulse/internal/...` import,
`HACKATHON_NOTES.md`

Today the module is the bare path `brandpulse`, because the repo has no remote.
Before the Nasiko fork PR it has to be `github.com/<org>/brandpulse`. This is
Task 0 and not a footnote, because every import in the tree changes with it and
it must happen once, cleanly, on `main`, with every worktree rebasing after.

- [ ] Agree the `<org>` first. Getting this wrong means doing it twice.
- [ ] `go mod edit -module github.com/<org>/brandpulse`. Exactly once.
- [ ] `go mod edit` rewrites `go.mod` and **nothing else**. Rewrite the import
      statements in the same commit: every `brandpulse/internal/...` becomes
      `github.com/<org>/brandpulse/internal/...`. A mechanical
      `gofmt -r` or a scoped find-and-replace across `internal/` and `agents/`
      is fine; check `git diff --stat` shows only import lines moving.
      CONTRACTS §0 says "nothing else in the tree needs to change", which is
      wrong about the imports. Flag it to B1 so that line gets corrected.
- [ ] `go mod tidy`.
- [ ] `go build ./... && go vet ./...`. Both clean, no exceptions.
- [ ] Land it on `main` through B1 as integrator, then post in
      `HACKATHON_NOTES.md`: "module path is now X, rebase before your next
      commit." Five tracks are compiling against the old path right now.
- [ ] Commit. One commit, one logical change, nothing else riding along.

## Task 1: Deploy one agent, today

Do this **before** the other eight exist. `bp-sov` is the target: no LLM, no
network, no database, and B3 builds it first for exactly this reason. One agent
proves the Dockerfile, the AgentCard, the flow guard wiring, the secrets path
and the OTel trace. Then the other eight are mechanical.

**Files:** `agents/bp-sov/AgentCard.json`, `agents/bp-sov/Dockerfile`,
`docs/DEPLOY-NOTES.md`

- [ ] Fork and clone `Nasiko-Labs/nasiko`, install the CLI
      (`cargo install --path cli/`), `nasiko connect`, `nasiko auth login`.
- [ ] Write the multi-stage Dockerfile below. Do **not** inherit
      `agents/currency-agent/Dockerfile`: it references a `pyproject.toml` that
      does not exist in the repo and cannot build, and it is Python anyway.

      ```dockerfile
      FROM golang:1.27 AS build
      WORKDIR /src
      COPY go.mod go.sum ./
      RUN go mod download
      COPY internal/ ./internal/
      COPY agents/bp-sov/ ./agents/bp-sov/
      RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
            -o /out/agent ./agents/bp-sov

      FROM gcr.io/distroless/static:nonroot
      COPY --from=build /out/agent /agent
      USER nonroot:nonroot
      EXPOSE 8000
      ENTRYPOINT ["/agent"]
      ```

- [ ] Build context is the repo root, because every agent needs `internal/`:
      `docker build -f agents/bp-sov/Dockerfile -t bp-sov .`
- [ ] **`CGO_ENABLED=0` is load-bearing.** Without it the binary links against
      glibc and a distroless-static or scratch base gives you "no such file or
      directory" on a binary that is visibly there. That error message is a
      liar and it will cost you fifteen minutes.
- [ ] Base is `gcr.io/distroless/static`, not `scratch`, because our agents
      make outbound HTTPS to the LLM router and to Anakin and need CA
      certificates. `scratch` works only for an agent with no outbound TLS. Do
      not switch to `scratch` to save two megabytes.
- [ ] Record the image size in `docs/DEPLOY-NOTES.md`. Expect roughly **15MB
      against roughly 300MB** for the equivalent Python image. This is a real
      win, not a vanity number: nine of these pull and cold-start on Nasiko
      noticeably faster, and "our whole fleet is under 150MB" is a line in the
      pitch. Paste the real `docker images` output, not the estimate.
- [ ] `"protocolVersion": "1.0"`. The example ships `"0.2.9"` and a real cluster
      rejects it with `-32009 VersionNotSupported`.
- [ ] The agent binds to `PORT` (CONTRACTS §5, injected by Nasiko) and falls
      back to 8000 locally. `EXPOSE 8000` is documentation; `PORT` is the
      truth. If B1's `internal/a2a` does not read `PORT`, that is a B1 bug and
      you file it, you do not patch it here.
- [ ] `nasiko validate`, then `nasiko deploy`, then call the deployed URL with a
      real A2A request and get a `ShareOfVoice` artifact back. A deploy that
      printed success is not a deploy that works.
- [ ] Save the request body you used as `docs/deploy/sov-request.json`. You will
      run it again at every gate and so will everyone else.
- [ ] **Record every surprise in `docs/DEPLOY-NOTES.md` as you hit it.** This
      file is the highest-value thing you write today.
- [ ] Answer the open question from research/nasiko.md §7: how a peer is
      addressed through the proxy, and which env var carries the proxy base URL
      and the caller's trust credential. Read it off the live deploy, then post
      it in `HACKATHON_NOTES.md` for **B1** to wire into `internal/a2a`. You
      find the answer; B1 writes the code. Nine agents must not each invent it.
- [ ] Commit.

## Task 2: Prove the OTel traces arrive, before the fleet deploys

**Files:** `docs/DEPLOY-NOTES.md`, `docs/screenshots/`

Nasiko auto-injects OpenTelemetry only into containers with a `FROM python`
base. Our bases are `golang:1.27` and `gcr.io/distroless/static`, so **nothing
is injected and nothing is traced unless our own code does it.** B1 writes
`internal/obs` (roughly 150 lines, once) and every agent's `main()` calls
`obs.Setup(serviceName)` on its first line, reading
`OTEL_EXPORTER_OTLP_ENDPOINT` from CONTRACTS §5.

Your job is to verify traces actually land in `nasiko observe` for **one** Go
agent before the other eight deploy.

- [ ] Confirm `OTEL_EXPORTER_OTLP_ENDPOINT` is present in the deployed bp-sov
      container's environment. If Nasiko does not inject it for non-Python
      agents either, that is finding number one and it changes B1's task.
- [ ] Make a real A2A call to bp-sov, then run `nasiko observe` and find the
      span. Not "the agent responded", the span, with the service name, in the
      observability view.
- [ ] Screenshot it into `docs/screenshots/`.
- [ ] **If no trace arrives, this is a deploy blocker, not a polish item.** Stop,
      post it in `HACKATHON_NOTES.md`, and work it with B1 before deploying the
      other eight. Discovering it after nine deploys means nine redeploys and a
      cost demo with nothing behind it.
- [ ] Why it blocks: `nasiko observe` is where the per-brand token spend and the
      call graph come from. That is the "₹11 for today's run" demo beat and the
      FinOps story. No spans means no beat.
- [ ] Record in `docs/DEPLOY-NOTES.md` exactly what made it work, so the other
      eight inherit it rather than rediscover it.
- [ ] Commit.

## Task 3: Flow guards

**Files:** `docs/DEPLOY-NOTES.md`, deploy env config

- [ ] Set on the control plane: `NASIKO_FLOW_MAX_DEPTH=3`,
      `NASIKO_FLOW_MAX_FAN_OUT=8`, `NASIKO_FLOW_MAX_TOKENS=60000`,
      `NASIKO_FLOW_TIMEOUT_SECS=180`. Defaults are fan-out 20 and depth 5, which
      are wider than our brief, so these must be set explicitly.
- [ ] Guards need **Redis**, and they fail closed: no Redis means every call is
      rejected with `GuardUnavailable`. Confirm Redis is up before blaming an
      agent. Write that sentence in DEPLOY-NOTES too.
- [ ] Verify the cap actually fires. Enable 10 sources on the demo brand and
      confirm the orchestrator degrades to 8 with `SourcesSkipped` populated and
      `DegradedReason` set. **This is a demo beat**, so it has to be
      reproducible on command, not incidental.
- [ ] Also confirm the cap **drops** the call rather than queuing it. Fail
      closed means an eleventh fan-out is a rejection, not a slow success. If it
      quietly succeeds, the guard is not configured and you are looking at the
      default 20.
- [ ] The orchestrator's own fan-out is bounded in code by
      `errgroup.SetLimit` (CONTRACTS §2). The guard is the backstop, not the
      primary control. If the guard is what stops you, B4 has a bug.
- [ ] Commit.

## Task 4: Validate the nine cards, do not write them

**Files:** none of `agents/`. You file findings in `HACKATHON_NOTES.md`.

CONTRACTS §4 gives `AgentCard.json` and `Dockerfile` to the track that writes
the agent. This task used to say you write nine of each, which contradicted it.
The contract wins: nine agents across five worktrees each editing eighteen
files a sixth worktree also edits is a guaranteed merge-day conflict, and the
Dockerfile varies only by binary name so there is nothing for you to tune.

What is yours here:

- [ ] The templates in `research/nasiko.md` §2 and §4. Already done, and the
      union card is the reason the first deploy will not fail.
- [ ] Every requirement those templates imply now lives in
      [agents/CLAUDE.md](../../agents/CLAUDE.md), which the owning tracks
      actually read: the union shape, non-empty `skills`, `llm_provider: null`
      on the four deterministic agents, the `sync_card_version` rewrite, and
      `.nasiko/agent.json`.
- [ ] `nasiko validate` on all nine before deploying any. A failure is a
      blocker row naming the owning track, not an edit by you.
- [ ] `research/nasiko.md` §2 says "five deterministic agents" and then lists
      four. Four is correct: onboarder, enricher, clusterer, responder and
      briefer all call an LLM. Fix the sentence, it is your file.

## Task 5: Deploy the fleet

- [ ] `nasiko secrets set ANAKIN_API_KEY <key>` and `DATABASE_URL`, vault-wide.
      Secrets are AES-256-GCM at rest and injected at deploy or restart only.
      **Nothing is baked into an image.** If a key appears in a Dockerfile or a
      committed env file, that is a rebuild, not a note.
- [ ] Secret precedence, highest first: inline `-e` on deploy, agent-specific
      secrets, vault-wide secrets. Use vault-wide for the two above so nine
      agents do not each need setting.
- [ ] `OPENAI_BASE_URL` and `OPENAI_API_KEY` are **injected by the router at
      deploy time**, not set by you. Do not create them as secrets, you will
      shadow the real ones.
- [ ] Confirm the full CONTRACTS §5 env list is satisfied per agent, including
      `PORT`, `OTEL_EXPORTER_OTLP_ENDPOINT` and `BP_FIXTURE_MODE`.
- [ ] Deploy all nine. Confirm each with a real A2A call returning its declared
      artifact type. Nine calls, nine pasted responses.
- [ ] `nasiko llm-config` per agent for model selection. Per research/nasiko.md
      §6 Finding A the router **ignores the `model` field in the request body**
      and resolves the model server-side from the agent's `llm_config`, so
      `BP_MODEL_SMALL` does nothing on Nasiko. Model choice is configured here
      or not at all. Set it for the five LLM agents.
- [ ] Capture `nasiko observe` output showing the call graph for one full run
      and the per-brand token spend. This is the "₹11 for today's run" demo
      beat, and Nasiko computes it. We read it, we do not build it.
- [ ] Screenshot everything. Commit to `docs/screenshots/`.

## Task 6: The Nasiko PR, fixing the validator bug you found

**Files:** in the Nasiko fork, `cli/src/commands/validate.rs` plus its tests.

The original plan was to copy the nine agent directories into the fork. That
PR cannot build. It carries Go source importing `brandpulse/internal/...`,
which is not a fetchable module for anyone outside this repo because Task 0
was skipped, and skipping Task 0 was the coordinating session's call. So the
agent-vendoring PR is dropped.

What replaces it is better. Your finding 7 is a genuine spec-conformance bug in
their CLI, found by running it rather than reading it:

- [x] `validate.rs` requires `url`, `protocolVersion` and `preferredTransport`
      at the top level of the card. A2A **1.0** moved all three into
      `supportedInterfaces[]`, so a spec-correct card written by any current
      A2A SDK fails `nasiko validate`. Today the only card that passes is one
      carrying both shapes, which is what we ship and what nobody should have
      to discover by hand.
- [x] Make the validator accept either placement: read the three from the top
      level, and fall back to the first entry of `supportedInterfaces[]`. Keep
      the existing error message for a card that has neither.
- [x] A test per placement: top-level only, `supportedInterfaces` only, both,
      neither. Rust, in their tree, importing nothing of ours.
- [x] One logical change, what and why, per their CONTRIBUTING.md.
- [x] Open the PR. Link it in `HACKATHON_NOTES.md`.

This is a stronger submission than vendoring nine directories would have been.
It says we used the platform hard enough to find a real bug in it and sent the
fix back, and it builds on its own.

## Task 7: DronaHQ dashboard (the ops and analyst view)

**Files:** `dronahq/dashboard-app.json`, `dronahq/README.md`, screenshots

Build against the **deployed** agent URLs from Task 5, never against a local
port. This is the ops surface, not the product surface: B6's `web/` is the
product surface and the two do not share screens.

- [ ] Sign up, 30-day Business trial, no card.
- [ ] Register a REST API connector per agent endpoint. Auth: API Key, target
      Header, value `Bearer <token>`. Interpolate variables as `{{name}}`.
- [ ] Screens:
      - **Mention stream**: Table Grid with source/sentiment/date filters,
        Detail View drawer on row click.
      - **Sentiment over 14 days**: Charts control (Plotly), line.
      - **Share of voice**: Charts, pie. Donut is a Plotly `hole` config;
        confirm the UI exposes it, otherwise ship pie.
      - **Topic clusters**: List/Cards, size and trend per topic.
      - **Alert log**: Table Grid, Approve/Snooze buttons per row wired to
        Action Flows, Toast on success.
      - **Cost panel**: Metric tiles: credits, tokens, ₹ per brand-day.
- [ ] Reply drafts render in a **Markdown Viewer**; raw artifacts in a **JSON
      Editor** for the "it's typed JSON all the way down" beat. Every agent
      returns one `application/json` artifact that is an `internal/models`
      struct, so DronaHQ binds to it with no translation layer.
- [ ] The approve/reject flow is the human-in-the-loop guardrail. DronaHQ's
      native HITL approval UI is UNVERIFIED: build it from Table Grid + Button +
      Action Flow + Toast, which are all confirmed.
- [ ] Export: publish first, then **Config → App Export → Export app json**.
      Commit the JSON. Git Sync is self-hosted-only, so this is manual.
- [ ] Commit with screenshots.

## Task 8: DronaHQ chat agent

**Files:** `dronahq/chat-agent.json`, setup notes, screenshots

Renamed from "WhatsApp agent" on 2026-09-20. Same agent, same instructions,
same tools, on the Chat trigger instead of a WhatsApp one. See
`HACKATHON_NOTES.md`, the scope-change entry.

**First console check of the day:** find out whether a DronaHQ **Agent** can be
exported to a file at all. App export is documented; agent export is not
documented anywhere, and `dronahq/CLAUDE.md` currently promises a committed
`chat-agent.json`. If no export exists, that file becomes a written runbook
plus screenshots and you correct `dronahq/CLAUDE.md` in the same commit. Find
this out before you build the agent, not after.

- [ ] **Read `docs/research/dronahq.md` §0 before you click anything.** WhatsApp
      is **out of the MVP**. The product is observability over review and social
      data; WhatsApp was one channel over the top and it was dragging a Meta app,
      a WhatsApp Business number and a multi-day template approval onto the
      critical path. Do not create a Meta app. Do not add a Twilio connector.
      Do not configure a WhatsApp trigger. §1 of the research doc is the
      verified answer for the day someone switches it on, and that day is not
      today.
- [ ] Create a DronaHQ **Agent**. Write its Instructions with the six
      components, putting our guardrails in "Rules & Guardrails": never promise
      a refund, never admit fault, never commit to a date, always escalate a
      crisis to a human.
- [ ] Put it on the **Chat** trigger. No external account, works the moment the
      agent exists, and it is the same agent that later gains a WhatsApp trigger
      alongside Chat rather than instead of it.
- [ ] Add a **Webhook** trigger so `bp-detector` can push a crisis in. It takes
      the `api-key` header and returns structured JSON synchronously. This is
      the verified inbound door for our Go agents.
- [ ] Attach the agent's tools: REST connectors to bp-onboarder (onboarding
      conversation), bp-briefer (on-demand query) and bp-responder (draft a
      reply).
- [ ] `WhatsappShort` keeps its name and its `models.WhatsappShortLimit` (600)
      cap. Do not rename the field, it is frozen in `internal/models` and
      renaming it is a tree-wide break for a cosmetic gain. It is the short,
      no-tables, no-markdown-links rendering, and it is what the chat bubble
      binds to. `Markdown` is for the dashboard.
- [ ] Add a **Scheduler trigger** for the 9am daily brief.
- [ ] Test the three flows: onboard, daily brief, alert with a "Draft reply"
      action. Nothing auto-posts: the draft goes to a human, always.
- [ ] Commit.

## Task 9: Docs and pitch

**Files:** `README.md`, `docs/ARCHITECTURE.md`, `docs/SETUP.md`,
`docs/PRICING.md`, `docs/DEMO-SCRIPT.md`, `docs/LINKEDIN.md`

- [ ] `README.md`: what it is, the honest source list from
      [SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md), the public-data-only
      statement, the never-auto-posts statement, and how to run it.
- [ ] `ARCHITECTURE.md`: the nine agents, the call graph, which four are
      deterministic, where every platform is load-bearing, and the two
      front-end surfaces with their audiences so the DronaHQ/`web/` split reads
      as a decision rather than an accident.
- [ ] `SETUP.md`: the Go toolchain version,
      `docker compose up -d --no-recreate postgres`, the migration,
      `BP_FIXTURE_MODE=replay`, and the module path. Somebody cloning this at
      2am should not need to ask. A fresh clone is a single checkout, so say
      that `--no-recreate` is there for the seven-worktree case and is a no-op
      for them.
- [ ] `PRICING.md`: unit economics from **B3's real `go run ./eval/cost`
      output**, versus Brandwatch/Sprinklr/Meltwater list prices. If the real
      number misses the ₹15 target, print it anyway and explain what would close
      it. No invented numbers.
- [ ] `DEMO-SCRIPT.md`: the 2-minute run with timings, and what to say when a
      step fails.
- [ ] Record the fallback video in Phase 5.

---

## Definition of done

All nine agents answering real A2A calls at deployed URLs; OTel spans visible in
`nasiko observe` for all nine; the flow-guard cap demonstrably dropping an
over-cap call; the DronaHQ dashboard rendering a live run; the chat agent
delivering a brief and an alert; the Nasiko PR open; real `eval/` numbers in
`PRICING.md`. Screenshots of each in the PR.

A deploy is not done because `nasiko deploy` printed success. It is done when a
real A2A call to the deployed URL returns the expected artifact and that
response is in the PR.

Run this and **paste the output into the PR**:

```bash
# module path, clean tree
go mod edit -json | jq -r .Module.Path      # github.com/<org>/brandpulse
go build ./... && go vet ./...

# the image win, measured not estimated
docker build -f agents/bp-sov/Dockerfile -t bp-sov:done .
docker images bp-sov:done --format 'bp-sov {{.Size}}'

# every card passes the CLI gate that actually runs on deploy
for a in onboarder collector enricher clusterer sov detector responder briefer orchestrator; do
  (cd agents/bp-$a && nasiko validate) || echo "FAILED bp-$a"
done

# nine live agents, nine real artifacts
nasiko ps --json | jq -r '.[] | "\(.name) \(.status) \(.url)"'

# the one that matters: a real A2A call returning a real artifact
curl -s -X POST "$BP_SOV_URL" \
  -H 'Content-Type: application/json' -H 'A2A-Version: 1.0' \
  --data @docs/deploy/sov-request.json \
| jq '.result.artifacts[0] | {name, mimeType, parts: (.parts | length)}'

# the card the cluster serves, not the one on disk
curl -s "$BP_SOV_URL/.well-known/agent-card.json" \
| jq '{protocolVersion, preferredTransport, skill: .skills[0].id}'

# traces exist, and the cost number is real
nasiko observe --agent bp-sov --last 5m
go run ./eval/cost
```

`protocolVersion` must read `"1.0"`. The artifact must be
`application/json` named `ShareOfVoice`. `nasiko observe` must show a span, not
an empty result. If any of those three is missing, the track is not done,
whatever the deploy log said.
