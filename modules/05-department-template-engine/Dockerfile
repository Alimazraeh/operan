# Module 05 Department Template Engine - Multi-stage Dockerfile
# Follows Operan platform conventions: multi-stage build, non-root user

# ─── Builder Stage ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Install git (required for go mod download with VCS)
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Download dependencies first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with security flags
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /module05-department-template-engine

# ─── Runtime Stage ───────────────────────────────────────────────────────────
FROM alpine:3.19

# Install ca-certificates for HTTPS and run as non-root user
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1001 operan \
    && adduser -u 1001 -G operan -s /bin/sh -D operan

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /module05-department-template-engine /app/module05-department-template-engine

# Run as non-root user
USER operan

# Expose port
EXPOSE 8005

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8005/health || exit 1

# Start the service
ENTRYPOINT ["/app/module05-department-template-engine"]
