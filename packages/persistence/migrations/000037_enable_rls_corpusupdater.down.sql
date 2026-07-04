DROP POLICY IF EXISTS tenant_isolation ON corpusupdater_amendments;
ALTER TABLE corpusupdater_amendments NO FORCE ROW LEVEL SECURITY;
ALTER TABLE corpusupdater_amendments DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON corpusupdater_jobs;
ALTER TABLE corpusupdater_jobs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE corpusupdater_jobs DISABLE ROW LEVEL SECURITY;
