-- Mirrors migrations/000017_enable_rls_notifications.up.sql exactly:
-- see that file (and 000015_enable_rls_case_version_snapshots.up.sql,
-- 000013_enable_rls_annotations.up.sql,
-- 000011_enable_rls_saved_searches.up.sql,
-- 000009_enable_rls_signoff.up.sql, 000007_enable_rls_cases.up.sql
-- before it) for the full rationale behind NULLIF(...,'')::uuid and why
-- SET LOCAL (never plain SET) is mandatory in
-- packages/tenancy.WithTenantScope.
ALTER TABLE key_metadata ENABLE ROW LEVEL SECURITY;
ALTER TABLE key_metadata FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON key_metadata
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE key_audit_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE key_audit_entries FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON key_audit_entries
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE break_glass_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE break_glass_grants FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON break_glass_grants
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
