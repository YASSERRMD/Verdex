-- Mirrors migrations/000027_enable_rls_compliance.up.sql exactly (and
-- 000025, 000023, 000021, 000019, 000017, 000015, 000013, 000011,
-- 000009, 000007 before it) for the full rationale behind
-- NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE alerting_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE alerting_rules FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON alerting_rules
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE alerting_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE alerting_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON alerting_events
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE alerting_escalation_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE alerting_escalation_policies FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON alerting_escalation_policies
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
