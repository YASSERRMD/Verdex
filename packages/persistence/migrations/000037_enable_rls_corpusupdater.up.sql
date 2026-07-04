-- Mirrors migrations/000035_enable_rls_integration.up.sql exactly (and
-- 000033, 000031, 000029, 000027, 000025, 000023, ... before it) for
-- the full rationale behind NULLIF(...,'')::uuid and why SET LOCAL
-- (never plain SET) is mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE corpusupdater_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE corpusupdater_jobs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON corpusupdater_jobs
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE corpusupdater_amendments ENABLE ROW LEVEL SECURITY;
ALTER TABLE corpusupdater_amendments FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON corpusupdater_amendments
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
