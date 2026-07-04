DROP INDEX IF EXISTS idx_privacy_erasure_tenant_status;
DROP INDEX IF EXISTS idx_privacy_erasure_tenant_subject;
DROP INDEX IF EXISTS idx_privacy_erasure_tenant_id;
DROP TABLE IF EXISTS privacy_erasure_requests;

DROP INDEX IF EXISTS idx_privacy_sar_tenant_due;
DROP INDEX IF EXISTS idx_privacy_sar_tenant_status;
DROP INDEX IF EXISTS idx_privacy_sar_tenant_subject;
DROP INDEX IF EXISTS idx_privacy_sar_tenant_id;
DROP TABLE IF EXISTS privacy_subject_access_requests;

DROP INDEX IF EXISTS idx_privacy_consent_records_tenant_subject_purpose;
DROP INDEX IF EXISTS idx_privacy_consent_records_tenant_subject;
DROP INDEX IF EXISTS idx_privacy_consent_records_tenant_id;
DROP TABLE IF EXISTS privacy_consent_records;

DROP INDEX IF EXISTS idx_privacy_data_inventory_tenant_category;
DROP INDEX IF EXISTS idx_privacy_data_inventory_tenant_id;
DROP TABLE IF EXISTS privacy_data_inventory;
