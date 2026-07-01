CREATE TABLE IF NOT EXISTS deployment_provisioning_records (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id  UUID NOT NULL REFERENCES deployments (id) ON DELETE CASCADE,
    outcome        TEXT NOT NULL DEFAULT 'started',
    error_detail   TEXT,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deployment_provisioning_records_outcome_allowed CHECK (
        outcome IN ('started', 'succeeded', 'failed')
    ),
    CONSTRAINT deployment_provisioning_records_completed_after_started CHECK (
        completed_at IS NULL OR completed_at >= started_at
    )
);

CREATE INDEX IF NOT EXISTS idx_deployment_provisioning_records_deployment_id
    ON deployment_provisioning_records (deployment_id);
