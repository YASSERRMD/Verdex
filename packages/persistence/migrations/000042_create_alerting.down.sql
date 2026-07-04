DROP TABLE IF EXISTS alerting_escalation_policies;

DROP INDEX IF EXISTS idx_alerting_events_tenant_created_at;
DROP INDEX IF EXISTS idx_alerting_events_tenant_rule;
DROP INDEX IF EXISTS idx_alerting_events_tenant_id;
DROP TABLE IF EXISTS alerting_events;

DROP INDEX IF EXISTS idx_alerting_rules_tenant_id;
DROP TABLE IF EXISTS alerting_rules;
