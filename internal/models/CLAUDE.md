# internal/models: the frozen wire format

Every byte of JSON that crosses an agent boundary is one of these structs. This
package is the machine-readable half of [docs/CONTRACTS.md](../../docs/CONTRACTS.md);
where the two disagree, this package wins and the doc gets corrected.

## What this module owns

- The domain types: `BrandProfile`, `Mention`, `Enrichment`, `EnrichedMention`,
  `Topic`, `Alert`, `ReplyDraft`, `ShareOfVoice`, `DailyBrief`, `RunRecord`.
- The enums, whose string values are the Postgres enum labels.
- The per-agent request and response envelopes (`agentio.go`): `MentionBatch`,
  `EnrichmentBatch`, `TopicSet`, `AlertSet`, `BaselineStats` and the nine
  `*Input` types.
- The `New*` constructors and the `Validate()` methods.

## What it must not know about

Anything that moves. No agent, no `anakin` client, no `db`, no `llm`, no
`net/http`, no `database/sql`. This package imports `encoding/json`, `fmt` and
`time` and that list is not to grow. It is pure schema, so every other package
can import it without a cycle and every test can construct one without a
fixture.

## Invariants and gotchas

- **Go has no field defaults and this port lost three of them.** pydantic gave
  `BrandProfile.Version` a default of 1, `Topic.Trend` a default of 1.0 and
  `Mention.Lang` a default of `"en"`. Here they decode as `0`, `0.0` and `""`.
  The `New*` constructors carry the intended value; `Validate()` catches the
  decoder path that bypasses them. Use both.
- **`ReplyDraft` has no `RequiresHumanApproval` field, on purpose.** It is an
  invariant rather than state, and a `bool` field would decode to `false` from
  any payload that omitted it. `MarshalJSON` emits
  `requires_human_approval: true` unconditionally. Consequence: the type does
  **not** round-trip, because `Unmarshal` has nowhere to put the field and
  drops it. Do not add the field back to "fix" that.
- **`Engagement.Total()` excludes `Views`.** Views measure reach, not
  engagement, and a video with 2M views would otherwise outrank every actual
  complaint in the clusterer's ranking.
- **Enum values are Postgres enum labels.** Renaming one is a migration in the
  same commit, not a rename here.
- **`Alert.DedupeKey` and `RunRecord.TimeBucket` look redundant and are not.**
  They back unique constraints. An insert without them fails.
- `internal/models/parity_test.go` parses `db/migrations/001_init.sql` and
  compares columns to `json` tags. If you add a field, that test tells you
  whether you also owe a migration. The intended divergences are listed in
  CONTRACTS §3b and the test knows about them.

## Who calls it

Every package in `internal/`, all nine agents, and `web/lib/types.ts`, which is
a **hand-maintained mirror**. TypeScript cannot catch a drift here, because the
data arrives as JSON at runtime. Change a `json` tag and you change
`web/lib/types.ts` in the same commit.

## Changing it

Only on `main`, only through B1, never inside a worktree. Propose the change in
`HACKATHON_NOTES.md` first. Five tracks compile against this package.
