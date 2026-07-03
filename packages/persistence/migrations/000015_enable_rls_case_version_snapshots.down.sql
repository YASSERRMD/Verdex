DROP POLICY IF EXISTS tenant_isolation ON case_version_snapshots;
ALTER TABLE case_version_snapshots NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_version_snapshots DISABLE ROW LEVEL SECURITY;
