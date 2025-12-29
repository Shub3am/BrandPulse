// Parses the query parameters the read routes accept.
//
// A limit is the only number the BFF chooses, and it is chosen here so that
// three routes cannot disagree about the ceiling. It must not grow a parameter
// that filters on meaning: "give me the alerts that matter" is a business rule
// and belongs to bp-detector, not to a query string.

export class BadRequestError extends Error {
  readonly httpStatus = 400;
}

/**
 * A ceiling exists because the browser does not get to ask for every mention a
 * brand has ever had; that is one query holding a connection open for everyone.
 */
export function limitFrom(raw: string | undefined, fallback: number, ceiling: number): number {
  if (raw === undefined || raw === "") return fallback;
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new BadRequestError(`limit must be a positive integer, got ${JSON.stringify(raw)}`);
  }
  return Math.min(parsed, ceiling);
}

/**
 * `since` is echoed straight into a timestamptz comparison, so it is validated
 * here rather than handed to Postgres to reject. The live alert feed passes back
 * a created_at it received from us, which is why the accepted form is RFC 3339
 * and not anything a human might type.
 */
export function sinceFrom(raw: string | undefined): string | undefined {
  if (raw === undefined || raw === "") return undefined;
  if (Number.isNaN(Date.parse(raw))) {
    throw new BadRequestError(`since must be an RFC 3339 timestamp, got ${JSON.stringify(raw)}`);
  }
  return raw;
}
