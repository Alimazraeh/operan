# Module 15 — Agent Marketplace Handoff

## Overview

Agent Marketplace provides a centralized procurement ecosystem where organizations browse, vet, and deploy agents and department templates. It handles subscriptions, licensing, approval workflows, and one-click deployment to M04 (Agent Registry) and M03 (Orchestration).

## Quick Start

```bash
cd modules/15-agent-marketplace
export JWT_SECRET="your-secret"
export DB_DSN="postgresql://localhost:5432/operan_marketplace?sslmode=disable"
export EVENT_BROKER_URL="kafka://localhost:9092"
export M04_BASE_URL="http://localhost:8004"
export M03_BASE_URL="http://localhost:8003"

go build && go run main.go
```

## Running Tests

```bash
go test ./... -v
go test ./... -cover
```

## Database Migration

```bash
psql $DB_DSN -f migrations/001_create_schema.sql
```

## Docker

```bash
docker build -t operan/agent-marketplace:latest .
docker run -p 8015:8015 -e JWT_SECRET=your-secret -e DB_DSN="postgresql://..." operan/agent-marketplace:latest
```

## Helm

```bash
helm install operan-m15 ./chart \
  --set env.JWT_SECRET=your-secret \
  --set env.DB_DSN="postgresql://..." \
  --set env.M04_BASE_URL=http://agent-registry:8004
```

## API Endpoints

See `contracts/openapi-15-agent-marketplace.yaml` for full API spec.

## Project Structure

```
modules/15-agent-marketplace/
├── go.mod
├── main.go
├── Dockerfile
├── chart/
├── contracts/
├── migrations/
├── README.md
└── internal/
    ├── config/
    ├── ctxkeys/
    ├── middleware/
    ├── handler/
    ├── store/
    ├── deploy/
    ├── events/
    └── clients/
```