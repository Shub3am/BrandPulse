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
| `brand.json` | The demo brand, its two competitors, ASINs, app ids. Ten sources, more than the orchestrator's fan-out cap, so the cap fires on cue. |
| `cmd/seed` | Loads `brand.json` and `fixtures/` into Postgres. `-reset` deletes the demo brand. |
| `cmd/replay` | Shifts fixture timestamps so the corpus ends "now". |
| `cmd/injectcrisis` | Clears its last injection, then drops the synthetic negative surge in. |
| `crisis/` | The synthetic corpus itself, importable. Rows only, not a command. |
| `cmd/recordfixtures` | The one live Anakin recording session. B2 only. |
| `run_demo.sh` | The full flow. `--reset` reseeds, `--live` adds the live call. |

Each command is its own `package main` under `demo/cmd/` and runs as
`go run ./demo/cmd/seed`. Four `func main` in one package does not compile.

`crisis/` is a library, not a command, so the detector's own test can import the
corpus and prove the injection against the real rules. Turning it around, by
splitting the detector's rules into an importable package, would put `demo/`
inside an agent's internals in the production graph instead of in one test file.
A claim about the corpus alone is tested in `crisis/`. A claim that couples the
corpus to the rules needs the detector's unexported thresholds, so it lives in
`agents/bp-detector/injection_test.go` instead.

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
  Every step has to be re-runnable on its own for that to hold, which is why an
  injection clears its own last one instead of colliding with it.
- **The live Anakin call is optional and separate.** Stage wifi must not be able
  to break the demo.

## Who calls this

A human, on stage. And CI, for `--dry-run` paths only.
