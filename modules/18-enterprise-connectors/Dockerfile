# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates git

# Copy go module files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/enterprise-connectors .

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/enterprise-connectors /app/enterprise-connectors

# Run as non-root user
RUN adduser -D -u 1000 operan
USER 1000

EXPOSE 8018

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8018/health || exit 1

ENTRYPOINT ["/app/enterprise-connectors"]