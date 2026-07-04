DROP INDEX IF EXISTS uq_bulkimport_records_job_source_index;
DROP INDEX IF EXISTS idx_bulkimport_records_tenant_job_dedup;
DROP INDEX IF EXISTS idx_bulkimport_records_tenant_job;
DROP INDEX IF EXISTS idx_bulkimport_records_tenant_id;
DROP TABLE IF EXISTS bulkimport_records;

DROP INDEX IF EXISTS idx_bulkimport_jobs_tenant_status;
DROP INDEX IF EXISTS idx_bulkimport_jobs_tenant_id;
DROP TABLE IF EXISTS bulkimport_jobs;
