# syntax=docker/dockerfile:1

# --- Builder stage ---
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Install build deps
# hadolint ignore=DL3018
RUN apk add --no-cache git ca-certificates && update-ca-certificates

# Allow automatic toolchain download if needed
ENV GOTOOLCHAIN=auto

# Copy full source (including go.mod/go.sum)
COPY . .

# Build the agent (static binary)
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
  go build -trimpath -ldflags "-s -w" -o /out/agent ./cmd/agent

# --- Runtime stage ---
FROM alpine:3.20
# hadolint ignore=DL3018
RUN apk add --no-cache \
  ca-certificates \
  tzdata \
  docker-cli \
  docker-cli-compose \
  && update-ca-certificates

WORKDIR /app

# Create directory for persistent agent ID storage
RUN mkdir -p /var/lib/flotilla && chown -R root:root /var/lib/flotilla

COPY --from=builder /out/agent /app/agent

# Minimal environment defaults (can be overridden)
ENV LOG_LEVEL=info \
  LOG_FORMAT=json \
  SERVER_ADDRESS=localhost \
  SERVER_PORT=8080 \
  SERVER_USE_TLS=false \
  AGENT_NAME="" \
  DOCKER_SOCKET=/var/run/docker.sock \
  AGENT_HEARTBEAT_INTERVAL=30s \
  AGENT_RECONNECT_INTERVAL=5s \
  AGENT_MAX_RECONNECT_ATTEMPTS=10 \
  METRICS_ENABLED=true \
  METRICS_COLLECTION_INTERVAL=30s \
  METRICS_COLLECT_HOST_STATS=true \
  METRICS_COLLECT_NETWORK=true

ENTRYPOINT ["/app/agent"]
