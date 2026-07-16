FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /execution-sandbox .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd -r sandbox && useradd -r -g sandbox -d /tmp/operan-sandbox sandbox
RUN mkdir -p /tmp/operan-sandbox && chown sandbox:sandbox /tmp/operan-sandbox

WORKDIR /app
COPY --from=builder /execution-sandbox /execution-sandbox
USER sandbox
EXPOSE 8016
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8016/health || exit 1
CMD ["/execution-sandbox"]