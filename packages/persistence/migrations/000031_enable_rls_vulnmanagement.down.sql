DROP POLICY IF EXISTS tenant_isolation ON vulnmanagement_triage_decisions;
ALTER TABLE vulnmanagement_triage_decisions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE vulnmanagement_triage_decisions DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON vulnmanagement_findings;
ALTER TABLE vulnmanagement_findings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE vulnmanagement_findings DISABLE ROW LEVEL SECURITY;
