ALTER TABLE deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployments FORCE ROW LEVEL SECURITY;

-- current_setting(..., true) (the "missing_ok" second argument) returns
-- NULL instead of raising when app.current_tenant_id has never been set
-- anywhere in the session. But pgxpool reuses physical connections
-- across many transactions: once ANY transaction on a given physical
-- connection has run `SET LOCAL app.current_tenant_id = ...`, Postgres
-- registers that custom GUC name for the session, and once the
-- transaction ends, the LOCAL setting reverts — not to NULL, but to an
-- EMPTY STRING (''), since the GUC had no prior session-level value to
-- revert to. A later, unscoped query on that same reused connection
-- would then evaluate `''::uuid`, which raises a hard "invalid input
-- syntax for type uuid" error rather than yielding NULL. NULLIF(...,'')
-- normalizes both the "never set at all" (NULL) and "reset after a
-- prior SET LOCAL" (empty string) cases to NULL before the cast, so
-- the cast is only ever attempted on a real UUID string or skipped
-- entirely. Comparing tenant_id (NOT NULL) to a NULL right-hand side
-- yields NULL under standard SQL three-valued logic, and a USING
-- clause treats NULL as "not matched" — so an unscoped connection sees
-- zero rows on this table, not an error, regardless of whether it was
-- freshly opened or is a pooled connection some other tenant's request
-- previously scoped. This is the load-bearing property WithTenantScope
-- depends on; see packages/tenancy/scope.go and its integration tests
-- (this exact empty-string reset was first caught by
-- TestIntegration_UnscopedQuery_SeesZeroRowsNotError failing in CI
-- against a real, connection-reusing pgxpool.Pool).
CREATE POLICY tenant_isolation ON deployments
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
