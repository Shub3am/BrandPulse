# Agent Contracts

The frozen interface between the nine BrandPulse agents. Tracks B1–B5 build in
parallel against this document and do not need to read each other's code.

**Freeze rule:** once `main` carries this file, no track changes a signature
alone. A change is proposed in `HACKATHON_NOTES.md`, agreed, then landed on
`main` by B1 and rebased into every worktree.

Every model referenced here lives in [shared/bp_core/models.py](../shared/bp_core/models.py).
That file is the machine-readable version of this document; where the two
disagree, the code wins and this file gets corrected.

---

## 1. Transport

Every agent is an A2A server. A request arrives as a message whose single part
is JSON matching the agent's **Input** below. Every agent replies with exactly
one artifact:

- `mimeType`: `application/json`
- `name`: the agent's output model name, e.g. `MentionBatch`
- body: `Model.model_dump(mode="json")`

Errors do not raise. An agent that fails returns its normal output model with
the `errors` list populated and partial data in place, so the orchestrator can
degrade instead of dying. The one exception is malformed input, which returns
an A2A task failure.

Datetimes are ISO 8601 with an explicit UTC offset. IDs are strings, generated
as `f"{prefix}_{ulid}"` via `bp_core.ids.new_id(prefix)`.

---

## 2. Per-agent signatures

### bp-onboarder

```
Input:  {"brand_id": str, "name": str, "website": str|null,
         "competitors": [str], "max_pages": int = 25}
Output: BrandProfile
```

Crawls the website with Anakin Map + Crawl, extracts product names, asks the
LLM for a keyword set (one call), returns an **unconfirmed** profile with
`version = 1`. The human confirms in DronaHQ; confirmation writes
`brand_profiles.confirmed_at` and does not re-run the agent.

Budget: ≤ 30 Anakin credits, ≤ 1 LLM call.

---

### bp-collector

```
Input:  {"profile": BrandProfile, "source": Source,
         "window_start": datetime, "window_end": datetime,
         "max_credits": int, "run_id": str}
Output: MentionBatch = {"mentions": [Mention], "credits_used": int,
                        "cache_hits": int, "source": Source,
                        "truncated": bool, "errors": [str]}
```

One call handles **one source**. The orchestrator fans out across sources and
is the only component that decides how many run. The collector never exceeds
`max_credits`; if it would, it stops early and sets `truncated: true`.

Dedupe is the collector's job: the returned list has unique `content_hash`
within it, and the collector skips hashes already in `mentions` for that brand.

No LLM calls. Ever. This keeps collection cost linear in credits alone.

---

### bp-enricher

```
Input:  {"mentions": [Mention], "profile": BrandProfile,
         "batch_size": int = 50}
Output: EnrichmentBatch = {"enrichments": [Enrichment],
                           "tokens_used": int, "cost_paise": float,
                           "cache_hits": int, "errors": [str]}
```

Batches ≤ 50 mentions per LLM call with a JSON-schema-constrained prompt.
Results are cached by `mention.content_hash`, so a re-run over the same corpus
costs zero tokens. Order of `enrichments` is not guaranteed; join on
`mention_id`.

`is_about_brand=false` mentions still get returned — the clusterer and SOV
agents filter them out, the collector does not delete them.

---

### bp-clusterer

```
Input:  {"enriched": [EnrichedMention], "brand_id": str,
         "window_start": datetime, "window_end": datetime,
         "prior_window_counts": {str: int} = {},
         "min_cluster_size": int = 3}
Output: TopicSet = {"topics": [Topic], "unclustered": [str],
                    "tokens_used": int, "errors": [str]}
```

Embeds mention text through the Nasiko LLM router, clusters with sklearn
agglomerative clustering, then spends **one** LLM call per cluster to produce
`label` and `summary`. `unclustered` holds mention ids that fell into noise.

`trend` is `size / prior_window_counts.get(label, size)`, defaulting to 1.0
when there is no prior window.

---

### bp-sov

