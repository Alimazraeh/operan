# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /arabic-language-core .

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget \
    && addgroup -g 1001 operan \
    && adduser -u 1001 -G operan -s /bin/sh -D operan
WORKDIR /app
COPY --from=builder /arabic-language-core /app/arabic-language-core
EXPOSE 8019
USER operan
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8019/health || exit 1
ENTRYPOINT ["/app/arabic-language-core"]