-- Mirrors migrations/000035_enable_rls_integration.up.sql exactly
-- (and 000033, 000031, 000029, 000027, 000025, 000023, 000021, 000019,
-- 000017, 000015, 000013, 000011, 000009, 000007 before it) for the
-- full rationale behind NULLIF(...,'')::uuid and why SET LOCAL (never
-- plain SET) is mandatory in packages/tenancy.WithTenantScope.
--
-- Both tables added in 000036_create_bulkimport.up.sql carry a
-- tenant_id column, so both get a tenant_isolation policy here. This
-- phase's migrations may be renumbered by a coordinator process to
-- avoid colliding with a sibling phase landing in parallel on main --
-- see packages/bulkimport/doc.go for that note.
ALTER TABLE bulkimport_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE bulkimport_jobs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON bulkimport_jobs
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE bulkimport_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE bulkimport_records FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON bulkimport_records
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
