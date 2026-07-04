-- Mirrors migrations/000023_enable_rls_accessgovernance.up.sql exactly
-- (and 000021, 000019, 000017, 000015, 000013, 000011, 000009, 000007
-- before it) for the full rationale behind NULLIF(...,'')::uuid and
-- why SET LOCAL (never plain SET) is mandatory in
-- packages/tenancy.WithTenantScope.
ALTER TABLE privacy_data_inventory ENABLE ROW LEVEL SECURITY;
ALTER TABLE privacy_data_inventory FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON privacy_data_inventory
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE privacy_consent_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE privacy_consent_records FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON privacy_consent_records
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE privacy_subject_access_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE privacy_subject_access_requests FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON privacy_subject_access_requests
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE privacy_erasure_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE privacy_erasure_requests FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON privacy_erasure_requests
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
