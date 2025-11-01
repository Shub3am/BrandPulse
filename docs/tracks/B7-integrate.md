# Track B7: Integration, merge and end-to-end test

**Branch:** `main` (you are the only track that commits to `main` directly)
**Owns:** the merge order, `docker-compose.yml`, the end-to-end test, the
release tag
**Blocked by:** everyone, which is the point

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md) and
[HACKATHON_NOTES.md](../../HACKATHON_NOTES.md) first. Read every other track's
brief too. You are the only agent allowed to, because you are the only agent
whose job spans them.

---

## What this track owns

Turning six green worktrees into one green repository. Six tracks each passing
their own tests proves six things, and none of them is that the product works.

You also own the one thing no single track can own: **the moment a contract
claim turns out to be false.** Five tracks compiled against
`docs/CONTRACTS.md` without ever running each other's code. Where it was wrong,
you are where that surfaces.

## What it must not know about

How any track implemented its work. You read their tests and their public
signatures, not their internals. If you find yourself rewriting a track's
algorithm to make integration pass, stop: that is a defect report for the
owning track, not a patch for you to apply. Your commits fix wiring, not logic.

The one exception is a one-line fix to an obviously wrong call site, which you
make and then state in `HACKATHON_NOTES.md`.

---

## Task 0: Be ready before anyone finishes

**Files:** `scripts/integration/` (you may create this), `.github/workflows/ci.yml`

Do not wait idle for six PRs. Every hour you are blocked is an hour you are not
spending on the thing that actually takes time.

- [ ] `docker compose up -d --no-recreate postgres`, confirm `(healthy)`, 12 tables
      and 7 enums with `docker exec brandpulse-postgres psql -U brandpulse -d
      brandpulse -c '\dt'`. If that fails, nothing downstream can work and it
      is your first bug.

      There is **one** Postgres for all seven of you, not one per worktree.
      `docker-compose.yml` pins `name: brandpulse`, so `up` from any checkout
      attaches to the same container and the same volume. That is deliberate:
      you cannot integrate six tracks against six databases. Two consequences:
      `docker compose down -v` from any worktree destroys everyone's data, so
      say so in `HACKATHON_NOTES.md` first; and plain `up` from a worktree that
      did not last start it **recreates** the container, because the project
      labels carry an absolute path. That is a Postgres restart under whoever
      is mid-test, which is why every documented command passes
      `--no-recreate`. Verified: three worktrees, three no-ops.
- [ ] Write the end-to-end test **before** the code it tests exists. It is the
      only test in the repo that is allowed to be red for hours. Name it
      `scripts/integration/e2e_test.go` with a `//go:build integration` tag so
      `go test ./...` does not run it by default.
- [ ] Decide the merge order and post it in `HACKATHON_NOTES.md` so tracks know
      when to rebase. Default order, dependency-first:
      **B1 → B2 → B3 → B4 → B6 → B5.** B5 last because it is the only track
      that touches all nine agent directories.
- [ ] Build the CI workflow. `.github/workflows/` is empty and the repo rule is
      "deploy through the automated pipeline". Minimum: `go build ./...`,
      `go vet ./...`, `BP_FIXTURE_MODE=replay go test ./...` with a
      `postgres:16` service container, and `npm run check:types` in `web/`.
      Zero credits, zero keys. If it needs `ANAKIN_API_KEY`, it is wrong.
- [ ] Commit. This is all on `main` and none of it blocks a track.

## Task 1: Merge, one track at a time, green between each

**Files:** whatever conflicts

Never merge two tracks in one step. When six changes land together and the
suite goes red, you have six suspects and no signal.

- [ ] For each track in the merge order: rebase its branch on `main`, run
      `go build ./... && go vet ./... && BP_FIXTURE_MODE=replay go test ./...`,
      merge only if green, then run the **full** suite again on `main` before
      starting the next track.
- [ ] A track that fails after rebase goes back to that track with the real
      output. Do not fix another track's test to make your merge pass. That is
      the single most damaging thing you can do in this role, because it
      converts a visible failure into a silent one.
- [ ] `HACKATHON_NOTES.md` conflicts are expected and always resolved by
      **keeping both sides**. It is an append-only log.
- [ ] `docs/CONTRACTS.md`, `internal/models/` and `001_init.sql` conflicts are
      different: those are frozen, so a conflict there means a track edited a
      frozen file. Reject it and make the track propose the change properly.
- [ ] Commit each merge separately, with the real test output in the message.

## Task 2: The contract audit

