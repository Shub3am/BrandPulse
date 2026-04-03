# Build context is the repo root, not this directory:
#   docker build -f agents/bp-clusterer/Dockerfile -t bp-clusterer .
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
COPY agents/bp-clusterer/ ./agents/bp-clusterer/

# CGO off so the binary runs on a scratch base. -trimpath keeps build paths out
# of the image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/bp-clusterer ./agents/bp-clusterer

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bp-clusterer /bp-clusterer

# Nasiko injects PORT at runtime. 8000 is the local default and matches the
# platform's own examples.
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/bp-clusterer"]
