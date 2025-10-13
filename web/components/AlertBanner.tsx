// Renders one alert with the numbers that fired it.
//
// The evidence table is the point of this component. bp-detector is pure
// statistics with no LLM, so the UI shows the observed value against the
// constant it was compared with, rather than claiming an AI found a crisis.

import type { Alert } from "@/lib/types";
import { clockTime, humanLabel, minutesAndSeconds } from "@/lib/format";

export function AlertBanner({ alert, secondsToWhatsapp }: { alert: Alert; secondsToWhatsapp: number }) {
  return (
    <div className="alert">
      <div className="alert-head">
        <span className="pill pill-negative">
          <i className="dot" />
          {alert.severity}
        </span>
        <span className="pill">{humanLabel(alert.kind)}</span>
        <span className="pill">fired {clockTime(alert.created_at)} IST</span>
        <span className="pill pill-accent">
          WhatsApp sent in {minutesAndSeconds(secondsToWhatsapp)}
        </span>
      </div>

      <h3>{alert.title}</h3>
      <p className="alert-why">{alert.why}</p>

      <div className="evidence">
        <div className="evidence-row evidence-head">
          <span>metric</span>
          <span>observed</span>
          <span>threshold</span>
          <span>window</span>
          <span>detail</span>
        </div>
        {alert.evidence.map((fact) => (
          <div className="evidence-row" key={fact.metric}>
            <span className="mono">{fact.metric}</span>
            <span className="evidence-fail">{fact.value}</span>
            <span>{fact.threshold}</span>
            <span>{fact.window}</span>
            <span className="muted">{fact.detail}</span>
          </div>
        ))}
      </div>

      <div className="draft-actions">
        <button className="btn btn-primary">Open suggested reply</button>
        <button className="btn">Acknowledge</button>
        <span className="mono note push-right">dedupe_key {alert.dedupe_key}</span>
      </div>
    </div>
  );
}
