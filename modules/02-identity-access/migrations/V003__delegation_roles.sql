CREATE TABLE IF NOT EXISTS delegation_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    parent_role_id UUID REFERENCES roles(id),
    scope VARCHAR(50) NOT NULL DEFAULT 'tenant',
    permissions JSONB NOT NULL DEFAULT '[]',
    max_delegation_depth INTEGER NOT NULL DEFAULT 0,
    delegated_to_ids JSONB NOT NULL DEFAULT '[]',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delegation_roles_tenant_id ON delegation_roles(tenant_id);
