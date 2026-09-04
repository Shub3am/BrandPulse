# web/: the product dashboard

Next.js 16 App Router, React 19, TypeScript, plain CSS. No Tailwind, no
component library, no state manager. The theme is a dense dark analytics
surface: near-black ground, cyan accent, hairline borders instead of shadows,
12px radii, tabular numerals on every figure.

## What this module owns

The screen a judge looks at: a brand's pulse, the live alert feed, the mention
stream, the topics, the drafted reply, and a run report that shows what Nasiko,
Anakin and DronaHQ each did. Plus the one screen that comes before it, where a
brand owner types their brand in and gets a dashboard for it.

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
| `app/brands/new/page.tsx` | Where a brand owner starts. A heading and the form. |
| `components/NewBrandForm.tsx` | Four fields, one POST, then that brand's dashboard. |
| `components/BrandBar.tsx` | The top bar's brand controls: which brand, switch, add. |
| `app/api/brands/route.ts` | Proxies the brand list and the create, for the reason below. |
| `components/DataSourceBadge.tsx` | Says what the data is, derived from the payload. |
| `app/api/alerts/route.ts` | Proxies the alert poll so the browser never learns the BFF's URL. |
| `app/api/runs/route.ts` | Proxies the on-demand run start, for the same reason. |
| `components/RunNowButton.tsx` | The top bar's one action. Starts a run and reports the failure. |
| `lib/brand.ts` | The default brand and the selected one. Throws when nothing says. |
| `app/loading.tsx` | Panel frames while the fetch is in flight. Holds no values. |
| `app/error.tsx` | Says the backend is unreachable. Substitutes nothing. |
| `lib/pulse.ts` | The BFF fetches. The only file here that knows the backend exists. |
| `lib/types.ts` | The wire format, mirrored from `internal/models/models.go`. |
| `lib/demoData.ts` | Synthetic sample artifacts. Nothing under `app/` imports it. |
| `lib/format.ts` | Display formatting. No business rules. |
| `lib/wire.ts` | Reads a nullable wire value into something renderable. See below. |
| `lib/derive.ts` | Counts what is on screen for display. No business rules. |
| `components/KpiStrip.tsx` | The six headline cells, each a payload figure or a sentence. |
| `components/SentimentMeter.tsx` | The segmented sentiment bar. CSS only, no chart library. |
| `components/BriefPanel.tsx` | The daily brief: its figures lifted out, its markdown as blocks. |
| `scripts/checkTypesParity.mjs` | Fails the build when a mirror drifts from `models.go`. |
| `components/*.tsx` | One component per panel, each named for what it renders. |
| `Dockerfile` | The runtime image. Its build context is the repo root, see below. |

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
  BFF directly. The route adds no field and filters nothing, and
  `app/api/runs/route.ts` exists for the same reason on the write side.
- **A missing `BRAND_ID` throws, it does not default.** `lib/brand.ts` is the one
  place the choice is made, because the page and the top bar have to reach the
  same answer. A default brand id is a brand with no row in Postgres, and this
  dashboard renders an unknown brand as empty panels, which reads as a quiet day
  rather than as a misconfiguration. `BRAND_ID` is now the default rather than
  the only brand: `?brand=` in the URL selects one, `selectedBrandId` resolves
  the two, and the picker in `BrandBar` is what writes that parameter. The
  picker always navigates to `/`, because switching brands means looking at that
  brand's dashboard and not at whatever page is open.
- **Run now sends a 504 hour window, and that number is load-bearing.**
  `RunNowButton` asks for 21 days because the recorded Anakin corpus was
  captured over 21 days and reddit's fixture key includes a time bucket derived
  from the width of the collection window, so a shorter window hashes to files
  that were never recorded and the run collects nothing. The Go default of 24 is
  deliberately not changed. A brand typed into the form has no recorded
  fixtures at all, so under `BP_FIXTURE_MODE=replay` its runs report missing
  fixtures per source and collect nothing: the flow is real, the corpus is only
  the demo brand's.
- **An empty panel is a sentence, never a blank box or a zero.** Every panel has
  a written empty state naming what is missing and which agent fills it. Zero
  alerts is a success state and says so, with the window it has been watching.
  Partial data renders, with `RunNotice` above it printing what the payload said
  went wrong. Nothing is ever stood in for: no cached figure, no last-known
  number and no placeholder digit, including in `loading.tsx`.
- **Every wire read goes through `lib/wire.ts`.** `lib/types.ts` describes what
  the contract promises, not what arrives: Go marshals a nil slice as `null`,
  the jsonb columns are nullable, and the BFF maps a SQL NULL to `undefined`.
  TypeScript cannot see any of that, because the payload is JSON at runtime.
  This dashboard has already crashed in production on
  `enrichment.aspects.join(", ")`. So no component calls `.join`, `.map`,
  `.length` or `.toFixed` on a wire value directly: `listOf` returns an array
  with the holes dropped, `mapOf` an object, `textOf` a non-empty string or
  null, `numberOf` a finite number or null, and the component picks a sentence
  when it gets null. A list of aspects, do-not-say lines or sources is rendered
  as chips, never joined into one string.
