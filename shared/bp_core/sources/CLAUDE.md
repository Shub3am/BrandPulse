# bp_core.sources

## What this module owns

Turning one Anakin response into a list of `Mention` objects, one file per
source.

## What it must not know about

Sentiment, topics, alerts, briefs, or the database. An adapter fetches and maps.
It does not enrich, it does not persist, and it never calls an LLM.

## Entry points

Every adapter is one function with the same signature:

```python
async def fetch(client: AnakinClient, profile: BrandProfile,
                window_start: datetime, window_end: datetime,
                budget: CreditBudget) -> list[Mention]
```

`__init__.py` exposes `ADAPTERS: dict[Source, Callable]` so the collector
dispatches without a branch per source.

## Invariants and gotchas

- **`posted_at` is timezone-aware UTC.** Every source formats dates differently.
  Normalise here, never downstream.
- **`content_hash` comes from `bp_core.hashing`**, never computed inline, or
  dedupe silently stops working across sources.
- **`rating` is set only by `amazon`, `appstore` and `playstore`.** The
  `review_bomb` rule reads it.
- **`author_followers` is 0 when the source does not give it.** Do not invent a
  value: the `influencer_mention` rule fires on it.
- **Negative-keyword filtering happens here**, before a mention costs an
  enrichment token.
- **Which sources actually exist is not the brief's list.** Read
  [docs/SOURCE-STRATEGY.md](../../../docs/SOURCE-STRATEGY.md) first. Anakin's
  Wire catalogue does not carry X, Instagram, Play Store reviews or App Store
  reviews, and each has a documented replacement or is dropped.
- The literal Wire response field names live in
  [docs/research/wire-schemas.md](../../../docs/research/wire-schemas.md),
  recorded from live calls. Do not guess a field name.

## Who calls this

`agents/bp-collector` only.
