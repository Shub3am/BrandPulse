// The frame the dashboard shows while page.tsx is awaiting the BFF.
//
// It draws panel outlines and nothing else. No stat carries a placeholder digit,
// not even a greyed one: in a monitoring product a number on screen is a claim,
// and "0" or "--" both read as a measurement that came back. A grey bar cannot
// be mistaken for a figure.
//
// It must not import lib/pulse.ts or fetch anything. Next renders this instantly
// while the real page's own fetch is in flight.

const STAT_FRAMES = 5;

/** Section titles are copied from page.tsx on purpose: the shell is what stays
    put while the values arrive, so the two files showing the same headings is
    the point rather than duplication to factor out. */
const PANEL_TITLES = ["Live alerts", "Mention stream", "What people are talking about", "Run report"];

export default function Loading() {
  return (
    <>
      <section className="hero">
        <span className="pill">
          <i className="dot" />
          reading the backend
        </span>
        <h1>Loading this brand&rsquo;s pulse.</h1>
        <p>
          Waiting on the BFF. Nothing is drawn until the payload arrives, because a
          placeholder digit in a monitoring dashboard is a fabricated measurement.
        </p>
      </section>

      <div className="stats">
        {Array.from({ length: STAT_FRAMES }, (_unused, index) => (
          <div className="card stat" key={index}>
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
