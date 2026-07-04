DROP TABLE IF EXISTS backupdr_targets;

DROP INDEX IF EXISTS idx_backupdr_drills_tenant_class;
DROP INDEX IF EXISTS idx_backupdr_drills_tenant_id;
DROP TABLE IF EXISTS backupdr_drills;

DROP INDEX IF EXISTS idx_backupdr_records_tenant_class_taken_at;
DROP INDEX IF EXISTS idx_backupdr_records_tenant_class;
DROP INDEX IF EXISTS idx_backupdr_records_tenant_id;
DROP TABLE IF EXISTS backupdr_records;

DROP TABLE IF EXISTS backupdr_policies;
