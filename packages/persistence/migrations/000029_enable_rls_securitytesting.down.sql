DROP POLICY IF EXISTS tenant_isolation ON securitytesting_findings;
ALTER TABLE securitytesting_findings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE securitytesting_findings DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON securitytesting_run_records;
ALTER TABLE securitytesting_run_records NO FORCE ROW LEVEL SECURITY;
ALTER TABLE securitytesting_run_records DISABLE ROW LEVEL SECURITY;
