# eval

## What this module owns

The only honest numbers in the pitch: how accurate the classifier actually is,
and what a brand-day actually costs.

## What it must not know about

How anything is implemented. It calls agents and reads the database.

## Entry points

```bash
go run ./eval/accuracy   # per-class precision/recall/F1, confusion matrix
go run ./eval/cost       # credits, tokens, ₹ per brand-day, cold and warm
```

Each is its own `package main` in its own directory, because two `func main`
cannot share one package. `accuracy` writes `eval/RESULTS.md`.

## Invariants and gotchas

- **Real values only.** Cost is summed from `RunRecord` and
  `Enrichment.cost_paise`, never from a constant. An invented number here
  poisons `docs/PRICING.md` and the pitch on top of it.
- **The labelled set is labelled honestly**, including the cases the model gets
  wrong. An eval set curated to flatter the model is worse than no eval set.
- **Report the number you got.** If sentiment accuracy is 71%, the README says
  71%. We do not round up and we do not drop the hardest class from the table.
- **Cold is the headline cost.** Warm-cache cost is reported next to it, not
  instead of it.
- **If cost exceeds the ₹15/brand-day target, say so** and list what would close
  the gap. Do not retune the target to match the result.
- The labelled set includes Hinglish, sarcasm and brand-name collisions on
  purpose. Those are where a classifier for Indian D2C actually fails.

## Who calls this

B3 during Phase 3, and B5 when writing `docs/PRICING.md`.
