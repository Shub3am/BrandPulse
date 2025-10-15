# Hackathon notes — shared decisions

The one place six parallel tracks talk to each other. Append, do not rewrite.
Newest entry at the bottom of its section.

**How to use this file**

- Anything another track needs to know goes here, not in a commit message.
- Format: `### YYYY-MM-DD HH:MM — <track> — <one line>` then the detail.
- If you are **blocked**, add a row to "Open blockers" and name the track that
  unblocks you.
- If you **answered an UNVERIFIED question**, put the answer under "Resolved
  unknowns" with how you verified it. Nine agents must not each rediscover it.
- Merge conflicts on this file are expected and always resolved by keeping
  both sides.

---

## Open blockers

| Raised by | Blocked on | What I need | Status |
|---|---|---|---|
| B2, B3, B4 | B1 | the `internal/` import surface as compiling signatures, on `main` | open, **B1 Task 1, twenty minutes** |
| B2, B3, B4 | B1 | `internal/anakin` with working `replay` mode | open |
| B3 (Task 4) | B2 | `fixtures/labelled/mentions.jsonl` sample | open |
| B5 (Tasks 4, 6, 7) | B2, B3, B4 | agents that run | open |
| B6 (`bff/`) | B4 | a deployed `bp-orchestrator` URL | open |

In Go a missing package is a compile error for everyone downstream, not a
runtime `ImportError` in one test. That is why B1's signature commit is its own
blocker row and why it comes before B1's own implementation.

---

## Resolved unknowns

Each entry: the question, the answer, and how it was verified. An unverified
answer stays in "Open questions".

### 2026-09-20 — main — Anakin Wire does not carry four of the brief's sources

X/Twitter, Instagram, Google Play reviews and Apple App Store reviews are not in
Anakin's Wire catalogue. Verified by direct 404 on `anakin.io/catalog/<slug>`
for each, corroborated by Anakin's own blog post dated 2026-07-24. Flipkart has
product actions but no reviews action.

Replacements and drops are in [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md).
The `Source` enum is unchanged, so this is a config change, not a schema change.

### 2026-09-20 — main — Anakin Search API returns a snippet, not full page content

The brief says "full page content". It returns `snippet`. Bodies require
chaining Search → URL Scraper at 3 + N credits. Budget accordingly.
Detail in [docs/research/anakin.md](docs/research/anakin.md) §2.

### 2026-09-20 — main — Nasiko AgentCard `protocolVersion` must be `"1.0"`

The `currency-agent` example ships `"0.2.9"`. A real cluster rejects that with
`-32009 VersionNotSupported`. Every BrandPulse card uses `"1.0"`.

### 2026-09-20 — main — the example Dockerfile does not build

`agents/currency-agent/Dockerfile` does `COPY pyproject.toml .` and no such file
exists anywhere in the Nasiko repo. Use ours from
[docs/research/nasiko.md](docs/research/nasiko.md) §4, which builds with the
repo root as context so `shared/` is available.

### 2026-09-20 — main — the Nasiko LLM router discards the request's `model` field

Per-agent model selection is configured with `nasiko llm-config`, not passed in
code. `BP_MODEL_SMALL` therefore does nothing once deployed. B5 owns this.
Detail in [docs/research/nasiko.md](docs/research/nasiko.md) §6 Finding A.

### 2026-09-20 — main — no verified embeddings endpoint on the Nasiko router

The catalogue lists chat models only. bp-clusterer therefore defaults to local
TF-IDF, behind `BP_VECTORISER=tfidf|embeddings`. Detail in §6 Finding B.

---

## Open questions

The things nobody has verified yet. Claim one by putting your track in the
"owner" column, and move it to "Resolved unknowns" when you have an answer.

| # | Question | Owner | Why it matters |
|---|---|---|---|
| 1 | Which `a2a-go/v2` helper emits a **JSON** artifact? | B1, Task 1 | All nine agents need it. Lands as `a2a.JSONArtifact`. |
| 2 | How is a peer agent addressed through the Nasiko proxy? The env var name is unverified. | B5, Task 1 | Lands as `a2a.Call`. No agent writes a peer URL directly. |
| 7 | Do OTel traces from a self-instrumented Go container actually reach `nasiko observe`? | B5, Task 2 | If not, nine agents are invisible in the control plane. Deploy blocker, not polish. |
| 3 | The literal field names in Wire responses per action. | B2, Task 1 | A guessed field name is an empty dashboard on stage. |
| 4 | Does the Play Store listing yield review text, rating and date through URL Scraper with `useBrowser: true`? | B2, Task 2 | Decides six sources or seven. |
| 5 | Does DronaHQ's WhatsApp trigger send outbound, or do we need the Twilio connector? | B5, Task 7 | The 9am brief depends on it. Meta's 24-hour window may force a template. |
| 6 | Does DronaHQ's Charts control expose the Plotly `hole` config for a donut? | B5, Task 6 | Cosmetic. Ship a pie if not. |

