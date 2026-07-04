DROP POLICY IF EXISTS tenant_isolation ON privacy_erasure_requests;
ALTER TABLE privacy_erasure_requests NO FORCE ROW LEVEL SECURITY;
ALTER TABLE privacy_erasure_requests DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON privacy_subject_access_requests;
ALTER TABLE privacy_subject_access_requests NO FORCE ROW LEVEL SECURITY;
ALTER TABLE privacy_subject_access_requests DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON privacy_consent_records;
ALTER TABLE privacy_consent_records NO FORCE ROW LEVEL SECURITY;
ALTER TABLE privacy_consent_records DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON privacy_data_inventory;
ALTER TABLE privacy_data_inventory NO FORCE ROW LEVEL SECURITY;
ALTER TABLE privacy_data_inventory DISABLE ROW LEVEL SECURITY;
