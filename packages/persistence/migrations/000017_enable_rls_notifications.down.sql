DROP POLICY IF EXISTS tenant_isolation ON notification_preferences;
ALTER TABLE notification_preferences NO FORCE ROW LEVEL SECURITY;
ALTER TABLE notification_preferences DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON notifications;
ALTER TABLE notifications NO FORCE ROW LEVEL SECURITY;
ALTER TABLE notifications DISABLE ROW LEVEL SECURITY;
