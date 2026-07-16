CREATE TABLE IF NOT EXISTS routing_rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           VARCHAR(255) NOT NULL,
    rule_name           VARCHAR(200) NOT NULL,
    description         TEXT,
    task_type           VARCHAR(50) NOT NULL CHECK (task_type IN ('summarize', 'classify', 'generate', 'extract', 'chat', 'embed', 'general')),
    priority            INT NOT NULL DEFAULT 50,
    min_cost_threshold  NUMERIC(12,6) DEFAULT 0,
    max_latency_ms      INT NOT NULL DEFAULT 5000,
    max_tokens          INT NOT NULL DEFAULT 4096,
    failover_enabled    BOOLEAN NOT NULL DEFAULT true,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rules_tenant ON routing_rules(tenant_id);

CREATE TABLE IF NOT EXISTS routing_rule_models (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           VARCHAR(255) NOT NULL,
    rule_id             UUID NOT NULL REFERENCES routing_rules(id),
    model_id            VARCHAR(200) NOT NULL,
    capability_score    NUMERIC(5,2) NOT NULL,
    cost_weight         NUMERIC(5,2) NOT NULL DEFAULT 50,
    latency_weight      NUMERIC(5,2) NOT NULL DEFAULT 50,
    reliability_weight  NUMERIC(5,2) NOT NULL DEFAULT 50,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(rule_id, model_id)
);
CREATE INDEX idx_rule_models_rule ON routing_rule_models(rule_id);

CREATE TABLE IF NOT EXISTS routing_performance (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           VARCHAR(255) NOT NULL,
    model_id            VARCHAR(200) NOT NULL,
    task_type           VARCHAR(50) NOT NULL,
    avg_latency_ms      NUMERIC(10,2) NOT NULL DEFAULT 0,
    p99_latency_ms      NUMERIC(10,2) NOT NULL DEFAULT 0,
    error_rate          NUMERIC(5,4) NOT NULL DEFAULT 0,
    calls_count         INT NOT NULL DEFAULT 0,
    avg_cost_usd        NUMERIC(12,6) NOT NULL DEFAULT 0,
    quality_score       NUMERIC(5,2) NOT NULL DEFAULT 50,
    last_call_at        TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_perf_tenant_model ON routing_performance(tenant_id, model_id);
CREATE INDEX idx_perf_task ON routing_performance(tenant_id, task_type);