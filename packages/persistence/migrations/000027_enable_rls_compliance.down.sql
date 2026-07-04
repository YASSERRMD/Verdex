DROP POLICY IF EXISTS tenant_isolation ON compliance_profiles;
ALTER TABLE compliance_profiles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE compliance_profiles DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON compliance_control_evidence;
ALTER TABLE compliance_control_evidence NO FORCE ROW LEVEL SECURITY;
ALTER TABLE compliance_control_evidence DISABLE ROW LEVEL SECURITY;
