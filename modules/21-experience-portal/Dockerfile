# Module 21 Experience Portal - Multi-stage Dockerfile
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /experience-portal .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget \
    && addgroup -g 1001 operan \
    && adduser -u 1001 -G operan -s /bin/sh -D operan
COPY --from=builder /experience-portal /experience-portal
USER operan
EXPOSE 8021
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8021/healthz || exit 1
ENTRYPOINT ["/experience-portal"]
