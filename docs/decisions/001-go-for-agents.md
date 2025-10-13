# ADR 001 — Go for the agents, TypeScript for the product surface

**Date:** 2026-09-20
**Status:** accepted
**Supersedes:** the Python 3.13 stack in the kickoff brief and in PLAN.md as
originally written.

---

## Decision

| Layer | Language | Why it is there |
|---|---|---|
| The nine A2A agents | **Go** | Containers on Nasiko. Concurrency and small images are the whole job. |
| BFF / API | **TypeScript, Fastify** | One typed HTTP surface in front of nine agents. |
| Product dashboard | **TypeScript, Next.js** | The demo UI a judge actually looks at. |
| WhatsApp agent + ops dashboard | **DronaHQ** | Hackathon requirement, and genuinely the fastest WhatsApp path. |
| Schema | **Postgres 16** | Unchanged. |

Rust was the other candidate. Go wins, and the margin is not close on a
deadline.

## Why Go over Rust

**Iteration speed is the binding constraint.** This is a hackathon with nine
agents, five parallel tracks and a live deploy. Rust's compile-check-fix loop
costs more per change, and the borrow checker taxes exactly the code we write
most of: passing slices of mentions between stages. Go's cost is verbosity,
which is cheap. Rust's cost is time, which is what we do not have.

**Goroutines are literally the orchestrator's job.** `bp-orchestrator` fans out
to N source collectors under a flow-guard cap, gathers partial results and
tolerates one source failing. That is `errgroup` with `SetLimit` and about
fifteen lines. In Rust it is `tokio`, `JoinSet` and a lifetime argument with
myself.

**Static binaries make the deploy story better.** A `FROM scratch` or
`distroless` image with one Go binary is single-digit megabytes. Nine of those
deploy and cold-start faster than nine Python images, and "our whole fleet is
40MB" is a real line in the pitch. Rust matches this; Python does not.

**The ML piece is affordable in Go.** `bp-clusterer` needs TF-IDF plus
agglomerative clustering with cosine distance over about 300 short documents.
There is no scikit-learn in Go, so we hand-roll it: tokenise, term-frequency
map, IDF over the corpus, cosine similarity matrix, average-linkage merge to a
distance threshold. That is a few hundred lines and a few hours, on a corpus
small enough that an O(n^2) similarity matrix is 90,000 floats. It is also
**deterministic and dependency-free**, which is strictly better for CI than
pulling scikit-learn into a container.

**Rust's one real advantage does not pay.** Nasiko's control plane is Rust, so
Rust agents would rhyme with the platform. That is a narrative point, not an
engineering one. Nasiko deploys containers and speaks A2A over HTTP; it does
not care what is inside.

## The SDK risk, now resolved

This decision was taken with the Go A2A SDK unverified and three fallbacks
budgeted. `docs/research/languages.md` has since landed and the best case is the
real one:

- **`github.com/a2aproject/a2a-go/v2` v2.5.0** (2026-08-18, verified on
  proxy.golang.org), official, with server support in `a2asrv`, artifact events,
  and `a2a.Version = "1.0"`. No hand-written JSON-RPC surface is needed.
- **Nasiko already runs four Go agents.** Their `books` agent is 154 lines, of
  which 8 are serving glue. The platform path is proven, not inferred.
- The supporting libraries are current: `pgx` v5.11.0, `openai-go` v3.64.0 with
  first-class strict JSON-schema response formats, which is exactly what
  `bp-enricher` needs.

Rust scored higher on raw fit: official `a2a-lf` / `a2a-server-lf` crates, ten
Rust agents on Nasiko, and `kodama` doing the clustering in one line. It loses
anyway, because Nasiko pins those crates **from git rather than crates.io** and
their docs trail the registry, which reads as needing unreleased fixes. Pre-1.0
dependencies pinned to a moving git ref is the wrong risk to carry on a
deadline. The Go clustering cost is known and bounded; the Rust packaging cost
is not.

