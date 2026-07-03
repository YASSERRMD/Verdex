DROP POLICY IF EXISTS tenant_isolation ON annotation_audit_events;
ALTER TABLE annotation_audit_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE annotation_audit_events DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON annotation_mentions;
ALTER TABLE annotation_mentions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE annotation_mentions DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON annotations;
ALTER TABLE annotations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE annotations DISABLE ROW LEVEL SECURITY;
