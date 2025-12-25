# syntax=docker/dockerfile:1

# The BFF image.
#
# Build context is bff/, not the repo root: nothing outside this directory is
# needed to compile it, and the smallest context is the fewest files that can
# accidentally end up in a layer. web/Dockerfile is the opposite case and says so.
#
# Nothing is baked in. ORCHESTRATOR_URL, DATABASE_URL and DASHBOARD_ORIGIN are
# read at startup by src/env.ts and injected at deploy time. A committed agent URL
# would have to be rebuilt to move, and a committed DATABASE_URL would be a
# credential in an image registry.
#
#   docker build -t brandpulse-bff bff/

FROM node:26-alpine AS build
WORKDIR /app

# Lockfile first, so a source edit does not reinstall the dependency tree.
COPY package.json package-lock.json ./
RUN npm ci

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# tsconfig.json compiles the test files on purpose, because tsx strips types
# without checking them and excluding the tests would leave them untypechecked.
# They must not reach the runtime image: they import node:test and they are not
# the server.
RUN find dist -name '*.test.*' -delete

FROM node:26-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production

# A second, production-only install rather than copying node_modules from the
# builder: tsx and typescript are ~60MB that a running BFF never loads.
COPY package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force

COPY --from=build /app/dist ./dist

# Not root. This process reads Postgres and posts to one agent URL; it writes
# nothing to its own filesystem.
USER node

EXPOSE 8080
CMD ["node", "dist/server.js"]
