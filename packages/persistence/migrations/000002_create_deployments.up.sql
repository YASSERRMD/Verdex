CREATE TABLE IF NOT EXISTS deployments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    profile     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'provisioning',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deployments_profile_not_blank CHECK (btrim(profile) <> ''),
    CONSTRAINT deployments_status_allowed CHECK (
        status IN ('provisioning', 'active', 'suspended', 'decommissioned')
    )
    -- Jurisdiction assignment is owned by Phase 007; no jurisdiction
    -- column is introduced here. A later migration adds it.
);

CREATE INDEX IF NOT EXISTS idx_deployments_tenant_id ON deployments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments (status);