---

## Decisions log

Things we chose, with the reason, so nobody relitigates them at 3am.

### 2026-09-20 — main — contracts frozen before any worktree branched

`docs/CONTRACTS.md`, `internal/models/` and `db/migrations/001_init.sql` are
frozen on `main`. A field rename is a breaking change for DronaHQ bindings and
every downstream agent. Changes go through B1 on `main`, never inside a
worktree.

### 2026-09-20 — main — we do not use the `anakin-sdk` package

It is alpha. We call the REST API from one package, `internal/anakin`, so a
wrong assumption about Anakin's shape is a one-package fix. Reasoning in
[docs/research/anakin.md](docs/research/anakin.md) §7.

### 2026-09-20 — main — the nine agents are Go, not Python

Full reasoning in [docs/decisions/001-go-for-agents.md](docs/decisions/001-go-for-agents.md).
The short version: six parallel tracks against a frozen contract, and in Go a
contract drift is a compile error instead of a runtime `KeyError` found on
stage. Verified before committing to it: `github.com/a2aproject/a2a-go/v2`
v2.5.0 exists, has server support in `a2asrv`, and Nasiko already runs Go
agents.

**The wire format did not change.** Field names, enum values, the five detector
rules and the divergence table are identical to the Python freeze. A fixture or
a DronaHQ binding made against the old contract still works. What changed is
the import surface (CONTRACTS §3), the naming rules (§4), and two costs that
pydantic and sklearn used to absorb:

- **Clustering is hand-rolled.** Go has no scikit-learn. `internal/cluster` is
  240 to 320 lines and it is B3's Task 1, before the agent that uses it.
- **OpenTelemetry is self-instrumented.** Nasiko auto-injects OTel for Python
  containers only. `internal/obs` is ~150 lines, written once by B1, called by
  every agent's `main()`.

A third finding that cost nothing but would have cost an hour live: Nasiko's
`validate_agent_zip` requires a `main.py`, but **only on the dashboard
zip-upload path**. `nasiko deploy` from the CLI does not run that gate. Never
use the dashboard uploader for these agents.

### 2026-09-20 — main — zero values replace pydantic defaults, and Validate is the guard

Go has no field defaults. Three that pydantic supplied are now hazards:
`BrandProfile.Version` (was 1), `Topic.Trend` (was 1.0) and `Mention.Lang`
(was `"en"`) decode as `0`, `0.0` and `""`. The `New*` constructors carry the
intended value and `Validate()` catches the decoder path that bypasses them.

The one that could not be left to a convention is
`ReplyDraft.requires_human_approval`: a `bool` field would decode to `false`
from any payload that omitted it. So the field does not exist on the struct at
all, and `MarshalJSON` emits `true` unconditionally. Consequence: `ReplyDraft`
does not round-trip. Do not add the field back to "fix" that.

### 2026-09-20 — main — a sixth track, B6, owns the product front end

`web/` (Next.js 16, already built and rendering against demo data) and `bff/`
(Fastify). This is a different surface from B5's DronaHQ dashboard: DronaHQ is
the ops and analyst view, `web/` is the product view a customer sees. The
overlap invites duplicated work, so the split is stated here.

### 2026-09-20 — main — no source is faked

If a probe fails, that source does not appear in a fixture, a dashboard, a count
or a sentence. Synthetic data exists in exactly one file,
`demo/inject_crisis.go`, and is labelled as an injection in the UI. `web/` has
its own synthetic set in `web/lib/demoData.ts`, for a deliberately fictional
brand, labelled in the top bar and in the page footnote. Both labels are
load-bearing, not decoration.

---

## Numbers for the pitch

Filled in as they are measured. **Real output only.** An estimate goes in
brackets and is replaced, never quietly promoted.

| Number | Value | Source | Owner |
|---|---|---|---|
| Credits spent recording fixtures | _(est. ~136)_ | B2 Task 7 actual | B2 |
| Mentions in the fixture corpus | — | `fixtures/` count | B2 |
| Sources shipped | _(7, or 6 if Play probe fails)_ | B2 Task 2 | B2 |
| Sentiment accuracy | — | `go run ./eval/accuracy` | B3 |
| Intent accuracy | — | `go run ./eval/accuracy` | B3 |
| Cost per brand-day, cold | _(target < ₹15)_ | `go run ./eval/cost` | B3 |
| Cost per brand-day, warm cache | — | `go run ./eval/cost` | B3 |
| Agent container image size | — | `docker images` after build | B5 |
| Agents deployed on Nasiko | 0 / 9 | `nasiko deploy` | B5 |
| Time from mention to WhatsApp | — | `demo/run_demo.sh` timing output | B4 |
| Nasiko PR | — | link | B5 |
