-- Mirrors migrations/000009_enable_rls_signoff.up.sql exactly: see that
-- file (and 000007_enable_rls_cases.up.sql before it) for the full
-- rationale behind NULLIF(...,'')::uuid and why SET LOCAL (never plain
-- SET) is mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE saved_searches ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_searches FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON saved_searches
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
