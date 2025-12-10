// Proves the A2A call against a stub agent: the request shape going out, and the
// artifact coming back unchanged.
//
// The stub is a real HTTP server rather than a mocked fetch, because the three
// things most likely to be wrong here are the method name, the role enum and the
// absence of a Part discriminator, and a mocked fetch would happily agree with
// whatever this file believes. It spends no credits and needs no network.

import assert from "node:assert/strict";
import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { after, test } from "node:test";
import type { RunRecord } from "./contracts.js";
import { OrchestratorError, createOrchestrator } from "./orchestrator.js";

const PARTIAL_RUN: RunRecord = {
  id: "run_01JFZ3P7QK8N2V5X9YB4C6D8EG",
  brand_id: "lumeo",
  kind: "on_demand",
  time_bucket: "2026-09-20T14",
  started_at: "2026-09-20T14:02:11Z",
  finished_at: "2026-09-20T14:03:58Z",
  status: "partial",
  credits_used: 37,
  tokens_used: 18422,
  cost_paise: 214.5,
  sources_attempted: ["x", "reddit", "youtube", "news", "playstore", "appstore"],
  sources_skipped: ["appstore"],
  degraded_reason: "Nasiko flow guard capped fan-out at 6 concurrent collectors",
  mentions_collected: 412,
  errors: ["appstore: probe returned 403"],
};

const servers: Server[] = [];

after(async () => {
  await Promise.all(
    servers.map(
      (server) =>
        new Promise<void>((resolve) => {
          server.closeAllConnections();
          server.close(() => resolve());
        }),
    ),
  );
});

async function stubAgent(
  handler: (request: IncomingMessage, response: ServerResponse, body: string) => void,
): Promise<string> {
  const server = createServer((request, response) => {
    let body = "";
    request.on("data", (chunk) => (body += chunk));
    request.on("end", () => handler(request, response, body));
  });
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.ok(address && typeof address === "object");
  return `http://127.0.0.1:${address.port}`;
}

function completedTask(artifactName: string, data: unknown): string {
  return JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    result: {
      task: {
        id: "task_1",
        contextId: "ctx_1",
        status: { state: "TASK_STATE_COMPLETED" },
        artifacts: [
          {
            artifactId: "artifact_1",
            name: artifactName,
            parts: [{ data, mediaType: "application/json" }],
          },
        ],
      },
    },
  });
}

function reply(response: ServerResponse, status: number, body: string): void {
  response.writeHead(status, { "content-type": "application/json" });
  response.end(body);
}

test("sends A2A v1.0 SendMessage with the RunInput as a data part", async () => {
  let seenMethod: unknown;
  let seenRole: unknown;
  let seenParts: unknown;
  let seenVersionHeader: unknown;
  let seenPath = "";

  const url = await stubAgent((request, response, body) => {
    const parsed = JSON.parse(body);
    seenMethod = parsed.method;
    seenRole = parsed.params.message.role;
    seenParts = parsed.params.message.parts;
    seenVersionHeader = request.headers["a2a-version"];
    seenPath = request.url ?? "";
    reply(response, 200, completedTask("RunRecord", PARTIAL_RUN));
  });

  await createOrchestrator(url, 5_000).startRun({ brand_id: "lumeo", trigger: "on_demand" });

  assert.equal(seenMethod, "SendMessage", "A2A v1.0 names the method SendMessage");
  assert.equal(seenRole, "ROLE_USER", "roles are enum names in v1.0, not 'user'");
  assert.equal(seenVersionHeader, "1.0", "the version travels as a header, not in the body");
  assert.equal(seenPath, "/");
  // One part, carrying data, with no "kind" discriminator: Part is a flattened
  // oneof in v1.0 and adding a kind field makes the agent reject the message.
  assert.deepEqual(seenParts, [
    { data: { brand_id: "lumeo", trigger: "on_demand" }, mediaType: "application/json" },
  ]);
});

test("omits window_hours and force when the caller did not send them", async () => {
  let seenData: unknown;
  const url = await stubAgent((_request, response, body) => {
    seenData = JSON.parse(body).params.message.parts[0].data;
    reply(response, 200, completedTask("RunRecord", PARTIAL_RUN));
  });

  await createOrchestrator(url, 5_000).startRun({ brand_id: "lumeo", trigger: "on_demand" });

  assert.deepEqual(Object.keys(seenData as object), ["brand_id", "trigger"]);
});

