// The one page that answers "what are we building": a brand's pulse, the alerts
// that got there first, the conversation behind them and what the run cost.
//
// It reads the BFF through lib/pulse.ts and passes each artifact to the
// component that renders it. No panel here computes a number, and there is no
// fallback to the synthetic fixtures in lib/: when the backend is down this page
// throws and app/error.tsx says so, because a dashboard that silently swaps in
// synthetic figures is the failure this product is pitched against.
//
// Nothing under app/ may name that fixture module, not even in a comment. The
// invariant is enforced by grep, and grep cannot tell a comment from an import.
//
// Every slot in the payload is nullable and every list can arrive empty, so each
// section below picks between a panel and a written empty state. There is no
// path here that reads a length or a field off a value the contract allows to be
// absent.

import { AlertFeed } from "@/components/AlertFeed";
import { BriefPanel } from "@/components/BriefPanel";
import { DataSourceBadge, DataSourceFootnote } from "@/components/DataSourceBadge";
import { DraftPanel } from "@/components/DraftCard";
import { KpiStrip } from "@/components/KpiStrip";
import { MentionStream } from "@/components/MentionStream";
import { PlatformPanel } from "@/components/PlatformPanel";
import { RunNotice } from "@/components/RunNotice";
import { SentimentMeter } from "@/components/SentimentMeter";
import { TopicList } from "@/components/TopicList";
import { BRAND_PARAM, selectedBrandId } from "@/lib/brand";
import { count, dayAndTime } from "@/lib/format";
import { fetchPulse } from "@/lib/pulse";
import { listOf, textOf } from "@/lib/wire";

/** Every paint is a fresh read. A cached crisis from an hour ago is not news. */
export const dynamic = "force-dynamic";

export default async function PulsePage({
  searchParams,
}: {
  // Awaited, because searchParams is a promise in this version of Next.
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const requested = (await searchParams)[BRAND_PARAM];
  const { answeredBy, data: pulse } = await fetchPulse(
    selectedBrandId(typeof requested === "string" ? requested : undefined),
  );

  const { profile, brief, run } = pulse;
  const mentions = listOf(pulse.mentions);
  const alerts = listOf(pulse.alerts);
  const topics = listOf(pulse.topics);
  const drafts = listOf(pulse.drafts);
  const errors = listOf(pulse.errors);

  const brandName = textOf(profile?.name) ?? pulse.brand_id;
  const sources = listOf(profile?.sources);
  const competitors = listOf(profile?.competitors);
  const keywords = listOf(profile?.keywords);
  const lastRunAt = dayAndTime(run?.started_at);

  return (
    <>
      <section className="brandhead">
        <span className="eyebrow">
          <i className="dot" />
          crisis before it trends
        </span>
        <h1>
          {brandName}
          <span className="brandhead-id">{pulse.brand_id}</span>
        </h1>
        <p className="brandhead-sub">
          BrandPulse watches Reddit, YouTube, Indian news, the open web, the Play Store and the
          App Store for this brand, classifies every mention, fires the alert the moment a rule
          crosses, and drafts the reply in your voice. Nothing is posted without you.
        </p>
        <div className="meta-row">
          <DataSourceBadge answeredBy={answeredBy} mentions={mentions} />
          {sources.length > 0 && <span className="pill">{count(sources.length)} sources watched</span>}
          {keywords.length > 0 && <span className="pill">{count(keywords.length)} keywords</span>}
          {competitors.length > 0 && (
            <span className="pill">{count(competitors.length)} competitors tracked</span>
          )}
          {lastRunAt && <span className="pill">last run {lastRunAt} IST</span>}
        </div>
      </section>

      {/* Above the numbers, not below them: a partial run has to be readable
          before the figures it produced are. */}
      <RunNotice run={run} errors={errors} />

      <KpiStrip mentions={mentions} alerts={alerts} topics={topics} brief={brief} run={run} />

      <section className="section">
        <div className="section-head">
          <h2>Sentiment across the mentions on this page</h2>
          <p>Counted from the label bp-enricher assigned, never re-classified here.</p>
        </div>
        <SentimentMeter mentions={mentions} />
      </section>

      <section className="section">
        <div className="section-head">
          <h2>Live alerts</h2>
          <p>Fired by rules, not by a model. Every number below is checked, not narrated.</p>
          <span className="section-count push-right">
            {count(alerts.length)} in the last window
          </span>
        </div>
        <AlertFeed brandId={pulse.brand_id} initial={alerts} />
      </section>

      <section className="section">
        <div className="cols">
          <div>
            <div className="section-head">
              <h2>Mention stream</h2>
              <p>Collected through Anakin, classified once, cached by content hash.</p>
              <span className="section-count push-right">{count(mentions.length)} rows</span>
            </div>
            {mentions.length > 0 ? (
              <MentionStream items={mentions} />
            ) : (
              <p className="empty">
                <b>No mentions stored for {pulse.brand_id}.</b>
                Either nothing has matched this brand&rsquo;s keywords yet or no collector has
                run, and the run report below says which.
              </p>
            )}
          </div>
          <div>
            <div className="section-head">
              <h2>Suggested replies</h2>
              <p>Drafted, never sent.</p>
            </div>
            {drafts.length > 0 ? (
              <DraftPanel drafts={drafts} />
            ) : (
              <p className="empty">
                <b>No reply drafted.</b>
                bp-responder drafts against an alert or a mention, so there is nothing here
                until one of those exists.
              </p>
            )}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section-head">
          <h2>What people are talking about</h2>
          <p>TF-IDF clusters over the run window, in the order bp-clusterer returned them.</p>
          <span className="section-count push-right">{count(topics.length)} clusters</span>
        </div>
        {topics.length > 0 ? (
          <TopicList topics={topics} />
        ) : (
          <p className="empty">
            <b>No topics for this window.</b>
            bp-clusterer needs enough mentions in one window to form a cluster, so a quiet day
            produces none.
          </p>
        )}
      </section>

      <section className="section">
        <div className="section-head">
          <h2>Daily brief</h2>
          <p>Written by bp-briefer from the period&rsquo;s own numbers.</p>
        </div>
        {brief ? (
          <BriefPanel brief={brief} />
        ) : (
          <p className="empty">
            <b>No daily brief for {pulse.brand_id} yet.</b>
            bp-briefer writes one per period, so there are no period figures to show. Every
            panel above reads the raw artifacts instead and does not depend on it.
          </p>
        )}
      </section>

      <section className="section">
        <div className="section-head">
          <h2>Run report</h2>
          <p>What each platform actually did, including what it had to skip.</p>
        </div>
        {run ? (
          <PlatformPanel run={run} drafts={drafts} />
        ) : (
          <p className="empty">
            <b>No run recorded for {pulse.brand_id}.</b>
            Nothing has collected for this brand yet, which is also why the panels above may be
            empty. Run now in the top bar starts one.
          </p>
        )}
      </section>

      <DataSourceFootnote answeredBy={answeredBy} mentions={mentions} />
    </>
  );
}
