-- Phase 099 (packages/pilot): PilotDeployment, PilotCase,
-- FeedbackEntry, PilotFinding, and RefinementRecord records. Every
-- table here carries a tenant_id column and gets an RLS policy in the
-- paired 000045_enable_rls_pilot.up.sql migration, mirroring
-- packages/compliance's compliance_control_evidence/
-- compliance_profiles pattern (000026/000027) and
-- packages/alerting's alerting_rules/alerting_events pattern
-- (000042/000043) -- unlike compliance_controls, which is shared
-- catalogue data with no tenant_id, every type this phase introduces
-- is genuinely per-tenant: a pilot deployment, its supervised cases,
-- collected feedback, surfaced findings, and applied refinements are
-- all scoped to the tenant running the pilot. See
-- packages/pilot/doc/pilot.md's "Persistence" section for the full
-- rationale.
CREATE TABLE IF NOT EXISTS pilot_deployments (
    id                UUID NOT NULL PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    jurisdiction_code TEXT NOT NULL,
    status            TEXT NOT NULL,
    start_date        TIMESTAMPTZ NOT NULL,
    end_date          TIMESTAMPTZ,
    created_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pilot_deployments_name_not_blank CHECK (length(trim(name)) > 0),
    CONSTRAINT pilot_deployments_jurisdiction_not_blank CHECK (length(trim(jurisdiction_code)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_pilot_deployments_tenant_id ON pilot_deployments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pilot_deployments_tenant_status ON pilot_deployments (tenant_id, status);

CREATE TABLE IF NOT EXISTS pilot_cases (
    id                 UUID NOT NULL PRIMARY KEY,
    tenant_id          UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    deployment_id      UUID NOT NULL REFERENCES pilot_deployments (id) ON DELETE CASCADE,
    case_id            UUID NOT NULL,
    supervisor_user_id UUID NOT NULL,
    outcome_observed   BOOLEAN NOT NULL DEFAULT false,
    assigned_at        TIMESTAMPTZ NOT NULL,
    observed_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pilot_cases_tenant_id ON pilot_cases (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pilot_cases_tenant_deployment ON pilot_cases (tenant_id, deployment_id);
CREATE INDEX IF NOT EXISTS idx_pilot_cases_tenant_case_id ON pilot_cases (tenant_id, case_id);

CREATE TABLE IF NOT EXISTS pilot_feedback_entries (
    id               UUID NOT NULL PRIMARY KEY,
    tenant_id        UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    pilot_case_id    UUID NOT NULL REFERENCES pilot_cases (id) ON DELETE CASCADE,
    reviewer_user_id UUID NOT NULL,
    ratings          JSONB NOT NULL DEFAULT '[]'::jsonb,
    trust            SMALLINT NOT NULL,
    comments         TEXT NOT NULL DEFAULT '',
    submitted_at     TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pilot_feedback_trust_range CHECK (trust BETWEEN 1 AND 5)
);

CREATE INDEX IF NOT EXISTS idx_pilot_feedback_tenant_id ON pilot_feedback_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pilot_feedback_tenant_case ON pilot_feedback_entries (tenant_id, pilot_case_id);

CREATE TABLE IF NOT EXISTS pilot_findings (
    id                  UUID NOT NULL PRIMARY KEY,
    tenant_id           UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    deployment_id       UUID NOT NULL REFERENCES pilot_deployments (id) ON DELETE CASCADE,
    source_feedback_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    priority            TEXT NOT NULL,
    status              TEXT NOT NULL,
    triage_notes        TEXT NOT NULL DEFAULT '',
    triaged_by          UUID,
    triaged_at          TIMESTAMPTZ,
    discovered_at       TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pilot_findings_title_not_blank CHECK (length(trim(title)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_pilot_findings_tenant_id ON pilot_findings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pilot_findings_tenant_deployment ON pilot_findings (tenant_id, deployment_id);
CREATE INDEX IF NOT EXISTS idx_pilot_findings_tenant_status ON pilot_findings (tenant_id, status);

CREATE TABLE IF NOT EXISTS pilot_refinement_records (
    id                UUID NOT NULL PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    finding_id        UUID NOT NULL REFERENCES pilot_findings (id) ON DELETE CASCADE,
    description       TEXT NOT NULL,
    verified_fixed    BOOLEAN NOT NULL DEFAULT false,
    verification_note TEXT NOT NULL DEFAULT '',
    applied_by        UUID NOT NULL,
    applied_at        TIMESTAMPTZ NOT NULL,
    verified_by       UUID,
    verified_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pilot_refinements_description_not_blank CHECK (length(trim(description)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_pilot_refinements_tenant_id ON pilot_refinement_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pilot_refinements_tenant_finding ON pilot_refinement_records (tenant_id, finding_id);
