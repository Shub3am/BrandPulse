// Writes a brand and its confirmed profile. The BFF's only write to Postgres.
//
// db.ts opens its pool with default_transaction_read_only=on so that an INSERT
// added to a read route fails at the Postgres server rather than in review.
// This file does not lift that guard, it opens its own pool beside it, so the
// read routes stay unable to write and the one path that can is a named module
// a reviewer finds by its filename.
//
// It must not write anything else. Mentions, runs, alerts, topics, briefs and
// drafts are the agents' rows, and a second author of those is a number on the
// dashboard that no agent can be held to. Two tables, one statement each.

import { Pool } from "pg";
import type { BrandProfile } from "./contracts.js";

/** What a route is allowed to depend on. The test fake implements this too. */
export interface Registry {
  createBrand(profile: BrandProfile): Promise<void>;
  close(): Promise<void>;
}

export function createRegistry(databaseUrl: string, timeoutMs: number): Registry {
  const pool = new Pool({
    connectionString: databaseUrl,
    // One brand at a time. This pool exists for a form submission, not for the
    // dashboard's first paint, so it does not need db.ts's eight connections.
    max: 2,
    connectionTimeoutMillis: timeoutMs,
    statement_timeout: timeoutMs,
  });

  pool.on("error", (cause) => {
    console.error("bff: postgres dropped an idle registry connection:", cause.message);
  });

  return {
    async createBrand(profile) {
      const client = await pool.connect();
      try {
        // Both rows or neither. A brand with no profile is a brand the
        // orchestrator refuses to run, and it is invisible on the dashboard,
        // so it would read as the form having done nothing.
        await client.query("BEGIN");
        await client.query(
          `INSERT INTO brands (id, name, website) VALUES ($1, $2, $3)`,
          [profile.brand_id, profile.name, profile.website ?? null],
        );
        // confirmed_at is now() because submitting the form is the confirmation
        // CONTRACTS §2 describes: bp-onboarder returns an unconfirmed draft and
        // a human agrees to it. The orchestrator loads only confirmed profiles,
        // so a NULL here is a brand that can never run.
        await client.query(
          `INSERT INTO brand_profiles (
             brand_id, version, keywords, hashtags, products, competitors,
             sources, negative_keywords, source_handles, voice, confirmed_at
           ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())`,
          [
            profile.brand_id,
            profile.version,
            // Every list below is stringified rather than passed as an array.
            // node-postgres encodes a JS array as a Postgres array literal,
            // which is the wrong type for a jsonb column and fails at insert.
            ...[
              profile.keywords,
              profile.hashtags,
              profile.products,
              profile.competitors,
              profile.sources,
              profile.negative_keywords,
              profile.source_handles,
              profile.voice,
            ].map((value) => JSON.stringify(value)),
          ],
        );
        await client.query("COMMIT");
      } catch (cause) {
        await client.query("ROLLBACK");
        throw cause;
      } finally {
        client.release();
      }
    },

    async close() {
      await pool.end();
    },
  };
}
