-- Creates a dedicated, non-superuser, non-BYPASSRLS role for
-- application traffic. This is what actually makes the RLS policy in
-- migrations/000003_enable_rls_deployments.up.sql load-bearing:
-- PostgreSQL never applies row security to a role with the BYPASSRLS
-- attribute, and superusers always have BYPASSRLS regardless of
-- FORCE ROW LEVEL SECURITY. A connection authenticated as a
-- superuser (e.g. testcontainers' or many managed-Postgres
-- providers' default bootstrap user) silently bypasses every RLS
-- policy in the database, no matter how the policy itself is
-- written. Application code (and cfg.Database.DSN in every
-- deployment) must connect as this role, never as the bootstrap
-- superuser, for tenant isolation to be real. See
-- packages/tenancy/role.go and its README section for the
-- accompanying runtime guard and rationale.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'verdex_app') THEN
        CREATE ROLE verdex_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOLOGIN;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO verdex_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO verdex_app;

-- Ensure tables created by later migrations are automatically granted
-- to verdex_app too, so this migration does not need a counterpart
-- every time a new phase adds a table.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO verdex_app;

-- Bootstrap-time password rotation for verdex_app. ALTER ROLE ...
-- PASSWORD does not accept a bind parameter (its grammar expects a
-- string literal, not a general expression), so a caller-supplied
-- password cannot be sent as a plain parameterized statement. This
-- function accepts the new password as an ordinary, safely-bound SQL
-- function argument and lets Postgres's own format(..., %L, ...)
-- perform correct literal-quoting server-side before EXECUTE, rather
-- than requiring any client-side string concatenation of untrusted
-- input into DDL text.
CREATE OR REPLACE FUNCTION verdex_set_app_role_password(new_password text)
RETURNS void
LANGUAGE plpgsql
AS $BODY$
BEGIN
    EXECUTE format('ALTER ROLE verdex_app WITH LOGIN PASSWORD %L', new_password);
END;
$BODY$;

-- Only a role that already holds privilege to ALTER ROLE verdex_app
-- (i.e. the bootstrap/migration superuser connection) should be able
-- to invoke this; verdex_app itself must never be able to rotate its
-- own password.
REVOKE ALL ON FUNCTION verdex_set_app_role_password(text) FROM PUBLIC;
