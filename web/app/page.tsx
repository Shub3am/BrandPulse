// The one page that answers "what are we building": a brand's pulse, the alerts
// that got there first, and the platforms that produced them.
//
// It reads the BFF through lib/pulse.ts and passes each artifact to the component
// that renders it. No panel here computes a number, and there is no fallback to
// the synthetic fixtures in lib/: when the backend is down this page throws and
// app/error.tsx says so, because a dashboard that silently swaps in synthetic
// figures is the failure this product is pitched against.
//
// Nothing under app/ may name that fixture module, not even in a comment. The
// invariant is enforced by grep, and grep cannot tell a comment from an import.

import { AlertFeed } from "@/components/AlertFeed";
import { DraftCard } from "@/components/DraftCard";
import { MentionStream } from "@/components/MentionStream";
import { PlatformPanel } from "@/components/PlatformPanel";
import { RunNotice } from "@/components/RunNotice";
import { StatRow } from "@/components/StatRow";
import { TopicList } from "@/components/TopicList";
import { fetchPulse } from "@/lib/pulse";

// One brand per deployment until there is a route segment for it. The BFF is
// per-brand on every route, so this is the only place the choice is made.
const BRAND_ID = process.env.BRAND_ID ?? "lumeo";

/** Every paint is a fresh read. A cached crisis from an hour ago is not news. */
export const dynamic = "force-dynamic";

export default async function PulsePage() {
  const { data: pulse } = await fetchPulse(BRAND_ID);
  const { profile, brief, run, drafts } = pulse;
  const draft = drafts[0];

  return (
    <>
      <section className="hero">
        <span className="pill pill-accent">
          <i className="dot" />
          crisis-before-it-trends
        </span>
        <h1>Know it&rsquo;s going wrong before your customers tell you.</h1>
        <p>
          BrandPulse watches Reddit, Amazon, YouTube, the Play Store and Indian news for
          your brand, and puts the thread that is about to blow up on this screen as soon as
          the run that found it writes the alert, with a reply already drafted in your voice.
        </p>
        <div className="hero-meta">
          <span className="pill">{profile ? profile.name : pulse.brand_id}</span>
          {profile && <span className="pill">{profile.sources.length} sources</span>}
          {profile && <span className="pill">{profile.competitors.length} competitors tracked</span>}
          <span className="pill">₹2,999 / month</span>
        </div>
      </section>

      {/* Above the numbers, not below them: a partial run has to be readable
          before the figures it produced are. */}
      <RunNotice run={run} errors={pulse.errors} />

      {brief ? (
        <StatRow numbers={brief.numbers} alerts={pulse.alerts} />
      ) : (
        <p className="empty">
          No daily brief for {pulse.brand_id} yet, so there are no headline numbers to show.
          bp-briefer writes one per period and the panels below do not depend on it.
        </p>
      )}

      <section className="section">
        <div className="section-head">
          <h2>Live alerts</h2>
          <p>Fired by rules, not by a model. Every number below is checked, not narrated.</p>
        </div>
        <AlertFeed brandId={pulse.brand_id} initial={pulse.alerts} />
      </section>

      <section className="section">
        <div className="cols">
          <div>
            <div className="section-head">
              <h2>Mention stream</h2>
              <p>Collected through Anakin, classified once, cached by content hash.</p>
            </div>
            {pulse.mentions.length > 0 ? (
              <MentionStream items={pulse.mentions} />
            ) : (
              <p className="empty">
                No mentions stored for {pulse.brand_id}. Either nothing has matched this
                brand&rsquo;s keywords yet or no collector has run, and the run report below
                says which.
              </p>
            )}
          </div>
          <div>
            <div className="section-head">
              <h2>Suggested reply</h2>
              <p>Drafted, never sent.</p>
            </div>
            {draft ? (
              <DraftCard draft={draft} />
            ) : (
              <p className="empty">
                No reply drafted. bp-responder drafts against an alert or a mention, so there is
                nothing here until one of those exists.
              </p>
            )}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section-head">
          <h2>What people are talking about</h2>
          <p>TF-IDF clusters over the last 24 hours.</p>
        </div>
        {pulse.topics.length > 0 ? (
          <TopicList topics={pulse.topics} />
        ) : (
          <p className="empty">
            No topics for the last 24 hours. bp-clusterer needs enough mentions in one
            window to form a cluster, so a quiet day produces none.
          </p>
        )}
      </section>

      <section className="section">
        <div className="section-head">
          <h2>Run report</h2>
          <p>What each platform actually did, including what it had to skip.</p>
        </div>
        {run ? (
          <PlatformPanel run={run} drafts={pulse.drafts} />
        ) : (
          <p className="empty">
            No run recorded for {pulse.brand_id}. Nothing has collected for this brand yet, which
            is also why the panels above may be empty.
          </p>
        )}
      </section>
    </>
  );
}
