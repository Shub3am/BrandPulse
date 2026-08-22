"use client";

// The top bar's Run now. It asks bp-orchestrator, through the BFF, to collect
// now rather than on its schedule.
//
// It posts to app/api/runs/route.ts rather than to the BFF, because BFF_BASE_URL
// is server-only and this component runs in the browser.
//
// A failed start is printed beside the button, not logged. The orchestrator is a
// separate process and the ordinary failure is that it is not running at all, so
// a message in a console nobody has open is a run that appears to do nothing.
//
// It holds no run. The panels below are server components, so a started run
// reaches the screen by re-running their fetch, which is what router.refresh does.

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { RunKind } from "@/lib/types";

/** A person pressed a button, which is the one trigger this UI can claim. */
const TRIGGER: RunKind = "on_demand";

/**
 * 504 hours, which is 21 days.
 *
 * The orchestrator defaults this to 24, and under BP_FIXTURE_MODE=replay a 24
 * hour run collects nothing. The recorded Anakin corpus was captured over a 21
 * day window (demo/record/main.go, recordingWindowDays), and reddit's fixture
 * key includes a time bucket derived from the width of the collection window,
 * so a shorter window hashes to fixture files that were never recorded and the
 * adapter reports every one of them as missing. 504 is that same 21 days, which
 * is why it is this number and not another one inside the bucket's range.
 *
 * The Go default is not changed: a window is the caller's request, and a demo
 * asking for the recorded span is not the orchestrator's policy.
 */
const WINDOW_HOURS = 504;

export function RunNowButton({ brandId }: { brandId: string }) {
  const router = useRouter();
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);

  async function start(): Promise<void> {
    setStarting(true);
    setStartError(null);
    try {
      const response = await fetch("/api/runs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          brand_id: brandId,
          trigger: TRIGGER,
          window_hours: WINDOW_HOURS,
        }),
      });
      const body = (await response.json()) as { error?: unknown } | null;
      if (!response.ok) {
        const reason = body && typeof body.error === "string" ? body.error : null;
        setStartError(reason ?? `the backend answered ${response.status}`);
        return;
      }
      router.refresh();
    } catch (cause) {
      setStartError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setStarting(false);
    }
  }

  return (
    <div className="run-now">
      <button className="btn btn-primary" onClick={start} disabled={starting}>
        {starting ? "Running..." : "Run now"}
      </button>
      {startError && (
        <p className="run-now-error mono">
          run not started: {startError}
        </p>
      )}
    </div>
  );
}
