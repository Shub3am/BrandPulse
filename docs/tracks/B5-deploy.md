# Track B5 — Nasiko deployment, DronaHQ front ends, docs, pitch

**Branch:** `track/b5-deploy`
**Owns:** `AgentCard.json` + `Dockerfile` for all nine agents, Nasiko deploy,
flow guards, the Nasiko fork PR, `dronahq/`, `docs/`, pitch materials
**Blocked by:** B2–B4 for a full deploy. **Not blocked** for Tasks 1–3.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md),
[research/nasiko.md](../research/nasiko.md) and
[research/dronahq.md](../research/dronahq.md) in full. Both research files
contain traps that will cost you an hour each if you skip them.

---

## What this track owns

Everything between working code and a judge seeing it work.

## The two things that sink this track

Deployment discovered late, and a front end built against an API that is not up
yet. Task 1 exists to kill the first. Task 6 is designed so the second cannot
happen.

---

## Task 1 — Deploy one agent, today

Do this **before** the other eight exist. `bp-sov` is the target: no LLM, no
network, no database, and B3 builds it first for exactly this reason.

**Files:** `agents/bp-sov/AgentCard.json`, `agents/bp-sov/Dockerfile`,
`docs/DEPLOY-NOTES.md`

- [ ] Fork and clone `Nasiko-Labs/nasiko`, install the CLI
      (`cargo install --path cli/`), `nasiko connect`, `nasiko auth login`.
- [ ] Write the Dockerfile from research/nasiko.md §4 — **not** the one in
      `agents/currency-agent/`, which references a `pyproject.toml` that does
      not exist and cannot build.
- [ ] `protocolVersion: "1.0"`. The example ships `"0.2.9"` and a real cluster
      rejects it with `-32009 VersionNotSupported`.
- [ ] `nasiko validate`, then `nasiko deploy`, then call the deployed URL with
      a real A2A request and get a `ShareOfVoice` artifact back.
- [ ] **Record every surprise in `docs/DEPLOY-NOTES.md` as you hit it.** The
      other eight deploys are then mechanical. This file is the highest-value
      thing you write today.
- [ ] Answer the open question from research/nasiko.md §3: which `a2a-sdk`
      1.1.0 helper emits a JSON artifact. Implement
      `bp_core.a2a.emit_json_artifact` and tell every track in
      `HACKATHON_NOTES.md`. **Nine agents must not each invent this.**
- [ ] Answer the open question from §7: how a peer is addressed through the
      proxy. Implement `bp_core.a2a.call_agent` accordingly. Post it.
- [ ] Commit.

## Task 2 — Flow guards

**Files:** `docs/DEPLOY-NOTES.md`, deploy env config

- [ ] Set on the control plane: `NASIKO_FLOW_MAX_DEPTH=3`,
      `NASIKO_FLOW_MAX_FAN_OUT=8`, `NASIKO_FLOW_MAX_TOKENS=60000`,
      `NASIKO_FLOW_TIMEOUT_SECS=180`.
- [ ] Guards need **Redis**, and they fail closed — no Redis means every call
      is rejected with `GuardUnavailable`. Confirm Redis is up before blaming
      an agent.
- [ ] Verify the cap fires: enable 10 sources on the demo brand and confirm the
      orchestrator degrades to 8 with `degraded_reason` set. **This is a demo
      beat**, so it has to be reproducible on command, not incidental.
- [ ] Commit.

## Task 3 — Cards and Dockerfiles for the other eight

**Files:** `agents/bp-*/AgentCard.json`, `agents/bp-*/Dockerfile`

- [ ] One card each. Required fields per research/nasiko.md §2:
      `name, description, url, version, capabilities, skills, protocolVersion,
      preferredTransport`.
- [ ] `skills` is never empty — an agent with no skills is invisible to
      routing. One skill per card with a real `id`, `description` and
      `examples`.
- [ ] `llm_provider: null` on the five deterministic agents (collector,
      detector, sov, orchestrator) — it is a true statement about the
      architecture and a judge will notice it.
- [ ] Commit per agent.

## Task 4 — Deploy the fleet

- [ ] `nasiko secrets set ANAKIN_API_KEY <key>` and `DATABASE_URL`, vault-wide.
- [ ] Deploy all nine. Confirm each with a real A2A call.
- [ ] `nasiko llm-config` per agent for model selection — per
      research/nasiko.md §6 Finding A, the router **ignores the `model` field
      in the request body**, so `BP_MODEL_SMALL` does nothing on Nasiko. Model
      choice is configured here or not at all.
- [ ] Capture `nasiko observe` output showing the call graph for one run and
      the per-brand token spend. This is the "₹11 for today's run" demo beat,
      and Nasiko computes it — we just read it.
- [ ] Screenshot everything. Commit to `docs/screenshots/`.

## Task 5 — The Nasiko fork PR

**Files:** in the fork: `agents/brandpulse-*/`, `agents/brandpulse/README.md`

