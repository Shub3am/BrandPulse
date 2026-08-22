// The brand picker's list and the new-brand form's target. It exists for the
// same reason the run and alert routes do: the browser must never need the
// BFF's URL, which is injected when the backend is deployed and therefore
// cannot be inlined into a bundle at image build time.
//
// It forwards the four fields and returns the BFF's answer unchanged. It
// validates nothing beyond their presence, because `newBrandFrom` in
// bff/src/routes/brands.ts is the one place that decides what a brand may say.

import { createBrand, fetchBrands } from "@/lib/pulse";

/** Every request hits the BFF. There is nothing here worth caching. */
export const dynamic = "force-dynamic";

export async function GET(): Promise<Response> {
  try {
    const answered = await fetchBrands();
    return Response.json(answered.data);
  } catch (cause) {
    return Response.json({ error: describe(cause) }, { status: 502 });
  }
}

export async function POST(request: Request): Promise<Response> {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "body must be JSON" }, { status: 400 });
  }

  const fields = (body ?? {}) as {
    name?: unknown;
    website?: unknown;
    keywords?: unknown;
    competitors?: unknown;
  };
  if (typeof fields.name !== "string" || fields.name === "") {
    return Response.json({ error: "name is required" }, { status: 400 });
  }
  if (typeof fields.website !== "string" || fields.website === "") {
    return Response.json({ error: "website is required" }, { status: 400 });
  }

  try {
    const answered = await createBrand({
      name: fields.name,
      website: fields.website,
      keywords: stringList(fields.keywords),
      competitors: stringList(fields.competitors),
    });
    return Response.json(answered.data, { status: 201 });
  } catch (cause) {
    // 502, not 500: the dashboard is up and the backend behind it is not, and
    // the form says exactly that rather than going quiet. A 400 from the BFF
    // arrives here as a thrown message carrying its own reason, which is the
    // sentence the form prints.
    return Response.json({ error: describe(cause) }, { status: 502 });
  }
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === "string") : [];
}

function describe(cause: unknown): string {
  return cause instanceof Error ? cause.message : String(cause);
}
