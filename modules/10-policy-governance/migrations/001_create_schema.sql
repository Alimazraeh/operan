-- Module 10: Policy Governance Engine Schema
-- Migrations for policy_groups, policies, and policy_audits tables

-- Policy groups table
CREATE TABLE IF NOT EXISTS policy_groups (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    name                    VARCHAR(200) NOT NULL,
    description             TEXT,
    priority                INT NOT NULL DEFAULT 50,
    is_active               BOOLEAN NOT NULL DEFAULT true,
    metadata                JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_groups_tenant ON policy_groups(tenant_id);

-- Individual policy rules
CREATE TABLE IF NOT EXISTS policies (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    group_id                UUID NOT NULL REFERENCES policy_groups(id),
    name                    VARCHAR(300) NOT NULL,
    description             TEXT,
    action                  VARCHAR(20) NOT NULL CHECK (action IN ('allow', 'deny', 'proxy')),
    scope                   VARCHAR(100) NOT NULL CHECK (scope IN ('agent', 'department', 'tenant', 'global')),
    resource_type           VARCHAR(50) NOT NULL CHECK (resource_type IN ('tool', 'model', 'workflow', 'data', 'all')),
    resource_target         VARCHAR(500),
    condition_expression    TEXT,
    effect                  VARCHAR(20) NOT NULL DEFAULT 'enforce' CHECK (effect IN ('enforce', 'warn', 'log')),
    priority                INT NOT NULL DEFAULT 50,
    is_active               BOOLEAN NOT NULL DEFAULT true,
    created_by              VARCHAR(255),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_policies_group ON policies(group_id);
CREATE INDEX idx_policies_tenant ON policies(tenant_id, is_active);
CREATE INDEX idx_policies_scope ON policies(tenant_id, scope);
CREATE INDEX idx_policies_resource ON policies(tenant_id, resource_type, resource_target);

-- Policy evaluation audit log
CREATE TABLE IF NOT EXISTS policy_audits (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    policy_id               UUID,
    group_id                UUID,
    request_id              VARCHAR(255),
    agent_id                VARCHAR(255),
    resource_type           VARCHAR(50) NOT NULL,
    resource_target         VARCHAR(500),
    requested_action        VARCHAR(50) NOT NULL,
    result                  VARCHAR(20) NOT NULL CHECK (result IN ('allowed', 'denied', 'proxied', 'warning')),
    matched_policy_name     VARCHAR(300),
    matched_rule_index      INT,
    evaluation_ms           INT NOT NULL DEFAULT 0,
    request_data            JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audits_tenant ON policy_audits(tenant_id, created_at DESC);
CREATE INDEX idx_audits_policy ON policy_audits(policy_id);