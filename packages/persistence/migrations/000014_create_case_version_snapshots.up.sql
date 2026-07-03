CREATE TABLE IF NOT EXISTS case_version_snapshots (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    case_id                UUID NOT NULL REFERENCES cases (id) ON DELETE CASCADE,
    artifact_kind          TEXT NOT NULL,
    artifact_revision_ref  TEXT NOT NULL DEFAULT '',
    payload                JSONB,
    created_by             UUID NOT NULL,
    reason                 TEXT NOT NULL DEFAULT '',
    label                  TEXT NOT NULL DEFAULT '',
    restored_from_id       UUID REFERENCES case_version_snapshots (id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT case_version_snapshots_artifact_kind_allowed CHECK (
        artifact_kind IN ('case-metadata', 'tree', 'evidence', 'opinion')
    )
    -- payload intentionally carries no NOT NULL / shape constraint:
    -- it is nil for ArtifactTree and ArtifactEvidence snapshots (which
    -- rely solely on artifact_revision_ref, a link into
    -- packages/irac's/packages/treeassembly's own revision store and
    -- packages/annotations's own audit trail respectively, rather than
    -- a duplicated copy — see doc/case-versioning.md), and is a
    -- CaseMetadataPayload or OpinionPayload JSON object otherwise.
);

CREATE INDEX IF NOT EXISTS idx_case_version_snapshots_tenant_id ON case_version_snapshots (tenant_id);
CREATE INDEX IF NOT EXISTS idx_case_version_snapshots_case_id ON case_version_snapshots (case_id);
CREATE INDEX IF NOT EXISTS idx_case_version_snapshots_case_kind ON case_version_snapshots (case_id, artifact_kind);
CREATE INDEX IF NOT EXISTS idx_case_version_snapshots_created_at ON case_version_snapshots (created_at);
