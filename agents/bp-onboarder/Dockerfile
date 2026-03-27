# The build context is the repo root, not this directory: the binary needs
# go.mod and the whole of internal/.
#
#   docker build -f agents/bp-onboarder/Dockerfile -t bp-onboarder .
#
# `nasiko build` runs a bare `docker build` from the agent directory, so it
# needs -f and a root context too. The Nasiko example's own Dockerfile does not
# build at all; this is not derived from it.
FROM golang:1.27 AS build

WORKDIR /src

# go.mod and go.sum first so the module download layer survives a code change.
COPY go.mod go.sum ./
RUN go mod download

# internal/ carries internal/prompts/onboarder.md, which is //go:embed'ed into
# the binary. A build that copies only .go files produces an agent whose prompt
# package fails to compile.
COPY internal/ ./internal/
COPY agents/bp-onboarder/ ./agents/bp-onboarder/

# CGO off, because the run stage is distroless static and has no libc.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/bp-onboarder ./agents/bp-onboarder

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# a2a.LoadCard reads a relative path, so the card has to sit beside the binary
# in the working directory.
COPY --from=build /out/bp-onboarder /app/bp-onboarder
COPY agents/bp-onboarder/AgentCard.json /app/AgentCard.json

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/app/bp-onboarder"]
