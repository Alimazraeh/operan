FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /policy-governance .

FROM alpine:3.20

RUN addgroup -g 65534 operan && adduser -D -u 65534 -G operan operan
USER operan

WORKDIR /app
COPY --from=builder /policy-governance /app/policy-governance

EXPOSE 8010
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8010/health || exit 1

CMD ["/app/policy-governance"]