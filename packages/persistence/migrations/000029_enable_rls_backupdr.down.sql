DROP POLICY IF EXISTS tenant_isolation ON backupdr_targets;
ALTER TABLE backupdr_targets NO FORCE ROW LEVEL SECURITY;
ALTER TABLE backupdr_targets DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON backupdr_drills;
ALTER TABLE backupdr_drills NO FORCE ROW LEVEL SECURITY;
ALTER TABLE backupdr_drills DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON backupdr_records;
ALTER TABLE backupdr_records NO FORCE ROW LEVEL SECURITY;
ALTER TABLE backupdr_records DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON backupdr_policies;
ALTER TABLE backupdr_policies NO FORCE ROW LEVEL SECURITY;
ALTER TABLE backupdr_policies DISABLE ROW LEVEL SECURITY;
