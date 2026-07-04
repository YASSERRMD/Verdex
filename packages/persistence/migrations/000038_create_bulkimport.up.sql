-- Phase 088: packages/bulkimport. Both tables here are per-tenant
-- data -- an ImportJob and its ImportRecords belong to exactly one
-- tenant's historical case-corpus onboarding run, mirroring
-- packages/integration's per-tenant tables rather than
-- packages/compliance's shared compliance_controls catalogue.

CREATE TABLE IF NOT EXISTS bulkimport_jobs (
    id                   UUID NOT NULL PRIMARY KEY,
    tenant_id            UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    source_description   TEXT NOT NULL,
    status               TEXT NOT NULL,
    total_records        INTEGER NOT NULL DEFAULT 0,
    processed_records    INTEGER NOT NULL DEFAULT 0,
    failed_records       INTEGER NOT NULL DEFAULT 0,
    skipped_records      INTEGER NOT NULL DEFAULT 0,
    imported_records     INTEGER NOT NULL DEFAULT 0,
    cursor               INTEGER NOT NULL DEFAULT 0,
    created_by           UUID NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at           TIMESTAMPTZ,
    finished_at          TIMESTAMPTZ,
    failure_reason       TEXT NOT NULL DEFAULT '',
    CONSTRAINT bulkimport_jobs_source_not_blank CHECK (length(trim(source_description)) > 0),
    CONSTRAINT bulkimport_jobs_status_not_blank CHECK (length(trim(status)) > 0),
    CONSTRAINT bulkimport_jobs_counts_nonneg CHECK (
        total_records >= 0 AND processed_records >= 0 AND failed_records >= 0 AND
        skipped_records >= 0 AND imported_records >= 0 AND cursor >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_bulkimport_jobs_tenant_id ON bulkimport_jobs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bulkimport_jobs_tenant_status ON bulkimport_jobs (tenant_id, status);

CREATE TABLE IF NOT EXISTS bulkimport_records (
    id                   UUID NOT NULL PRIMARY KEY,
    tenant_id            UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    job_id               UUID NOT NULL REFERENCES bulkimport_jobs (id) ON DELETE CASCADE,
    source_index         INTEGER NOT NULL,
    payload_ref          TEXT NOT NULL DEFAULT '',
    case_number          TEXT NOT NULL DEFAULT '',
    jurisdiction         TEXT NOT NULL DEFAULT '',
    party_names          JSONB NOT NULL DEFAULT '[]'::jsonb,
    dedup_key            TEXT NOT NULL DEFAULT '',
    validation_status    TEXT NOT NULL,
    validation_errors    JSONB NOT NULL DEFAULT '[]'::jsonb,
    outcome              TEXT NOT NULL,
    outcome_reason       TEXT NOT NULL DEFAULT '',
    created_case_id      UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT bulkimport_records_source_index_nonneg CHECK (source_index >= 0),
    CONSTRAINT bulkimport_records_validation_status_not_blank CHECK (length(trim(validation_status)) > 0),
    CONSTRAINT bulkimport_records_outcome_not_blank CHECK (length(trim(outcome)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_bulkimport_records_tenant_id ON bulkimport_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bulkimport_records_tenant_job ON bulkimport_records (tenant_id, job_id);
CREATE INDEX IF NOT EXISTS idx_bulkimport_records_tenant_job_dedup ON bulkimport_records (tenant_id, job_id, dedup_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_bulkimport_records_job_source_index ON bulkimport_records (job_id, source_index);
