<!-- The kickoff brief, verbatim. This is the spec PLAN.md argues from.
Nothing in this file is edited. Where research contradicts it, the correction
lives in docs/SOURCE-STRATEGY.md and docs/research/, and PLAN.md points at both.
-->

# Track B — "BrandPulse" (working name): social listening & brand monitoring for D2C brands, on Nasiko + Anakin + DronaHQ

Kickoff brief for a Claude Code session in a fresh repo (`brandpulse/`) plus `agents/brandpulse-*` copied into the Nasiko fork for the PR. Paste this whole file as the first message. Sub-briefs at the bottom are for parallel worktrees/agents. (Supersedes the earlier Refundly brief.)

## 0. Product in one paragraph

Brandwatch, Sprinklr, Meltwater and Talkwalker sell social listening at ₹1L–10L+/year to enterprises. Mid-size Indian D2C brands (₹1–50 Cr revenue: skincare, apparel, F&B, gadgets) and the agencies serving them do it by hand: someone searches X, Instagram, Reddit, YouTube comments, Play Store reviews and Google News once a day and pastes screenshots into a group. BrandPulse gives them the enterprise product at ₹2,999/month per brand: continuous monitoring of brand + competitor + category keywords across public web sources, clustering into topics, sentiment and urgency scoring, spike/crisis detection, a daily WhatsApp brief, and a live dashboard. The cost structure is the pitch: incumbents' price is mostly scraping infrastructure and human analysts; Anakin commoditises the first and agents replace the second, so unit cost per brand-day is a few rupees.

Wedge: **crisis-before-it-trends**. A negative thread about a brand on Reddit or X at 11pm, or a burst of 1-star Play Store reviews after an app update, reaches the founder's WhatsApp within minutes with a suggested response, not in next week's report.

## 1. How the three platforms are used (each must be visibly load-bearing)

- **Nasiko** — hosts and orchestrates the agents as A2A containers; routes requests, proxies inter-agent calls, meters LLM spend per brand, flow-guards cap fan-out (a brand with 40 keywords must not spawn 400 scrapes). Each agent under `agents/brandpulse-<name>/` with `AgentCard.json`, `Dockerfile`, `main.py` (structure from `agents/currency-agent`: Python 3.13, Starlette, `a2a` SDK, JSON artifacts). Deploy with `nasiko deploy`. LLM calls only via injected `OPENAI_BASE_URL` (Nasiko LLM router). Show per-brand cost from Nasiko's observability in the demo.
- **Anakin** — the entire data acquisition layer. `collector` agents use: **Search API** (full page content) for news, blogs, forums; **Wire** for X, Reddit, YouTube (comments), Instagram public posts/hashtags, Google Play / App Store reviews, Amazon/Flipkart product reviews; **URL Scraper** for any specific URL a user pastes; **Crawl/Map** to onboard a brand (crawl its site to auto-discover product names, hashtags, competitor mentions → keyword set). Every fetch is cached in Postgres keyed by `(source, query, time_bucket)`, deduped by content hash, with a per-brand daily credit budget. 300 free credits: spend them on the demo brand only; everything else runs on recorded fixtures.
- **DronaHQ** — (1) WhatsApp/chat agent for brand owners: onboarding conversation ("what's your brand, site, top 3 competitors?"), daily brief delivery, on-demand queries ("what are people saying about our new serum this week?"), alert delivery with one-tap "draft a reply" / "snooze" / "escalate". (2) Ops/analyst dashboard app: mention stream with filters, topic clusters, sentiment over time, share-of-voice vs competitors, alert log, credit/LLM cost per brand. DronaHQ guardrails on outbound drafts (no promises, no legal admissions). This competes for "Top 3 Agents of the Day".

## 2. Agents on Nasiko

