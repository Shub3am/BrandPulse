# demo

## What this module owns

Making the 2-minute pitch reproducible: seeding the demo brand, replaying the
fixture corpus as if it were live, injecting the crisis, and running the whole
flow end to end.

## What it must not know about

Agent internals. Everything here goes through the orchestrator or the database.

## Entry points

| Path | What it does |
|---|---|
| `brand.json` | The demo brand, its two competitors, ASINs, app ids. Ten sources, so the fan-out cap of 8 fires on cue. |
| `cmd/seed` | Loads `brand.json` and `fixtures/` into Postgres. `-reset` deletes the demo brand. |
| `cmd/replay` | Shifts fixture timestamps so the corpus ends "now". |
| `cmd/injectcrisis` | Drops 40 synthetic negative mentions to fire the crisis rule. |
| `crisis/` | The synthetic corpus itself, importable. Not a command. |
| `cmd/recordfixtures` | The one live Anakin recording session. B2 only. |
| `run_demo.sh` | The full flow. `--reset` reseeds, `--live` adds the live call. |

Each command is its own `package main` under `demo/cmd/` and runs as
`go run ./demo/cmd/seed`. Four `func main` in one package does not compile.

`crisis/` is a library, not a command, because `bp-detector` is `package main`
and cannot be imported: the detector's own test imports this corpus instead,
which is how the injection is proved against the real rules.

## Invariants and gotchas

- **`cmd/recordfixtures` is the only irreversible thing in this repo.** It
  prints a dry-run credit estimate and exits unless `--confirm` is passed. 300
  free credits, spent once.
- **`cmd/replay` is deterministic.** Same fixtures in, same timestamps out,
  every time. The whole of its logic is the pure `shift(postedAt, corpusEnd,
  now)`, and that function is what the test pins. A demo that differs between
  runs cannot be rehearsed.
- **Nothing here drops the database.** One Postgres is shared by seven
  worktrees, so `-reset` deletes the demo brand and lets the cascade do the
  rest. `docker compose down -v` would wipe six other tracks.
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
