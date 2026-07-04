-- Mirrors migrations/000027_enable_rls_compliance.up.sql exactly (and
-- 000025, 000023, 000021, 000019, 000017, 000015, 000013, 000011,
-- 000009, 000007 before it) for the full rationale behind
-- NULLIF(...,'')::uuid and why SET LOCAL (never plain SET) is
-- mandatory in packages/tenancy.WithTenantScope.
--
-- Every table added in 000030_create_integration.up.sql carries a
-- tenant_id column, unlike packages/compliance's shared
-- compliance_controls catalogue, so every one of them gets a
-- tenant_isolation policy here. Numbered 000030/000031 rather than
-- 000028/000029 because Phases 084 (vulnmanagement) and 085 (backupdr)
-- landed on main first and had already claimed those two numbers by
-- the time this phase rebased onto the current tip.
ALTER TABLE integration_connector_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_connector_configs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_connector_configs
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE integration_connector_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_connector_credentials FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_connector_credentials
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE integration_field_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_field_mappings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_field_mappings
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE integration_import_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_import_runs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_import_runs
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE integration_delivery_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_delivery_runs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_delivery_runs
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE integration_reconciliation_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_reconciliation_results FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_reconciliation_results
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
