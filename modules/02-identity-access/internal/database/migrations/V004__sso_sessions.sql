CREATE TABLE IF NOT EXISTS sso_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    sso_provider VARCHAR(50),
    assertion_id VARCHAR(255),
    assertion_type VARCHAR(50),
    auth_result VARCHAR(20) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_sso_sessions_tenant_id ON sso_sessions(tenant_id);
CREATE INDEX idx_sso_sessions_user_id ON sso_sessions(user_id);