test("forwards window_hours and force when the caller did send them", async () => {
  let seenData: unknown;
  const url = await stubAgent((_request, response, body) => {
    seenData = JSON.parse(body).params.message.parts[0].data;
    reply(response, 200, completedTask("RunRecord", PARTIAL_RUN));
  });

  await createOrchestrator(url, 5_000).startRun({
    brand_id: "lumeo",
    trigger: "crisis_replay",
    window_hours: 6,
    force: true,
  });

  assert.deepEqual(seenData, {
    brand_id: "lumeo",
    trigger: "crisis_replay",
    window_hours: 6,
    force: true,
  });
});

test("returns the RunRecord artifact unchanged, field for field", async () => {
  const url = await stubAgent((_request, response) => {
    reply(response, 200, completedTask("RunRecord", PARTIAL_RUN));
  });

  const run = await createOrchestrator(url, 5_000).startRun({
    brand_id: "lumeo",
    trigger: "on_demand",
  });

  // deepEqual, not a field-by-field check: this is the assertion that no key was
  // renamed, no float rounded and no list summed on the way through.
  assert.deepEqual(run, PARTIAL_RUN);
  assert.deepEqual(Object.keys(run), Object.keys(PARTIAL_RUN));
});

test("a status of partial with a populated errors array is not an error", async () => {
  const url = await stubAgent((_request, response) => {
    reply(response, 200, completedTask("RunRecord", PARTIAL_RUN));
  });

  const run = await createOrchestrator(url, 5_000).startRun({
    brand_id: "lumeo",
    trigger: "on_demand",
  });

  assert.equal(run.status, "partial");
  assert.deepEqual(run.errors, ["appstore: probe returned 403"]);
  assert.deepEqual(run.sources_skipped, ["appstore"]);
});

test("a failed task becomes a 502 carrying the agent's reason", async () => {
  const url = await stubAgent((_request, response) => {
    reply(
      response,
      200,
      JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        result: {
          task: {
            id: "task_1",
            status: {
              state: "TASK_STATE_FAILED",
              message: { parts: [{ text: "anakin returned 429 on every source" }] },
            },
            artifacts: [],
          },
        },
      }),
    );
  });

  await assert.rejects(
    createOrchestrator(url, 5_000).startRun({ brand_id: "lumeo", trigger: "on_demand" }),
    (error: unknown) => {
      assert.ok(error instanceof OrchestratorError);
      assert.equal(error.httpStatus, 502);
      assert.match(error.message, /anakin returned 429 on every source/);
      return true;
    },
  );
});

test("a JSON-RPC error becomes a 502 and never a silent empty run", async () => {
  const url = await stubAgent((_request, response) => {
    reply(
      response,
      200,
      JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        error: { code: -32601, message: "method not found" },
      }),
    );
  });

  await assert.rejects(
    createOrchestrator(url, 5_000).startRun({ brand_id: "lumeo", trigger: "on_demand" }),
    (error: unknown) => {
      assert.ok(error instanceof OrchestratorError);
      assert.equal(error.httpStatus, 502);
      assert.match(error.message, /-32601/);
      return true;
    },
  );
});

test("an artifact under the wrong name is refused rather than forwarded", async () => {
  const url = await stubAgent((_request, response) => {
    reply(response, 200, completedTask("MentionBatch", { mentions: [] }));
  });

  await assert.rejects(
    createOrchestrator(url, 5_000).startRun({ brand_id: "lumeo", trigger: "on_demand" }),
    (error: unknown) => {
      assert.ok(error instanceof OrchestratorError);
      assert.match(error.message, /expected a RunRecord artifact, got: MentionBatch/);
      return true;
    },
  );
});

test("an agent that never answers becomes a 504, not a hung request", async () => {
  const url = await stubAgent(() => {
    // Deliberately never responds.
  });

  await assert.rejects(
    createOrchestrator(url, 150).startRun({ brand_id: "lumeo", trigger: "on_demand" }),
    (error: unknown) => {
      assert.ok(error instanceof OrchestratorError);
      assert.equal(error.httpStatus, 504);
      assert.match(error.message, /did not answer within 150ms/);
      return true;
    },
  );
});