```
Input:  {"enriched": [EnrichedMention], "profile": BrandProfile,
         "window_start": datetime, "window_end": datetime}
Output: ShareOfVoice
```

Pure counting, no LLM, no network. A mention counts for the brand when
`is_about_brand` is true and `about_competitor` is null; it counts for a
competitor when `about_competitor` matches a name in `profile.competitors`.
Mentions matching neither are excluded from the denominator.

---

### bp-detector

```
Input:  {"brand_id": str, "enriched": [EnrichedMention],
         "baseline": BaselineStats, "now": datetime}
Output: AlertSet = {"alerts": [Alert], "rules_evaluated": int, "errors": [str]}

BaselineStats = {"per_source_hourly_mean": {Source: float},
                 "per_source_hourly_std":  {Source: float},
                 "negative_share_mean": float,
                 "negative_share_std": float,
                 "mean_rating": float|null,
                 "days": int}
```

**Deterministic. No LLM call in this agent, at all.** The five rules:

| Rule | Fires when | Severity |
|---|---|---|
| `spike` | hourly volume z-score ≥ 3.0 on any source | z≥3 medium, z≥5 high |
| `crisis` | z ≥ 3.0 **and** negative share ≥ 0.6 in the same hour | high, critical at ≥0.8 |
| `review_bomb` | ≥ 5 reviews in 1h on a review source with mean rating ≤ 2.0 | high |
| `influencer_mention` | any mention with `author_followers` ≥ 50 000 | medium, high if negative |
| `competitor_move` | competitor mention volume z ≥ 3.0 | low |

Every alert carries one `AlertEvidence` per term in its rule, with the real
number and the real threshold. `dedupe_key` is
`f"{kind}:{source}:{hour_bucket}"` so a sustained crisis produces one alert per
hour, not one per run.

Baselines come from `bp_core.stats.compute_baseline(brand_id, days=14)`, which
B1 owns.

---

### bp-responder

```
Input:  {"alert": Alert|null, "mention": Mention|null,
         "profile": BrandProfile, "channel": str}
Output: ReplyDraft
```

Exactly one of `alert` / `mention` is set. One LLM call. The prompt injects
`profile.voice.do_not_say` plus the global guardrail list from
`bp_core.prompts.GUARDRAILS` and the output is checked against both before
returning; a draft containing a banned phrase is regenerated once, then
returned with the phrase removed and a note in `tone`.

`requires_human_approval` is always `true`. Nothing in this repo posts to any
platform — there is no posting code path to audit.

---

### bp-briefer

```
Input:  {"brand_id": str, "profile": BrandProfile, "period": "daily"|"weekly",
         "period_start": datetime, "period_end": datetime,
         "topics": [Topic], "alerts": [Alert], "sov": ShareOfVoice,
         "numbers": BriefNumbers}
Output: DailyBrief
```

One LLM call producing `headline`, `suggested_actions` and the narrative.
`markdown` is the full brief; `whatsapp_short` is ≤ 600 characters with no
tables and no markdown links. The briefer does not query the database — the
orchestrator hands it everything.

---

### bp-orchestrator

```
Input:  {"brand_id": str, "trigger": RunKind,
         "window_hours": int = 24, "force": bool = false}
Output: RunRecord
```

The pipeline:

1. Load `BrandProfile` (latest confirmed version).
2. Compute `time_bucket`. If a `runs` row exists for
   `(brand_id, kind, time_bucket)` and `force` is false, return it unchanged.
3. Choose sources: `profile.sources`, ranked by yesterday's `source_yield`
   (mentions per credit, descending), truncated to the flow-guard fan-out cap.
   Dropped sources go in `sources_skipped` with `degraded_reason` set.
4. Split the remaining credit budget across chosen sources, call bp-collector
   once per source **in parallel**, persist mentions.
5. bp-enricher over new mentions → persist.
6. bp-clusterer, bp-sov, bp-detector in parallel.
7. bp-responder for each alert with severity ≥ high.
8. bp-briefer for the day.
9. Write the `runs` row with real credits, tokens and cost.

