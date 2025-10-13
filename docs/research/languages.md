# Language choice for BrandPulse A2A agents: Go, Rust, Zig

Research date: 2026-09-20. Every claim below carries a URL and a VERIFIED or UNVERIFIED verdict.
Versions were read from authoritative registries (`proxy.golang.org`, `crates.io` API, PyPI, GitHub API)
on that date, not from memory.

Question: we are building 9 agents that run as containers on the Nasiko platform
(github.com/Nasiko-Labs/nasiko), speaking A2A `protocolVersion` "1.0". The reference implementation is
Python 3.13 with `a2a-sdk[http-server]` on Starlette. Should we rewrite in Go, Rust or Zig?

---

## Summary

| Question | Go | Rust | Zig |
|---|---|---|---|
| **1/2/3. Official A2A SDK** | YES. `github.com/a2aproject/a2a-go/v2` v2.5.0 | YES. `a2a-lf` 0.3.1 + `a2a-server-lf` 0.4.4 | **NO. Nothing exists.** |
| Server side (receive, emit artifacts) | YES, `a2asrv` | YES, `a2a_server::jsonrpc` | n/a |
| protocolVersion 1.0 | YES, `a2a.Version = "1.0"` | YES | n/a |
| **4. Nasiko language requirement** | None. Go agents already ship in-repo | None. Rust agents already ship in-repo | None, but you write the protocol yourself |
| **5. TF-IDF + agglomerative cosine** | Weak. One 21-star lib, else hand-roll | Good. `kodama` 0.3.0 takes a precomputed matrix | Hand-roll everything |
| **6. HTTP / Postgres / LLM stack** | Mature. net/http, pgx v5.11.0, openai-go v3.64.0 (beta) | Mature. reqwest 0.13.5, sqlx 0.9.0, async-openai 0.42.0 (community) | Immature. Language is pre-1.0 |
| **Verdict** | Viable today | Viable today, best-supported on Nasiko | Not viable |

Baseline for comparison: Python `a2a-sdk` is at **1.1.4**, released 2026-09-18, requires Python >=3.10.
VERIFIED: https://pypi.org/pypi/a2a-sdk/json

The headline finding is question 4. The Nasiko repository already contains **10 working Rust agents and
4 working Go agents** alongside its 12 Python ones. The language question is not speculative. It is
already answered by the platform's own reference code.

---

## 1. Go A2A SDK

**VERDICT: VERIFIED. An official Go SDK exists, supports the server side, and implements protocol 1.0.**

| Fact | Value | Verdict |
|---|---|---|
| Package | `github.com/a2aproject/a2a-go` | VERIFIED https://github.com/a2aproject/a2a-go |
| Import path | `github.com/a2aproject/a2a-go/v2` (note the `/v2`) | VERIFIED https://pkg.go.dev/github.com/a2aproject/a2a-go/v2 |
| Latest version | **v2.5.0**, published **2026-08-18** | VERIFIED https://proxy.golang.org/github.com/a2aproject/a2a-go/v2/@latest |
| Full v2 tag list | v2.0.0, v2.0.1, v2.1.0, v2.2.0, v2.2.1, v2.3.0, v2.3.1, v2.4.0, v2.5.0 | VERIFIED https://proxy.golang.org/github.com/a2aproject/a2a-go/v2/@v/list |
| Go floor | Go 1.25.0 or newer | VERIFIED https://github.com/a2aproject/a2a-go |
| Official status | Listed as an official SDK by the A2A project | VERIFIED https://a2a-protocol.org/latest/sdk/ |
| License | Apache 2.0 | VERIFIED https://pkg.go.dev/github.com/a2aproject/a2a-go/v2 |

### Protocol version

VERIFIED https://pkg.go.dev/github.com/a2aproject/a2a-go/v2/a2a#Version

```go
const Version ProtocolVersion = "1.0"
```

The SDK exports the supported protocol version as a constant, and its value is exactly the `"1.0"` we need.

### Server side

VERIFIED https://pkg.go.dev/github.com/a2aproject/a2a-go/v2/a2asrv

The `a2asrv` package is the server half. An agent implements `AgentExecutor`:

```go
Execute(ctx context.Context, execCtx *ExecutorContext) iter.Seq2[a2a.Event, error]
Cancel(ctx context.Context, execCtx *ExecutorContext) iter.Seq2[a2a.Event, error]
```

Events are emitted by yielding from a Go 1.23 iterator. **Artifact emission is first-class**: the event
constructors include `a2a.NewArtifactEvent`, `a2a.NewStatusUpdateEvent` and `a2a.NewSubmittedTask`, with
`ArtifactUpdateEvent` for streamed chunks. VERIFIED, same URL.

Transports: gRPC, REST and JSON-RPC. `a2asrv.NewJSONRPCHandler(handler)` returns a plain `http.Handler`,
and the SDK serves the agent card itself via `a2asrv.WellKnownAgentCardPath` plus
`NewStaticAgentCardHandler(card)`. VERIFIED, same URL.

### Proof it works on Nasiko

VERIFIED by reading `agents/books/main.go` in the Nasiko repo at commit `58cfe60`. The whole agent is
**154 lines**, and the serving wiring is 8 of them:

```go
handler := a2asrv.NewHandler(&booksExecutor{})
mux := http.NewServeMux()
mux.Handle("/a2a", a2asrv.NewJSONRPCHandler(handler))
mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))
```

Its `go.mod` pins `github.com/a2aproject/a2a-go/v2 v2.3.1` with `go 1.25.0`, and only two indirect
dependencies (`google/uuid`, `golang.org/x/sync`). Note Nasiko pins v2.3.1 while v2.5.0 is current, so
there is a small upgrade decision but no blocker.

**Caveat, UNVERIFIED:** I did not test streaming (SSE) end to end. The `books` agent declares
`Capabilities{Streaming: false}` and returns a single message rather than streaming artifacts, so the
in-repo Go proof covers the non-streaming path only. The SDK exposes the streaming types, but no Go
agent in the Nasiko repo exercises them.

---

## 2. Rust A2A SDK

**VERDICT: VERIFIED. An official Rust SDK exists under the a2aproject org, is published to crates.io,
supports the server side including SSE streaming, and implements protocol v1.**

