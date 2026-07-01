ALTER TABLE deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployments FORCE ROW LEVEL SECURITY;

-- current_setting(..., true) (the "missing_ok" second argument) returns
-- NULL instead of raising when app.current_tenant_id has not been set
-- in the current session/transaction, rather than erroring. Comparing
-- tenant_id (NOT NULL) to a NULL cast uuid yields NULL under standard
-- SQL three-valued logic, and a USING/WHERE clause treats NULL as "not
-- matched" — so a connection that never ran
-- `SET LOCAL app.current_tenant_id = ...` sees zero rows on this table,
-- not an error. This is the load-bearing property WithTenantScope
-- depends on; see packages/tenancy/scope.go and its integration tests.
CREATE POLICY tenant_isolation ON deployments
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);
