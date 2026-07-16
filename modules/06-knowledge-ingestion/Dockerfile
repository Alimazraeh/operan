# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app

RUN apk --no-cache add git ca-certificates

COPY go.mod go.sum ./
RUN go mod download 2>/dev/null || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /knowledge-ingestion .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /knowledge-ingestion .

RUN adduser -D -u 1000 knowledge-ingestion
USER knowledge-ingestion

EXPOSE 8006

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8006/health || exit 1

CMD ["/knowledge-ingestion"]