- **The sentiment meter is CSS and SVG, and there is no chart dependency.**
  `package.json` carries Next, React and the types, and nothing else belongs in
  it. A chart library would also bring its own colours, which would break the
  tokens-only rule below.
- **A price on the KPI strip is labelled as a price.** Five of the six cells in
  `KpiStrip` are counted from the payload. The sixth compares the two list
  prices, which are copy rather than measurements, so they are declared once at
  the top of that file and the cell says so on screen. Nothing else on this
  page may carry a figure that did not come off the wire.
- **`RunNotice` reports, it does not judge.** It prints `status`,
  `sources_skipped`, `degraded_reason` and both `errors` arrays as they arrived.
  It does not map a status to a severity and its CSS is deliberately neutral,
  because "which of these is bad" is `bp-detector`'s call. It prints a line for a
  healthy run too: a notice that appears only on failure teaches a reader to
  treat its absence as proof, and absence is also what a missing field looks
  like.
- **A browser clock is read after mount, never during render.** The server and
  the browser render at different moments, so a `new Date()` in a client
  component's first render is a hydration mismatch on every load and React
  answers one by throwing the subtree away. `AlertFeed`'s "watching since" is set
  in a mount effect and the sentence that carries it is absent until it exists,
  which is also the honest reading: the feed has not started watching until it is
  alive in a browser. Nothing stands in for it in the meantime.
- **Every button either does something or says it cannot.** The working actions
  are Run now, the brand picker and the new-brand form, and each reports its own
  failure on screen rather than in a console nobody has open. The draft and alert actions have no route in the
  BFF, so they are `disabled` with a `title` saying so, and the five unbuilt nav
  sections are labels rather than anchors to `#`. Wiring one of them up means
  adding the BFF route first; do not add a handler that pretends.
- **Only the browser may date an alert's arrival.** `AlertFeed` records its own
  receipt timestamp when a poll brings in an alert it has not seen, and
  `AlertBanner` prints a duration only when that receipt exists and is later
  than `created_at`. An alert already on screen at first paint carries no
  figure, because nothing measured it. There is no field on `RunRecord` for
  time-to-alert and the UI must not invent one.
- **Synthetic data is labelled from the payload, never from a flag.**
  `components/DataSourceBadge.tsx` derives both the pill and the footnote from
  two facts: which backend answered (`Fetched.answeredBy`) and how many rendered
  mentions carry `raw.synthetic === true`, which is what `demo/cmd/injectcrisis`
  marks. Live data gets a "live via bff" pill and no footnote at all, because a
  standing disclaimer is a label readers learn to stop seeing. There is no
  `IS_DEMO` constant and there must not be one. The badge never claims the brand
  is fictional: nothing on the wire says so, and that is a different claim from
  "a synthetic mention was injected into a real run", so the two never share a
  sentence.
- **`Dockerfile`'s build context is the repo root, not this directory.**
  `docker build -f web/Dockerfile -t brandpulse-web .` The image has to contain
  `internal/models/models.go` and `bff/src/contracts.ts` at build time because
  `check:types` reads them, and a `web/`-only context does not fail: it builds a
  working image with the drift check skipped, which is the one outcome the
  checker exists to prevent. `bff/Dockerfile` is the ordinary `bff/` context and
  says so, so the asymmetry is deliberate rather than an oversight.
- **`next.config.mjs` pins `outputFileTracingRoot`, and that is load-bearing.**
  `output: "standalone"` puts `server.js` at
  `.next/standalone/<project dir relative to the tracing root>/server.js`, and
  Next infers that root by walking up for a lockfile or a `workspaces`
  `package.json`. There is none above `web/` today, so the entrypoint is
  `.next/standalone/server.js` and `CMD` matches. The day anyone adds a
  `package.json` at the repo root the inferred root moves and the entrypoint
  silently becomes `.next/standalone/web/server.js`, so it is pinned to this
  directory instead of inferred. `.next/static` is not part of the trace and is
  copied separately; there is no `public/` here, so nothing is copied for it.
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

# The image. Context is the repo root, hence -f and the trailing dot.
docker build -f web/Dockerfile -t brandpulse-web ..
docker run -p 3000:3000 -e BFF_BASE_URL=http://host.docker.internal:8080 brandpulse-web
```

`BFF_BASE_URL` and `BRAND_ID` are the only variables this module reads. There is
no `DATABASE_URL` and no agent URL here, in the image or in the bundle.
