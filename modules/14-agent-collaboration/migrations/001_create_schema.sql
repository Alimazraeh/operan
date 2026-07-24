CREATE TABLE IF NOT EXISTS channels (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    name                    VARCHAR(200) NOT NULL,
    description             TEXT,
    channel_type            VARCHAR(30) NOT NULL DEFAULT 'general' CHECK (channel_type IN ('general', 'department', 'project', 'private', 'workflow')),
    creator_id              VARCHAR(255) NOT NULL,
    max_members             INT NOT NULL DEFAULT 50,
    is_public               BOOLEAN NOT NULL DEFAULT false,
    metadata                JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);
CREATE INDEX IF NOT EXISTS idx_channels_tenant ON channels(tenant_id);

CREATE TABLE IF NOT EXISTS channel_members (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id              UUID NOT NULL REFERENCES channels(id),
    agent_id                VARCHAR(255) NOT NULL,
    role                    VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member', 'readonly')),
    joined_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(channel_id, agent_id)
);
CREATE INDEX IF NOT EXISTS idx_members_channel ON channel_members(channel_id);
CREATE INDEX IF NOT EXISTS idx_members_agent  ON channel_members(agent_id);

CREATE TABLE IF NOT EXISTS messages (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    channel_id              UUID NOT NULL REFERENCES channels(id),
    parent_id               UUID,
    sender_id               VARCHAR(255) NOT NULL,
    sender_name             VARCHAR(200),
    message_type            VARCHAR(30) NOT NULL DEFAULT 'text' CHECK (message_type IN ('text', 'task', 'system', 'tool_call', 'tool_result', 'handoff')),
    content                 TEXT NOT NULL,
    attachments             JSONB NOT NULL DEFAULT '[]',
    reply_count             INT NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_parent  ON messages(parent_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender  ON messages(tenant_id, sender_id);

CREATE TABLE IF NOT EXISTS handoffs (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    from_agent_id           VARCHAR(255) NOT NULL,
    to_agent_id             VARCHAR(255) NOT NULL,
    channel_id              UUID REFERENCES channels(id),
    parent_message_id       UUID REFERENCES messages(id),
    title                   VARCHAR(500) NOT NULL,
    description             TEXT,
    context                 JSONB NOT NULL DEFAULT '{}',
    priority                VARCHAR(20) NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'critical')),
    status                  VARCHAR(30) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'in_progress', 'completed', 'rejected', 'expired', 'cancelled')),
    expires_at              TIMESTAMPTZ,
    assigned_at             TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    response                TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_handoffs_tenant ON handoffs(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_handoffs_to     ON handoffs(tenant_id, to_agent_id, status);
CREATE INDEX IF NOT EXISTS idx_handoffs_from   ON handoffs(tenant_id, from_agent_id);

CREATE TABLE IF NOT EXISTS presence (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               VARCHAR(255) NOT NULL,
    agent_id                VARCHAR(255) NOT NULL,
    status                  VARCHAR(20) NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'away', 'offline')),
    last_heartbeat          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata                JSONB NOT NULL DEFAULT '{}',
    UNIQUE(tenant_id, agent_id)
);
CREATE INDEX IF NOT EXISTS idx_presence_tenant ON presence(tenant_id, status);