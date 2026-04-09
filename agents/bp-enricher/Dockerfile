# Build context is the repo root, not this directory:
#   docker build -f agents/bp-enricher/Dockerfile -t bp-enricher .
# Every agent needs internal/, so a per-agent context cannot see its own
# dependencies. docs/research/nasiko.md §4 shows the Python form of this and is
# stale on the base image; the build-context rule it states still holds.

FROM golang:1.27-alpine AS build
WORKDIR /src

# go.* rather than an explicit go.sum: the module has no third-party
# dependencies until internal/a2a lands, and COPY fails on a glob that matches
# nothing.
COPY go.* ./
RUN go mod download

COPY internal/ ./internal/
COPY agents/bp-enricher/ ./agents/bp-enricher/

# CGO off so the binary runs on a scratch base. -trimpath keeps build paths out
# of the image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/bp-enricher ./agents/bp-enricher

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bp-enricher /bp-enricher

# a2a.LoadCard reads the card at startup, so it has to be in the run stage too.
# Image root rather than beside the binary's source path, because distroless
# has no shell to set a working directory from and main.go hardcodes /.
COPY --from=build /src/agents/bp-enricher/AgentCard.json /AgentCard.json

# Nasiko injects PORT at runtime. 8000 is the local default and matches the
# platform's own examples.
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/bp-enricher"]
