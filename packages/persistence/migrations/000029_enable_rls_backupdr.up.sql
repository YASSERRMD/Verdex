-- Mirrors migrations/000027_enable_rls_compliance.up.sql exactly (and
-- 000025, 000023, 000021, 000019, 000017, 000015, 000013, 000011,
-- 000009, 000007 before it) for the full rationale behind
-- NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
--
-- Unlike compliance_controls, every backupdr_* table carries a
-- tenant_id column (see 000028_create_backupdr.up.sql's comment), so
-- every one of them gets the standard tenant_isolation policy here.
ALTER TABLE backupdr_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE backupdr_policies FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON backupdr_policies
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE backupdr_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE backupdr_records FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON backupdr_records
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE backupdr_drills ENABLE ROW LEVEL SECURITY;
ALTER TABLE backupdr_drills FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON backupdr_drills
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE backupdr_targets ENABLE ROW LEVEL SECURITY;
ALTER TABLE backupdr_targets FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON backupdr_targets
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
