"use client";

// The top bar's brand controls: which brand is on screen, how to reach another
// one, and how to add one.
//
// It is a client component inside a server layout because a layout receives no
// params in the App Router, and the brand being viewed lives in the URL. Reading
// it here is what lets the picker and Run now agree without the page passing it
// down through a layout that cannot take props.
//
// It fetches the brand list through app/api/brands/route.ts rather than the BFF,
// because BFF_BASE_URL is server-only and this component runs in the browser.
//
// A failed list is printed, not logged. The ordinary failure is that the BFF is
// not running, and a picker that silently shows one brand teaches a reader that
// one brand is all there is.

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { RunNowButton } from "@/components/RunNowButton";
import { BRAND_PARAM } from "@/lib/brand";
import type { BrandSummary } from "@/lib/pulse";

/** The dashboard's path. Run now belongs to the screen that shows a brand. */
const DASHBOARD_PATH = "/";

export function BrandBar({ defaultBrandId }: { defaultBrandId: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const requested = useSearchParams().get(BRAND_PARAM);
  const brandId = requested ?? defaultBrandId;

  const [brands, setBrands] = useState<BrandSummary[]>([]);
  const [listError, setListError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    void (async () => {
      try {
        const response = await fetch("/api/brands", { cache: "no-store" });
        const body = (await response.json()) as BrandSummary[] | { error?: string };
        if (!live) return;
        if (!response.ok || !Array.isArray(body)) {
          const reason = !Array.isArray(body) && typeof body.error === "string" ? body.error : null;
          setListError(reason ?? `the backend answered ${response.status}`);
          return;
        }
        setBrands(body);
      } catch (cause) {
        if (live) setListError(cause instanceof Error ? cause.message : String(cause));
      }
    })();
    return () => {
      live = false;
    };
  }, []);

  // A brand that is being viewed but is not in the list yet still has to appear
  // as the selected option, or the picker reads as if some other brand were on
  // screen. This covers the moment after a brand is created and the moment the
  // list fetch failed.
  const options = brands.some((brand) => brand.id === brandId)
    ? brands
    : [{ id: brandId, name: brandId }, ...brands];

  function show(nextBrandId: string): void {
    // Switching brands is always a move to that brand's dashboard, so this
    // pushes the dashboard path rather than adding a parameter to whatever
    // page happens to be open.
    router.push(`${DASHBOARD_PATH}?${BRAND_PARAM}=${encodeURIComponent(nextBrandId)}`);
    router.refresh();
  }

  return (
    <div className="brand-bar">
      <label className="brand-pick">
        <span className="brand-pick-label">Brand</span>
        <select
          value={brandId}
          onChange={(event) => show(event.target.value)}
          aria-label="Which brand to show"
        >
          {options.map((brand) => (
            <option key={brand.id} value={brand.id}>
              {brand.name}
            </option>
          ))}
        </select>
      </label>
      <Link className="btn" href="/brands/new">
        Add brand
      </Link>
      {pathname === DASHBOARD_PATH && <RunNowButton brandId={brandId} />}
      {listError && <p className="run-now-error mono">brand list unavailable: {listError}</p>}
    </div>
  );
}
