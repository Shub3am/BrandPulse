# Built with the repo root as context, not this directory, so internal/ and
# go.mod are in scope:
#   docker build -f agents/bp-detector/Dockerfile -t bp-detector .
#
# Two stages on purpose. CGO_ENABLED=0 makes a static binary, which is the only
# thing a distroless static image can run. The Nasiko currency-agent example
# Dockerfile does not build at all; do not start from it.

FROM golang:1.27 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY agents/bp-detector/ ./agents/bp-detector/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/bp-detector ./agents/bp-detector

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/bp-detector /bp-detector
COPY agents/bp-detector/AgentCard.json /AgentCard.json

# a2a.Serve reads PORT and falls back to 8000, which is what the url in
# AgentCard.json says. Setting PORT here would be a second place for it to drift.
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/bp-detector"]
