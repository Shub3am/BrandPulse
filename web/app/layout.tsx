// The document shell and the top bar. Holds no product state.
//
// The "demo data" pill used to live here, where it was a hardcoded warning that
// could not be wrong in the reassuring direction and could not be right in the
// other. A layout receives no data, so it cannot know what the data is. The pill
// is now components/DataSourceBadge.tsx, rendered from the page that did the
// fetch.

import type { Metadata } from "next";
import { Suspense } from "react";
import { BrandBar } from "@/components/BrandBar";
import { brandId } from "@/lib/brand";
import "./globals.css";

export const metadata: Metadata = {
  title: "BrandPulse",
  description: "Crisis-before-it-trends social listening for Indian D2C brands.",
};

const NAV = ["Pulse", "Mentions", "Topics", "Alerts", "Replies", "Cost"];

// Pulse is this page. The other five name sections of the product that have no
// screen yet, so they are labels rather than links: an anchor to "#" is an
// invitation to click something that cannot happen.
const NO_SCREEN_YET = "Not built yet. Everything the dashboard has is on this screen.";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="topbar">
          <div className="topbar-inner">
            <div className="wordmark">
              <i className="wordmark-mark" aria-hidden="true" />
              Brand<span>Pulse</span>
            </div>
            <nav className="topbar-nav">
              {NAV.map((item, i) =>
                i === 0 ? (
                  <b key={item} aria-current="page">
                    {item}
                  </b>
                ) : (
                  <span key={item} className="nav-unbuilt" title={NO_SCREEN_YET}>
                    {item}
                  </span>
                ),
              )}
            </nav>
            <div className="spacer" />
            {/* BrandBar reads the selected brand from the URL, and a component
                that calls useSearchParams has to sit under a Suspense boundary
                or every route that renders this layout opts out of static
                prerendering. The fallback is empty because the bar carries no
                value worth showing before it knows which brand is on screen. */}
            <Suspense fallback={null}>
              <BrandBar defaultBrandId={brandId()} />
            </Suspense>
          </div>
        </header>
        <main className="shell">{children}</main>
      </body>
    </html>
  );
}
