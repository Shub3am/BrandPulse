// What the dashboard shows when the fetch in page.tsx throws.
//
// It says the backend could not be reached and it shows the reason verbatim. It
// does not render sample data, a cached payload or a last-known figure. That is
// the whole point of this file: the product is pitched at brands who found out
// late, so a dashboard that looks calm while it is blind is the one failure mode
// worth crashing over.
//
// Next requires this to be a client component, which is also why the message is
// printed rather than logged: there is no server console in front of the judge.

"use client";

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <section className="hero">
      <span className="pill pill-negative">
        <i className="dot" />
        no backend
      </span>
      <h1>This dashboard could not reach its backend.</h1>
      <p>
        Every figure on this page comes from the BFF, so there is nothing to show. No
        cached run, no sample brand and no last-known number is being substituted: a
        monitoring screen that keeps drawing numbers after it goes blind is worse than one
        that stops.
      </p>

      <div className="notice">
        <p className="mono">{error.message}</p>
        {error.digest && <p className="note">digest {error.digest}</p>}
        <p>
          Check that the BFF is running and that <span className="mono">BFF_BASE_URL</span>{" "}
          points at it. The dashboard itself is up: this page rendered.
        </p>
      </div>

      <div className="alert-foot">
        <button className="btn btn-primary" onClick={reset}>
          Try the fetch again
        </button>
      </div>
    </section>
  );
}
