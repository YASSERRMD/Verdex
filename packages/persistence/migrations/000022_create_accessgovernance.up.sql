CREATE TABLE IF NOT EXISTS access_policies (
    id           UUID NOT NULL PRIMARY KEY,
    tenant_id    UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    rules        JSONB NOT NULL DEFAULT '[]'::jsonb,
    active       BOOLEAN NOT NULL DEFAULT false,
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT access_policies_name_not_blank CHECK (length(trim(name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_access_policies_tenant_id ON access_policies (tenant_id);
CREATE INDEX IF NOT EXISTS idx_access_policies_tenant_active ON access_policies (tenant_id, active);

CREATE TABLE IF NOT EXISTS access_case_grants (
    id               UUID NOT NULL PRIMARY KEY,
    tenant_id        UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    case_id          UUID NOT NULL,
    grantee_user_id  UUID NOT NULL,
    permissions      JSONB NOT NULL DEFAULT '[]'::jsonb,
    deny             BOOLEAN NOT NULL DEFAULT false,
    expires_at       TIMESTAMPTZ NOT NULL,
    granted_by       UUID NOT NULL,
    granted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_access_case_grants_tenant_id ON access_case_grants (tenant_id);
CREATE INDEX IF NOT EXISTS idx_access_case_grants_tenant_case ON access_case_grants (tenant_id, case_id);
CREATE INDEX IF NOT EXISTS idx_access_case_grants_tenant_grantee ON access_case_grants (tenant_id, grantee_user_id);
CREATE INDEX IF NOT EXISTS idx_access_case_grants_tenant_expires ON access_case_grants (tenant_id, expires_at);

CREATE TABLE IF NOT EXISTS access_elevation_grants (
    id               UUID NOT NULL PRIMARY KEY,
    tenant_id        UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    grantee_user_id  UUID NOT NULL,
    action           TEXT NOT NULL,
    case_id          UUID,
    justification    TEXT NOT NULL,
    granted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    requested_by     UUID NOT NULL,
    revoked_at       TIMESTAMPTZ,
    CONSTRAINT access_elevation_grants_action_not_blank CHECK (length(trim(action)) > 0),
    CONSTRAINT access_elevation_grants_justification_not_blank CHECK (length(trim(justification)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_access_elevation_grants_tenant_id ON access_elevation_grants (tenant_id);
CREATE INDEX IF NOT EXISTS idx_access_elevation_grants_tenant_grantee ON access_elevation_grants (tenant_id, grantee_user_id);
CREATE INDEX IF NOT EXISTS idx_access_elevation_grants_tenant_expires ON access_elevation_grants (tenant_id, expires_at);

CREATE TABLE IF NOT EXISTS access_reviews (
    id             UUID NOT NULL PRIMARY KEY,
    tenant_id      UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject_kind   TEXT NOT NULL,
    subject_id     UUID NOT NULL,
    requested_by   UUID NOT NULL,
    due_at         TIMESTAMPTZ NOT NULL,
    decision       TEXT NOT NULL DEFAULT '',
    attested_by    UUID,
    attested_at    TIMESTAMPTZ,
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT access_reviews_subject_kind_allowed CHECK (subject_kind IN ('case_grant', 'elevation')),
    CONSTRAINT access_reviews_decision_allowed CHECK (decision IN ('', 'approve', 'revoke'))
);

CREATE INDEX IF NOT EXISTS idx_access_reviews_tenant_id ON access_reviews (tenant_id);
CREATE INDEX IF NOT EXISTS idx_access_reviews_tenant_due ON access_reviews (tenant_id, due_at);
