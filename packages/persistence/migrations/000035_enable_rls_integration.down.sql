DROP POLICY IF EXISTS tenant_isolation ON integration_reconciliation_results;
ALTER TABLE integration_reconciliation_results NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integration_reconciliation_results DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON integration_delivery_runs;
ALTER TABLE integration_delivery_runs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integration_delivery_runs DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON integration_import_runs;
ALTER TABLE integration_import_runs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integration_import_runs DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON integration_field_mappings;
ALTER TABLE integration_field_mappings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integration_field_mappings DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON integration_connector_credentials;
ALTER TABLE integration_connector_credentials NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integration_connector_credentials DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON integration_connector_configs;
ALTER TABLE integration_connector_configs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integration_connector_configs DISABLE ROW LEVEL SECURITY;
