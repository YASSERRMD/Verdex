-- Mirrors migrations/000035_enable_rls_integration.up.sql exactly (and
-- 000033, 000031, 000027, 000025, 000023, 000021, 000019, 000017,
-- 000015, 000013, 000011, 000009, 000007 before it) for the full
-- rationale behind NULLIF(...,'')::uuid and why SET LOCAL (never plain
-- SET) is mandatory in packages/tenancy.WithTenantScope.
--
-- localization_preferences carries a tenant_id column on every row, so
-- it gets a tenant_isolation policy here, same as every table added in
-- 000036_create_localization.up.sql.
--
-- Numbered 000036/000037 per this phase's own migration authoring at
-- the time it branched from main; if a sibling phase landing in
-- parallel has already claimed these numbers by the time this merges,
-- a coordinator renumbers centrally afterward (see
-- 000034/000035_enable_rls_integration's own note on this same
-- collision pattern).
ALTER TABLE localization_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE localization_preferences FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON localization_preferences
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);
