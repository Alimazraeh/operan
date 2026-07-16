-- Module 12: Model Abstraction Layer — Database Schema
-- Run before starting the service.

BEGIN;

-- Registered LLM backends (OpenAI, Anthropic, Ollama, Azure, LiteLLM, custom)
CREATE TABLE IF NOT EXISTS model_providers (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             VARCHAR(255) NOT NULL,
    name                  VARCHAR(100) NOT NULL,
    description           TEXT,
    type                  VARCHAR(30) NOT NULL CHECK (type IN ('openai', 'anthropic', 'litellm', 'ollama', 'azure', 'custom')),
    base_url              VARCHAR(500) NOT NULL,
    api_key_secret_name   VARCHAR(255),
    is_active             BOOLEAN NOT NULL DEFAULT true,
    priority              INT NOT NULL DEFAULT 50,
    max_retries           INT NOT NULL DEFAULT 2,
    timeout_ms            INT NOT NULL DEFAULT 30000,
    metadata              JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_providers_tenant ON model_providers(tenant_id);

-- Model name → provider mapping
CREATE TABLE IF NOT EXISTS model_registry (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             VARCHAR(255) NOT NULL,
    model_name            VARCHAR(200) NOT NULL,
    provider_id           UUID NOT NULL REFERENCES model_providers(id),
    provider_model_name   VARCHAR(200),
    supports_chat         BOOLEAN NOT NULL DEFAULT true,
    supports_embed        BOOLEAN NOT NULL DEFAULT true,
    max_tokens            INT NOT NULL DEFAULT 8192,
    cost_per_token        JSONB NOT NULL DEFAULT '{"prompt": 0.0, "completion": 0.0}',
    is_default            BOOLEAN NOT NULL DEFAULT false,
    is_active             BOOLEAN NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, model_name)
);
CREATE INDEX IF NOT EXISTS idx_registry_tenant ON model_registry(tenant_id);

-- Inference call audit trail
CREATE TABLE IF NOT EXISTS model_calls (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             VARCHAR(255) NOT NULL,
    agent_id              VARCHAR(255),
    workflow_id           VARCHAR(255),
    model_name            VARCHAR(200) NOT NULL,
    provider_id           UUID NOT NULL REFERENCES model_providers(id),
    prompt_tokens         INT NOT NULL DEFAULT 0,
    completion_tokens     INT NOT NULL DEFAULT 0,
    total_tokens          INT NOT NULL DEFAULT 0,
    cost_usd              FLOAT NOT NULL DEFAULT 0.0,
    status                VARCHAR(20) NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'error', 'timeout', 'failover')),
    error_message         TEXT,
    latency_ms            INT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_calls_tenant ON model_calls(tenant_id);
CREATE INDEX IF NOT EXISTS idx_calls_workflow ON model_calls(tenant_id, workflow_id);

COMMIT;