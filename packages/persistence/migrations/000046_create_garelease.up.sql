-- Phase 100 (packages/garelease): ReleaseCandidate and Release
-- records. Unlike the tenant-scoped tables most recent phases add
-- (e.g. pilot_deployments, 000044/000045), a software release is not
-- itself a per-tenant record -- one release is shared by every tenant
-- of this deployment, mirroring packages/compliance's shared,
-- non-tenant-scoped compliance_controls catalogue (000026) exactly.
-- Neither table here carries a tenant_id column, and neither gets a
-- paired enable_rls migration -- see
-- packages/garelease/doc/ga-release.md's "Persistence" section for the
-- full rationale.
CREATE TABLE IF NOT EXISTS garelease_candidates (
    id             UUID NOT NULL PRIMARY KEY,
    version        TEXT NOT NULL UNIQUE,
    commit_sha     TEXT NOT NULL,
    readiness      JSONB NOT NULL,
    frozen_by      UUID NOT NULL,
    frozen_at      TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT garelease_candidates_version_not_blank CHECK (length(trim(version)) > 0),
    CONSTRAINT garelease_candidates_commit_sha_not_blank CHECK (length(trim(commit_sha)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_garelease_candidates_frozen_at ON garelease_candidates (frozen_at);

CREATE TABLE IF NOT EXISTS garelease_releases (
    id             UUID NOT NULL PRIMARY KEY,
    candidate_id   UUID NOT NULL UNIQUE REFERENCES garelease_candidates (id) ON DELETE CASCADE,
    version        TEXT NOT NULL,
    commit_sha     TEXT NOT NULL,
    cut_by         UUID NOT NULL,
    cut_at         TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT garelease_releases_version_not_blank CHECK (length(trim(version)) > 0),
    CONSTRAINT garelease_releases_commit_sha_not_blank CHECK (length(trim(commit_sha)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_garelease_releases_cut_at ON garelease_releases (cut_at);
