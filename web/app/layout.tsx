// The document shell and the top bar. Holds no product state.
//
// The "demo data" pill used to live here, where it was a hardcoded warning that
// could not be wrong in the reassuring direction and could not be right in the
// other. A layout receives no data, so it cannot know what the data is. The pill
// is now components/DataSourceBadge.tsx, rendered from the page that did the
// fetch.

import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "BrandPulse",
  description: "Crisis-before-it-trends social listening for Indian D2C brands.",
};

const NAV = ["Pulse", "Mentions", "Topics", "Alerts", "Replies", "Cost"];

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="topbar">
          <div className="topbar-inner">
            <div className="wordmark">
              Brand<span>Pulse</span>
            </div>
            <nav className="topbar-nav">
              {NAV.map((item, i) => (
                <a key={item} href="#">
                  {i === 0 ? <b>{item}</b> : item}
                </a>
              ))}
            </nav>
            <div className="spacer" />
            <button className="btn btn-primary">Run now</button>
          </div>
        </header>
        <main className="shell">{children}</main>
      </body>
    </html>
  );
}
