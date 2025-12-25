// Prints what the payload itself says about the completeness of the run behind
// this page: its status, the sources it skipped, the reason it degraded, and
// every error string it carried up.
//
// This component classifies nothing. It does not map a status to a severity, it
// does not colour a value, and it does not hide a field because the run looked
// fine. Deciding which of these is bad is bp-detector's job, and a second
// opinion rendered in the browser would be a rule `nasiko observe` cannot see.
// The consequence is deliberate: an "ok" run still prints a line, because a
// notice that appears only on failure teaches a judge to read its absence as
// proof, and absence is also what a missing field looks like.

import type { RunRecord } from "@/lib/types";

/**
 * `errors` is the transport-level array from the /pulse artifact. It is separate
 * from `run.errors`, which is what the run recorded about itself, and the two
 * are concatenated rather than merged: a duplicate line is honest, a dropped
 * one is not.
 */
export function RunNotice({ run, errors }: { run: RunRecord | null; errors: string[] }) {
  const reported = run ? [...errors, ...run.errors] : errors;
  if (!run && reported.length === 0) return null;

  return (
    <div className="notice">
      {run && (
        <p>
          Run <span className="mono">{run.id}</span> reported status <b>{run.status}</b>,
          collecting {run.mentions_collected} mentions from{" "}
          {run.sources_attempted.length} attempted sources.
        </p>
      )}

      {run && run.sources_skipped.length > 0 && (
        <p>
          Skipped: <b>{run.sources_skipped.join(", ")}</b>. No count, chart or sentence on
          this page includes anything from {run.sources_skipped.length === 1 ? "it" : "them"}.
        </p>
      )}

      {run?.degraded_reason && (
        <p>
          Degraded, in the orchestrator&rsquo;s own words: <b>{run.degraded_reason}</b>
        </p>
      )}

      {reported.length > 0 && (
        <>
          <p>
            {reported.length} error{reported.length === 1 ? "" : "s"} came back with this
            payload. What arrived is rendered below; what did not is absent rather than
            filled in.
          </p>
          <ul className="notice-errors">
            {reported.map((message, index) => (
              // No stable id on the wire: these are free-text strings and two
              // collectors can fail identically.
              <li className="mono" key={`${index}-${message}`}>
                {message}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
