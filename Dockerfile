# ---- Frontend build stage ----
FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- Go build stage ----
FROM golang:1.25-alpine AS go-build
WORKDIR /app
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-build /web/dist ./web/dist
RUN go build -o bin/server ./cmd/server
RUN go build -o bin/healthcheck ./cmd/healthcheck

# ---- Runtime stage ----
FROM gcr.io/distroless/base-debian13:nonroot

# FROM debian:trixie
WORKDIR /app

COPY --from=go-build /app/bin/server /app/server
COPY --from=go-build /app/web/dist /app/web/dist
COPY --from=go-build /app/bin/healthcheck /app/healthcheck


ENV SERVER_PORT=8081
EXPOSE 8081

USER 65532:65532

ENTRYPOINT ["/app/server"]
