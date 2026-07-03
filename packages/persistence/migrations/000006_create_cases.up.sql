CREATE TABLE IF NOT EXISTS cases (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    jurisdiction_id   UUID NOT NULL,
    category_id       TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL,
    reference         TEXT NOT NULL DEFAULT '',
    state             TEXT NOT NULL DEFAULT 'draft',
    metadata          JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata_version  INTEGER NOT NULL DEFAULT 1,
    created_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at       TIMESTAMPTZ,
    CONSTRAINT cases_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT cases_state_allowed CHECK (
        state IN ('draft', 'active', 'under_review', 'closed', 'archived')
    ),
    CONSTRAINT cases_metadata_version_positive CHECK (metadata_version > 0)
    -- jurisdiction_id and category_id intentionally carry no foreign
    -- key here: packages/jurisdiction and packages/category own their
    -- own tables in separate migration histories outside this phase's
    -- scope, and category_id is a packages/category.CategoryCode
    -- string key, not a surrogate id. This package only stores the
    -- reference.
);

CREATE INDEX IF NOT EXISTS idx_cases_tenant_id ON cases (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cases_state ON cases (state);
CREATE INDEX IF NOT EXISTS idx_cases_jurisdiction_id ON cases (jurisdiction_id);
CREATE INDEX IF NOT EXISTS idx_cases_category_id ON cases (category_id);

CREATE TABLE IF NOT EXISTS case_transitions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID NOT NULL REFERENCES cases (id) ON DELETE CASCADE,
    tenant_id    UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    from_state   TEXT NOT NULL,
    to_state     TEXT NOT NULL,
    actor        UUID NOT NULL,
    reason       TEXT NOT NULL DEFAULT '',
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT case_transitions_from_state_allowed CHECK (
        from_state IN ('draft', 'active', 'under_review', 'closed', 'archived')
    ),
    CONSTRAINT case_transitions_to_state_allowed CHECK (
        to_state IN ('draft', 'active', 'under_review', 'closed', 'archived')
    )
    -- No CHECK constraint enforces the allowed-transitions table
    -- itself: that state machine (packages/caselifecycle's
    -- allowedTransitions) is intentionally kept as application logic
    -- so it can evolve without a migration, exactly as
    -- packages/category's taxonomy rules are data/application logic
    -- rather than hard database constraints.
);

CREATE INDEX IF NOT EXISTS idx_case_transitions_case_id ON case_transitions (case_id);
CREATE INDEX IF NOT EXISTS idx_case_transitions_tenant_id ON case_transitions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_case_transitions_occurred_at ON case_transitions (occurred_at);
