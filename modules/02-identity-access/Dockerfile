# Build stage
FROM golang:1.21-alpine AS builder
RUN apk --no-cache add git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /identity-access ./cmd/identity-access

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /identity-access /app/identity-access
EXPOSE 8002
USER nobody
ENTRYPOINT ["/app/identity-access"]
