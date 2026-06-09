# Operan Sovereign Deployment Fabric — Service Image
# Module 20

FROM golang:1.22-alpine AS builder

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
    -o /deployments-service ./cmd/deployments-service

# Runtime image
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /deployments-service /deployments-service

USER 65534:65534

EXPOSE 8080
EXPOSE 9090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/deployments-service", "healthcheck"]

ENTRYPOINT ["/deployments-service"]
