// Speaks A2A to one agent and hands back the artifact body untouched.
//
// This is the only file in the BFF that knows how to call an agent. It does not
// know where any agent lives: a URL arrives from env.ts through server.ts into
// the thin client that names its agent, so adding a second agent is a five-line
// client rather than a second copy of this transport.
//
// It must not reshape the artifact. Unwrapping the JSON-RPC result, the Task and
// the Part is transport: those three layers are envelope, not data. Renaming a
// field inside the body, flattening it, rounding a float or summing a list is
// not transport, and none of it happens here.
//
// A2A v1.0 is protobuf-derived and its JSON differs from the 0.x shapes in
// three ways that each fail silently if you guess. Verified against
// a2aproject/A2A specification/a2a.proto and a2a-go v2.5.0 a2a/core.go:
//   - the method is "SendMessage", not "message/send"
//   - roles and states are enum names: "ROLE_USER", "TASK_STATE_COMPLETED"
//   - Part has no "kind" discriminator. It is a flattened oneof, so a data part
//     is {"data": {...}}, with no tag saying so.
// The protocol version travels as the A2A-Version HTTP header, not in the body.

const A2A_VERSION = "1.0";
const SEND_MESSAGE = "SendMessage";
const ROLE_USER = "ROLE_USER";
const TASK_STATE_FAILED = "TASK_STATE_FAILED";

/** How far down an error's cause chain the message is allowed to grow. */
const MAX_CAUSE_DEPTH = 4;

/**
 * Carries the HTTP status the route should return, because the reason the
 * agent call failed is the only thing that can decide between a timeout and a
 * bad reply, and the route has thrown that context away by the time it catches.
 */
export class AgentError extends Error {
  readonly httpStatus: number;

  constructor(message: string, httpStatus: number, options?: { cause?: unknown }) {
    super(message, options);
    this.name = "AgentError";
    this.httpStatus = httpStatus;
  }
}

interface JsonRpcPart {
  data?: unknown;
  mediaType?: string;
  text?: string;
}

interface JsonRpcArtifact {
  name?: string;
  parts?: JsonRpcPart[];
}

interface JsonRpcTask {
  id?: string;
  status?: { state?: string; message?: { parts?: JsonRpcPart[] } };
  artifacts?: JsonRpcArtifact[];
}

interface JsonRpcReply {
  error?: { code?: number; message?: string };
  result?: { task?: JsonRpcTask };
}

/**
 * Sends one input to one agent and returns the named artifact's data part.
 *
 * `agent` is only ever printed. It is what turns "the agent is unreachable"
 * into a sentence naming which of the nine is down, which is the whole
 * difference between a usable failure on the dashboard and a shrug.
 */
export async function callAgent<Artifact>(
  agent: string,
  baseUrl: string,
  input: unknown,
  expectedArtifact: string,
  timeoutMs: number,
): Promise<Artifact> {
  const task = await sendMessage(agent, baseUrl, input, timeoutMs);
  return artifactBody(agent, task, expectedArtifact) as Artifact;
}

async function sendMessage(
  agent: string,
  baseUrl: string,
  input: unknown,
  timeoutMs: number,
): Promise<JsonRpcTask> {
  const body = {
    jsonrpc: "2.0",
    id: 1,
    method: SEND_MESSAGE,
    params: {
      message: {
        messageId: crypto.randomUUID(),
        role: ROLE_USER,
        parts: [{ data: input, mediaType: "application/json" }],
      },
    },
  };

  let response: Response;
  try {
    response = await fetch(baseUrl, {
      method: "POST",
      headers: { "content-type": "application/json", "A2A-Version": A2A_VERSION },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(timeoutMs),
    });
  } catch (cause) {
    // AbortSignal.timeout rejects with a TimeoutError DOMException. Everything
    // else here is DNS, TLS or a refused connection.
    const timedOut = cause instanceof Error && cause.name === "TimeoutError";
    throw new AgentError(
      timedOut
        ? `${agent} did not answer within ${timeoutMs}ms`
        : `${agent} is unreachable: ${describe(cause)}`,
      timedOut ? 504 : 502,
      { cause },
    );
  }

  if (!response.ok) {
    throw new AgentError(`${agent} answered HTTP ${response.status}`, 502);
  }

  let reply: JsonRpcReply;
  try {
    reply = (await response.json()) as JsonRpcReply;
  } catch (cause) {
    throw new AgentError(`${agent} sent a body that is not JSON`, 502, { cause });
  }

  if (reply.error) {
    throw new AgentError(
      `${agent} returned JSON-RPC error ${reply.error.code ?? "?"}: ${reply.error.message ?? "no message"}`,
      502,
    );
  }

  const task = reply.result?.task;
  if (!task) {
    throw new AgentError(`${agent} returned no task in its result`, 502);
  }
  return task;
}

/**
 * Pulls the one JSON artifact out of a completed task.
 *
 * The name check is not ceremony. A wrong agent URL in the environment answers
 * this call perfectly well with somebody else's artifact, and forwarding that
 * would put another agent's numbers on the dashboard under our field names.
 */
function artifactBody(agent: string, task: JsonRpcTask, expectedName: string): unknown {
  if (task.status?.state === TASK_STATE_FAILED) {
    throw new AgentError(
      `${agent} failed the task: ${statusText(task) ?? "no reason given"}`,
      502,
    );
  }

  const artifacts = task.artifacts ?? [];
  if (artifacts.length === 0) {
    throw new AgentError(
      `${agent} returned no artifact, task state ${task.status?.state ?? "unknown"}`,
      502,
    );
  }

  const artifact = artifacts.find((candidate) => candidate.name === expectedName);
  if (!artifact) {
    const names = artifacts.map((candidate) => candidate.name ?? "unnamed").join(", ");
    throw new AgentError(`expected a ${expectedName} artifact, got: ${names}`, 502);
  }

  const part = (artifact.parts ?? []).find((candidate) => candidate.data !== undefined);
  if (!part) {
    throw new AgentError(`the ${expectedName} artifact carries no data part`, 502);
  }
  return part.data;
}

function statusText(task: JsonRpcTask): string | undefined {
  return (task.status?.message?.parts ?? [])
    .map((part) => part.text)
    .filter((text): text is string => typeof text === "string" && text !== "")
    .join(" ") || undefined;
}

function describe(cause: unknown): string {
  const chain: string[] = [];
  collectMessages(cause, chain);
  return chain.length > 0 ? chain.join(": ") : String(cause);
}

/**
 * Node's fetch rejects with a bare "fetch failed" and puts the only part worth
 * printing, the ECONNREFUSED and the port, in `error.cause`. A host that
 * resolves to both an IPv4 and an IPv6 address rejects with an AggregateError
 * instead, whose own message is empty and whose `errors` hold one per attempt.
 * Without walking both the dashboard is told "unreachable" and not which port.
 */
function collectMessages(error: unknown, into: string[]): void {
  if (into.length >= MAX_CAUSE_DEPTH || !(error instanceof Error)) return;
  if (error.message && !into.includes(error.message)) into.push(error.message);
  if (error instanceof AggregateError) {
    for (const attempt of error.errors) collectMessages(attempt, into);
    return;
  }
  collectMessages(error.cause, into);
}
