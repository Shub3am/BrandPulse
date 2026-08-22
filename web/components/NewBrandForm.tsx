"use client";

// The form that onboards a brand. Four fields, one POST, then that brand's
// dashboard.
//
// It posts to app/api/brands/route.ts rather than to the BFF, because
// BFF_BASE_URL is server-only and this component runs in the browser.
//
// It validates nothing beyond the two fields the button needs to be pressable.
// `newBrandFrom` in bff/src/routes/brands.ts is the one place that decides what
// a brand may say, and a second opinion here would drift from it.

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { BRAND_PARAM } from "@/lib/brand";
import type { CreatedBrand } from "@/lib/pulse";

export function NewBrandForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [website, setWebsite] = useState("");
  const [keywords, setKeywords] = useState("");
  const [competitors, setCompetitors] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [partial, setPartial] = useState<CreatedBrand | null>(null);

  async function save(event: React.FormEvent): Promise<void> {
    event.preventDefault();
    setSaving(true);
    setSaveError(null);
    setPartial(null);
    try {
      const response = await fetch("/api/brands", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          website: website.trim(),
          keywords: terms(keywords),
          competitors: terms(competitors),
        }),
      });
      const body = (await response.json()) as (CreatedBrand & { error?: unknown }) | null;
      if (!response.ok || !body || typeof body.brand_id !== "string") {
        const reason = body && typeof body.error === "string" ? body.error : null;
        setSaveError(reason ?? `the backend answered ${response.status}`);
        return;
      }
      // A brand created without bp-onboarder is a real brand with only the
      // keywords that were typed, so the dashboard link is offered rather than
      // followed: a redirect would take the one sentence that says so off the
      // screen before it had been read.
      if (body.onboarder_error) {
        setPartial(body);
        return;
      }
      router.push(dashboardOf(body.brand_id));
    } catch (cause) {
      setSaveError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="form" onSubmit={save}>
      <label className="field">
        <span>Brand name</span>
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Lumeo"
          required
        />
      </label>

      <label className="field">
        <span>Website</span>
        <input
          value={website}
          onChange={(event) => setWebsite(event.target.value)}
          placeholder="https://lumeo.in"
          required
        />
        <small>bp-onboarder reads this to work out what the brand sells and how it speaks.</small>
      </label>

      <label className="field">
        <span>Keywords</span>
        <input
          value={keywords}
          onChange={(event) => setKeywords(event.target.value)}
          placeholder="lumeo, lumeo serum, lumeo india"
        />
        <small>
          Comma separated, optional. Whatever is typed here is kept even when the crawl adds its
          own, and it is all the brand has if bp-onboarder cannot be reached.
        </small>
      </label>

      <label className="field">
        <span>Competitors</span>
        <input
          value={competitors}
          onChange={(event) => setCompetitors(event.target.value)}
          placeholder="minimalist, dot and key"
        />
        <small>Comma separated, optional. Tracked for share of voice, not collected for.</small>
      </label>

      <div className="form-actions">
        <button
          className="btn btn-primary"
          type="submit"
          disabled={saving || name.trim() === "" || website.trim() === ""}
        >
          {saving ? "Onboarding..." : "Add brand"}
        </button>
        <Link className="btn" href="/">
          Cancel
        </Link>
      </div>

      {saveError && <p className="form-error mono">brand not created: {saveError}</p>}

      {partial && (
        <p className="notice">
          {partial.brand_id} was created from the keywords you typed. bp-onboarder did not
          answer, so nothing was crawled from the website: {partial.onboarder_error}{" "}
          <Link className="link" href={dashboardOf(partial.brand_id)}>
            Open the dashboard
          </Link>
        </p>
      )}
    </form>
  );
}

function terms(typed: string): string[] {
  return typed
    .split(",")
    .map((term) => term.trim())
    .filter((term) => term !== "");
}

function dashboardOf(brandId: string): string {
  return `/?${BRAND_PARAM}=${encodeURIComponent(brandId)}`;
}
