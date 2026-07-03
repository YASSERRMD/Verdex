-- Mirrors migrations/000013_enable_rls_annotations.up.sql exactly: see
-- that file (and 000011_enable_rls_saved_searches.up.sql,
-- 000009_enable_rls_signoff.up.sql, 000007_enable_rls_cases.up.sql
-- before it) for the full rationale behind NULLIF(...,'')::uuid and why
-- SET LOCAL (never plain SET) is mandatory in
-- packages/tenancy.WithTenantScope.
ALTER TABLE case_version_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_version_snapshots FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON case_version_snapshots
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
