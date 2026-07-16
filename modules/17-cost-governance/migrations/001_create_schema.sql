-- Module 17: Cost Governance Engine — Initial schema

CREATE TABLE IF NOT EXISTS cost_budgets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           VARCHAR(255) NOT NULL,
    agent_id            VARCHAR(255),                     -- NULL = tenant-wide budget
    description         VARCHAR(500),
    budget_amount       NUMERIC(15,4) NOT NULL,           -- USD amount
    currency            VARCHAR(3) NOT NULL DEFAULT 'USD',
    period              VARCHAR(20) NOT NULL CHECK (period IN ('daily', 'weekly', 'monthly', 'quarterly')),
    soft_limit_pct      INT NOT NULL DEFAULT 80,          -- soft throttle at 80%
    hard_limit_pct      INT NOT NULL DEFAULT 95,          -- hard throttle at 95%
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_budgets_tenant ON cost_budgets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_budgets_agent ON cost_budgets(tenant_id, agent_id);

-- Every cost event recorded here
CREATE TABLE IF NOT EXISTS cost_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           VARCHAR(255) NOT NULL,
    agent_id            VARCHAR(255),
    source_module       VARCHAR(20) NOT NULL CHECK (source_module IN ('m12', 'm08', 'manual')),
    source_id           VARCHAR(255),                     -- model_call_id or tool_call_id from source
    model_name          VARCHAR(200),
    cost_usd            NUMERIC(15,6) NOT NULL,
    prompt_tokens       INT DEFAULT 0,
    completion_tokens   INT DEFAULT 0,
    event_type          VARCHAR(50) NOT NULL,             -- model_call, tool_execution, manual_adjustment
    billing_tag         VARCHAR(100),
    event_timestamp     TIMESTAMPTZ NOT NULL,             -- when the original event occurred
    recorded_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_tenant ON cost_events(tenant_id, event_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_agent  ON cost_events(tenant_id, agent_id, event_timestamp DESC);

-- Alert history
CREATE TABLE IF NOT EXISTS cost_alerts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           VARCHAR(255) NOT NULL,
    budget_id           UUID REFERENCES cost_budgets(id),
    agent_id            VARCHAR(255),
    alert_type          VARCHAR(20) NOT NULL CHECK (alert_type IN ('soft_limit', 'hard_limit', 'budget_exceeded', 'budget_reset')),
    current_spend       NUMERIC(15,4) NOT NULL,
    budget_amount       NUMERIC(15,4) NOT NULL,
    percentage_used     NUMERIC(5,2) NOT NULL,
    severity            VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical', 'fatal')),
    is_resolved         BOOLEAN NOT NULL DEFAULT false,
    resolved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_tenant ON cost_alerts(tenant_id, created_at DESC);