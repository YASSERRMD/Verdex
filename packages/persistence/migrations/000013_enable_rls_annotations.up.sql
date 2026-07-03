-- Mirrors migrations/000011_enable_rls_saved_searches.up.sql exactly:
-- see that file (and 000009_enable_rls_signoff.up.sql,
-- 000007_enable_rls_cases.up.sql before it) for the full rationale
-- behind NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
ALTER TABLE annotations ENABLE ROW LEVEL SECURITY;
ALTER TABLE annotations FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON annotations
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE annotation_mentions ENABLE ROW LEVEL SECURITY;
ALTER TABLE annotation_mentions FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON annotation_mentions
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE annotation_audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE annotation_audit_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON annotation_audit_events
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
