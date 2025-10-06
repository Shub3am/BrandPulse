// Makes the three platforms visible in the product rather than only in the
// architecture diagram: what Nasiko orchestrated, what Anakin cost, what
// DronaHQ delivered. The numbers come from the RunRecord the orchestrator
// wrote, so this panel is also the honest degradation report.

import type { RunRecord } from "@/lib/types";
import { minutesAndSeconds, rupees } from "@/lib/format";

export function PlatformPanel({ run, secondsToWhatsapp }: { run: RunRecord; secondsToWhatsapp: number }) {
  const runSeconds = run.finished_at
    ? Math.round((Date.parse(run.finished_at) - Date.parse(run.started_at)) / 1000)
    : 0;

  return (
    <div className="platforms">
      <div className="card platform">
        <h4>
          <i className="dot" style={{ background: "var(--accent)" }} />
          Nasiko
        </h4>
        <p>Nine A2A agents, routed and metered.</p>
        <div className="kv"><span>Agents invoked</span><span>9</span></div>
        <div className="kv"><span>Run status</span><span>{run.status}</span></div>
        <div className="kv"><span>Wall clock</span><span>{minutesAndSeconds(runSeconds)}</span></div>
        <div className="kv"><span>LLM spend</span><span>{rupees(run.cost_paise)}</span></div>
        {run.degraded_reason && (
          <p style={{ marginTop: 12, marginBottom: 0, fontSize: 12.5, color: "var(--warn)" }}>
            {run.degraded_reason}
          </p>
        )}
      </div>

      <div className="card platform">
        <h4>
          <i className="dot" style={{ background: "var(--accent)" }} />
          Anakin
        </h4>
        <p>Every mention on this page was fetched here.</p>
        <div className="kv"><span>Credits this run</span><span>{run.credits_used}</span></div>
        <div className="kv"><span>Mentions collected</span><span>{run.mentions_collected}</span></div>
        <div className="kv"><span>Sources hit</span><span>{run.sources_attempted.join(", ")}</span></div>
        <div className="kv"><span>Skipped</span><span>{run.sources_skipped.join(", ") || "none"}</span></div>
      </div>

      <div className="card platform">
        <h4>
          <i className="dot" style={{ background: "var(--accent)" }} />
          DronaHQ
        </h4>
        <p>The founder never opens a dashboard at 2am.</p>
        <div className="kv"><span>Alert to WhatsApp</span><span>{minutesAndSeconds(secondsToWhatsapp)}</span></div>
        <div className="kv"><span>Channel</span><span>WhatsApp Business</span></div>
        <div className="kv"><span>Reply drafts queued</span><span>1</span></div>
        <div className="kv"><span>Auto-posted</span><span>0, by design</span></div>
      </div>
    </div>
  );
}
