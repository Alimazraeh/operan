# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tenant-control-plane ./cmd/tenant-control-plane

# Runtime stage
FROM scratch
WORKDIR /app
COPY --from=builder /tenant-control-plane /app/tenant-control-plane
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/tenant-control-plane"]
