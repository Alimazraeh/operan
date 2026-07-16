-- Module 15: Agent Marketplace Schema
-- Migration: 001_create_schema.sql

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Marketplace listings (agents, templates, tools, integrations, skills)
CREATE TABLE marketplace_listings (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id               VARCHAR(255) NOT NULL,
    name                    VARCHAR(300) NOT NULL,
    description             TEXT NOT NULL,
    category                VARCHAR(50) NOT NULL CHECK (category IN ('agent', 'template', 'tool', 'integration', 'skill')),
    listing_type            VARCHAR(30) NOT NULL DEFAULT 'vetted' CHECK (listing_type IN ('vetted', 'user_generated')),
    status                  VARCHAR(30) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending_review', 'approved', 'rejected', 'deactivated')),
    version                 VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    compatibility_versions  JSONB NOT NULL DEFAULT '{}',
    capabilities            TEXT[] NOT NULL DEFAULT '{}',
    supported_languages     TEXT[] NOT NULL DEFAULT '{en}',
    requires_subscription   BOOLEAN NOT NULL DEFAULT false,
    subscription_tier       VARCHAR(50),
    trial_days              INT NOT NULL DEFAULT 0,
    price_usd               NUMERIC(10,2) NOT NULL DEFAULT 0,
    rating_avg              NUMERIC(3,2) NOT NULL DEFAULT 0,
    rating_count            INT NOT NULL DEFAULT 0,
    download_count          INT NOT NULL DEFAULT 0,
    metadata                JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_listings_category ON marketplace_listings(category);
CREATE INDEX idx_listings_status ON marketplace_listings(status);
CREATE INDEX idx_listings_vendor ON marketplace_listings(vendor_id);
CREATE INDEX idx_listings_search ON marketplace_listings USING gin(to_tsvector('simple', name || ' ' || description));

-- Tenant subscriptions to marketplace listings
CREATE TABLE tenant_subscriptions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    listing_id              UUID NOT NULL REFERENCES marketplace_listings(id),
    status                  VARCHAR(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'cancelled', 'trial')),
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at              TIMESTAMPTZ,
    auto_renew              BOOLEAN NOT NULL DEFAULT true,
    subscription_tier       VARCHAR(50) NOT NULL DEFAULT 'basic',
    trial_used              BOOLEAN NOT NULL DEFAULT false,
    deployed                BOOLEAN NOT NULL DEFAULT false,
    deployed_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, listing_id)
);

CREATE INDEX idx_subscriptions_tenant ON tenant_subscriptions(tenant_id, status);
CREATE INDEX idx_subscriptions_listing ON tenant_subscriptions(listing_id);

-- Reviews and ratings
CREATE TABLE reviews (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    listing_id              UUID NOT NULL REFERENCES marketplace_listings(id),
    rating                  INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    title                   VARCHAR(300),
    review_text             TEXT,
    verified_purchase       BOOLEAN NOT NULL DEFAULT false,
    helpful_count           INT NOT NULL DEFAULT 0,
    status                  VARCHAR(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'flagged', 'removed')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, listing_id)
);

CREATE INDEX idx_reviews_listing ON reviews(listing_id);
CREATE INDEX idx_reviews_rating ON reviews(listing_id, rating);