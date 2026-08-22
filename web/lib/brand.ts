// Which brand the dashboard is showing.
//
// BRAND_ID is the deployment's default, and the page and the top bar's Run now
// button both have to reach the same answer, so the choice is made once here
// instead of in each of them.
//
// A missing BRAND_ID throws rather than falling back to a name. A brand id with
// no row in Postgres returns empty panels, and empty panels on this dashboard
// read as a quiet day rather than as a misconfiguration.
//
// Both functions here are server-only: BRAND_ID has no NEXT_PUBLIC_ prefix, for
// the same reason BFF_BASE_URL has none. BRAND_PARAM is not, because the picker
// in the top bar writes the parameter and the page reads it, and a URL both
// sides have to agree on cannot be spelled out twice.

/** The query parameter the brand picker writes and the dashboard reads. */
export const BRAND_PARAM = "brand";

export function brandId(): string {
  const configured = process.env.BRAND_ID;
  if (!configured) {
    throw new Error("BRAND_ID is not set, so the dashboard does not know which brand to show.");
  }
  return configured;
}

/**
 * The brand a request is asking for, which is the one in the URL when the
 * picker put one there and the deployment's default otherwise. An unknown id
 * is not rejected here: the BFF answers for whatever it is asked about, and
 * the dashboard's empty states already say when a brand holds nothing.
 */
export function selectedBrandId(requested: string | undefined): string {
  return requested && requested !== "" ? requested : brandId();
}
