-- Mirrors migrations/000007_enable_rls_cases.up.sql exactly: see that
-- file (and 000003_enable_rls_deployments.up.sql before it) for the
-- full rationale behind NULLIF(...,'')::uuid and why SET LOCAL (never
-- plain SET) is mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE signoff_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE signoff_records FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON signoff_records
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE signoff_audit_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE signoff_audit_entries FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON signoff_audit_entries
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
