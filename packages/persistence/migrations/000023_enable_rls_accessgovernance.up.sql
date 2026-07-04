-- Mirrors migrations/000021_enable_rls_auditlog.up.sql exactly (and
-- 000019, 000017, 000015, 000013, 000011, 000009, 000007 before it)
-- for the full rationale behind NULLIF(...,'')::uuid and why SET
-- LOCAL (never plain SET) is mandatory in
-- packages/tenancy.WithTenantScope.
ALTER TABLE access_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE access_policies FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON access_policies
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE access_case_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE access_case_grants FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON access_case_grants
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE access_elevation_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE access_elevation_grants FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON access_elevation_grants
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE access_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE access_reviews FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON access_reviews
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
