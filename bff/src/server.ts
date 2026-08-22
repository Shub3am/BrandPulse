// Starts the BFF: reads the environment, builds the real collaborators, listens.
//
// This is the only file that turns configuration into connections, which is why
// it is also the only file that has to shut them down. A container that is
// killed mid-request should drain it and close the pool rather than drop both.

import { buildApp } from "./app.js";
import { createHistory } from "./db.js";
import { loadConfig } from "./env.js";
import { createOnboarder } from "./onboarder.js";
import { createOrchestrator } from "./orchestrator.js";
import { createRegistry } from "./registry.js";

const config = loadConfig();
const history = createHistory(config.databaseUrl, config.databaseTimeoutMs);
const registry = createRegistry(config.databaseUrl, config.databaseTimeoutMs);
const orchestrator = createOrchestrator(config.orchestratorUrl, config.orchestratorTimeoutMs);
const onboarder = createOnboarder(config.onboarderUrl, config.onboarderTimeoutMs);
const app = buildApp({
  history,
  registry,
  orchestrator,
  onboarder,
  dashboardOrigin: config.dashboardOrigin,
});

for (const signal of ["SIGTERM", "SIGINT"] as const) {
  process.once(signal, () => {
    app.log.info({ signal }, "shutting down");
    void app
      .close()
      .then(() => Promise.all([history.close(), registry.close()]))
      .then(() => process.exit(0));
  });
}

try {
  await app.listen({ port: config.port, host: config.host });
} catch (error) {
  app.log.error({ err: error }, "failed to listen");
  process.exit(1);
}
