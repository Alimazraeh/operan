# Module 15 — Agent Marketplace

The Agent Marketplace is a centralized procurement ecosystem where organizations browse, vet, and deploy agents and department templates. It handles subscriptions, licensing, approval workflows, and one-click deployment to M04 (Agent Registry) and M03 (Orchestration).

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   M21 Portal     │────▶│   M15 Marketplace │────▶│    M04 Registry │
│  (Discovery UI)  │◀────│  (port 8015)     │◀────│ (Agent Registry) │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                    │                     │
                                    ▼                     ▼
                            ┌──────────────┐     ┌─────────────────┐
                            │    M03        │     │    M12 Model     │
                            │Orchestration  │     │  Abstraction     │
                            └──────────────┘     └─────────────────┘
                                    │
                                    ▼
                            ┌──────────────┐
                            │   Kafka      │
                            │  Events      │
                            └──────────────┘
```

## Marketplace Flow

```
1. Vendor creates listing → draft
2. Platform admin approves → approved
3. Tenant browses/searches listings
4. Tenant subscribes (free=active, paid=14-day trial)
5. Tenant deploys with one-click
6. M15 creates agents in M04 + workflows in M03
7. Tenant reviews the listing
8. Kafka events propagate deployment status
```

## Quick Start

```bash
# Build
cd modules/15-agent-marketplace
go build ./...

# Run tests
go test ./...

# Run with environment variables
export JWT_SECRET="your-secret"
export DB_DSN="postgresql://localhost:5432/operan_marketplace?sslmode=disable"
export EVENT_BROKER_URL="kafka://localhost:9092"
export M04_BASE_URL="http://localhost:8004"
export M03_BASE_URL="http://localhost:8003"

go run main.go
```

## Docker

```bash
docker build -t operan/agent-marketplace:latest .
docker run -p 8015:8015 \
  -e JWT_SECRET=your-secret \
  -e DB_DSN="postgresql://localhost:5432/operan_marketplace" \
  operan/agent-marketplace:latest
```

## Helm

```bash
helm install operan-m15 ./chart \
  --set env.JWT_SECRET=your-secret \
  --set env.DB_DSN="postgresql://..." \
  --set env.M04_BASE_URL=http://agent-registry:8004
```

## API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | /health | — | Health check (no auth) |
| GET | /v1/listings | ListListings | Search & browse marketplace |
| GET | /v1/listings/{id} | GetListing | Listing detail |
| POST | /v1/listings/{id}/deploy | Deploy | Deploy to tenant environment |
| POST | /v1/subscriptions | Subscribe | Subscribe to a listing |
| GET | /v1/subscriptions | ListSubscriptions | My subscriptions |
| POST | /v1/reviews | CreateReview | Rate a listing |
| GET | /v1/reviews | ListReviews | Reviews for a listing |

All authenticated routes require `Bearer JWT` + `X-Tenant-ID` headers.

## Search Filters

GET /v1/listings supports these query parameters:

- `page`, `page_size` — pagination
- `category` — agent, template, tool, integration, skill
- `listing_type` — vetted, user_generated
- `capability` — filter by capability name
- `language` — filter by supported language
- `requires_subscription` — true/false filter
- `subscription_tier` — free, trial, basic, pro, enterprise
- `price_min`, `price_max` — price range
- `rating_min` — minimum rating threshold
- `search` — full-text search on name + description

## Kafka Events

| Topic | Trigger |
|-------|---------|
| `operan.marketplace.listing_created` | New listing published |
| `operan.marketplace.listing_approved` | Listing approved by admin |
| `operan.marketplace.subscription_created` | Tenant subscribes |
| `operan.marketplace.subscription_expired` | Subscription expires |
| `operan.marketplace.deployed` | Deployment succeeds |
| `operan.marketplace.deploy_failed` | Deployment fails |
| `operan.marketplace.review_created` | Review submitted |

## Database

Run migrations:
```bash
psql $DB_DSN -f migrations/001_create_schema.sql
```

## Integration Points

| Module | Integration |
|--------|-------------|
| M04 Agent Registry | M15 calls M04 `/v1/agents` to register agents on deploy |
| M03 Orchestration | M15 calls M03 `/v1/workflows` to create workflows |
| M02 IAM | M04 validates tenant context when registering agents |
| M12 Model Abstraction | M15 uses M12 to resolve model references |
| M19 Arabic Language Core | Arabic terminology for Arabic-language listings |
| M21 Experience Portal | M21 calls M15 `/v1/listings` for marketplace UI |

## Project Structure

```
modules/15-agent-marketplace/
├── go.mod
├── main.go                          # Listen port 8015
├── Dockerfile
├── chart/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml
│       └── service.yaml
├── contracts/
│   └── openapi-15-agent-marketplace.yaml
├── migrations/
│   └── 001_create_schema.sql
├── README.md
└── internal/
    ├── config/
    │   └── config.go
    ├── ctxkeys/
    │   └── ctxkeys.go
    ├── middleware/
    │   └── middleware.go            # JWT + tenant isolation
    ├── handler/
    │   ├── listings.go              # Browse/search/listing CRUD
    │   ├── subscriptions.go         # Subscribe/unsubscribe/status
    │   ├── reviews.go               # Rate/review listings
    │   ├── router.go                # HTTP router setup
    │   └── shared.go                # JSON helpers
    ├── store/
    │   ├── listings.go              # PostgreSQL listings queries
    │   ├── subscriptions.go         # PostgreSQL subscriptions queries
    │   ├── reviews.go               # PostgreSQL reviews queries
    │   └── models.go                # Go structs
    ├── deploy/
    │   ├── deployer.go              # Create agent in M04, workflow in M03
    │   └── validator.go             # Pre-deployment compatibility checks
    ├── events/
    │   └── events.go                # Kafka event publishing
    └── clients/
        ├── m04.go                   # HTTP client for M04
        └── m03.go                   # HTTP client for M03
```

## Testing

```bash
go test ./... -v
go test ./... -cover
```

## Licensing Models

| Model | Description |
|-------|-------------|
| Free | No subscription required, immediate deploy |
| Trial | 14-day trial period, then requires subscription |
| Subscription | Monthly/annual, auto-renewable |
| One-Time | Single purchase, no recurring billing |