# Built with the repo root as context, not this directory, so internal/ and
# go.mod are in scope:
#   docker build -f agents/bp-responder/Dockerfile -t bp-responder .
#
# Two stages on purpose. CGO_ENABLED=0 makes a static binary, which is the only
# thing a distroless static image can run. The Nasiko currency-agent example
# Dockerfile does not build at all; do not start from it.

FROM golang:1.27 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY agents/bp-responder/ ./agents/bp-responder/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/bp-responder ./agents/bp-responder

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/bp-responder /bp-responder
COPY agents/bp-responder/AgentCard.json /AgentCard.json

# a2a.Serve reads PORT. 8080 matches the url in AgentCard.json.
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/bp-responder"]