### The official crates

The a2aproject Rust SDK publishes under a `-lf` suffix on crates.io. VERIFIED
https://github.com/a2aproject/a2a-rs

| Crate | Version | Total downloads | Recent (90d) | Last updated | Verdict |
|---|---|---|---|---|---|
| `a2a-lf` (core types) | **0.3.1** | 57,041 | 45,602 | **2026-09-14** | VERIFIED https://crates.io/crates/a2a-lf |
| `a2a-server-lf` (server) | **0.4.4** | 34,325 | 27,556 | **2026-09-14** | VERIFIED https://crates.io/crates/a2a-server-lf |
| `a2a-client-lf` (client) | **0.2.5** | 35,782 | 26,449 | **2026-09-15** | VERIFIED https://crates.io/crates/a2a-client-lf |

All three report `repository = https://github.com/a2aproject/a2a-rs`, which confirms they are the official
org's crates and not a namesquat. VERIFIED via https://crates.io/api/v1/crates/a2a-lf and siblings.

The repo also publishes `a2a-pb` (protobuf), `a2a-grpc` and `a2a-slimrpc`. VERIFIED
https://github.com/a2aproject/a2a-rs

Rust is listed as an official SDK alongside Python, Go, Java, JS and .NET. VERIFIED
https://a2a-protocol.org/latest/sdk/

Server support is explicit: `a2a-server` provides "request handler, routers, streaming, and stores" built
on **axum**, with REST/HTTP+JSON and JSON-RPC transports and "Server-Sent Events for streaming responses."
VERIFIED https://github.com/a2aproject/a2a-rs

**UNVERIFIED:** the a2a-rs README carries no explicit stability or maturity disclaimer, so I cannot quote
one either way. The crate version numbers (0.3.x, 0.4.x) are pre-1.0, which is a signal of expected churn.
The Go SDK at v2.5.0 is nominally further along.

### Community Rust crates (not official, listed for completeness)

| Crate | Version | Downloads | Last updated | Repo | Verdict |
|---|---|---|---|---|---|
| `a2a-rs` | 0.9.2 | 11,465 (4,401 recent) | 2026-09-07 | emillindfors/a2a-rs | VERIFIED https://crates.io/crates/a2a-rs |
| `a2a-rs-core` | 1.0.26 | 29,150 (27,068 recent) | 2026-04-18 | tolgaki/a2a-rs | VERIFIED https://crates.io/crates/a2a-rs-core |
| `a2a-rs-client` | 1.0.26 | 919 (218 recent) | 2026-04-18 | tolgaki/a2a-rs | VERIFIED https://crates.io/crates/a2a-rs-client |
| `ra2a` | 0.10.1 | 2,515 (664 recent) | 2026-04-08 | qntx/ra2a | VERIFIED https://crates.io/crates/ra2a |
| `a2a-protocol-sdk` | 0.12.1 | 982 (696 recent) | 2026-09-17 | tomtom215/a2a-rust | VERIFIED https://crates.io/crates/a2a-protocol-sdk |

Note the name collision: the crate literally named **`a2a-rs` on crates.io is NOT the a2aproject one**. It
belongs to `emillindfors`. The official org's repo is *named* `a2a-rs` but publishes as `a2a-lf` and
friends. This is an easy and expensive mistake to make. Use the `-lf` crates.

**Recommendation: ignore the community crates.** Nasiko uses the official ones and so should we.

### Proof it works on Nasiko

VERIFIED by reading `agents/nutrition/Cargo.toml` and `agents/nutrition/src/main.rs` at commit `58cfe60`.
All 10 Rust agents in the repo declare:

```toml
a2a-server = { git = "https://github.com/a2aproject/a2a-rs", package = "a2a-server-lf" }
a2a        = { git = "https://github.com/a2aproject/a2a-rs", package = "a2a-lf" }
axum = "0.8"
```

Serving is 4 lines:

```rust
let app = axum::Router::new()
    .merge(a2a_server::jsonrpc::jsonrpc_router(handler.clone()))
    .merge(a2a_server::agent_card::agent_card_router(card_producer));
```

Note Nasiko pins these as **git dependencies, not crates.io versions**, even though the crates are
published. `docs/A2A_PROTOCOL.md` references `a2a-lf` 0.3.0 and `a2a-server-lf` 0.4.0 while crates.io is at
0.3.1 and 0.4.4. **UNVERIFIED:** I did not determine why Nasiko tracks git rather than the registry. The
likely reason is that they need unreleased fixes, which would be a mild warning sign about release cadence.

---

## 3. Zig A2A SDK

**VERDICT: VERIFIED that nothing exists. Blunt answer: do not do this.**

### There is no Zig A2A SDK, official or community

| Check | Result | Verdict |
|---|---|---|
| a2aproject GitHub org | 6 language SDKs: Python, JS, Go, Java, Rust, .NET. **No Zig.** | VERIFIED https://github.com/orgs/a2aproject/repositories |
| Official SDK list | Python, Go, Java, JS, C#/.NET, Rust. **No Zig.** | VERIFIED https://a2a-protocol.org/latest/sdk/ |
| GitHub search `a2a protocol language:zig` | **0 results** | VERIFIED https://api.github.com/search/repositories?q=a2a+protocol+language:zig |
| GitHub search `agent2agent language:zig` | **1 result**: `dravenk/adk-zig` | VERIFIED https://api.github.com/search/repositories?q=agent2agent+language:zig |

The single hit, `dravenk/adk-zig`, is an Agent Development Kit port, not an A2A protocol SDK. It has
**0 stars, 0 forks, 1 watcher and 2 total commits**, with no README detail on protocol implementation and
no version. VERIFIED https://github.com/dravenk/adk-zig

The closest useful building block is `williamw520/zigjr`, a JSON-RPC 2.0 library with **50 stars, last
pushed 2025-12-19**. VERIFIED https://github.com/williamw520/zigjr. That gives you the RPC envelope and
nothing else: no A2A types, no task state machine, no agent card, no SSE.

### The language itself is the bigger problem

- Zig 0.16.0 was released **2026-04-14** and is described as a **Beta** release. VERIFIED
  https://ziglang.org/news/0.16.0-released/