- [ ] Copy the nine agent directories in as `agents/brandpulse-<name>/`.
- [ ] `agents/brandpulse/README.md`: what BrandPulse is, the nine-agent
      topology with a diagram, which agents use an LLM and which are
      deterministic, a link to the product repo, and the flow-guard settings it
      expects.
- [ ] There is **no registry or index file to update** — `agents/` is a flat
      directory. Touch `agents/THIRD_PARTY_LICENSES.md` only if a Dockerfile
      installs a third-party binary at build time.
- [ ] One logical change per PR, clear what/why description, per their
      CONTRIBUTING.md.
- [ ] Open the PR. Link it in `HACKATHON_NOTES.md`.

## Task 6 — DronaHQ dashboard

**Files:** `dronahq/dashboard-app.json`, `dronahq/README.md`, screenshots

Build against the **deployed** agent URLs from Task 4, never against a local
port. See research/dronahq.md for exact control names.

- [ ] Sign up, 30-day Business trial, no card.
- [ ] Register a REST API connector per agent endpoint. Auth: API Key, target
      Header, value `Bearer <token>`. Interpolate variables as `{{name}}`.
- [ ] Screens:
      - **Mention stream** — Table Grid with source/sentiment/date filters,
        Detail View drawer on row click.
      - **Sentiment over 14 days** — Charts control (Plotly), line.
      - **Share of voice** — Charts, pie. Donut is a Plotly `hole` config;
        confirm the UI exposes it, otherwise ship pie.
      - **Topic clusters** — List/Cards, size and trend per topic.
      - **Alert log** — Table Grid, Approve/Snooze buttons per row wired to
        Action Flows, Toast on success.
      - **Cost panel** — Metric tiles: credits, tokens, ₹ per brand-day.
- [ ] Reply drafts render in a **Markdown Viewer**; raw artifacts in a
      **JSON Editor** for the "it's typed JSON all the way down" beat.
- [ ] The approve/reject flow is the human-in-the-loop guardrail. DronaHQ's
      native HITL approval UI is UNVERIFIED — build it from Table Grid +
      Button + Action Flow + Toast, which are all confirmed.
- [ ] Export: publish first, then **Config → App Export → Export app json**.
      Commit the JSON. Git Sync is self-hosted-only, so this is manual.
- [ ] Commit with screenshots.

## Task 7 — DronaHQ WhatsApp agent

**Files:** `dronahq/whatsapp-agent.json`, setup notes, screenshots

- [ ] Create a DronaHQ **Agent**. Write its Instructions with the six
      components, putting our guardrails in "Rules & Guardrails": never promise
      a refund, never admit fault, never commit to a date, always escalate a
      crisis to a human.
- [ ] Add the native **WhatsApp trigger** (Meta WhatsApp Business API via
      webhook + verify token, configured in Meta → My Apps → WhatsApp →
      Configuration). Inbound fields: `contacts[0].wa_id`,
      `contacts[0].profile.name`, `messages[0].text.body`.
- [ ] Outbound send mechanics are UNVERIFIED in the docs — confirm on the
      trigger page early. **Fallback:** the Twilio connector
      (`SendWhatsappTextMessage`, numbers prefixed `whatsapp:`). Meta's 24-hour
      customer-service window applies either way, which matters for a 9am brief
      to a user who has not messaged that day — the brief may need a template.
      Find this out on day one, not at 8am on demo day.
- [ ] Attach the agent's tools: REST connectors to bp-onboarder (onboarding
      conversation), bp-briefer (on-demand query) and bp-responder (draft a
      reply).
- [ ] Add a **Scheduler trigger** for the 9am daily brief.
- [ ] Test the three flows: onboard, daily brief, alert with a "Draft reply"
      action.
- [ ] Commit.

## Task 8 — Docs and pitch

**Files:** `README.md`, `docs/ARCHITECTURE.md`, `docs/SETUP.md`,
`docs/PRICING.md`, `docs/DEMO-SCRIPT.md`, `docs/LINKEDIN.md`

- [ ] `README.md`: what it is, the honest source list from
      [SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md), the public-data-only
      statement, the never-auto-posts statement, and how to run it.
- [ ] `ARCHITECTURE.md`: the nine agents, the call graph, which are
      deterministic, where every platform is load-bearing.
- [ ] `PRICING.md`: unit economics from **B3's real `eval/cost.py` output**,
      versus Brandwatch/Sprinklr/Meltwater list prices. If the real number
      misses the ₹15 target, print it anyway and explain what would close it.
- [ ] `DEMO-SCRIPT.md`: the 2-minute run with timings, and what to say when a
      step fails.
- [ ] Record the fallback video in Phase 5.

---

## Definition of done

All nine agents answering real A2A calls at deployed URLs; the DronaHQ
dashboard rendering a live run; the WhatsApp agent delivering a brief and an
alert; the Nasiko PR open; `eval/` numbers in PRICING.md. Screenshots of each
in the PR.
