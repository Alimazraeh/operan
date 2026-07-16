-- Module 06: Knowledge Ingestion — Database Schema
-- Run before starting the service.

BEGIN;

-- Document ingestion sources
CREATE TABLE IF NOT EXISTS ingestion_sources (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    name            VARCHAR(500) NOT NULL,
    source_type     VARCHAR(50) NOT NULL CHECK (source_type IN ('file', 'url', 'sharepoint', 'email', 'web_crawl', 's3')),
    source_url      TEXT NOT NULL,
    file_type       VARCHAR(20),
    file_size_bytes INT DEFAULT 0,
    file_hash       VARCHAR(64),
    normalize_arabic BOOLEAN NOT NULL DEFAULT false,
    chunk_strategy  VARCHAR(30) NOT NULL DEFAULT 'adaptive' CHECK (chunk_strategy IN ('adaptive', 'fixed', 'by_heading', 'by_paragraph')),
    chunk_size      INT NOT NULL DEFAULT 512,
    chunk_overlap   INT NOT NULL DEFAULT 50,
    status          VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'error')),
    last_ingested   TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, source_url)
);
CREATE INDEX IF NOT EXISTS idx_sources_tenant ON ingestion_sources(tenant_id);

-- Ingestion jobs (each job = one file source)
CREATE TABLE IF NOT EXISTS ingestion_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    source_id       UUID NOT NULL REFERENCES ingestion_sources(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'extracting', 'chunking', 'embedding', 'storing', 'completed', 'failed', 'cancelled')),
    total_chunks    INT NOT NULL DEFAULT 0,
    processed_chunks INT NOT NULL DEFAULT 0,
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_jobs_tenant ON ingestion_jobs(tenant_id, status);

-- Ingestion results (per-chunk metadata, NOT vectors — vectors go to M07)
CREATE TABLE IF NOT EXISTS ingestion_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    job_id          UUID NOT NULL REFERENCES ingestion_jobs(id),
    source_id       UUID NOT NULL,
    chunk_index     INT NOT NULL,
    chunk_hash      VARCHAR(64) NOT NULL,
    chunk_text      TEXT NOT NULL,
    chunk_metadata  JSONB NOT NULL DEFAULT '{}',
    embedding_model VARCHAR(200),
    vector_dim      INT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'embedding', 'stored', 'failed')),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(job_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_results_tenant ON ingestion_results(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_results_hash   ON ingestion_results(tenant_id, chunk_hash);
CREATE INDEX IF NOT EXISTS idx_results_source  ON ingestion_results(tenant_id, source_id);

COMMIT;