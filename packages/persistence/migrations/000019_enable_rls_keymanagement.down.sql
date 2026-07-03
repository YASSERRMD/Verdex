DROP POLICY IF EXISTS tenant_isolation ON break_glass_grants;
ALTER TABLE break_glass_grants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE break_glass_grants DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON key_audit_entries;
ALTER TABLE key_audit_entries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE key_audit_entries DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON key_metadata;
ALTER TABLE key_metadata NO FORCE ROW LEVEL SECURITY;
ALTER TABLE key_metadata DISABLE ROW LEVEL SECURITY;