- Current master is `0.17.0-dev.2234+80fe9b2b7` dated 2026-09-19. VERIFIED
  https://ziglang.org/download/index.json
- Zig remains **pre-1.0**, with breaking changes between minor releases as a matter of course.
- The 0.16 async I/O story is unfinished: the only usable implementation shipped is `std.Io.Threaded`, while
  `std.Io.Evented` (io_uring / kqueue) "is still a work in progress, missing many functions and doesn't
  currently compile." VERIFIED https://lalinsky.com/2026/05/11/async-io-in-zig-016-today.html
- Third-party HTTP frameworks such as `httpz` are themselves described as experimental. VERIFIED, same URL.

### What choosing Zig would actually cost

You would hand-write, from the specification: the A2A type system, JSON-RPC dispatch for `SendMessage`,
`SendStreamingMessage`, `GetTask` and `CancelTask`, the task state machine, agent card serving, SSE
streaming with correct chunk framing, and the artifact append/lastChunk semantics. Then you would
re-verify all of it against the spec on every Zig minor release, because the language is not stable.

**UNVERIFIED (judgment, not a cited fact):** that is multiple weeks of work to reach parity with something
that is 4 lines of glue in Rust and 8 in Go, for zero functional gain. There is no plausible reason to
pick Zig for this project.

---

## 4. What Nasiko actually requires of an agent container

**VERDICT: VERIFIED. `nasiko deploy` and `nasiko validate` impose NO language requirement. The one
Python-specific gate in the platform sits on a different code path (dashboard zip upload), and it is real.**

All quotes below are from a clone of github.com/Nasiko-Labs/nasiko at commit
`58cfe600559c67d58100ec2856d7b29838e2859f` (2026-09-14).

### The platform says so explicitly

VERIFIED, `README.md` line 217:

> **A2A protocol** | Spec **v1.0** exactly (latest upstream release: v1.0.1, Linux Foundation) | Nasiko
> requires and hardcodes the `A2A-Version: 1.0` header on every request; agents on older/pre-1.0 spec
> versions (e.g. 0.2.x, 0.3.0) are rejected with `-32009 VersionNotSupported`. **Any agent speaking v1.0
> works, regardless of its implementation language.**

And line 106: "**Different language, same A2A protocol** - bring your own agents in Python, Rust, Go, or
TypeScript."

Upstream spec confirmation: A2A spec **v1.0.1** released 2026-05-28 is current. VERIFIED
https://github.com/a2aproject/A2A/releases

### `nasiko validate` requires two files, and warns about a third

VERIFIED, `cli/src/commands/validate.rs`:

```rust
const REQUIRED_FILES: &[&str] = &["Dockerfile", "AgentCard.json"];
const REQUIRED_CARD_FIELDS: &[&str] = &[
    "name", "description", "url", "version",
    "capabilities", "skills", "protocolVersion", "preferredTransport",
];
```

The source-directory check is a **warning, not an error**, and it already knows about Go:

```rust
// src/ directory (Python) or cmd/ (Go) — at least one
let has_src =
    root.join("src").is_dir() || root.join("cmd").is_dir() || root.join("main.go").exists();
if has_src { ... } else {
    warnings.push("no src/ or cmd/ directory found".into());
}
```

A Rust agent passes on `src/`, a Go agent on `main.go` or `cmd/`. Nothing else is language-aware.

### `nasiko build` is a bare `docker build`

VERIFIED, `cli/src/commands/build.rs`:

```rust
if !root.join("Dockerfile").exists() {
    bail!("No Dockerfile found at {}. Run `nasiko new` first.", root.display());
}
...
let mut cmd = Command::new(&bin);
cmd.args(["build", "-t", &resolved_tag]);
```

That is the entire build contract. The same file's error text even references `cargo zigbuild --release`
as an example fix, which shows Rust is an anticipated case, not an afterthought.

### `nasiko deploy` reads AgentCard.json and pushes an image

VERIFIED, `cli/src/commands/deploy.rs`:

```rust
/// Deploy an agent from a directory (reads AgentCard.json) or a raw image.
///
/// Flow:
/// 1. Read AgentCard.json for name + version
/// 2. Push image to CP OCI registry
```

No language detection anywhere in the file. A grep for `python|rust|golang|pyproject|cargo|go.mod` across
`cli/src/commands/deploy.rs` returns **zero matches**. VERIFIED.

### Runtime contract

| Requirement | Detail | Verdict |
|---|---|---|
| Container port | **8000**, canonical | VERIFIED `server/src/agents/mod.rs:25-27`: "Canonical container port for Nasiko agents. Every agent image serves on 8000 ... `pub(crate) const DEFAULT_AGENT_PORT: u16 = 8000;`" |
| `PORT` env var | Injected, defaults to 8000 | VERIFIED `server/src/state.rs:470`: `env.entry("PORT".into()).or_insert_with(\|\| "8000".into());` |
| Agent card path | `/.well-known/agent-card.json`, falling back to `/.well-known/agent.json` | VERIFIED `server/src/agents/utils.rs:24-27` |
| Card fetch retry | 30 attempts, 3s apart (90s budget) | VERIFIED `server/src/agents/utils.rs:176-177` |
| A2A endpoint path | **Agent's choice**, declared in the card's `supportedInterfaces[].url` | VERIFIED: in-repo cards use `/` (17), `/a2a` (4). Go agent mounts `/a2a`, Python agents mount `/` |
| Header | `A2A-Version: 1.0` hardcoded on every request | VERIFIED README line 217 |
| Methods | `SendMessage`, `SendStreamingMessage`, `GetTask`, `CancelTask`. Nasiko primarily uses `SendStreamingMessage` | VERIFIED https://github.com/Nasiko-Labs/nasiko/blob/main/docs/A2A_PROTOCOL.md |
| Health endpoint | **Not required.** No `/health` or liveness contract for agents | VERIFIED: `docs/A2A_PROTOCOL.md` defines none; the only `/health` route in `server/src/lib.rs:351` is the control plane's own |
| Injected headers | `x-user-id`, `x-username`, `x-is-superuser`, `traceparent` | VERIFIED docs/A2A_PROTOCOL.md |

