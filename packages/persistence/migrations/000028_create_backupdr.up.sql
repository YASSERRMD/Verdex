-- Phase 085 (packages/backupdr): backup policy, backup record,
-- restore-drill, and RPO/RTO-target persistence. Unlike
-- compliance_controls (000026_create_compliance.up.sql), every table
-- here is tenant-scoped -- a backup policy, a completed backup, a
-- restore drill, and an RPO/RTO target are all inherently per-tenant
-- facts, not shared reference data.
CREATE TABLE IF NOT EXISTS backupdr_policies (
    tenant_id             UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    class                 TEXT NOT NULL,
    frequency_seconds     BIGINT NOT NULL,
    retention_seconds     BIGINT NOT NULL,
    encryption_required   BOOLEAN NOT NULL DEFAULT TRUE,
    cross_region_required BOOLEAN NOT NULL DEFAULT FALSE,
    created_by            UUID NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, class),
    CONSTRAINT backupdr_policies_class_not_blank CHECK (length(trim(class)) > 0),
    CONSTRAINT backupdr_policies_frequency_positive CHECK (frequency_seconds > 0),
    CONSTRAINT backupdr_policies_retention_positive CHECK (retention_seconds > 0)
);

CREATE TABLE IF NOT EXISTS backupdr_records (
    id             UUID NOT NULL PRIMARY KEY,
    tenant_id      UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    class          TEXT NOT NULL,
    taken_at       TIMESTAMPTZ NOT NULL,
    location       TEXT NOT NULL,
    reference      TEXT NOT NULL,
    integrity_hash TEXT NOT NULL DEFAULT '',
    size_bytes     BIGINT NOT NULL DEFAULT 0,
    encrypted      BOOLEAN NOT NULL DEFAULT FALSE,
    status         TEXT NOT NULL,
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT backupdr_records_class_not_blank CHECK (length(trim(class)) > 0),
    CONSTRAINT backupdr_records_location_not_blank CHECK (length(trim(location)) > 0),
    CONSTRAINT backupdr_records_reference_not_blank CHECK (length(trim(reference)) > 0),
    CONSTRAINT backupdr_records_status_not_blank CHECK (length(trim(status)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_backupdr_records_tenant_id ON backupdr_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_backupdr_records_tenant_class ON backupdr_records (tenant_id, class);
CREATE INDEX IF NOT EXISTS idx_backupdr_records_tenant_class_taken_at ON backupdr_records (tenant_id, class, taken_at);

CREATE TABLE IF NOT EXISTS backupdr_drills (
    id           UUID NOT NULL PRIMARY KEY,
    tenant_id    UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    class        TEXT NOT NULL,
    record_id    UUID NOT NULL REFERENCES backupdr_records (id) ON DELETE CASCADE,
    executed_at  TIMESTAMPTZ NOT NULL,
    executor     UUID NOT NULL,
    outcome      TEXT NOT NULL,
    duration_ns  BIGINT NOT NULL DEFAULT 0,
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT backupdr_drills_class_not_blank CHECK (length(trim(class)) > 0),
    CONSTRAINT backupdr_drills_outcome_not_blank CHECK (length(trim(outcome)) > 0),
    CONSTRAINT backupdr_drills_duration_non_negative CHECK (duration_ns >= 0)
);

CREATE INDEX IF NOT EXISTS idx_backupdr_drills_tenant_id ON backupdr_drills (tenant_id);
CREATE INDEX IF NOT EXISTS idx_backupdr_drills_tenant_class ON backupdr_drills (tenant_id, class);

CREATE TABLE IF NOT EXISTS backupdr_targets (
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    class       TEXT NOT NULL,
    rpo_seconds BIGINT NOT NULL,
    rto_seconds BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, class),
    CONSTRAINT backupdr_targets_class_not_blank CHECK (length(trim(class)) > 0),
    CONSTRAINT backupdr_targets_rpo_positive CHECK (rpo_seconds > 0),
    CONSTRAINT backupdr_targets_rto_positive CHECK (rto_seconds > 0)
);
