CREATE TABLE IF NOT EXISTS signoff_records (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id          UUID NOT NULL REFERENCES cases (id) ON DELETE CASCADE,
    tenant_id        UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    status           TEXT NOT NULL DEFAULT 'pending',
    reviewer_id      UUID,
    notes            TEXT NOT NULL DEFAULT '',
    case_version     INTEGER NOT NULL DEFAULT 1,
    source           TEXT NOT NULL DEFAULT 'initial',
    decided_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT signoff_records_case_id_unique UNIQUE (case_id),
    CONSTRAINT signoff_records_status_allowed CHECK (
        status IN ('pending', 'approved', 'rejected')
    ),
    CONSTRAINT signoff_records_source_allowed CHECK (
        source IN ('reviewer', 're_review', 'initial')
    ),
    CONSTRAINT signoff_records_case_version_positive CHECK (case_version > 0),
    CONSTRAINT signoff_records_rejected_requires_notes CHECK (
        status <> 'rejected' OR btrim(notes) <> ''
    )
);

CREATE INDEX IF NOT EXISTS idx_signoff_records_tenant_id ON signoff_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_signoff_records_status ON signoff_records (status);

CREATE TABLE IF NOT EXISTS signoff_audit_entries (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id       UUID NOT NULL REFERENCES cases (id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    from_status   TEXT NOT NULL,
    to_status     TEXT NOT NULL,
    actor         UUID,
    source        TEXT NOT NULL,
    notes         TEXT NOT NULL DEFAULT '',
    case_version  INTEGER NOT NULL DEFAULT 1,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT signoff_audit_from_status_allowed CHECK (
        from_status IN ('pending', 'approved', 'rejected')
    ),
    CONSTRAINT signoff_audit_to_status_allowed CHECK (
        to_status IN ('pending', 'approved', 'rejected')
    ),
    CONSTRAINT signoff_audit_source_allowed CHECK (
        source IN ('reviewer', 're_review', 'initial')
    )
    -- Append-only by convention (mirrors case_transitions): no UPDATE
    -- or DELETE is ever issued against this table by
    -- packages/signoff.PostgresRepository.
);

CREATE INDEX IF NOT EXISTS idx_signoff_audit_entries_case_id ON signoff_audit_entries (case_id);
CREATE INDEX IF NOT EXISTS idx_signoff_audit_entries_tenant_id ON signoff_audit_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_signoff_audit_entries_occurred_at ON signoff_audit_entries (occurred_at);