The docs are explicit that no SDK is mandated: "**Other languages:** Implement JSON-RPC 2.0 handler
directly. No language-specific SDK is mandated; HTTP POST to a single endpoint is sufficient." VERIFIED
https://github.com/Nasiko-Labs/nasiko/blob/main/docs/A2A_PROTOCOL.md

### The Python-specific assumptions that DO exist

There are exactly two, and neither blocks the CLI deploy path.

**(a) The zip-upload endpoint hard-requires a Python entrypoint.** This is a genuine gate. VERIFIED,
`server/src/agents/upload.rs:738-747`, reached from `POST /upload` (`upload.rs:31`):

```rust
// At least one Python entrypoint must exist
let entrypoints = ["main.py", "src/main.py", "src/__main__.py", "__main__.py"];
let has_entrypoint = entrypoints.iter().any(|p| dest.join(p).exists());
if !has_entrypoint {
    return Err(
        "no Python entrypoint found (main.py, src/main.py, __main__.py, or src/__main__.py)"
            .into(),
    );
}
```

Scope, VERIFIED: `validate_agent_zip` has exactly one caller (`upload.rs:451`). The GitHub-import path
(`server/src/github.rs`) and the catalog-import path (`server/src/catalog/import.rs`) contain **no Python
checks at all** (grep returns only unrelated comments). So this affects **dashboard/API zip upload only**.
`nasiko deploy` builds and pushes an OCI image and never touches this function.

**Practical consequence:** if we go Go or Rust, we deploy via `nasiko deploy` / CLI or GitHub import, not
via dashboard zip upload. Worth confirming which path the hackathon submission flow uses.

**(b) OTel auto-instrumentation is Python-only, and correctly skips others.** VERIFIED,
`server/src/agents/upload.rs:942-965`:

```rust
/// Patch a Python agent's Dockerfile to auto-install OTel packages and inject
/// the bootstrap script. Skips non-Python Dockerfiles (no `python` base image).
let is_python = contents
    .lines()
    .any(|l| l.trim().starts_with("FROM ") && l.contains("python"));
if !is_python {
    tracing::debug!("otel patch: Dockerfile does not appear Python-based, skipping");
    return;
}
```

The skip is deliberate and the comment explains why the match is narrow: a looser match would inject
`pip install` into a non-Python image and fail the build. So a Go or Rust agent is skipped safely, **but
gets no free tracing**. It must instrument itself. Both in-repo reference agents do:

- Go: the `books` Dockerfile uses `alibaba/loongsuite-go-agent` v1.12.0 for compile-time OTel injection
  via `otel go build`. VERIFIED, `agents/books/Dockerfile`.
- Rust: `agents/nutrition/Cargo.toml` pulls `opentelemetry` 0.28, `opentelemetry_sdk` 0.28,
  `opentelemetry-otlp` 0.28 and `tracing-opentelemetry` 0.29, with a dedicated 152-line
  `src/telemetry.rs`. VERIFIED.

This is the single largest hidden cost of leaving Python: **~150 lines of telemetry wiring per language
that Python gets for free.**

### The decisive evidence: Nasiko already ships Go and Rust agents

VERIFIED by enumerating `agents/*/` manifests at commit `58cfe60`:

| Language | Count | Agents |
|---|---|---|
| **Python** (`pyproject.toml`) | 12 | langgraph, crewai, langchain, claude-sdk, translator, doc-reader, assistant-agent, google-adk, seed-py, summarizer, simulated-agent, openai |
| **Rust** (`Cargo.toml`) | **10** | paper, hr-agent, repo-watch-agent, devops-agent, legal-agent, docs, finance-agent, coding, infra-agent, nutrition |
| **Go** (`go.mod`) | **4** | hackernews, books, github, weather |

Rust is the second most-represented agent language in the platform's own repo, and all 10 Rust agents call
an OpenAI-compatible LLM endpoint. The reference Rust agent `nutrition` is 1,099 lines total across
`main.rs` (460), `tools.rs` (487) and `telemetry.rs` (152). VERIFIED.

Rust also has a structural advantage: **Nasiko itself is written in Rust** (server, CLI, runtime,
orchestrator). Its own `Cargo.toml` uses `axum` 0.8, `sqlx` 0.8, `reqwest` 0.12, `serde` 1. VERIFIED. A
Rust agent shares idioms, and probably bug reports, with the platform team.

---

## 5. TF-IDF plus agglomerative clustering with cosine distance

Target to reproduce: `TfidfVectorizer()` then
`AgglomerativeClustering(metric='cosine', linkage='average', distance_threshold=...)`, on ~300 short docs.

VERIFIED https://scikit-learn.org/stable/modules/generated/sklearn.cluster.AgglomerativeClustering.html
(scikit-learn 1.9.1). Signature takes `metric` (not the old `affinity`), allows `cosine` and `precomputed`,
allows `average` linkage, and supports `distance_threshold`. Only `ward` is restricted to euclidean.

**At n=300, performance is irrelevant.** A naive O(n^3) agglomerative run is ~27 million float operations
and a full distance matrix is 90,000 floats. Both finish in well under a second in either language. The
only real question is library availability and code volume.

**The property that matters in both languages: the usable clustering libraries accept a precomputed
distance matrix.** That means you compute cosine distances yourself and the "does it support cosine"
question disappears entirely.

### Rust: good. Use `kodama`.

| Fact | Value | Verdict |
|---|---|---|
| Version | **0.3.0**, published **2023-01-04** | VERIFIED https://crates.io/api/v1/crates/kodama |
| Total downloads | **552,278** | VERIFIED, same |
| Recent (90d) | **141,414** | VERIFIED, same |
| Stars / last commit | 106 stars, last commit 2025-04-09, not archived | VERIFIED https://api.github.com/repos/diffeo/kodama |

API, VERIFIED https://docs.rs/kodama and source at
https://raw.githubusercontent.com/diffeo/kodama/master/src/lib.rs:

```rust
pub fn linkage<T: Float>(
    condensed_dissimilarity_matrix: &mut [T],
    observations: usize,
    method: Method,
) -> Dendrogram<T>
```

It takes a **condensed distance matrix in exactly scipy's format**, and `Method` has all seven variants
including `Average` and `Complete`. Implementation follows Mullner 2011, the same lineage as scipy and
fastcluster. `Dendrogram` yields `N-1` steps of `(cluster1, cluster2, dissimilarity, size)`.

