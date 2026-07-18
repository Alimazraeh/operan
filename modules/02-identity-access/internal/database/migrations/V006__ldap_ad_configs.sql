CREATE TABLE IF NOT EXISTS ldap_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    server_url VARCHAR(500) NOT NULL,
    bind_dn VARCHAR(500),
    bind_password VARCHAR(500),
    search_base VARCHAR(500),
    user_search_filter VARCHAR(500),
    group_search_filter VARCHAR(500),
    connection_security VARCHAR(20) NOT NULL DEFAULT 'none',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ad_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    server_url VARCHAR(500) NOT NULL,
    domain VARCHAR(255),
    bind_dn VARCHAR(500),
    bind_password VARCHAR(500),
    base_dn VARCHAR(500),
    user_search_filter VARCHAR(500),
    connection_security VARCHAR(20) NOT NULL DEFAULT 'none',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