| Agent | Input → Output | LLM? | Notes |
|---|---|---|---|
| `bp-orchestrator` | brand_id + trigger(schedule/on-demand/onboard) → runs the pipeline, persists run record | tiny | Fan-out per source under flow-guard limits; idempotent per `(brand, time_bucket)`. |
| `bp-onboarder` | brand name + website (+ competitors) → `BrandProfile{keywords[], hashtags[], products[], competitors[], sources_enabled[], negative_keywords[]}` | 1 call | Anakin Map+Crawl the site, extract product names; LLM proposes keyword set; user confirms in DronaHQ. |
| `bp-collector` | BrandProfile + source + time window → `Mention[]{id, source, url, author, text, posted_at, engagement{likes,replies,shares,views}, lang, raw}` | none | One agent, source adapters inside (`sources/x.py, reddit.py, youtube.py, news.py, playstore.py, amazon.py, instagram.py`). Anakin calls + cache + dedupe + budget meter. Fixtures for tests. |
| `bp-enricher` | Mention[] → adds `sentiment{-1..1, label}`, `emotion`, `intent{complaint, praise, question, purchase_intent, comparison, spam}`, `aspects[]` (e.g. "delivery", "packaging", "price"), `is_about_brand` (disambiguation: "Mamaearth" vs "mama earth"), `lang` normalised (hinglish handled) | batched, small model, ≤50 mentions/call | Cheap classifier prompt with JSON schema; cache by content hash so re-runs cost nothing. |
| `bp-clusterer` | enriched mentions (window) → `Topic[]{label, summary, size, sentiment_mix, top_examples[], trend vs prior window}` | embeddings via LLM router + 1 labeling call per cluster | Embedding + HDBSCAN/agglomerative (sklearn); LLM only names clusters. |
| `bp-detector` | mention stream stats → `Alert[]{kind: spike|crisis|influencer_mention|competitor_move|review_bomb, severity, evidence[], why}` | none | Pure stats: z-score of volume vs 14-day baseline per source, negative-share jump, high-follower author threshold, star-rating drop. Deterministic → explainable. |
| `bp-responder` | Alert or Mention → `ReplyDraft{channel, text, tone, do_not_say[]}` | 1 call | Templates per intent; brand voice from profile; guardrails list applied. Never auto-posts — human taps send in DronaHQ. |
| `bp-briefer` | brand + day → `DailyBrief{headline, numbers{mentions, sentiment, sov}, top_topics[], alerts[], competitor_watch[], suggested_actions[]}` (+ markdown + WhatsApp-short variant) | 1 call | Also produces weekly PDF via the same path. |
| `bp-sov` | brand + competitors + window → `ShareOfVoice{brand: %, competitors: {..}, by_source}` | none | Pure counting over enriched mentions. |

Nasiko flow guard config: depth 3, fan-out ≤ 8 per orchestrator call (sources), token budget per brand-day; the demo shows a keyword-heavy brand hitting the guard and degrading gracefully (sources prioritised by past yield).

## 3. Data model (Postgres; migrations in `db/`)

`brands(id, name, website, plan, whatsapp_number, created_at)`, `brand_profiles(brand_id, keywords jsonb, hashtags jsonb, products jsonb, competitors jsonb, sources jsonb, negative_keywords jsonb, voice jsonb, version)`, `mentions(id, brand_id, source, external_id, url, author, author_followers, text, lang, posted_at, engagement jsonb, content_hash unique, raw jsonb)`, `mention_enrichment(mention_id, sentiment real, sentiment_label, emotion, intent, aspects jsonb, is_about_brand bool, model, cost_paise)`, `topics(id, brand_id, window_start, window_end, label, summary, size, sentiment_mix jsonb, trend real)`, `topic_mentions(topic_id, mention_id)`, `alerts(id, brand_id, kind, severity, evidence jsonb, why, status enum[open, acked, snoozed, resolved], created_at)`, `reply_drafts(id, alert_id|mention_id, text, tone, status)`, `briefs(id, brand_id, period, payload jsonb, delivered_at)`, `fetch_cache(source, query_hash, time_bucket, payload jsonb, credits, fetched_at)`, `runs(id, brand_id, kind, started_at, finished_at, credits_used, tokens_used, cost_paise, status)`.

## 4. Repo layout

