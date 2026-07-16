-- Module 19: Arabic Language Core schema

-- Terminology glossary for government Arabic terminology governance
CREATE TABLE IF NOT EXISTS terminology_glossary (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    term_arabic     VARCHAR(500) NOT NULL,
    term_english    VARCHAR(500),
    term_transliterated VARCHAR(500),
    category        VARCHAR(100) NOT NULL,
    domain          VARCHAR(100) DEFAULT 'general',
    status          VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'rejected')),
    approved_by     VARCHAR(255),
    preferred_form  VARCHAR(500),
    alternatives    TEXT[],
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, term_arabic, category, domain)
);

CREATE INDEX IF NOT EXISTS idx_glossary_tenant_category ON terminology_glossary(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_glossary_tenant_status ON terminology_glossary(tenant_id, status);

-- Usage log: every time a term is checked/flagged
CREATE TABLE IF NOT EXISTS terminology_usage_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    source_text     TEXT NOT NULL,
    matched_terms   JSONB NOT NULL DEFAULT '[]',
    flagged_terms   JSONB NOT NULL DEFAULT '[]',
    checked_by      VARCHAR(255),
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_usage_log_tenant ON terminology_usage_log(tenant_id, timestamp DESC);

-- Embedding request log (monitoring M12 call volume)
CREATE TABLE IF NOT EXISTS arabic_embedding_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(255) NOT NULL,
    text_length     INT NOT NULL,
    embedding_model VARCHAR(200) NOT NULL,
    vector_dim      INT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'failed')),
    error_message   TEXT,
    duration_ms     INT,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_embedding_requests_tenant ON arabic_embedding_requests(tenant_id, timestamp DESC);