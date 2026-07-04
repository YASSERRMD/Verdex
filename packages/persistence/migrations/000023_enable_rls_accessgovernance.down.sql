DROP POLICY IF EXISTS tenant_isolation ON access_reviews;
ALTER TABLE access_reviews NO FORCE ROW LEVEL SECURITY;
ALTER TABLE access_reviews DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON access_elevation_grants;
ALTER TABLE access_elevation_grants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE access_elevation_grants DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON access_case_grants;
ALTER TABLE access_case_grants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE access_case_grants DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON access_policies;
ALTER TABLE access_policies NO FORCE ROW LEVEL SECURITY;
ALTER TABLE access_policies DISABLE ROW LEVEL SECURITY;
