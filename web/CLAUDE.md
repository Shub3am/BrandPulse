# web/ — the product dashboard

Next.js 16 App Router, React 19, TypeScript, plain CSS. No Tailwind, no
component library, no state manager. The theme is taken from anakin.io: light,
white, cyan accent, hairline borders instead of shadows, 10px radii.

## What this module owns

The one screen a judge looks at: a brand's pulse, the live alert feed, the
mention stream, the topics, the drafted reply, and a run report that shows what
Nasiko, Anakin and DronaHQ each did.

Since the 2026-09-20 scope change this is the primary alert surface, not a
secondary one. Time-to-alert is measured to this UI, so the feed polls rather
than rendering one server-side snapshot.

## What it must not know about

- **Any agent, any database, any Anakin call.** The dashboard's only backend is
  the Fastify BFF. If a page needs a number, the BFF returns it.
- **Business rules.** Nothing here decides whether an alert should fire, what a
  sentiment score means, or how share of voice is computed. A rule that runs in
  the browser is a rule invisible to `nasiko observe`. `lib/format.ts` is
  display formatting and nothing else.
- **Posting.** There is no send path and there must not be one. Approving a
  draft is a handoff to DronaHQ.

## Entry points

| Path | Job |
|---|---|
| `app/page.tsx` | The dashboard. Awaits the BFF and composes the panels, owns no logic. |
| `app/layout.tsx` | Document shell and top bar. Holds no product state and no label. |
| `components/DataSourceBadge.tsx` | Says what the data is, derived from the payload. |
| `app/api/alerts/route.ts` | Proxies the alert poll so the browser never learns the BFF's URL. |
| `app/loading.tsx` | Panel frames while the fetch is in flight. Holds no values. |
| `app/error.tsx` | Says the backend is unreachable. Substitutes nothing. |
| `lib/pulse.ts` | The BFF fetches. The only file here that knows the backend exists. |
| `lib/types.ts` | The wire format, mirrored from `internal/models/models.go`. |
| `lib/demoData.ts` | Synthetic sample artifacts. Nothing under `app/` imports it. |
| `lib/format.ts` | Display formatting. No business rules. |
| `scripts/checkTypesParity.mjs` | Fails the build when a mirror drifts from `models.go`. |
| `components/*.tsx` | One component per panel, each named for what it renders. |

## Invariants and gotchas

- **`lib/types.ts` is a mirror, not a source.** Field names must match the Go
  `json` tags in `internal/models/models.go` exactly, because the BFF passes
  agent artifacts through unreshaped. When `models.go` changes, this file
  changes in the same commit or the dashboard renders `undefined` silently.
  TypeScript will not catch it: the data arrives as JSON at runtime.
  `npm run check:types` does, and `npm run build` runs it first. It compares
  field names and optionality against the `json` tags, in both directions, and
  it also checks `bff/src/contracts.ts`, which is the same mirror kept
  deliberately duplicated. An `omitempty` tag means the TS field is `?`. There
  is exactly one permitted divergence, `ReplyDraft.requires_human_approval`,
  and it is a constant in the script rather than an allowlist.
- **The page is live and there is no fallback.** `app/page.tsx` awaits
  `lib/pulse.ts`. When the BFF is unreachable the fetch throws and
  `app/error.tsx` says so. Falling back to the sample artifacts in `lib/` would
  put invented numbers on a screen that claims to be monitoring a real brand,
  so `grep -r demoData app/` returning nothing is an invariant, enforced by
  `grep` in the Definition of done. `grep` cannot tell a comment from an import,
  so not even a comment under `app/` may name that module.
- **`lib/pulse.ts` is the only file that knows where the backend is.**
  `BFF_BASE_URL` has no `NEXT_PUBLIC_` prefix, on purpose: a `NEXT_PUBLIC_` var
  is inlined when the image is built, and the BFF's URL is only known at deploy
  time. That is why the live feed polls `app/api/alerts/route.ts` instead of the
  BFF directly. The route adds no field and filters nothing.
- **An empty panel is a sentence, never a blank box or a zero.** Every panel has
  a written empty state naming what is missing and which agent fills it. Zero
  alerts is a success state and says so, with the window it has been watching.
  Partial data renders, with `RunNotice` above it printing what the payload said
  went wrong. Nothing is ever stood in for: no cached figure, no last-known
  number and no placeholder digit, including in `loading.tsx`.
- **`RunNotice` reports, it does not judge.** It prints `status`,
  `sources_skipped`, `degraded_reason` and both `errors` arrays as they arrived.
  It does not map a status to a severity and its CSS is deliberately neutral,
  because "which of these is bad" is `bp-detector`'s call. It prints a line for a
  healthy run too: a notice that appears only on failure teaches a reader to
  treat its absence as proof, and absence is also what a missing field looks
  like.
- **Only the browser may date an alert's arrival.** `AlertFeed` records its own
  receipt timestamp when a poll brings in an alert it has not seen, and
  `AlertBanner` prints a duration only when that receipt exists and is later
  than `created_at`. An alert already on screen at first paint carries no
  figure, because nothing measured it. There is no field on `RunRecord` for
  time-to-alert and the UI must not invent one.
- **Synthetic data is labelled from the payload, never from a flag.**
  `components/DataSourceBadge.tsx` derives both the pill and the footnote from
  two facts: which backend answered (`Fetched.answeredBy`) and how many rendered
  mentions carry `raw.synthetic === true`, which is what `demo/inject_crisis`
  marks. Live data gets a "live via bff" pill and no footnote at all, because a
  standing disclaimer is a label readers learn to stop seeing. There is no
  `IS_DEMO` constant and there must not be one. The badge never claims the brand
  is fictional: nothing on the wire says so, and that is a different claim from
  "a synthetic mention was injected into a real run", so the two never share a
  sentence.
- **Tokens only.** Components reference CSS custom properties from
  `app/globals.css`, never raw hex, so a re-theme is one file.
- Next reconfigures `tsconfig.json` on build (`jsx` and `include`). That edit is
  expected; do not revert it.

## Who calls it

Nobody. This is a leaf. It calls the Fastify BFF and nothing calls it.

## Run

```bash
npm install
export BFF_BASE_URL=http://localhost:8080   # no page renders without this
npm run dev     # http://localhost:3000
npm run build   # runs check:types, then typechecks
```
