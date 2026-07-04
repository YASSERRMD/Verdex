DROP POLICY IF EXISTS tenant_isolation ON localization_preferences;
ALTER TABLE localization_preferences NO FORCE ROW LEVEL SECURITY;
ALTER TABLE localization_preferences DISABLE ROW LEVEL SECURITY;
