# Module 08 Tool Execution - Multi-stage Dockerfile
# Follows Operan platform conventions: multi-stage build, non-root user

# ─── Builder Stage ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Download dependencies first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with security flags
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /module08-tool-execution

# ─── Runtime Stage ───────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -g 1001 operan \
    && adduser -u 1001 -G operan -s /bin/sh -D operan

WORKDIR /app

COPY --from=builder /module08-tool-execution /app/module08-tool-execution

USER operan

EXPOSE 8008

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8008/health || exit 1

ENTRYPOINT ["/app/module08-tool-execution"]
