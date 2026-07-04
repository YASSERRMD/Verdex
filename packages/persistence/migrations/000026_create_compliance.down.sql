DROP TABLE IF EXISTS compliance_profiles;

DROP INDEX IF EXISTS idx_compliance_evidence_tenant_control;
DROP INDEX IF EXISTS idx_compliance_evidence_tenant_id;
DROP TABLE IF EXISTS compliance_control_evidence;

DROP INDEX IF EXISTS idx_compliance_controls_category;
DROP INDEX IF EXISTS idx_compliance_controls_framework;
DROP TABLE IF EXISTS compliance_controls;
