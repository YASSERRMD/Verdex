DROP INDEX IF EXISTS idx_integration_reconciliation_tenant_connector;
DROP INDEX IF EXISTS idx_integration_reconciliation_tenant_id;
DROP TABLE IF EXISTS integration_reconciliation_results;

DROP INDEX IF EXISTS idx_integration_delivery_runs_tenant_connector;
DROP INDEX IF EXISTS idx_integration_delivery_runs_tenant_id;
DROP TABLE IF EXISTS integration_delivery_runs;

DROP INDEX IF EXISTS idx_integration_import_runs_tenant_connector;
DROP INDEX IF EXISTS idx_integration_import_runs_tenant_id;
DROP TABLE IF EXISTS integration_import_runs;

DROP INDEX IF EXISTS idx_integration_field_mappings_tenant_id;
DROP TABLE IF EXISTS integration_field_mappings;

DROP INDEX IF EXISTS idx_integration_credentials_tenant_id;
DROP TABLE IF EXISTS integration_connector_credentials;

DROP INDEX IF EXISTS idx_integration_connector_configs_tenant_id;
DROP TABLE IF EXISTS integration_connector_configs;
