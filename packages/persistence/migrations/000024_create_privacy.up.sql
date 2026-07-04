CREATE TABLE IF NOT EXISTS privacy_data_inventory (
    id               UUID NOT NULL PRIMARY KEY,
    tenant_id        UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    category         TEXT NOT NULL,
    source_tag       TEXT NOT NULL,
    sensitivity      TEXT NOT NULL,
    legal_basis      TEXT NOT NULL,
    retention_period_seconds BIGINT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT privacy_data_inventory_source_tag_not_blank CHECK (length(trim(source_tag)) > 0),
    CONSTRAINT privacy_data_inventory_retention_positive CHECK (retention_period_seconds > 0)
);

CREATE INDEX IF NOT EXISTS idx_privacy_data_inventory_tenant_id ON privacy_data_inventory (tenant_id);
CREATE INDEX IF NOT EXISTS idx_privacy_data_inventory_tenant_category ON privacy_data_inventory (tenant_id, category);

CREATE TABLE IF NOT EXISTS privacy_consent_records (
    id            UUID NOT NULL PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject_id    TEXT NOT NULL,
    purpose       TEXT NOT NULL,
    legal_basis   TEXT NOT NULL,
    granted_at    TIMESTAMPTZ NOT NULL,
    withdrawn_at  TIMESTAMPTZ,
    recorded_by   UUID NOT NULL,
    notes         TEXT NOT NULL DEFAULT '',
    CONSTRAINT privacy_consent_records_subject_id_not_blank CHECK (length(trim(subject_id)) > 0),
    CONSTRAINT privacy_consent_records_purpose_not_blank CHECK (length(trim(purpose)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_privacy_consent_records_tenant_id ON privacy_consent_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_privacy_consent_records_tenant_subject ON privacy_consent_records (tenant_id, subject_id);
CREATE INDEX IF NOT EXISTS idx_privacy_consent_records_tenant_subject_purpose ON privacy_consent_records (tenant_id, subject_id, purpose);

CREATE TABLE IF NOT EXISTS privacy_subject_access_requests (
    id                UUID NOT NULL PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject_id        TEXT NOT NULL,
    case_refs         JSONB NOT NULL DEFAULT '[]'::jsonb,
    status            TEXT NOT NULL,
    received_at       TIMESTAMPTZ NOT NULL,
    due_at            TIMESTAMPTZ NOT NULL,
    resolved_at       TIMESTAMPTZ,
    resolution_notes  TEXT NOT NULL DEFAULT '',
    handled_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT privacy_sar_subject_id_not_blank CHECK (length(trim(subject_id)) > 0),
    CONSTRAINT privacy_sar_status_allowed CHECK (status IN ('received', 'in_progress', 'fulfilled', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_privacy_sar_tenant_id ON privacy_subject_access_requests (tenant_id);
CREATE INDEX IF NOT EXISTS idx_privacy_sar_tenant_subject ON privacy_subject_access_requests (tenant_id, subject_id);
CREATE INDEX IF NOT EXISTS idx_privacy_sar_tenant_status ON privacy_subject_access_requests (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_privacy_sar_tenant_due ON privacy_subject_access_requests (tenant_id, due_at);

CREATE TABLE IF NOT EXISTS privacy_erasure_requests (
    id                    UUID NOT NULL PRIMARY KEY,
    tenant_id             UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject_id            TEXT NOT NULL,
    category              TEXT NOT NULL,
    source_tag            TEXT NOT NULL,
    record_ref            TEXT NOT NULL DEFAULT '',
    provenance_record_id  UUID,
    provenance_hash       TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL,
    requested_at          TIMESTAMPTZ NOT NULL,
    resolved_at           TIMESTAMPTZ,
    resolution_notes      TEXT NOT NULL DEFAULT '',
    handled_by            UUID,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT privacy_erasure_subject_id_not_blank CHECK (length(trim(subject_id)) > 0),
    CONSTRAINT privacy_erasure_source_tag_not_blank CHECK (length(trim(source_tag)) > 0),
    CONSTRAINT privacy_erasure_status_allowed CHECK (status IN ('received', 'completed', 'rejected')),
    -- Mirrors ErasureRequest.Validate's ErrProvenanceHashRequired
    -- guard at the database layer: a row can never reference a
    -- provenance record without also carrying the hash that must
    -- survive erasure untouched.
    CONSTRAINT privacy_erasure_provenance_hash_required CHECK (
        provenance_record_id IS NULL OR length(trim(provenance_hash)) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_privacy_erasure_tenant_id ON privacy_erasure_requests (tenant_id);
CREATE INDEX IF NOT EXISTS idx_privacy_erasure_tenant_subject ON privacy_erasure_requests (tenant_id, subject_id);
CREATE INDEX IF NOT EXISTS idx_privacy_erasure_tenant_status ON privacy_erasure_requests (tenant_id, status);
