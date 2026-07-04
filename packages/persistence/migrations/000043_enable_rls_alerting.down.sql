DROP POLICY IF EXISTS tenant_isolation ON alerting_escalation_policies;
ALTER TABLE alerting_escalation_policies NO FORCE ROW LEVEL SECURITY;
ALTER TABLE alerting_escalation_policies DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON alerting_events;
ALTER TABLE alerting_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE alerting_events DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON alerting_rules;
ALTER TABLE alerting_rules NO FORCE ROW LEVEL SECURITY;
ALTER TABLE alerting_rules DISABLE ROW LEVEL SECURITY;
