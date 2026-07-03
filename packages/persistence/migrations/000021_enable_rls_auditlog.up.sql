-- Mirrors migrations/000019_enable_rls_keymanagement.up.sql exactly
-- (and 000017_enable_rls_notifications.up.sql, 000015, 000013, 000011,
-- 000009, 000007 before it) for the full rationale behind
-- NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON audit_events
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
