# syntax=docker/dockerfile:1

# The dashboard image.
#
# Build context is the REPO ROOT, not web/. That is the opposite of
# bff/Dockerfile and it is deliberate: `npm run build` runs
# scripts/checkTypesParity.mjs first, and that script reads
# internal/models/models.go and bff/src/contracts.ts to fail the build when a
# type mirror has drifted from the Go wire format. With a web/-only context those
# two files are absent, and an image that builds with its only drift check
# skipped is worse than one with no drift check at all.
#
#   docker build -f web/Dockerfile -t brandpulse-web .
#
# Nothing about the backend is baked in. BFF_BASE_URL has no NEXT_PUBLIC_ prefix
# on purpose, so it is read per request by lib/pulse.ts on the server and the
# same image can be pointed at a different BFF without a rebuild. A
# NEXT_PUBLIC_ variable would be inlined into the browser bundle here, at build
# time, which would both pin the URL to the image and publish it to every
# visitor. There is no DATABASE_URL and no agent URL in this image at all: the
# dashboard has never been able to reach either.

FROM node:26-alpine AS build
WORKDIR /app/web

# Lockfile first, so editing a component does not reinstall the dependency tree.
COPY web/package.json web/package-lock.json ./
RUN npm ci

# The two files outside web/ that check:types reads. It resolves them from
# import.meta.url as ../../ of web/scripts/, so they have to sit at these exact
# paths relative to WORKDIR, not anywhere convenient.
COPY internal/models/models.go /app/internal/models/models.go
COPY bff/src/contracts.ts /app/bff/src/contracts.ts

COPY web/tsconfig.json web/next.config.mjs web/next-env.d.ts ./
COPY web/scripts ./scripts
COPY web/app ./app
COPY web/components ./components
COPY web/lib ./lib

RUN npm run build

FROM node:26-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production

# standalone's server.js takes no arguments and reads both of these. Without
# HOSTNAME it binds localhost inside the container, where nothing can reach it.
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# Two copies, because `standalone` traces server code only: .next/static is the
# CSS and client JS the browser fetches and it is not part of that trace, so a
# one-line image serves an unstyled page. There is no web/public directory in
# this project, so there is nothing to copy for it; add a third COPY the day one
# appears.
#
# chowned to node because Next writes its fetch cache under .next at runtime.
COPY --from=build --chown=node:node /app/web/.next/standalone ./
COPY --from=build --chown=node:node /app/web/.next/static ./.next/static

USER node

EXPOSE 3000
CMD ["node", "server.js"]
