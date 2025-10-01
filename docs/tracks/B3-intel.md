# Track B3 — Enricher, clusterer, share-of-voice, eval

**Branch:** `track/b3-intel`
**Owns:** `agents/bp-enricher/`, `agents/bp-clusterer/`, `agents/bp-sov/`, `eval/`
**Blocked by:** B1's Phase 1 gate. Task 4 onward needs B2's fixtures.
**Blocks:** nothing, but `eval/` produces the cost number the pitch depends on.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md) and
[research/nasiko.md](../research/nasiko.md) §6 — that section changes the
clusterer's design and you must not skip it.

---

## What this track owns

Turning raw mentions into meaning: sentiment, intent, aspects, topics and
share-of-voice. Plus the only honest numbers in the pitch — classifier accuracy
and real cost per brand-day.

## What it must not know about

Where mentions came from, and what happens to topics downstream. You read
`Mention` and `EnrichedMention` and emit `Enrichment`, `Topic`, `ShareOfVoice`.
No Anakin call in this track, at all.

---

## Task 1 — bp-sov first

Start here. It is pure counting, has no LLM and no network, and it is the agent
B5 deploys first to prove the Nasiko path works. Getting it done early unblocks
deployment.

**Files:** `agents/bp-sov/main.py`, `AgentCard.json`, `Dockerfile`, `tests/`

- [ ] Input and `ShareOfVoice` output exactly per CONTRACTS §2.
- [ ] Counting rule: a mention counts for the brand when `is_about_brand` is
      true and `about_competitor` is null; for a competitor when
      `about_competitor` matches a name in `profile.competitors`. Mentions
      matching neither are **excluded from the denominator**, not counted as
      brand mentions. Write that as a test — it is the easiest thing to get
      quietly wrong.
- [ ] `by_source` breaks the same percentages down per source.
- [ ] Zero total mentions returns zeros, not a `ZeroDivisionError`. Test it.
- [ ] `AgentCard.json` with `protocolVersion: "1.0"`, `llm_provider: null`.
- [ ] Commit.

## Task 2 — bp-enricher

**Files:** `agents/bp-enricher/main.py`, `AgentCard.json`, `Dockerfile`,
`shared/bp_core/prompts/enricher.md`, `agents/bp-enricher/tests/`

- [ ] Batch ≤ 50 mentions per LLM call via `bp_core.llm.chat_json` with a JSON
      schema constraining the output to a list of `Enrichment` objects.
- [ ] **Redact before the prompt.** Every mention's text goes through
      `bp_core.redact.redact_pii` first. Assert this in a test — it is a
      guardrail in PLAN.md, not a nicety.
- [ ] Cache by `mention.content_hash`, so a re-run over the same corpus costs
      zero tokens. This is what makes the cost number work. Test cache hits.
- [ ] The prompt must handle **Hinglish** — Roman-script Hindi mixed with
      English is the norm in Indian D2C discourse, and a classifier that reads
      "bakwas product yaar" as neutral English is useless. Put Hinglish
      examples in the prompt and in the eval set.
- [ ] `is_about_brand` is the disambiguation field: "Mamaearth" the brand vs
      "mama earth" in an unrelated sentence. Give the model the brand's
      products and category as context so it can judge.
- [ ] Set `about_competitor` when the mention is about a competitor instead.
- [ ] A malformed batch response retries once, then returns neutral
      enrichments for that batch with an entry in `errors`. One bad batch must
      not fail a run.
- [ ] Record real `tokens_used` and `cost_paise` from `Usage`. The cost
      dashboard reads these; invented numbers poison the pitch.
- [ ] Tests mock the LLM. No live call in CI.
- [ ] Commit.

## Task 3 — bp-clusterer

**Files:** `agents/bp-clusterer/main.py`, `AgentCard.json`, `Dockerfile`,
`shared/bp_core/prompts/clusterer.md`, `agents/bp-clusterer/tests/`

**Read this before designing:** [research/nasiko.md](../research/nasiko.md) §6
Finding B. There is no verified embeddings endpoint on the Nasiko LLM router.
The catalogue lists chat models only.

- [ ] **Default vectoriser is local TF-IDF**, not embeddings.
      `sklearn.feature_extraction.text.TfidfVectorizer` with cosine distance,
      then `AgglomerativeClustering(metric="cosine", linkage="average",
      distance_threshold=...)`. No network, no tokens, deterministic, runs free
      in CI. On a 300-mention corpus TF-IDF separates topics at least as well
      as embeddings would.
- [ ] Put it behind `BP_VECTORISER=tfidf|embeddings`. If someone confirms
      `/v1/embeddings` works on a live cluster, flipping the flag is the whole
      change. Do not block on that confirmation.
- [ ] `min_cluster_size` defaults to 3; smaller groups go to `unclustered`.
- [ ] **One LLM call per cluster** for `label` and `summary` — that is the part
      that genuinely needs a model. Cap at 8 clusters per window so a noisy day
      cannot fan out into 40 calls.
- [ ] `trend = size / prior_window_counts.get(label, size)`, defaulting to 1.0.
- [ ] `top_examples` holds at most 3 mentions, chosen by engagement.
- [ ] Filter out `is_about_brand=false` before clustering.
- [ ] Test with a hand-built corpus where the right answer is obvious: fifteen
      mentions, three clear topics. Assert three clusters.
- [ ] Commit.

## Task 4 — Label the eval set

**Files:** `fixtures/labelled/mentions.jsonl`

Blocked until B2 posts that fixtures are ready.

- [ ] Hand-label all 300: `sentiment_label`, `intent`, `is_about_brand`.
- [ ] Label honestly, including the ones the model will get wrong. An eval set
      curated to make the model look good is worse than no eval set.
- [ ] Include Hinglish, sarcasm, and brand-name collisions deliberately.
- [ ] Commit.

## Task 5 — eval/accuracy

**Files:** `eval/accuracy.py`, `eval/__init__.py`, `eval/RESULTS.md`

- [ ] Run bp-enricher over the labelled set, report per-class precision, recall
      and F1 for sentiment and intent, plus accuracy for `is_about_brand`, plus
      a confusion matrix.
- [ ] `python -m eval.accuracy` prints a table and writes `eval/RESULTS.md`.
- [ ] **Paste the real output into the PR.** If sentiment accuracy is 71%, the
      README says 71%. We do not round up and we do not quietly drop the
      hardest class from the report.
- [ ] Commit.

## Task 6 — eval/cost

**Files:** `eval/cost.py`

This produces the number the entire pricing argument rests on.

- [ ] Measure a full brand-day: credits consumed per source, tokens per agent,
      and the rupee total. Read real values from `RunRecord` and `Enrichment.cost_paise`,
      not from a constant.
- [ ] Report cold (no cache) and warm (cache hit) cost separately. The honest
      headline is the cold number.
- [ ] Compare against the ₹15/brand-day target and against the ₹2,999/month
      price. State the gross margin that falls out.
- [ ] If the real number exceeds ₹15, **say so** and list what would bring it
      down (fewer sources, bigger enrichment batches, longer cache buckets).
      Do not retune the target to match the result.
- [ ] Commit, and post the number in `HACKATHON_NOTES.md` — B5 needs it for
      PRICING.md and the pitch.

---

## Definition of done

```bash
BP_FIXTURE_MODE=replay pytest agents/bp-enricher agents/bp-clusterer agents/bp-sov -q \
  --disable-socket --allow-unix-socket
python -m eval.accuracy
python -m eval.cost
```

All three run, and their **real output** is pasted in the PR.
