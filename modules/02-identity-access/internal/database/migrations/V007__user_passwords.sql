-- Real per-user authentication.
--
-- Until now the only way into the platform was one shared admin password that
-- always returned sub="admin-001", so every action in the audit trail was
-- attributed to the same synthetic identity. A user needs somewhere to keep a
-- credential of their own.
--
-- The column holds a bcrypt hash and nothing else; it is never selected into
-- any API response (models.User marks it json:"-").

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- When the credential was last changed, so a rotation policy has something to
-- read later. NULL means "never set" — such a user cannot log in with a
-- password.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_set_at TIMESTAMP WITH TIME ZONE;

-- Login looks a user up by tenant + email on every attempt.
CREATE INDEX IF NOT EXISTS idx_users_tenant_email ON users(tenant_id, email);

-- Tenant identity across this platform is a slug string ("smoke-tenant"),
-- issued in the JWT tenant_id claim and stored as VARCHAR by every other
-- module. M02 alone declared users.tenant_id as UUID with a foreign key to a
-- local tenants table it does not own — M01 is the tenant control plane — so
-- inserting a real user failed on both the type and the constraint.
--
-- Align with the rest of the platform. Idempotent: re-running is a no-op once
-- the column is already character data.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'tenant_id'
          AND data_type = 'uuid'
    ) THEN
        ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_fkey;
        ALTER TABLE users ALTER COLUMN tenant_id TYPE VARCHAR(255) USING tenant_id::text;
    END IF;
END $$;
