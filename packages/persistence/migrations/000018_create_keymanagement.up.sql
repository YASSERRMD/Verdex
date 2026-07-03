CREATE TABLE IF NOT EXISTS key_metadata (
    id               TEXT NOT NULL,
    tenant_id        UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    version          INTEGER NOT NULL CHECK (version >= 1),
    state            TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ,
    wrapped_key_ref  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    UNIQUE (tenant_id, version),
    CONSTRAINT key_metadata_state_allowed CHECK (
        state IN ('active', 'rotating', 'retired', 'revoked')
    )
);

-- At most one Active key version per tenant: this is the database-layer
-- enforcement of "per-tenant key isolation" / "CurrentKey resolves to
-- exactly one key" backing Repository.GetActive and Service.Rotate,
-- mirroring signoff_records' UNIQUE (case_id) "one current record"
-- constraint (packages/persistence/migrations/000008_create_signoff.up.sql).
CREATE UNIQUE INDEX IF NOT EXISTS idx_key_metadata_one_active_per_tenant
    ON key_metadata (tenant_id)
    WHERE state = 'active';

CREATE INDEX IF NOT EXISTS idx_key_metadata_tenant_id ON key_metadata (tenant_id);
CREATE INDEX IF NOT EXISTS idx_key_metadata_tenant_state ON key_metadata (tenant_id, state);

-- key_audit_entries is the queryable persisted half of AuditRecorder
-- (task 7): every CurrentKey/Key/Rotate/break-glass call, not just a
-- log line. Append-only by convention (no UPDATE/DELETE path in this
-- package), mirroring signoff_audit_entries and
-- caseversioning snapshots' immutable-history pattern.
CREATE TABLE IF NOT EXISTS key_audit_entries (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    actor          TEXT NOT NULL,
    action         TEXT NOT NULL,
    key_id         TEXT NOT NULL DEFAULT '',
    outcome        TEXT NOT NULL,
    justification  TEXT NOT NULL DEFAULT '',
    detail         TEXT NOT NULL DEFAULT '',
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT key_audit_entries_action_allowed CHECK (
        action IN ('current_key', 'key_lookup', 'rotate', 'revoke', 'view_metadata', 'break_glass_grant', 'break_glass_use')
    ),
    CONSTRAINT key_audit_entries_outcome_allowed CHECK (
        outcome IN ('success', 'denied', 'error')
    )
);

CREATE INDEX IF NOT EXISTS idx_key_audit_entries_tenant_id ON key_audit_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_key_audit_entries_tenant_occurred ON key_audit_entries (tenant_id, occurred_at);

-- break_glass_grants records every emergency-access grant (task 6):
-- admin-only, justified, time-bound. A grant row is created at
-- request time and checked (state + expiry) on every use; using it
-- is separately recorded in key_audit_entries
-- (action = 'break_glass_use').
CREATE TABLE IF NOT EXISTS break_glass_grants (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    key_id         TEXT NOT NULL,
    granted_to     UUID NOT NULL,
    justification  TEXT NOT NULL,
    granted_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    used_at        TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    CONSTRAINT break_glass_grants_justification_not_blank CHECK (
        length(trim(justification)) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_break_glass_grants_tenant_id ON break_glass_grants (tenant_id);
