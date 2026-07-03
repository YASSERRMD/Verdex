-- Mirrors migrations/000003_enable_rls_deployments.up.sql exactly: see
-- that file for the full rationale behind NULLIF(...,'')::uuid and why
-- SET LOCAL (never plain SET) is mandatory in
-- packages/tenancy.WithTenantScope.
ALTER TABLE cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON cases
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE case_transitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_transitions FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON case_transitions
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
