// Renders one alert with the numbers that fired it.
//
// The evidence table is the point of this component. bp-detector is pure
// statistics with no LLM, so the UI shows the observed value against the
// constant it was compared with, rather than claiming an AI found a crisis.
//
// Severity decides the colour of the rule down the left edge and nothing else.
// It does not decide the order these render in, because that would be a
// priority rule running in a browser where `nasiko observe` cannot see it: the
// feed keeps whatever order the BFF returned.
//
// `receivedAt` is the browser's own timestamp for when this alert arrived, and
// the only reason this component does arithmetic. It is absent for an alert
// that was already on screen at first paint, and the pill is then absent too:
// nothing measured this one, so nothing claims a figure for it.

import type { CSSProperties } from "react";
import type { Alert } from "@/lib/types";
import { clockTime, durationBetween, humanLabel, severityColor } from "@/lib/format";
import { listOf, numberOf, textOf } from "@/lib/wire";

// Neither action has a route in the BFF, so both are disabled and say why. An
// enabled button that does nothing is a worse claim than a disabled one.
const NO_ROUTE = "Not wired up yet. The BFF has no route behind this action.";

/** A figure from the evidence row, printed as it arrived or reported absent. */
function figure(value: number | null | undefined): string {
  const real = numberOf(value);
  return real === null ? "not recorded" : String(real);
}

export function AlertBanner({ alert, receivedAt }: { alert: Alert; receivedAt?: string }) {
  const timeToScreen = durationBetween(alert.created_at, receivedAt);
  const firedAt = clockTime(alert.created_at);
  const severity = alert.severity;
  const evidence = listOf(alert.evidence);
  const samples = listOf(alert.sample_mentions);
  const dedupe = textOf(alert.dedupe_key);
  const status = textOf(alert.status);

  return (
    <div
      className={`alert${severity ? ` alert-${severity}` : ""}`}
      style={{ "--sev": severityColor(severity) } as CSSProperties}
    >
      <div className="alert-head">
        <span className="alert-sev">
          <i className="dot" />
          {severity ?? "unrated"}
        </span>
        <span className="pill">{humanLabel(alert.kind) ?? "kind not recorded"}</span>
        {firedAt && <span className="pill">fired {firedAt} IST</span>}
        {status && <span className="pill">{status}</span>}
        {timeToScreen && <span className="pill pill-accent">on screen in {timeToScreen}</span>}
      </div>

      <h3>{textOf(alert.title) ?? "This alert arrived without a title."}</h3>
      {textOf(alert.why) && <p className="alert-why">{alert.why}</p>}

      {evidence.length > 0 ? (
        <div className="evidence">
          <div className="evidence-row evidence-head">
            <span>metric</span>
            <span>observed</span>
            <span>threshold</span>
            <span>window</span>
            <span>detail</span>
          </div>
          {evidence.map((fact, index) => (
            <div className="evidence-row" key={`${textOf(fact.metric) ?? "metric"}-${index}`}>
              <span className="mono">{textOf(fact.metric) ?? "unnamed"}</span>
              <span className="evidence-fail">{figure(fact.value)}</span>
              <span>{figure(fact.threshold)}</span>
              <span>{textOf(fact.window) ?? "no window"}</span>
              <span className="muted">{textOf(fact.detail) ?? ""}</span>
            </div>
          ))}
        </div>
      ) : (
        <p className="note" style={{ marginTop: 14 }}>
          This alert carried no evidence rows, so there is no observed value to check it
          against. Nothing has been filled in for them.
        </p>
      )}

      {samples.length > 0 && (
        <div className="alert-samples">
          {samples.slice(0, 3).map((sample, index) => (
            <p className="alert-sample" key={textOf(sample.id) ?? `sample-${index}`}>
              {textOf(sample.source) && <b>{sample.source} </b>}
              {textOf(sample.text) ?? "This sample mention arrived with no text."}
            </p>
          ))}
        </div>
      )}

      <div className="alert-foot">
        <button className="btn btn-primary" disabled title={NO_ROUTE}>
          Open suggested reply
        </button>
        <button className="btn" disabled title={NO_ROUTE}>
          Acknowledge
        </button>
        {dedupe && <span className="mono note push-right">dedupe_key {dedupe}</span>}
      </div>
    </div>
  );
}