`Method::Average` over your own cosine distances is an **exact** match for
`AgglomerativeClustering(metric='cosine', linkage='average')`.

Honest maintenance read, UNVERIFIED (judgment): no release in 3.5 years and the last commit was a docs
fix. But 141k downloads in 90 days with zero API churn on a self-contained numeric algorithm reads as
"finished", not "abandoned". The risk profile differs sharply from a stale web framework.

**Rust crates to avoid, with reasons:**

- `linfa-hierarchical` 0.8.1 (VERIFIED https://crates.io/crates/linfa-hierarchical, updated 2025-12-23, but
  only **236 downloads in 90 days**). It is a thin wrapper over kodama that **pins kodama 0.2** and takes a
  `linfa_kernel::Kernel`, not a distance matrix. VERIFIED from source
  (https://raw.githubusercontent.com/rust-ml/linfa/master/algorithms/linfa-hierarchical/src/lib.rs) it
  converts similarity to distance by **negative log**: `.map(|x| if x > threshold { -x.ln() } else ... )`.
  And `linfa-kernel` offers only `Gaussian`, `Linear` and `Polynomial`, **no cosine** (VERIFIED
  https://raw.githubusercontent.com/rust-ml/linfa/master/algorithms/linfa-kernel/src/lib.rs). Even the
  L2-normalize-plus-`Linear` trick fails, because clustering on `-ln(cos_sim)` rather than `1 - cos_sim`
  **does not preserve average linkage** (the mean of `-ln(x)` is not `-ln` of the mean) and your threshold
  ends up in log space instead of the `[0,2]` cosine range you tuned in Python.
- `linfa-clustering` 0.8.1: **no agglomerative at all.** VERIFIED, module list is
  `mod dbscan; mod gaussian_mixture; mod k_means; mod optics;`
  (https://raw.githubusercontent.com/rust-ml/linfa/master/algorithms/linfa-clustering/src/lib.rs).
- `smartcore` 0.6.14 has a `cluster::agglomerative` module and is the most actively maintained of the lot
  (updated 2026-08-26, VERIFIED https://crates.io/crates/smartcore), but it is a trap. VERIFIED from source
  (https://raw.githubusercontent.com/smartcorelib/smartcore/main/src/cluster/agglomerative.rs): **single
  linkage only** (hardcoded), **squared Euclidean only** (hardcoded inline, no metric parameter), and
  `n_clusters` only with no distance threshold. Unusable here.

**Rust TF-IDF crates: all dead or trivial.** Best of a bad lot is `rust-tfidf` 1.1.1, last updated
2021-05-18 with 1,232 downloads in 90 days (VERIFIED https://crates.io/crates/rust-tfidf). `vtext` 0.2.0
last updated 2020-06-14 (VERIFIED https://crates.io/crates/vtext). `tfidf` 0.3.0 last updated 2017
(VERIFIED https://crates.io/crates/tfidf). **No maintained general-purpose TF-IDF vectorizer exists in
Rust.** Hand-roll it.

### Go: weak. One lightly-maintained library, or hand-roll.

**Blunt finding, VERIFIED: https://pkg.go.dev/search?q=agglomerative returns exactly ONE package.** That
package is the entire Go ecosystem for this problem.

| Fact | Value | Verdict |
|---|---|---|
| Package | `github.com/knightjdr/hclust` | VERIFIED https://pkg.go.dev/github.com/knightjdr/hclust |
| Latest tag | **v1.0.2, published 2020-01-13** | VERIFIED https://proxy.golang.org/github.com/knightjdr/hclust/@latest |
| Last commit | **2023-03-28** | VERIFIED https://api.github.com/repos/knightjdr/hclust/commits |
| Stars / forks | **21 / 6**, not archived | VERIFIED https://api.github.com/repos/knightjdr/hclust |
| Imported by | 6 packages | VERIFIED https://pkg.go.dev/github.com/knightjdr/hclust |

The saving grace, VERIFIED from source
(https://raw.githubusercontent.com/knightjdr/hclust/master/cluster/cluster.go):

```go
// Cluster clusters a square symmetric matrix and returns a dendrogram. Linkage
// method options are: average, centroid, complete, mcquitty, median, single and ward.
func Cluster(matrix [][]float64, method string) (dendrogram []typedef.SubCluster, err error)
```

It takes a **precomputed square symmetric matrix** and supports `average` and `complete`, routed through
the nearest-neighbour-chain algorithm following Mullner. Its own `hclust.Distance` does not offer cosine
(VERIFIED, README) but that is irrelevant since you bypass it.

**UNVERIFIED:** threshold cutting. `Cluster` returns `[]SubCluster{Leafa, Leafb, Lengtha, Lengthb, Node}`,
which is branch lengths rather than a scipy-style linkage matrix. `hclust.GetNodeHeight` exists (VERIFIED
https://raw.githubusercontent.com/knightjdr/hclust/master/hclust.go) but the exact code to convert heights
into flat labels at a threshold was not verified. Budget 30 to 50 lines for an `fcluster` equivalent.

**Go libraries that do NOT work:**

- `gonum.org/v1/gonum` v0.17.0 (2025-12-29, healthy, 8,428 stars) has **no clustering package at all**.
  VERIFIED: top-level dirs are `blas, cmplxs, diff, dsp, floats, graph, integrate, interp, lapack, mat,
  mathext, num, optimize, spatial, stat, unit` and `stat` subpackages are `card, combin, distmat, distmv,
  distuv, mds, samplemv, sampleuv, spatial` (https://api.github.com/repos/gonum/gonum/contents/stat).
  Matrices yes, clustering no.
- `github.com/james-bowman/nlp`: **abandoned**, latest is pseudo-version
  `v0.0.0-20210511120306-26d441fa0ded` from **2021-05-11**, no tags, no go.mod. VERIFIED
  https://proxy.golang.org/github.com/james-bowman/nlp/@latest. It does have `CountVectoriser`,
  `TfidfTransformer` and `pairwise.CosineDistance`, but two gotchas make it worse than useless here, both
  VERIFIED from https://raw.githubusercontent.com/james-bowman/nlp/master/weightings.go: **(1) it does not
  L2-normalize** (there is a literal `// todo: possibly L2 norm matrix` comment in `Transform`, while
  sklearn L2-normalizes by default), and **(2) its IDF formula is `math.Log(float64(1+n)/float64(1+df))`,
  missing sklearn's trailing `+ 1`** under the default `smooth_idf=True`. Output will not match Python.
  Also the matrix is terms-by-documents, transposed relative to sklearn.
- `github.com/muesli/kmeans` v0.3.1 (2022-07-22) and `github.com/muesli/clusters` (2020): k-means only.
  `github.com/pbnjay/clustering` (2018): dead. All VERIFIED via proxy.golang.org and the GitHub API.

### Honest effort estimate

**Both are hours, not days.** Specifically:

| Language | Estimate | Lines | Dependencies |
|---|---|---|---|
| **Rust** | **3 to 5 hours** | ~150 to 200 | `kodama = "0.3"`, that is all |
| **Go (with hclust)** | **4 to 6 hours** | ~180 to 250 | `github.com/knightjdr/hclust` |
| **Go (zero deps)** | **5 to 7 hours** | ~240 to 320 | none |

Breakdown for Rust: tokenize 15-25 lines, vocabulary and counts 25-35, IDF and L2 normalize 20-30,
condensed cosine matrix 10-15 (trivial once rows are L2-normalized: `dist = 1.0 - dot(a,b)`),
`kodama::linkage(...)` **1 line**, dendrogram cut 30-45, glue and tests 30-50.

Go is slightly more because `hclust` wants a full square matrix rather than a condensed one and its
dendrogram format is less convenient. The zero-dependency Go option adds 60 to 80 lines for the linkage
loop itself: maintain `active []bool`, `sizes []int`, `dist [][]float64`, loop `n-1` times scanning O(n^2)
for the minimum active pair, merge, Lance-Williams update, deactivate. Given hclust's maintenance status
(21 stars, no release since 2020), vendoring it or hand-rolling is defensible rather than paranoid.

These estimates are **UNVERIFIED judgment**, not measured.

**The gotcha nobody budgets for, UNVERIFIED:** matching sklearn bit-for-bit. The defaults you are
reimplementing are `lowercase=True`, `token_pattern=r"(?u)\b\w\w+\b"`, `smooth_idf=True`,
`sublinear_tf=False`, `norm='l2'`. If cluster output must match the Python version exactly, add an
afternoon of diffing vectors on a fixed corpus. If "similar clusters" is good enough, skip that.

---

## 6. HTTP client, Postgres driver, and JSON-schema-constrained LLM calls

Versions below were confirmed twice: once by a research pass and once by a direct query to
`crates.io/api/v1/crates/<name>` and `go.dev/dl/?mode=json` on 2026-09-20. The two passes agreed exactly.

### Recommended stacks

| Requirement | Go | Rust |
|---|---|---|
| Language version | **go1.27.1** (2026-09-01) | stable, edition 2024 |
| HTTP client | stdlib `net/http` | `reqwest` **0.13.5** |
| HTTP server | stdlib `ServeMux`, or `chi` **v5.3.2** | `axum` **0.8.9** |
| Postgres | `pgx/v5` **v5.11.0** + `pgxpool` | `sqlx` **0.9.0** with built-in `PgPool` |
| LLM json_schema | `openai-go` **v3.64.0** (official, **beta**) | `async-openai` **0.42.0** (community, **no official SDK**) |
| Schema generation | `invopop/jsonschema` **v0.14.0** (still v0) | `schemars` **1.2.2** (stable 1.x) |
| Serialization | `encoding/json` + `encoding/json/v2` (now stable) | `serde` **1.0.229** / `serde_json` **1.0.151** |

### Go details

| Item | Fact | Verdict |
|---|---|---|
| Go stable | **go1.27.1**, released 2026-09-01. go1.26.8 also supported | VERIFIED https://go.dev/dl/?mode=json |
| `net/http` | Still the obvious client. Go 1.27 added auto-draining of unread `Response.Body` on `Close()` for better connection reuse, RFC 9218 priority, `Server.MaxHeaderValueCount`, and `httptest.NewTestServer()` for `testing/synctest` | VERIFIED https://go.dev/doc/go1.27 |
| Enhanced `ServeMux` | Landed **Go 1.22**. Method matching (`"GET /posts/{id}"`, 405 on mismatch), `{name}` wildcards, `{name...}` catch-all, `{$}`, `Request.PathValue` | VERIFIED https://go.dev/blog/routing-enhancements |
| `chi` | **v5.3.2** (2026-08-20), last commit 2026-09-18 | VERIFIED https://proxy.golang.org/github.com/go-chi/chi/v5/@latest |
| `gin` | **v1.12.0** (2026-02-28), very active | VERIFIED https://proxy.golang.org/github.com/gin-gonic/gin/@latest |
| `echo` | **v5.3.1** (2026-07-21). **Note the module path moved to `/v5`**; v4 is at v4.15.4 | VERIFIED https://proxy.golang.org/github.com/labstack/echo/v5/@latest |
| **pgx** | **v5.11.0**, released **2026-09-07**. Still the mature choice, actively released | VERIFIED https://proxy.golang.org/github.com/jackc/pgx/v5/@latest |
| **pgx v6?** | **Does not exist.** Proxy returns `not found: module github.com/jackc/pgx/v6: no matching versions` | VERIFIED, direct query |
| `database/sql` adapter | `github.com/jackc/pgx/v5/stdlib`. Costs you native types and the binary fast path | VERIFIED https://pkg.go.dev/github.com/jackc/pgx/v5 |
| Pooling | `github.com/jackc/pgx/v5/pgxpool`, ships in the same module. Do not stack it with `sql.DB` pooling | VERIFIED, same |

**Correction to a widely-repeated claim: `lib/pq` is NOT deprecated.** VERIFIED: the current README at
https://raw.githubusercontent.com/lib/pq/master/README.md contains **no maintenance-mode and no
deprecation notice**. The repo has commits through 2026-08-20 including SCRAM iteration controls and ALPN
support, with v1.12.3 released 2026-04-03 (VERIFIED https://api.github.com/repos/lib/pq/commits and
https://proxy.golang.org/github.com/lib/pq/@latest). Prefer pgx anyway, but do not repeat the deprecation
claim.

**LLM structured outputs in Go.** `github.com/openai/openai-go` is at **v3.64.0, tagged 2026-09-20**
(VERIFIED https://proxy.golang.org/github.com/openai/openai-go/v3/@latest). Module path is
`github.com/openai/openai-go/v3`. OpenAI labels the Go SDK **beta** (VERIFIED
https://developers.openai.com/api/docs/libraries). SDK v3.45.0+ requires Go 1.25+.

The exact wire shape we need is first-class. VERIFIED from source in `chatcompletion.go` and `shared/shared.go`:

```go
ResponseFormat ChatCompletionNewParamsResponseFormatUnion `json:"response_format,omitzero"`
OfJSONSchema *shared.ResponseFormatJSONSchemaParam `json:",omitzero,inline"`

type ResponseFormatJSONSchemaJSONSchemaParam struct {
    Strict param.Opt[bool] `json:"strict,omitzero"`
    Schema any             `json:"schema,omitzero"`
}
```

Community alternative `github.com/sashabaranov/go-openai` is at **v1.42.1 (2026-09-11)**, still actively
maintained with a 2026-09-11 commit literally titled "fix: preserve json_schema verbatim when unmarshaling
response_format". VERIFIED https://proxy.golang.org/github.com/sashabaranov/go-openai/@latest. It ships its
own `jsonschema.Definition`, so it needs no separate schema crate.

**Go serialization, the biggest change versus stale assumptions: `encoding/json/v2` is STABLE and generally
available in Go 1.27**, no longer behind a GOEXPERIMENT. VERIFIED https://pkg.go.dev/encoding/json/v2 and
https://go.dev/doc/go1.27. Critically, **v1 `encoding/json` is now implemented on top of v2**, and v2
defaults reject invalid UTF-8 and duplicate object names. Escape hatch is `GOEXPERIMENT=nojsonv2`, expected
to be removed later. **Practical risk for us:** if an LLM provider returns duplicate keys or invalid UTF-8,
Go 1.27 will now reject what Go 1.26 accepted, even through the v1 API. Test that path.

### Rust details

| Crate | Version | Last updated | 90d downloads | Verdict |
|---|---|---|---|---|
| `reqwest` | **0.13.5** | 2026-09-08 | 186,849,081 | VERIFIED https://crates.io/api/v1/crates/reqwest |
| `axum` | **0.8.9** | 2026-04-14 | 118,135,320 | VERIFIED https://crates.io/api/v1/crates/axum |
| `actix-web` | 4.15.0 | 2026-08-21 | 10,805,586 | VERIFIED https://crates.io/api/v1/crates/actix-web |
| `hyper` | 1.11.1 | 2026-08-28 | 205,837,649 | VERIFIED https://crates.io/api/v1/crates/hyper |
| `sqlx` | **0.9.0** | 2026-05-21 | 37,910,937 | VERIFIED https://crates.io/api/v1/crates/sqlx |
| `tokio-postgres` | 0.7.18 | 2026-06-12 | 17,362,413 | VERIFIED https://crates.io/api/v1/crates/tokio-postgres |
| `deadpool-postgres` | 0.14.2 | 2026-08-26 | 5,943,835 | VERIFIED https://crates.io/api/v1/crates/deadpool-postgres |
| `diesel` | 2.3.13 | 2026-09-04 | 7,219,108 | VERIFIED https://crates.io/api/v1/crates/diesel |
| `bb8` | 0.9.1 | 2025-11-24 | 7,272,112 | VERIFIED https://crates.io/api/v1/crates/bb8 |
| `async-openai` | **0.42.0** | 2026-09-09 | 2,573,708 | VERIFIED https://crates.io/api/v1/crates/async-openai |
| `schemars` | **1.2.2** | 2026-07-27 | 169,111,475 | VERIFIED https://crates.io/api/v1/crates/schemars |
| `serde` | 1.0.229 | 2026-07-18 | 314,267,592 | VERIFIED https://crates.io/api/v1/crates/serde |
| `serde_json` | **1.0.151** | 2026-07-20 | 317,495,418 | VERIFIED https://crates.io/api/v1/crates/serde_json |

**Version-line corrections to stale assumptions, all VERIFIED by direct registry query:** reqwest is on
**0.13.x** not 0.12.x. sqlx is on **0.9.0** not 0.8.x. schemars is on **1.2.2** not 0.8.x. **There is no
axum 0.9** (latest tags are `axum-v0.8.9`, `axum-extra-v0.12.6`, all 2026-04-14, VERIFIED
https://api.github.com/repos/tokio-rs/axum/releases). Any migration guide you have for these is stale.

**Is sqlx still maintained? VERIFIED YES.** Commits on `main` as recent as **2026-09-14**, with a steady
stream through August and September 2026 (PgInterval infinity checks, Postgres connection and transaction
fixes, TCP_NODELAY on 2026-08-17). VERIFIED https://github.com/launchbadge/sqlx/commits/main

**Mainstream async Postgres choice is sqlx** (UNVERIFIED judgment): ~2.2x tokio-postgres' recent download
volume, a built-in async pool so you need no second crate, compile-time checked queries, built-in
migrations. tokio-postgres is the lower-level client others build on; pick it only with `deadpool-postgres`
0.14.2, which is far more recently maintained than `bb8` (2026-08-26 vs 2025-11-24). diesel is a full ORM
and the wrong shape for a thin agent.

**Mainstream server is axum** (UNVERIFIED judgment): it out-downloads actix-web roughly 11 to 1 over 90
days, it is in the Tokio org so it shares `tower` middleware with reqwest and the rest of the stack, and
the official a2a-rs server crate is **built on axum anyway**, which settles it for us. hyper's higher
download count is because axum and reqwest depend on it, not because people use it directly. One honest
caveat: axum 0.8.9 has not shipped in five months, the longest gap of anything in this table. That reads as
stable given Tokio's cadence, but it is UNVERIFIED whether it signals maturity or slowing maintenance.

**There is still NO official OpenAI Rust SDK.** VERIFIED https://developers.openai.com/api/docs/libraries
lists exactly TypeScript/JavaScript, Python, .NET, Java (beta), Go (beta), Ruby. `async-openai` appears
under community libraries with OpenAI's standard disclaimer that it "does not verify the correctness or
security of these projects."

`async-openai` 0.42.0 does support the exact shape we need. VERIFIED from source at
https://raw.githubusercontent.com/64bit/async-openai/main/async-openai/src/types/shared/response_format.rs:

```rust
#[serde(tag = "type", rename_all = "snake_case")]
enum ResponseFormat { Text, JsonObject, JsonSchema { json_schema: ResponseFormatJsonSchema } }

struct ResponseFormatJsonSchema {
    description: Option<String>,
    name: String,          // a-zA-Z0-9_- , max 64 chars
    schema: serde_json::Value,
    strict: Option<bool>,
}
```

It ships `examples/structured-outputs`, `examples/structured-outputs-schemars` and, importantly,
`examples/gemini-openai-compatibility`, which confirms it works against non-OpenAI OpenAI-compatible
endpoints. VERIFIED via GitHub code search on the repo.

`schemars` 1.2.2 integrates with serde derive: "Schemars will check for any `#[serde(...)]` attributes on
types that derive `JsonSchema`, and adjust the generated schema accordingly." VERIFIED
https://graham.cool/schemars/. So `#[serde(rename)]`, `#[serde(skip)]` and `#[serde(rename_all)]` are all
reflected, which means the Rust type, the wire JSON and the schema cannot drift. The async-openai schemars
example pins `schemars = "1.1"` and passes `schema_for!(T)` straight into `ResponseFormatJsonSchema` with
`strict: Some(true)`. VERIFIED.

### What Nasiko's own code actually does

Worth weighing against the library research. VERIFIED by reading the repo at commit `58cfe60`:

- **Nasiko's server** uses `axum` 0.8, `sqlx` 0.8 (with `runtime-tokio`, `postgres`, `chrono`, `uuid`,
  `rust_decimal`, `migrate`, `json`), `reqwest` 0.12 (`rustls-tls`), `serde` 1, `uuid` 1, `chrono` 0.4.
  Note these are a version line behind current on sqlx and reqwest.
- **All 10 Rust agents hand-roll the LLM call** with `reqwest` plus `serde_json::Value`. None uses
  `async-openai`. VERIFIED, `agents/nutrition/src/main.rs:71-89`:
  ```rust
  let resp = self.http
      .post(format!("{}/chat/completions", self.base_url))
      .bearer_auth(&self.api_key)
      .json(&body)
      .send().await
  ```
- **No agent in the repo, in any language, uses `response_format: json_schema`** except one Python LangGraph
  agent. VERIFIED, grep for `response_format|json_schema` across `agents/` returns a single hit at
  `agents/langgraph/src/agent.py:99`.
- **No agent uses Postgres.** Grep for `sqlx|tokio-postgres|pgx|postgres` across all agent manifests
  returns nothing. The reference agents are stateless; persistence lives in the control plane.

So if BrandPulse agents need Postgres and schema-constrained LLM calls, we are ahead of what any Nasiko
reference agent does, in any language. That is not a blocker, but it means less copy-paste and more
first-party work.

**Hand-rolling the LLM call** is 80 to 120 lines in Rust and 120 to 180 in Go, including retry-on-429 with
backoff and a context timeout. UNVERIFIED judgment. The transport is not the hard part. The hard parts are
generating the schema and handling provider deviations: refusals, `finish_reason: "length"` truncating a
JSON body mid-object, and vendors that accept `json_schema` but silently ignore `strict`. No SDK solves
those.

---

## Gaps and risks not closed

Stated plainly rather than papered over.

1. **Provider-side `strict: true` fidelity is UNVERIFIED.** I did not test how any specific
   OpenAI-compatible endpoint (Nasiko's own llm-router, vLLM, Ollama, Together) honours strict JSON schema
   mode. This is the single biggest risk to the schema-constrained LLM plan, and it is **language
   independent**, so it is not an argument for or against the rewrite.
2. **Streaming was not tested end to end in Go.** The in-repo Go agent declares `streaming: false`. Nasiko
   "primarily uses `SendStreamingMessage`" per its own docs, so this gap matters. The Rust agents do stream.
3. **Nothing was benchmarked.** All performance statements here are structural reasoning.
4. **Nasiko pins the Rust A2A crates from git, not crates.io**, and its docs reference older versions
   (`a2a-lf` 0.3.0, `a2a-server-lf` 0.4.0) than crates.io carries (0.3.1, 0.4.4). Why is UNVERIFIED. It may
   indicate they depend on unreleased fixes.
5. **axum 0.8.9's five-month release gap** is verified as a fact; whether it signals maturity or slowing
   maintenance is UNVERIFIED.
6. **The `nasiko deploy` path was read, not executed.** I verified the code requires only a Dockerfile plus
   AgentCard.json, but I did not run a Go or Rust deploy against a live control plane.

## The argument against rewriting at all

Recorded because the research supports it and omitting it would be dishonest.

Nothing found here blocks Go or Rust. Both have an official A2A SDK with server support and protocol 1.0,
both have mature HTTP, Postgres and LLM tooling, and Nasiko ships working reference agents in both. The
rewrite is feasible.

But the costs are concrete and the benefits were never stated. The costs: about 150 lines of OpenTelemetry
wiring per agent that Python gets injected for free; 150 to 300 lines of hand-rolled TF-IDF and
agglomerative clustering that you then own and must test, replacing 10 lines of scikit-learn; loss of the
dashboard zip-upload deploy path; and no Postgres or structured-output precedent to copy from in any
Nasiko reference agent. The benefits, for 9 agents doing HTTP calls and LLM calls on 300 documents, are
UNVERIFIED and likely small, since nothing in this workload is CPU-bound.

A rewrite is justified by a deployment constraint, such as needing a single static binary or a sub-10MB
`FROM scratch` image (the in-repo Go and Rust agents both build to `FROM scratch`), not by the numerics.
If no such constraint exists, the honest recommendation is to stay on Python and spend the time elsewhere.

If we do rewrite, **Rust is the better choice**: the platform is written in Rust, 10 of the repo's agents
are Rust, `kodama` is a genuinely good clustering answer where Go has one 21-star package, and the official
Rust A2A server crate is built on axum which is already the mainstream Rust server.
