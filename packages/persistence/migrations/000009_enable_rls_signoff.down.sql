DROP POLICY IF EXISTS tenant_isolation ON signoff_audit_entries;
ALTER TABLE signoff_audit_entries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE signoff_audit_entries DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON signoff_records;
ALTER TABLE signoff_records NO FORCE ROW LEVEL SECURITY;
ALTER TABLE signoff_records DISABLE ROW LEVEL SECURITY;
