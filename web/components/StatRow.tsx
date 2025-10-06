// The five headline numbers from a DailyBrief, computed by the agents.

import type { BriefNumbers } from "@/lib/types";
import { signedPct } from "@/lib/format";

export function StatRow({ numbers }: { numbers: BriefNumbers }) {
  const rising = numbers.mentions_delta_pct >= 0;
  return (
    <div className="stats">
      <div className="card stat">
        <div className="stat-label">Mentions 24h</div>
        <div className="stat-value">{numbers.mentions}</div>
        <div className={`stat-delta ${rising ? "up" : "down"}`}>
          {signedPct(numbers.mentions_delta_pct)} vs yesterday
        </div>
      </div>
      <div className="card stat">
        <div className="stat-label">Sentiment</div>
        <div className="stat-value" style={{ color: numbers.sentiment_avg < 0 ? "var(--negative)" : "var(--positive)" }}>
          {numbers.sentiment_avg.toFixed(2)}
        </div>
        <div className="stat-delta">mean, range -1 to 1</div>
      </div>
      <div className="card stat">
        <div className="stat-label">Negative share</div>
        <div className="stat-value">{(numbers.negative_share * 100).toFixed(0)}%</div>
        <div className="stat-delta up">above the 20% baseline</div>
      </div>
      <div className="card stat">
        <div className="stat-label">Share of voice</div>
        <div className="stat-value">{numbers.share_of_voice.toFixed(1)}%</div>
        <div className="stat-delta">against 4 competitors</div>
      </div>
      <div className="card stat">
        <div className="stat-label">Open alerts</div>
        <div className="stat-value" style={{ color: "var(--negative)" }}>1</div>
        <div className="stat-delta">1 critical, 0 high</div>
      </div>
    </div>
  );
}
