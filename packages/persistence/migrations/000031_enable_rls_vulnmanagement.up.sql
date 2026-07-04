-- Mirrors migrations/000027_enable_rls_compliance.up.sql exactly (and
-- 000025, 000023, 000021, 000019, 000017, 000015, 000013, 000011,
-- 000009, 000007 before it) for the full rationale behind
-- NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE vulnmanagement_findings ENABLE ROW LEVEL SECURITY;
ALTER TABLE vulnmanagement_findings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON vulnmanagement_findings
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE vulnmanagement_triage_decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE vulnmanagement_triage_decisions FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON vulnmanagement_triage_decisions
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