## Two costs the research found that this decision did not price

Both are real and neither reverses the decision.

**1. OTel auto-instrumentation is Python-only.** Nasiko injects tracing only
into Dockerfiles with a `FROM python` base, so Go agents self-instrument. The
research put this at roughly 150 lines per agent, which across nine agents would
be the single largest hidden cost of leaving Python. It is not, because it is
150 lines **once** in `internal/obs/`, exporting one `obs.Init(serviceName)` that
every agent calls on the first line of `main`. Nine copies would be the mistake;
one shared package is the design we already have for `internal/a2a/`. B5 owns it
and it is on the critical path for `nasiko observe` showing our spans.

**2. There is a Python gate, but not on our path.** `validate_agent_zip` in
Nasiko's `server/src/agents/upload.rs` hard-rejects any zip without
`main.py`, `src/main.py` or `__main__.py`. It has exactly one caller and it
guards **dashboard zip upload only**. `nasiko deploy`, GitHub import and catalog
import all bypass it. B5 must confirm the submission uses `nasiko deploy` and
never the dashboard upload, because that one path would reject our fleet on a
filename.

**3. The clustering really is hand-rolled.** pkg.go.dev returns exactly one
agglomerative clustering package, `knightjdr/hclust`, 21 stars and no release
since 2020, and gonum has no clustering package at all. We hand-roll 240 to 320
lines in `internal/cluster/`, as the ADR already assumed. Depending on a dead
21-star repo for the demo's centrepiece is the worse option.

**Zig is rejected.** No A2A tooling, no mature Postgres driver, no HTTP
ecosystem to speak of, and a language still changing under us. Nine protocol
servers in Zig on a deadline is not a stretch goal, it is a way to ship nothing.

## Why TypeScript sits in front, and what it must not become

Fastify is a **backend for frontend**, not a second brain. It owns:

- one typed REST surface that Next.js, DronaHQ and the WhatsApp webhook all
  call, instead of three clients each wiring up nine agent URLs
- aggregation reads that are plain SQL against Postgres (mention stream,
  sentiment series, alert log, cost panel) and have no business waking an agent
- the Meta WhatsApp webhook endpoint, verify token and signature check
- auth for the demo brand

It must **not** hold business logic. Sentiment, clustering, alerting, drafting
and briefing live in the agents, because that is what makes Nasiko load-bearing.
A rule that fires in Fastify is a rule that is invisible to `nasiko observe`.

**DronaHQ stays load-bearing** and this is not negotiable, because it is a
judged criterion. It keeps the WhatsApp agent and the ops dashboard. Next.js is
the polished product surface on top, not a replacement. If time runs short,
Next.js is the thing we cut, and the demo still works.

## What this costs us

Honest list, so nobody is surprised at 3am:

- **The contracts get rewritten.** `models.py` becomes Go structs with JSON
  tags, and the parity test that guards the schema has to be rewritten in Go.
  That work is already-understood work, which is why it is cheap to redo now
  and expensive to redo in six hours.
- **No pydantic.** Go's `encoding/json` does not validate. Validation becomes
  explicit `Validate() error` methods on the structs that need it, and we lose
  the "the model is the validator" property.
- **No scikit-learn.** Covered above. Hand-rolled, tested against a hand-built
  corpus where the right answer is obvious.
- **Two languages in one repo**, plus DronaHQ's exported JSON. The module
  boundary has to be sharp or the BFF starts growing logic.

## What does not change

Everything that mattered about Phase 0 survives, because it was never
Python-specific:

- the nine-agent topology and their input/output signatures
- `db/migrations/001_init.sql`, unchanged, including both idempotency keys
- the five deterministic detector rules and their thresholds
- the source strategy and the 300-credit budget
- the guardrails: no LLM in the detector, nothing auto-posts, redact before the
  prompt, no faked source
