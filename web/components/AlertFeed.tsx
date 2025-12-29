"use client";

// The live alert feed. Since the scope change this is the primary alert surface
// and time-to-alert is measured to this component, not to a WhatsApp send.
//
// It polls with the newest `created_at` it already holds, so Postgres decides
// what is new and a browser clock running fast cannot skip an alert. It renders
// every alert the BFF returns, in the order it returns them: picking a hero alert
// would be a severity rule running in a browser, where `nasiko observe` cannot
// see it.
//
// The measurement is deliberately narrow. An alert already on screen at first
// paint was detected before this page existed, so it carries no number. Only an
// alert that arrives while the feed is open gets a receipt, and only when that
// receipt is later than the alert's own `created_at`: a browser clock behind the
// database's would otherwise produce a negative measurement, and a fabricated
// number is what this feed replaced.

import { useEffect, useRef, useState } from "react";
import type { Alert } from "@/lib/types";
import { clockTime } from "@/lib/format";
import { AlertBanner } from "./AlertBanner";

const POLL_INTERVAL_MS = 2_000;

export function AlertFeed({ brandId, initial }: { brandId: string; initial: Alert[] }) {
  const [alerts, setAlerts] = useState<Alert[]>(initial);
  /** Alert id to the browser's own ISO timestamp for when it arrived. */
  const [receipts, setReceipts] = useState<Record<string, string>>({});
  const [pollError, setPollError] = useState<string | null>(null);
  const [watchingSince] = useState(() => new Date().toISOString());

  // A ref, not state: the poll reads it and writing it must not restart the loop.
  const sinceRef = useRef<string | undefined>(newestCreatedAt(initial));

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function poll(): Promise<void> {
      try {
        const query = new URLSearchParams({ brand: brandId });
        if (sinceRef.current) query.set("since", sinceRef.current);

        const response = await fetch(`/api/alerts?${query.toString()}`, { cache: "no-store" });
        const body: unknown = await response.json();
        if (cancelled) return;

        if (!response.ok) {
          setPollError(errorMessage(body) ?? `the backend answered ${response.status}`);
        } else {
          const arrived = body as Alert[];
          const receivedAt = new Date().toISOString();
          setPollError(null);
          if (arrived.length > 0) {
            setAlerts((current) => mergeNewestFirst(current, arrived));
            setReceipts((current) => ({ ...current, ...measurable(arrived, receivedAt) }));
            sinceRef.current = newestCreatedAt(arrived) ?? sinceRef.current;
          }
        }
      } catch (cause) {
        // A failed poll leaves the alerts already on screen alone. Emptying the
        // feed because one request failed would read as "the crisis is over".
        if (!cancelled) setPollError(cause instanceof Error ? cause.message : String(cause));
      }

      if (!cancelled) timer = setTimeout(poll, POLL_INTERVAL_MS);
    }

    // Chained timeouts rather than an interval, so a slow answer cannot stack up
    // a queue of overlapping requests against the BFF.
    timer = setTimeout(poll, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [brandId]);

  return (
    <div className="feed">
      <div className="feed-status">
        <span className="pill">
          <i className="dot" style={{ background: pollError ? "var(--warn)" : "var(--accent)" }} />
          {pollError ? "feed stalled" : "watching live"}
        </span>
        <span className="note">
          since {clockTime(watchingSince)} IST, every {POLL_INTERVAL_MS / 1000}s
        </span>
        {pollError && <span className="note push-right">last poll failed: {pollError}</span>}
      </div>

      {alerts.length === 0 ? (
        <p className="empty">
          No alerts for {brandId}. This feed has been watching since {clockTime(watchingSince)} IST
          and has seen nothing fire, which is the good outcome.
        </p>
      ) : (
        alerts.map((alert) => (
          <AlertBanner key={alert.id} alert={alert} receivedAt={receipts[alert.id]} />
        ))
      )}
    </div>
  );
}

/** Newest first, and one entry per id: a repeated alert must not render twice. */
function mergeNewestFirst(current: Alert[], arrived: Alert[]): Alert[] {
  const byId = new Map(current.map((alert) => [alert.id, alert]));
  for (const alert of arrived) byId.set(alert.id, alert);
  return [...byId.values()].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
}

function newestCreatedAt(alerts: Alert[]): string | undefined {
  let newest: string | undefined;
  for (const alert of alerts) {
    if (!newest || Date.parse(alert.created_at) > Date.parse(newest)) newest = alert.created_at;
  }
  return newest;
}

/** Receipts only for alerts whose arrival is actually measurable. */
function measurable(arrived: Alert[], receivedAt: string): Record<string, string> {
  const receipts: Record<string, string> = {};
  for (const alert of arrived) {
    if (Date.parse(receivedAt) >= Date.parse(alert.created_at)) receipts[alert.id] = receivedAt;
  }
  return receipts;
}

function errorMessage(body: unknown): string | undefined {
  if (typeof body !== "object" || body === null) return undefined;
  const error = (body as { error?: unknown }).error;
  return typeof error === "string" ? error : undefined;
}
