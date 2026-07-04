-- Phase 096 (packages/alerting): AlertRule definitions, fired
-- AlertEvent history, and EscalationPolicy configuration. Each table
-- carries a tenant_id column and gets an RLS policy in the paired
-- 000043_enable_rls_alerting.up.sql migration, mirroring
-- packages/compliance's compliance_control_evidence/
-- compliance_profiles pattern (000026/000027) -- unlike
-- packages/reliability (Phase 093), which explicitly skips a migration
-- because its types are live, in-process operational state with no
-- tenant-facing historical value (see packages/reliability/doc.go).
-- An AlertRule, a fired AlertEvent, and an EscalationPolicy are
-- exactly the kind of tenant-facing, queryable-after-the-fact record
-- an operator or auditor legitimately wants to query days or months
-- later -- see packages/alerting/doc.go's "Persistence" section for
-- the full rationale.
CREATE TABLE IF NOT EXISTS alerting_rules (
    id           UUID NOT NULL PRIMARY KEY,
    tenant_id    UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    condition_kind TEXT NOT NULL,
    metric_name  TEXT NOT NULL,
    threshold    DOUBLE PRECISION NOT NULL DEFAULT 0,
    severity     TEXT NOT NULL,
    runbook_name TEXT NOT NULL DEFAULT '',
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT alerting_rules_name_not_blank CHECK (length(trim(name)) > 0),
    CONSTRAINT alerting_rules_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_alerting_rules_tenant_id ON alerting_rules (tenant_id);

CREATE TABLE IF NOT EXISTS alerting_events (
    id             UUID NOT NULL PRIMARY KEY,
    tenant_id      UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    rule_id        UUID,
    rule_name      TEXT NOT NULL,
    severity       TEXT NOT NULL,
    condition_kind TEXT NOT NULL,
    trigger_value  DOUBLE PRECISION NOT NULL DEFAULT 0,
    threshold      DOUBLE PRECISION NOT NULL DEFAULT 0,
    detail         TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT alerting_events_rule_name_not_blank CHECK (length(trim(rule_name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_alerting_events_tenant_id ON alerting_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_alerting_events_tenant_rule ON alerting_events (tenant_id, rule_id);
CREATE INDEX IF NOT EXISTS idx_alerting_events_tenant_created_at ON alerting_events (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS alerting_escalation_policies (
    tenant_id    UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    min_severity TEXT NOT NULL,
    tiers        JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, name),
    CONSTRAINT alerting_policies_name_not_blank CHECK (length(trim(name)) > 0)
);
