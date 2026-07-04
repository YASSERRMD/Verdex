DROP INDEX IF EXISTS idx_securitytesting_findings_tenant_status;
DROP INDEX IF EXISTS idx_securitytesting_findings_tenant_id;
DROP TABLE IF EXISTS securitytesting_findings;

DROP INDEX IF EXISTS idx_securitytesting_run_records_tenant_scenario;
DROP INDEX IF EXISTS idx_securitytesting_run_records_tenant_id;
DROP INDEX IF EXISTS idx_securitytesting_run_records_id_unique;
DROP TABLE IF EXISTS securitytesting_run_records;
