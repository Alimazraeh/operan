# Operan Agent Orchestration Engine — Service Image
# Module 03

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with version metadata
ARG VERSION=dev
ARG GIT_COMMIT=$(git rev-parse --short HEAD)
ARG BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.version=${VERSION} -X main.commit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /orchestration-engine ./cmd/orchestration-engine

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget \
    && addgroup -g 1001 operan \
    && adduser -u 1001 -G operan -s /bin/sh -D operan

COPY --from=builder /orchestration-engine /orchestration-engine

USER operan

EXPOSE 8080
EXPOSE 9090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/orchestration-engine"]
