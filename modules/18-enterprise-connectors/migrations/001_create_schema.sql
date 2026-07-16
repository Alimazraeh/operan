-- Enterprise Connector Fabric schema
-- Module 18: Enterprise Connector Fabric

CREATE TABLE IF NOT EXISTS connector_definitions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    name                    VARCHAR(200) NOT NULL,
    description             TEXT,
    connector_type          VARCHAR(50) NOT NULL CHECK (connector_type IN ('smtp', 'salesforce', 'hubspot', 'm365', 'sap', 'generic_rest', 'sharepoint', 'slack', 'custom')),
    status                  VARCHAR(30) NOT NULL DEFAULT 'inactive' CHECK (status IN ('inactive', 'active', 'syncing', 'error', 'disconnected')),
    auth_method             VARCHAR(30) NOT NULL DEFAULT 'api_key' CHECK (auth_method IN ('api_key', 'oauth2', 'basic', 'custom')),
    config                  JSONB NOT NULL DEFAULT '{}',
    credentials             JSONB NOT NULL DEFAULT '{}',
    sync_frequency          VARCHAR(30) NOT NULL DEFAULT 'manual' CHECK (sync_frequency IN ('manual', 'hourly', 'daily', 'realtime')),
    last_sync_at            TIMESTAMPTZ,
    last_sync_status        VARCHAR(30),
    last_error              TEXT,
    tools_registered        BOOLEAN NOT NULL DEFAULT false,
    metadata                JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connectors_tenant ON connector_definitions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_connectors_type ON connector_definitions(tenant_id, connector_type);
CREATE INDEX IF NOT EXISTS idx_connectors_status ON connector_definitions(tenant_id, status);

CREATE TABLE IF NOT EXISTS connector_sync_history (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    connector_id            UUID NOT NULL REFERENCES connector_definitions(id),
    sync_type               VARCHAR(30) NOT NULL DEFAULT 'full' CHECK (sync_type IN ('full', 'incremental', 'initial')),
    status                  VARCHAR(30) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'error', 'cancelled')),
    objects_fetched         INT NOT NULL DEFAULT 0,
    objects_updated         INT NOT NULL DEFAULT 0,
    objects_failed          INT NOT NULL DEFAULT 0,
    error_message           TEXT,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ,
    duration_ms             INT
);

CREATE INDEX IF NOT EXISTS idx_sync_history_connector ON connector_sync_history(connector_id);
CREATE INDEX IF NOT EXISTS idx_sync_history_tenant ON connector_sync_history(tenant_id, started_at DESC);