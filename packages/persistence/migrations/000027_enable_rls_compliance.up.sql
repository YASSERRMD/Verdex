-- Mirrors migrations/000025_enable_rls_privacy.up.sql exactly (and
-- 000023, 000021, 000019, 000017, 000015, 000013, 000011, 000009,
-- 000007 before it) for the full rationale behind
-- NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
--
-- compliance_controls carries no tenant_id column (see
-- 000026_create_compliance.up.sql's comment) and therefore has no RLS
-- policy here -- it is shared reference data, not per-tenant data.
ALTER TABLE compliance_control_evidence ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_control_evidence FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON compliance_control_evidence
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE compliance_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_profiles FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON compliance_profiles
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