**Files:** `docs/CONTRACTS.md`, `HACKATHON_NOTES.md`

Five tracks compiled against a document. Now run the document against reality.

- [ ] For each of the nine agents, unmarshal its real output into the
      `internal/models` type CONTRACTS §2 says it returns. A field the contract
      promised and the agent does not send is a bug in one of them, and which
      one is your call to make and record.
- [ ] Assert the `ReplyDraft` invariant survived the round trip through every
      layer: marshal a real draft from `bp-responder`, unmarshal to
      `map[string]any`, assert `requires_human_approval` is present and `true`.
      This is the one invariant that cannot be caught by the type system,
      because the field does not exist on the struct.
- [ ] Assert no `Mention` reaching the database has an empty `Lang`, a `Topic`
      a zero `Trend`, or a `BrandProfile` a zero `Version`. These are the three
      pydantic defaults the Go port lost, and `Validate()` only catches them if
      somebody actually called it.
- [ ] Run `go test ./internal/models/` and confirm the parity test still passes
      against the migration after six tracks have touched things.
- [ ] Every divergence found goes in `HACKATHON_NOTES.md` under "Resolved
      unknowns" with which side you changed and why.

## Task 3: The end-to-end test

**Files:** `scripts/integration/e2e_test.go`

One test, one sentence: a brand goes in, a WhatsApp-ready alert comes out.

- [ ] Seed a clean database: `docker compose down -v && docker compose up -d
      postgres`, wait for `(healthy)`, then `go run ./demo/seed`.
- [ ] Drive the real chain in `replay` mode with zero network:
      `bp-onboarder` → `bp-collector` → `bp-enricher` → `bp-clusterer` →
      `bp-detector` → `bp-responder`. Assert at each hop, not only at the end.
      A test that only checks the last value tells you the pipeline is broken
      but not where.
- [ ] Assert the crisis path specifically, because it is the entire product
      wedge: inject the synthetic negative burst, assert `bp-detector` fires,
      assert the `Alert` carries the numbers and thresholds that fired it, and
      assert a `ReplyDraft` exists for it.
- [ ] Assert idempotency for real. Run the whole chain twice against the same
      time bucket and assert the row counts in `mentions`, `alerts` and `runs`
      are identical. The unique constraints exist for this and nobody has
      tested them under a second run.
- [ ] **Measure time from mention to alert** and put the real number in the
      `HACKATHON_NOTES.md` numbers table. It is currently blank, and `web/`
      already renders a hardcoded `107` that nobody measured. Either your
      number replaces it or the panel stops claiming it.
- [ ] Paste the full output. Commit.

## Task 4: The deployed smoke test

**Files:** `scripts/integration/smoke.sh`

Green locally and green on Nasiko are different claims. Only this one matters
on stage.

- [ ] After B5's fleet deploy, call each of the nine agents at its real
      deployed URL with a minimal valid input and assert an
      `application/json` artifact comes back that unmarshals into its contract
      type. Nine curls and nine assertions.
- [ ] Run `demo/run_demo.sh` against the real deployment three times. Three
      consecutive clean runs is the Phase 5 gate. Two is not.
- [ ] Confirm traces for all nine agents appear in `nasiko observe`. B5 proved
      one agent's traces arrive; you confirm the fleet's do.
- [ ] Confirm the WhatsApp brief actually lands on a real handset.
- [ ] Paste every output. Anything that fails here is a demo-day failure found
      one day early, which is the whole reason this task exists.

## Task 5: Tag and hand over

- [ ] `git tag -a v1.0-demo` with the test output summarised in the message.
- [ ] Fill every remaining blank in the `HACKATHON_NOTES.md` numbers table, or
      write "not measured" in it. **An estimate is never quietly promoted to a
      measurement.** A blank cell on stage is survivable; a number we made up
      is not.
- [ ] Record the fallback video against the tagged commit.

---

## Definition of done

```bash
docker compose down -v && docker compose up -d postgres
docker compose ps                      # must show (healthy)
go build ./... && go vet ./...
BP_FIXTURE_MODE=replay go test ./...
go test -tags=integration ./scripts/integration/ -v
cd web && npm run check:types && npm run build
./demo/run_demo.sh                     # three times, all clean
./scripts/integration/smoke.sh         # against the real deployment
```

Paste all of it in the PR. Not a summary of it, the output.

## The one rule for this track

**You never make a test pass by changing the test.** You are the last gate
before a live demo, and the only thing that makes this role worth having is
that your green means something. A track's failing test is that track's bug and
it goes back to them with the output attached.
