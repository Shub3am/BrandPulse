# demo

## What this module owns

Making the 2-minute pitch reproducible: seeding the demo brand, replaying the
fixture corpus as if it were live, injecting the crisis, and running the whole
flow end to end.

## What it must not know about

Agent internals. Everything here goes through the orchestrator or the database.

## Entry points

| File | What it does |
|---|---|
| `brand.json` | The demo brand, its two competitors, ASINs, app ids. |
| `seed.py` | Loads `brand.json` and `fixtures/` into Postgres. |
| `replay.py` | Shifts fixture timestamps so the corpus ends "now". |
| `inject_crisis.py` | Drops 40 synthetic negative mentions to fire the crisis rule. |
| `record_fixtures.py` | The one live Anakin recording session. B2 only. |
| `run_demo.sh` | The full flow. `--reset` reseeds, `--live` adds the live call. |

## Invariants and gotchas

- **`record_fixtures.py` is the only irreversible thing in this repo.** It
  prints a dry-run credit estimate and exits unless `--confirm` is passed. 300
  free credits, spent once.
- **`replay.py` is deterministic.** Same fixtures in, same timestamps out, every
  time. A demo that differs between runs cannot be rehearsed.
- **Injected mentions are labelled.** Every one carries `raw: {"synthetic":
  true}` and the dashboard shows an "injected demo data" badge. We do not blur
  the line between scraped and synthetic.
- **The injection must fire the real detector rule.** If it does not, the
  injection is wrong. Never special-case the detector to make the demo work.
- **`run_demo.sh` is re-runnable.** It will be run thirty times before the pitch.
- **The live Anakin call is optional and separate.** Stage wifi must not be able
  to break the demo.

## Who calls this

A human, on stage. And CI, for `--dry-run` paths only.
