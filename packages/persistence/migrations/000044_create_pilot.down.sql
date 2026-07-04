DROP INDEX IF EXISTS idx_pilot_refinements_tenant_finding;
DROP INDEX IF EXISTS idx_pilot_refinements_tenant_id;
DROP TABLE IF EXISTS pilot_refinement_records;

DROP INDEX IF EXISTS idx_pilot_findings_tenant_status;
DROP INDEX IF EXISTS idx_pilot_findings_tenant_deployment;
DROP INDEX IF EXISTS idx_pilot_findings_tenant_id;
DROP TABLE IF EXISTS pilot_findings;

DROP INDEX IF EXISTS idx_pilot_feedback_tenant_case;
DROP INDEX IF EXISTS idx_pilot_feedback_tenant_id;
DROP TABLE IF EXISTS pilot_feedback_entries;

DROP INDEX IF EXISTS idx_pilot_cases_tenant_case_id;
DROP INDEX IF EXISTS idx_pilot_cases_tenant_deployment;
DROP INDEX IF EXISTS idx_pilot_cases_tenant_id;
DROP TABLE IF EXISTS pilot_cases;

DROP INDEX IF EXISTS idx_pilot_deployments_tenant_status;
DROP INDEX IF EXISTS idx_pilot_deployments_tenant_id;
DROP TABLE IF EXISTS pilot_deployments;
