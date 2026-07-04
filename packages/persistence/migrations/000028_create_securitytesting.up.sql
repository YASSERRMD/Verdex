-- Security-testing findings and run-record history are per-tenant
-- operational data (a Finding tracks a real vulnerability discovered
-- against a specific tenant's deployment; a RunRecord is the audit
-- trail of when a Scenario ran and what it found), so both tables
-- carry a tenant_id column and get an RLS policy in the companion
-- 000029_enable_rls_securitytesting.up.sql migration -- mirroring
-- 000024_create_privacy.up.sql / 000026_create_compliance.up.sql's
-- split between "create tables" and "enable RLS" migrations exactly.
CREATE TABLE IF NOT EXISTS securitytesting_run_records (
    id                UUID NOT NULL PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    scenario_name     TEXT NOT NULL,
    scenario_category TEXT NOT NULL,
    outcome           TEXT NOT NULL,
    detail            TEXT NOT NULL DEFAULT '',
    evidence          JSONB NOT NULL DEFAULT '{}'::jsonb,
    run_by            UUID,
    ran_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT securitytesting_run_records_scenario_name_not_blank CHECK (length(trim(scenario_name)) > 0),
    CONSTRAINT securitytesting_run_records_scenario_category_not_blank CHECK (length(trim(scenario_category)) > 0),
    CONSTRAINT securitytesting_run_records_outcome_not_blank CHECK (length(trim(outcome)) > 0)
);

-- RunRecords are append-only (see
-- packages/securitytesting.RunRecordRepository's doc comment): this
-- unique constraint is the database-layer enforcement of the same
-- ErrDuplicateRunRecord guard InMemoryRunRecordRepository.Create
-- applies in-process, so a replayed RunRecord.ID is rejected at the
-- storage layer too, not just by the in-memory fixture used in tests.
CREATE UNIQUE INDEX IF NOT EXISTS idx_securitytesting_run_records_id_unique ON securitytesting_run_records (id);
CREATE INDEX IF NOT EXISTS idx_securitytesting_run_records_tenant_id ON securitytesting_run_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_securitytesting_run_records_tenant_scenario ON securitytesting_run_records (tenant_id, scenario_name);

CREATE TABLE IF NOT EXISTS securitytesting_findings (
    id                          UUID NOT NULL PRIMARY KEY,
    tenant_id                   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    title                       TEXT NOT NULL,
    category                    TEXT NOT NULL,
    severity                    TEXT NOT NULL,
    source_scenario             TEXT NOT NULL,
    source_run_id               UUID NOT NULL REFERENCES securitytesting_run_records (id) ON DELETE CASCADE,
    detail                      TEXT NOT NULL DEFAULT '',
    status                      TEXT NOT NULL,
    risk_accepted_justification TEXT NOT NULL DEFAULT '',
    opened_by                   UUID,
    opened_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT securitytesting_findings_title_not_blank CHECK (length(trim(title)) > 0),
    CONSTRAINT securitytesting_findings_category_not_blank CHECK (length(trim(category)) > 0),
    CONSTRAINT securitytesting_findings_severity_not_blank CHECK (length(trim(severity)) > 0),
    CONSTRAINT securitytesting_findings_source_scenario_not_blank CHECK (length(trim(source_scenario)) > 0),
    CONSTRAINT securitytesting_findings_status_not_blank CHECK (length(trim(status)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_securitytesting_findings_tenant_id ON securitytesting_findings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_securitytesting_findings_tenant_status ON securitytesting_findings (tenant_id, status);