Fan-out from a single orchestrator call must stay ≤ 8 and depth ≤ 3. Step 4 is
the only wide fan-out; steps 6 and 7 are capped at 3 and 5 respectively.

---

## 3. Shared library surface (B1 owns, everyone imports)

```python
from bp_core.models import (BrandProfile, Mention, Enrichment, EnrichedMention,
                            Topic, Alert, AlertEvidence, ReplyDraft,
                            ShareOfVoice, DailyBrief, BriefNumbers, RunRecord,
                            Source, SentimentLabel, Intent, AlertKind, Severity)

from bp_core.ids import new_id                # new_id("mnt") -> "mnt_01J..."
from bp_core.hashing import content_hash      # (text, source) -> sha256 hex
from bp_core.redact import redact_pii         # strips emails/phones pre-LLM
from bp_core.llm import chat_json             # LLM router call, JSON-schema out
from bp_core.llm import embed                 # list[str] -> list[list[float]]
from bp_core.db import pool, fetch, execute   # asyncpg wrappers
from bp_core.stats import compute_baseline, zscore
from bp_core.anakin import AnakinClient       # cache + dedupe + budget wrapper
from bp_core.budget import CreditBudget       # per-brand-day ceiling
from bp_core.prompts import GUARDRAILS
```

### AnakinClient

```python
class AnakinClient:
    async def search(self, query: str, *, limit: int, freshness: str) -> dict
    async def wire(self, platform: str, query: str, *, limit: int, **kw) -> dict
    async def scrape(self, url: str, *, formats: list[str]) -> dict
    async def map(self, url: str, *, limit: int) -> dict
    async def crawl(self, url: str, *, limit: int, max_depth: int) -> dict
```

Every method: checks `fetch_cache` first, decrements `CreditBudget`, raises
`BudgetExceeded` rather than overspending, records the raw payload, and in
`BP_FIXTURE_MODE=replay` reads from `fixtures/` instead of the network.

**The three modes, set by `BP_FIXTURE_MODE`:**

| Mode | Behaviour |
|---|---|
| `replay` | Never touches the network. Reads `fixtures/<source>/<query_hash>.json`. **CI default.** |
| `record` | Calls Anakin for real, writes the response to `fixtures/`. Used once, today, on the demo brand. |
| `live` | Calls Anakin, caches to Postgres, writes no fixtures. Stage demo only. |

CI runs with zero credits and zero API keys. Any test that needs the network is
a broken test.

---

## 4. Naming rules that prevent merge pain

- Agent directories: `agents/bp-<name>/`. Module inside: `main.py`.
- Every agent's executor class: `<Name>Executor`, e.g. `CollectorExecutor`.
- Source adapters: `shared/bp_core/sources/<source_value>.py`, one function
  `async def fetch(client, profile, window_start, window_end, budget) -> list[Mention]`.
- Fixtures: `fixtures/<source>/<query_hash>.json`, plus
  `fixtures/labelled/mentions.jsonl` for the eval set.
- Prompts: `shared/bp_core/prompts/<agent>.md`, loaded by name, never inlined
  in `main.py`.
- Tests: `agents/bp-<name>/tests/test_<thing>.py`, pytest, async via
  `pytest-asyncio`.

## 5. Environment variables

| Var | Who sets it | Used by |
|---|---|---|
| `ANAKIN_API_KEY` | Nasiko secret | B1 anakin client |
| `OPENAI_BASE_URL` | injected by Nasiko LLM router | B1 llm client |
| `OPENAI_API_KEY` | injected by Nasiko | B1 llm client |
| `DATABASE_URL` | Nasiko secret / compose | B1 db |
| `BP_FIXTURE_MODE` | `replay` in CI, `live` on stage | B1 anakin client |
| `BP_MODEL_SMALL` | compose default | enricher, responder |
| `BP_MODEL_EMBED` | compose default | clusterer |

No agent reads an API key directly. Everything goes through `bp_core`.
