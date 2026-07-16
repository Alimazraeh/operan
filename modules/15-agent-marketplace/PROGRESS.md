# Module 15 — Agent Marketplace

## Status: ✅ COMPLETE

**Build:** Clean (`go build ./...`, `go vet ./...`)  
**Tests:** 102 passing across 6 packages  
**Coverage:**  
- config: 100.0%  
- ctxkeys: 100.0%  
- events: 68.8%  
- handler: 42.7%  
- middleware: 57.1%  
- store: 75.1%  

## Test Summary

| Package | Tests | Coverage | Key Tests |
|---------|-------|----------|-----------|
| config | 6 | 100% | Default values, custom port, missing JWT secret fail-closed, env overrides |
| ctxkeys | 7 | 100% | All 5 context key getters: present/missing scenarios |
| events | 8 | 68.8% | All 7 event types, constant validation |
| handler | 22 | 42.7% | Listings CRUD, subscriptions, reviews, deploy, auth, tenant isolation |
| middleware | 12 | 57.1% | JWT validation, JWT middleware, tenant middleware |
| store | 47 | 75.1% | Listings CRUD+search+filters, subscriptions CRUD+isActive, reviews CRUD |

## Endpoints Implemented

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | /health | healthHandler | Health check (no auth) |
| GET | /v1/listings | ListListings | Search & browse marketplace |
| GET | /v1/listings/{id} | GetListing | Listing detail |
| POST | /v1/listings/{id}/deploy | DeployListing | Deploy to tenant environment |
| POST | /v1/subscriptions | Subscribe | Subscribe to a listing |
| GET | /v1/subscriptions | ListSubscriptions | My subscriptions |
| POST | /v1/reviews | CreateReview | Rate a listing |
| GET | /v1/reviews | ListReviews | Reviews for a listing |

## Search Filters

GET /v1/listings supports: `page`, `page_size`, `category`, `listing_type`, `status`, `capability`, `language`, `requires_subscription`, `subscription_tier`, `price_min`, `price_max`, `rating_min`, `search`

## Kafka Events

- `operan.marketplace.listing_created`
- `operan.marketplace.listing_approved`
- `operan.marketplace.subscription_created`
- `operan.marketplace.subscription_expired`
- `operan.marketplace.deployed`
- `operan.marketplace.deploy_failed`
- `operan.marketplace.review_created`

## Integration Points

- M04 Agent Registry: Register agents on deploy
- M03 Orchestration: Create workflows on deploy
- M02 IAM: JWT validation + tenant isolation
- M12 Model Abstraction: Resolve model references
- M19 Arabic Language Core: Arabic terminology
- M21 Experience Portal: Marketplace UI