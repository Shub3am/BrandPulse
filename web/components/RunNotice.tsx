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
import { count } from "@/lib/format";
import { listOf, numberOf, textOf } from "@/lib/wire";

/**
 * `errors` is the transport-level array from the /pulse artifact. It is separate
 * from `run.errors`, which is what the run recorded about itself, and the two
 * are concatenated rather than merged: a duplicate line is honest, a dropped
 * one is not.
 */
export function RunNotice({ run, errors }: { run: RunRecord | null; errors: string[] }) {
  const reported = [...listOf(errors), ...listOf(run?.errors)]
    .map((message) => textOf(message))
    .filter((message): message is string => message !== null);

  if (!run && reported.length === 0) return null;

  const skipped = listOf(run?.sources_skipped)
    .map((source) => textOf(source))
    .filter((source): source is string => source !== null);
  const attempted = numberOf(run?.sources_attempted?.length);
  const collected = numberOf(run?.mentions_collected);
  const degraded = textOf(run?.degraded_reason);
  const status = textOf(run?.status);
  const runId = textOf(run?.id);

  return (
    <div className="notice">
      {run && (
        <p>
          Run <span className="mono">{runId ?? "with no id"}</span> reported status{" "}
          <b>{status ?? "none"}</b>, collecting{" "}
          {collected === null ? "an unrecorded number of" : count(collected)} mentions from{" "}
          {attempted === null ? "an unrecorded number of" : count(attempted)} attempted sources.
        </p>
      )}

      {skipped.length > 0 && (
        <p>
          Skipped: <b>{skipped.join(", ")}</b>. No count, chart or sentence on this page
          includes anything from {skipped.length === 1 ? "it" : "them"}.
        </p>
      )}

      {degraded && (
        <p>
          Degraded, in the orchestrator&rsquo;s own words: <b>{degraded}</b>
        </p>
      )}

      {reported.length > 0 && (
        <>
          <p>
            {count(reported.length)} error{reported.length === 1 ? "" : "s"} came back with this
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
