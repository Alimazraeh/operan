-- Finish the tenant_id conversion V007 started.
--
-- V007 explained the problem and fixed exactly one table: tenant identity
-- across this platform is a slug string ("acme-corp"), issued in the JWT
-- tenant_id claim, and M02 alone declared tenant_id as UUID with a foreign key
-- into a local `tenants` table it does not own — M01 is the tenant control
-- plane. V007 converted `users`. The other ten tables were left as they were,
-- so every one of them still rejects a real tenant id twice over: 22P02 on the
-- slug, and 23503 on a well-formed UUID, because nothing populates the local
-- tenants table.
--
-- M03 had the identical bug and it cost a demo: driving the department work
-- loop on 2026-11-07 died at the deploy_workflows stage on
-- `invalid input syntax for type uuid: "acme-corp"`.
--
-- The Go side already agrees with this change — every models.*.TenantID is a
-- string, and no query casts to ::uuid — so only the schema disagreed.
--
-- Tables are listed explicitly rather than discovered: this database is shared
-- with M01, M04, M06, M08, M10, M13, M14, M16, M17 and M18, and a migration
-- owned by M02 has no business reshaping their columns.
--
-- Idempotent, in the same shape as V007: each table converts only while its
-- column is still uuid, so re-running is a no-op.

DO $$
DECLARE
    tbl text;
    con record;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'abac_policies', 'ad_configs', 'agent_identities', 'audit_events',
        'delegation_roles', 'ldap_configs', 'roles', 'service_identities',
        'sso_configs', 'sso_sessions'
    ]
    LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = tbl
              AND column_name = 'tenant_id'
              AND data_type = 'uuid'
        ) THEN
            -- Drop whatever foreign key sits on tenant_id by its real name,
            -- rather than assuming Postgres' default, then widen the column.
            FOR con IN
                SELECT c.conname AS name
                FROM pg_constraint c
                JOIN pg_attribute a
                  ON a.attrelid = c.conrelid
                 AND a.attnum = ANY (c.conkey)
                WHERE c.contype = 'f'
                  AND a.attname = 'tenant_id'
                  AND c.conrelid = format('public.%I', tbl)::regclass
            LOOP
                EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', tbl, con.name);
            END LOOP;

            EXECUTE format(
                'ALTER TABLE %I ALTER COLUMN tenant_id TYPE VARCHAR(255) USING tenant_id::text',
                tbl);
        END IF;
    END LOOP;
END $$;

-- Every other tenant-scoped table in this module indexes the column it filters
-- by; these two, both from V006, never did.
CREATE INDEX IF NOT EXISTS idx_ad_configs_tenant_id ON ad_configs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ldap_configs_tenant_id ON ldap_configs(tenant_id);
