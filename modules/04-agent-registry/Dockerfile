# Build stage
FROM golang:1.22-alpine AS builder
RUN apk --no-cache add git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agent-registry .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /agent-registry /app/agent-registry
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/agent-registry"]
