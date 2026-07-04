-- Control catalogue rows are shared reference data, not per-tenant
-- records (see packages/compliance.ControlRepository's doc comment),
-- so this table carries no tenant_id column and no RLS policy --
-- mirroring how packages/jurisdiction's jurisdiction definitions are
-- global reference data rather than per-tenant rows. A deployment
-- narrows which catalogued controls actually apply to it via
-- compliance_profiles below, not by forking the catalogue per tenant.
CREATE TABLE IF NOT EXISTS compliance_controls (
    id          UUID NOT NULL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    framework   TEXT NOT NULL,
    category    TEXT NOT NULL,
    mapped_to   JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT compliance_controls_code_not_blank CHECK (length(trim(code)) > 0),
    CONSTRAINT compliance_controls_title_not_blank CHECK (length(trim(title)) > 0),
    CONSTRAINT compliance_controls_framework_not_blank CHECK (length(trim(framework)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_compliance_controls_framework ON compliance_controls (framework);
CREATE INDEX IF NOT EXISTS idx_compliance_controls_category ON compliance_controls (category);

CREATE TABLE IF NOT EXISTS compliance_control_evidence (
    id           UUID NOT NULL PRIMARY KEY,
    tenant_id    UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    control_id   UUID NOT NULL REFERENCES compliance_controls (id) ON DELETE CASCADE,
    kind         TEXT NOT NULL,
    reference    TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    collected_by UUID NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT compliance_evidence_reference_not_blank CHECK (length(trim(reference)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_compliance_evidence_tenant_id ON compliance_control_evidence (tenant_id);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_tenant_control ON compliance_control_evidence (tenant_id, control_id);

CREATE TABLE IF NOT EXISTS compliance_profiles (
    tenant_id            UUID NOT NULL PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    frameworks           JSONB NOT NULL DEFAULT '[]'::jsonb,
    excluded_control_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    set_by               UUID NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
