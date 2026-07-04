DROP INDEX IF EXISTS idx_access_reviews_tenant_due;
DROP INDEX IF EXISTS idx_access_reviews_tenant_id;
DROP TABLE IF EXISTS access_reviews;

DROP INDEX IF EXISTS idx_access_elevation_grants_tenant_expires;
DROP INDEX IF EXISTS idx_access_elevation_grants_tenant_grantee;
DROP INDEX IF EXISTS idx_access_elevation_grants_tenant_id;
DROP TABLE IF EXISTS access_elevation_grants;

DROP INDEX IF EXISTS idx_access_case_grants_tenant_expires;
DROP INDEX IF EXISTS idx_access_case_grants_tenant_grantee;
DROP INDEX IF EXISTS idx_access_case_grants_tenant_case;
DROP INDEX IF EXISTS idx_access_case_grants_tenant_id;
DROP TABLE IF EXISTS access_case_grants;

DROP INDEX IF EXISTS idx_access_policies_tenant_active;
DROP INDEX IF EXISTS idx_access_policies_tenant_id;
DROP TABLE IF EXISTS access_policies;
