DROP INDEX IF EXISTS idx_corpusupdater_amendments_tenant_effective;
DROP INDEX IF EXISTS idx_corpusupdater_amendments_tenant_target;
DROP INDEX IF EXISTS idx_corpusupdater_amendments_tenant_job;
DROP INDEX IF EXISTS idx_corpusupdater_amendments_tenant_id;
DROP TABLE IF EXISTS corpusupdater_amendments;

DROP INDEX IF EXISTS idx_corpusupdater_jobs_tenant_status;
DROP INDEX IF EXISTS idx_corpusupdater_jobs_tenant_jurisdiction;
DROP INDEX IF EXISTS idx_corpusupdater_jobs_tenant_id;
DROP TABLE IF EXISTS corpusupdater_jobs;
