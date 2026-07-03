-- Mirrors migrations/000015_enable_rls_case_version_snapshots.up.sql
-- exactly: see that file (and 000013_enable_rls_annotations.up.sql,
-- 000011_enable_rls_saved_searches.up.sql,
-- 000009_enable_rls_signoff.up.sql, 000007_enable_rls_cases.up.sql
-- before it) for the full rationale behind NULLIF(...,'')::uuid and why
-- SET LOCAL (never plain SET) is mandatory in
-- packages/tenancy.WithTenantScope.
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON notifications
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE notification_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_preferences FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON notification_preferences
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
