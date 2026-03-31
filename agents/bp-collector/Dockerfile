# The build context is the repo root, not this directory: the binary needs
# go.mod and the whole of internal/.
#
#   docker build -f agents/bp-collector/Dockerfile -t bp-collector .
#
# `nasiko build` runs a bare `docker build` from the agent directory, so it
# needs -f and a root context too. The Nasiko example's own Dockerfile does not
# build at all; this is not derived from it.
FROM golang:1.27 AS build

WORKDIR /src

# go.mod and go.sum first so the module download layer survives a code change.
COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY agents/bp-collector/ ./agents/bp-collector/

# CGO off, because the run stage is distroless static and has no libc.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/bp-collector ./agents/bp-collector

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# a2a.LoadCard reads a relative path, so the card has to sit beside the binary
# in the working directory.
COPY --from=build /out/bp-collector /app/bp-collector
COPY agents/bp-collector/AgentCard.json /app/AgentCard.json

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/app/bp-collector"]
