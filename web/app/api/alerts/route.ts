// The alert feed's poll target. It exists so the browser never needs the BFF's
// URL, which is injected when the backend is deployed and therefore cannot be
// inlined into a bundle at image build time.
//
// It forwards two query parameters and returns the BFF's array unchanged. It adds
// no field, drops none, and filters nothing: deciding which alerts matter is
// bp-detector's job.

import { fetchAlerts } from "@/lib/pulse";

/** Every request hits the BFF. There is nothing here worth caching. */
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const query = new URL(request.url).searchParams;
  const brandId = query.get("brand");
  if (!brandId) {
    return Response.json({ error: "brand is required" }, { status: 400 });
  }

  try {
    const answered = await fetchAlerts(brandId, query.get("since") ?? undefined);
    return Response.json(answered.data);
  } catch (cause) {
    // 502, not 500: the dashboard is up and the backend behind it is not, and the
    // feed says exactly that rather than emptying itself.
    const reason = cause instanceof Error ? cause.message : String(cause);
    return Response.json({ error: reason }, { status: 502 });
  }
}
