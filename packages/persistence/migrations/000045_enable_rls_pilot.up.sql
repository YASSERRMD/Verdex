-- Mirrors migrations/000043_enable_rls_alerting.up.sql exactly (and
-- 000041, 000039, 000037, ... 000007 before it) for the full
-- rationale behind NULLIF(...,'')::uuid and why SET LOCAL (never
-- plain SET) is mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE pilot_deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE pilot_deployments FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON pilot_deployments
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE pilot_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE pilot_cases FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON pilot_cases
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE pilot_feedback_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE pilot_feedback_entries FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON pilot_feedback_entries
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE pilot_findings ENABLE ROW LEVEL SECURITY;
ALTER TABLE pilot_findings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON pilot_findings
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE pilot_refinement_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE pilot_refinement_records FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON pilot_refinement_records
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
