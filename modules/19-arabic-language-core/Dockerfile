# Build stage
FROM golang:1.21-bookworm AS builder
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./ 2>/dev/null || true
RUN go mod download 2>/dev/null || true

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /arabic-language-core .

# Runtime stage
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /arabic-language-core .

# Run as non-root
RUN useradd -r -s /bin/false arabic-language-core
USER arabic-language-core

EXPOSE 8019

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8019/health"]

CMD ["/arabic-language-core"]