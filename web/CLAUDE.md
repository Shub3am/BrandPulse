# web/ — the product dashboard

Next.js 16 App Router, React 19, TypeScript, plain CSS. No Tailwind, no
component library, no state manager. The theme is taken from anakin.io: light,
white, cyan accent, hairline borders instead of shadows, 10px radii.

## What this module owns

The one screen a judge looks at: a brand's pulse, the crisis alert that fired,
the mention stream, the topics, the drafted reply, and a run report that shows
what Nasiko, Anakin and DronaHQ each did.

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
| `app/page.tsx` | The dashboard. Composes the six components, owns no logic. |
| `app/layout.tsx` | Document shell and top bar. Holds no product state. |
| `lib/types.ts` | The wire format, mirrored from `internal/models/models.go`. |
| `lib/demoData.ts` | Synthetic demo data. The only thing live mode replaces. |
| `lib/format.ts` | Display formatting. No business rules. |
| `components/*.tsx` | One component per panel, each named for what it renders. |

## Invariants and gotchas

- **`lib/types.ts` is a mirror, not a source.** Field names must match the Go
  `json` tags in `internal/models/models.go` exactly, because the BFF passes
  agent artifacts through unreshaped. When `models.go` changes, this file
  changes in the same commit or the dashboard renders `undefined` silently.
  TypeScript will not catch it: the data arrives as JSON at runtime.
- **Going live is one file.** Swap the `lib/demoData` imports in `app/page.tsx`
  for fetches against the BFF. Nothing else changes, which is why no component
  computes a derived number.
- **Synthetic data is labelled in the UI.** The "demo data" pill in the top bar
  and the footnote on the page are load-bearing, not decoration. The repo rule
  is that no figure is ever presented as real when it is not.
- **Tokens only.** Components reference CSS custom properties from
  `app/globals.css`, never raw hex, so a re-theme is one file.
- Next reconfigures `tsconfig.json` on build (`jsx` and `include`). That edit is
  expected; do not revert it.

## Who calls it

Nobody. This is a leaf. It calls the Fastify BFF and nothing calls it.

## Run

```bash
npm install
npm run dev     # http://localhost:3000
npm run build   # also typechecks
```