```
brandpulse/
  agents/bp-{orchestrator,onboarder,collector,enricher,clusterer,detector,responder,briefer,sov}/  # main.py, AgentCard.json, Dockerfile, tests/
  shared/bp_core/        # pydantic models, anakin client (cache, dedupe, budget), sources/ adapters, redact, prompts/, stats.py (baselines, z-scores)
  fixtures/              # recorded Anakin responses per source (demo brand + 2 competitors), ≥300 mentions with hand-labelled sentiment for eval
  db/migrations/*.sql
  dronahq/               # exported WhatsApp agent + dashboard app JSON, screenshots, setup notes
  demo/                  # seed brand, replay script that streams fixtures as if live (for a deterministic video), crisis injection script
  eval/                  # sentiment/intent accuracy on the labelled fixtures; cost per brand-day report
  docs/ARCHITECTURE.md, docs/SETUP.md, docs/PRICING.md, README.md
  docker-compose.yml     # postgres + all agents for local dev
```
Copy `agents/bp-*` into the Nasiko fork under `agents/` for the PR, with `agents/brandpulse/README.md` linking the product repo and explaining the multi-agent topology.

## 5. Demo brand & data

Pick a real, chatty Indian D2C brand with public discourse (e.g. a skincare or audio brand) plus two competitors. Spend the live Anakin credits once today to record real fixtures; the demo replays fixtures with timestamps shifted to "now" so it is deterministic, then does **one** live Anakin call on stage (a fresh Reddit/X search) to prove it's real. `demo/inject_crisis.py` drops 40 synthetic negative mentions about "rash after using X" within 10 minutes to trigger the crisis alert live.

## 6. Guardrails & honesty
- Public data only; respect robots and platform ToS via Anakin; no login-walled scraping in the demo. Say so in README.
- PII: store handles/URLs (public), never emails/phones found in text (redact before LLM).
- Drafted replies are never auto-posted; human approves in DronaHQ.
- Every alert shows its evidence and the statistical rule that fired — no black-box "AI detected a crisis".
- Cost transparency: dashboard shows credits + tokens + ₹ per brand-day; target < ₹15/brand-day at 8 sources.

## 7. Demo (2 minutes)
(0:00) WhatsApp: "Onboard brand: <Brand>, site <url>, competitors A and B." Onboarder crawls the site (Anakin), proposes 18 keywords; tap Confirm. (0:25) Dashboard fills: mention stream across 7 sources, sentiment over 14 days, share-of-voice donut (brand 41%, A 35%, B 24%), topic clusters ("delivery delays", "new launch hype", "price vs A"). (0:50) Nasiko dashboard: 9 agents, call graph for one run, tokens and credits: "₹11 for today's run". (1:05) Run `inject_crisis.py`. Within seconds the detector fires **crisis / severity high**, WhatsApp alert lands with evidence (volume z=4.1, negative share 78%, 3 high-follower authors) and a suggested holding statement; tap "Draft reply" → responder returns brand-voice reply; show it is not auto-posted. (1:35) Daily brief PDF + WhatsApp-short version. (1:50) Close: "Brandwatch starts at ₹1L a year. This is ₹2,999 a month, and it woke you up before it trended."

## 8. Parallel work split

| # | Agent owns | Depends on |
|---|-----------|-----------|
| B1 | `shared/bp_core` (models, anakin client with cache/dedupe/budget, stats.py, redact, prompts), `db/`, docker-compose. Commit models in first 20 min. | — |
| B2 | `bp-collector` with all source adapters + fixture recorder + fixtures for demo brand and competitors; `bp-onboarder` | B1 |
| B3 | `bp-enricher`, `bp-clusterer`, `bp-sov` + `eval/` (label 300 fixtures, report accuracy and cost) | B1, B2 fixtures |
| B4 | `bp-detector`, `bp-responder`, `bp-briefer`, `bp-orchestrator` (fan-out, idempotency, flow-guard-aware source prioritisation), `demo/inject_crisis.py` + replay script | B1, interfaces of B2/B3 |
| B5 | Nasiko: AgentCards, Dockerfiles, `nasiko deploy` ×9, flow guards, copy into fork + agents README. DronaHQ: WhatsApp agent + dashboard app. Docs, PRICING.md (unit economics vs incumbents), video script, LinkedIn post. | B2–B4 |

Rules: typed JSON artifacts from every agent (pydantic → `application/json`) so DronaHQ renders them directly; every external call cached and fixture-tested (CI needs zero credits and zero keys); no raw API keys in containers; deterministic detector (no LLM in alerting); commit small; shared decisions in `HACKATHON_NOTES.md`.

create git branch and git worktree and write everything in markdown so other agents can coordinate too, plan first so we can pin up multiple agents to do this


we want to make end to end project with deployment live
plan