# Pricing

What BrandPulse costs to run, against what the incumbents charge.

**Status: the BrandPulse column is not measured yet.** `eval/cost` is B3's and
has not been built, so every cell below that would come from it is marked
`not measured` rather than filled with a plausible number. An invented figure
here would poison the pitch that sits on top of it, and this document exists
partly to make that impossible.

The competitor figures are real, sourced, and dated below.

---

## 1. How our number will be produced

```bash
go run ./eval/cost       # credits, tokens, ₹ per brand-day, cold and warm
```

It sums real values from `RunRecord` and `Enrichment.cost_paise`. It never
reads a constant. **Cold cost is the headline**, warm-cache cost is reported
beside it and not instead of it, because a cache hit rate measured on a
rehearsed demo is not a cache hit rate a customer will see.

**Target: under ₹15 per brand-day at eight sources.** If the measurement comes
in above that, this document prints the real number and section 4 explains what
would close the gap. The target does not get retuned to match the result.

---

## 2. What we will report

| Metric | Value | Source |
|---|---|---|
| Anakin credits per brand-day, cold | not measured | `go run ./eval/cost` |
| Anakin credits per brand-day, warm cache | not measured | `go run ./eval/cost` |
| LLM tokens per brand-day | not measured | `go run ./eval/cost` |
| **₹ per brand-day, cold** | **not measured** | `go run ./eval/cost` |
| ₹ per brand-day, warm cache | not measured | `go run ./eval/cost` |
| ₹ per brand-month, cold, ×30 | not measured | derived |
| Sentiment accuracy on the labelled set | not measured | `go run ./eval/accuracy` |
| Agent container image size | **14.6 MB** | `docker images`, measured |

The image size is the one row that is measured today: a Go agent on
`gcr.io/distroless/static:nonroot`, built `CGO_ENABLED=0`, running nonroot and
serving. That is the number behind
[decisions/001-go-for-agents.md](decisions/001-go-for-agents.md).

### Where the cost actually goes

Five of the nine agents call a model and four never do, so the cost profile is
not uniform. In rough order of expected spend:

1. **`bp-enricher`**, one batched call per group of new mentions. This is the
   volume driver: it scales with mentions collected, and everything else
   scales with brands or alerts.
2. **`bp-clusterer`**, one call per cluster to name it.
3. **Anakin credits** in `bp-collector`, one call per source per window, plus
   the Search-then-Scrape chain for `web` and `news` bodies.
4. **`bp-responder`**, one call per high-severity alert, capped at five per run.
5. **`bp-briefer`** and **`bp-onboarder`**, one call each, per day and per
   brand onboarding respectively.
6. **`bp-orchestrator`, `bp-collector`, `bp-detector`, `bp-sov`**: zero model
   cost by construction.

The `fetch_cache` table is what separates cold from warm. The same source and
window fetched twice in a day costs credits once.

---

## 3. What the incumbents charge

None of these three publish a list price. All are custom enterprise quotes, so
the figures below are reported contract data from pricing aggregators and
vendor-comparison sources, not numbers off a pricing page. That distinction
matters and we state it on stage.

Converted at **$1 = ₹96**, the mid-September 2026 rate.

| Product | Reported cost | In rupees | Terms |
|---|---|---|---|
| **Brandwatch** | from ~$800/month at the low end; enterprise from ~$36,000/year, median ~$50,000/year | ~₹77,000/month; ~₹34.5 lakh/year entry, ~₹48 lakh/year median | Annual, 12 to 24 months. Monthly billing rarely available. Implementation and overage fees reported at $5,000 to $20,000 in year one. |
| **Sprinklr** | enterprise from ~$50,000/year, median ~$129,000/year | ~₹48 lakh to ~₹1.24 crore/year | Self-serve tier ($249 to $299/month) was **discontinued 30 April 2026**. Enterprise only now. |
| **Meltwater** | ~$6,000 to $10,000/year floor for a single-region setup; $10,000 to $57,000/year typical | ~₹5.8 lakh to ~₹55 lakh/year | Custom quote. No published prices at all. |

**The honest framing.** These are enterprise tools sold to companies with a
social team. They are not overpriced for what they are. The point is not that
they are bad value; it is that a two-person D2C brand doing ₹40 lakh a month
cannot clear the *floor*. Even Meltwater's cheapest reported single-region
setup is roughly ₹48,000 a month, and Sprinklr removed the only self-serve
option in the market in April 2026.

That is the gap. Not "cheaper than Brandwatch", but "exists at all for someone
Brandwatch will not sell to".

---

## 4. If the number misses ₹15

Written before the measurement, so it cannot be back-fitted to whatever comes
out. In descending order of expected saving:

1. **Raise the enrichment batch size.** `bp-enricher` batches already; a larger
   batch is fewer prompt-token repeats of the same instruction block for the
   same output tokens. This is the first lever because it is pure overhead
   removal with no loss of coverage.
2. **Cut the source list by yield.** The orchestrator already ranks sources by
   yesterday's mentions per credit and truncates to the fan-out cap. Lowering
   that cap drops the worst-value source first, by construction. X/Twitter is
   the first to go, which we already expect.
3. **Widen the fetch-cache window.** Currently a cache hit requires the same
   source and window. A brand checked twice a day pays twice for an overlapping
   window it mostly already has.
4. **Move `bp-clusterer`'s naming call to one call for all clusters** instead
   of one per cluster. Cheaper, slightly worse names.
5. **Run daily instead of twice daily** for brands with low baseline volume.
   Fixed per-run costs dominate for a quiet brand.

What we will **not** do to hit the number:

- Drop a source and keep claiming it. See
  [SOURCE-STRATEGY.md](SOURCE-STRATEGY.md).
- Put a model in `bp-detector` to "summarise" its way out of collecting real
  mentions.
- Report the warm-cache figure as the headline.
- Change the ₹15 target.

---

## 5. Sources

Competitor pricing, retrieved 20 September 2026:

- [Brandwatch Pricing: How Much It Costs in 2026](https://archive.com/blog/brandwatch-pricing)
- [Brandwatch Software Pricing & Plans 2026, Vendr](https://www.vendr.com/marketplace/brandwatch)
- [Sprinklr Pricing 2026: Self-Serve Dead, Enterprise from $50k](https://chatarmin.com/en/blog/sprinklr-pricing)
- [Actual Sprinklr Pricing 2026, Spendhound](https://www.spendhound.com/marketplace/sprinklr-pricing)
- [Meltwater Pricing in 2026: What It Actually Costs (Real Contract Data)](https://octolens.com/blog/meltwater-pricing)
- [What Social Listening Tools Cost in 2026, Market Intelligence Tools](https://marketintelligencetools.com/reports/social-listening-pricing/)

Exchange rate:

- [Indian Rupee, Trading Economics](https://tradingeconomics.com/india/currency), $1 = ₹95.9 to ₹96.1 through mid-September 2026.
