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
