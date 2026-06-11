// Which brand this deployment monitors.
//
// One brand per deployment until there is a route segment for it, and the page
// and the top bar's Run now button both have to reach the same answer, so the
// choice is made once here instead of in each of them.
//
// A missing BRAND_ID throws rather than falling back to a name. A brand id with
// no row in Postgres returns empty panels, and empty panels on this dashboard
// read as a quiet day rather than as a misconfiguration.
//
// This file is server-only: BRAND_ID has no NEXT_PUBLIC_ prefix, for the same
// reason BFF_BASE_URL has none.

export function brandId(): string {
  const configured = process.env.BRAND_ID;
  if (!configured) {
    throw new Error("BRAND_ID is not set, so the dashboard does not know which brand to show.");
  }
  return configured;
}
