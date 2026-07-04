-- Phase 084: vulnerability & dependency management. Findings and
-- triage decisions are per-tenant operational data (unlike
-- packages/compliance's shared compliance_controls catalogue, every
-- row here belongs to exactly one tenant's deployment), so both
-- tables carry tenant_id and get an RLS policy in
-- 000029_enable_rls_vulnmanagement.up.sql.
CREATE TABLE IF NOT EXISTS vulnmanagement_findings (
    id            UUID NOT NULL PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    source        TEXT NOT NULL,
    package       TEXT NOT NULL,
    version       TEXT NOT NULL DEFAULT '',
    severity      TEXT NOT NULL,
    advisory_id   TEXT NOT NULL,
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'open',
    discovered_at TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT vulnmanagement_findings_package_not_blank CHECK (length(trim(package)) > 0),
    CONSTRAINT vulnmanagement_findings_advisory_not_blank CHECK (length(trim(advisory_id)) > 0),
    CONSTRAINT vulnmanagement_findings_title_not_blank CHECK (length(trim(title)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_vulnmanagement_findings_tenant_id ON vulnmanagement_findings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_vulnmanagement_findings_tenant_source ON vulnmanagement_findings (tenant_id, source);
CREATE INDEX IF NOT EXISTS idx_vulnmanagement_findings_tenant_status ON vulnmanagement_findings (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_vulnmanagement_findings_severity ON vulnmanagement_findings (severity);

CREATE TABLE IF NOT EXISTS vulnmanagement_triage_decisions (
    id          UUID NOT NULL PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    finding_id  UUID NOT NULL REFERENCES vulnmanagement_findings (id) ON DELETE CASCADE,
    from_status TEXT NOT NULL,
    to_status   TEXT NOT NULL,
    notes       TEXT NOT NULL,
    actor       UUID NOT NULL,
    decided_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT vulnmanagement_triage_notes_not_blank CHECK (length(trim(notes)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_vulnmanagement_triage_tenant_id ON vulnmanagement_triage_decisions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_vulnmanagement_triage_tenant_finding ON vulnmanagement_triage_decisions (tenant_id, finding_id);
