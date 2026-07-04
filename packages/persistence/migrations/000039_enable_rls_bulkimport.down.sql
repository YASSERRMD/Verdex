DROP POLICY IF EXISTS tenant_isolation ON bulkimport_records;
ALTER TABLE bulkimport_records NO FORCE ROW LEVEL SECURITY;
ALTER TABLE bulkimport_records DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON bulkimport_jobs;
ALTER TABLE bulkimport_jobs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE bulkimport_jobs DISABLE ROW LEVEL SECURITY;
