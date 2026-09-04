// The frame the dashboard shows while page.tsx is awaiting the BFF.
//
// It draws the panel outlines the real page will fill and nothing else. No cell
// carries a placeholder digit, not even a greyed one: in a monitoring product a
// number on screen is a claim, and "0" or "--" both read as a measurement that
// came back. A shimmering bar cannot be mistaken for a figure.
//
// It must not import lib/pulse.ts or fetch anything. Next renders this instantly
// while the real page's own fetch is in flight.

const KPI_FRAMES = 6;

/** Section titles are copied from page.tsx on purpose: the shell is what stays
    put while the values arrive, so the two files showing the same headings is
    the point rather than duplication to factor out. */
const PANEL_TITLES = [
  "Sentiment across the mentions on this page",
  "Live alerts",
  "Mention stream",
  "What people are talking about",
  "Daily brief",
  "Run report",
];

export default function Loading() {
  return (
    <>
      <section className="brandhead">
        <span className="eyebrow">
          <i className="dot dot-live" />
          reading the backend
        </span>
        <h1>Loading this brand&rsquo;s pulse.</h1>
        <p className="brandhead-sub">
          Waiting on the BFF. Nothing is drawn until the payload arrives, because a
          placeholder digit in a monitoring dashboard is a fabricated measurement.
        </p>
      </section>

      <div className="kpis">
        {Array.from({ length: KPI_FRAMES }, (_unused, index) => (
          <div className="kpi" key={index}>
            <span className="skeleton skeleton-label" />
            <span className="skeleton skeleton-value" />
          </div>
        ))}
      </div>

      {PANEL_TITLES.map((title) => (
        <section className="section" key={title}>
          <div className="section-head">
            <h2>{title}</h2>
          </div>
          <div className="card">
            <span className="skeleton skeleton-line" />
            <span className="skeleton skeleton-line" />
            <span className="skeleton skeleton-line" />
          </div>
        </section>
      ))}
    </>
  );
}
