// The one page that answers "what are we building": a brand's pulse, the
// crisis alert that got there first, and the platforms that produced it.
//
// It reads from lib/demoData today. Swapping those imports for fetches against
// the Fastify BFF is the only change needed to make it live, which is why no
// component here computes anything the agents should have sent.

import { AlertBanner } from "@/components/AlertBanner";
import { DraftCard } from "@/components/DraftCard";
import { MentionStream } from "@/components/MentionStream";
import { PlatformPanel } from "@/components/PlatformPanel";
import { StatRow } from "@/components/StatRow";
import { TopicList } from "@/components/TopicList";
import {
  BRAND, CRISIS_ALERT, DRAFT, MENTIONS, NUMBERS, RUN, TIME_TO_WHATSAPP_SECONDS, TOPICS,
} from "@/lib/demoData";

export default function PulsePage() {
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
          your brand, and puts the thread that is about to blow up on your WhatsApp in
          under two minutes, with a reply already drafted in your voice.
        </p>
        <div className="hero-meta">
          <span className="pill">{BRAND.name}</span>
          <span className="pill">{BRAND.tagline}</span>
          <span className="pill">6 sources</span>
          <span className="pill">₹2,999 / month</span>
        </div>
      </section>

      <StatRow numbers={NUMBERS} />

      <section className="section">
        <div className="section-head">
          <h2>Live alert</h2>
          <p>Fired by rules, not by a model. Every number below is checked, not narrated.</p>
        </div>
        <AlertBanner alert={CRISIS_ALERT} secondsToWhatsapp={TIME_TO_WHATSAPP_SECONDS} />
      </section>

      <section className="section">
        <div className="cols">
          <div>
            <div className="section-head">
              <h2>Mention stream</h2>
              <p>Collected through Anakin, classified once, cached by content hash.</p>
            </div>
            <MentionStream items={MENTIONS} />
          </div>
          <div>
            <div className="section-head">
              <h2>Suggested reply</h2>
              <p>Drafted, never sent.</p>
            </div>
            <DraftCard draft={DRAFT} />
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section-head">
          <h2>What people are talking about</h2>
          <p>TF-IDF clusters over the last 24 hours.</p>
        </div>
        <TopicList topics={TOPICS} />
      </section>

      <section className="section">
        <div className="section-head">
          <h2>Run report</h2>
          <p>What each platform actually did, including what it had to skip.</p>
        </div>
        <PlatformPanel run={RUN} secondsToWhatsapp={TIME_TO_WHATSAPP_SECONDS} />
      </section>

      <p className="footnote">
        Every figure on this page is synthetic demo data for a fictional brand. Nothing here
        is a claim about a real company. Live mode replaces lib/demoData with the BFF.
      </p>
    </>
  );
}
